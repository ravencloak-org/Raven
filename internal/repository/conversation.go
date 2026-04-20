package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
)

// ErrConversationNotFound is returned when the requested conversation session
// does not exist or is not visible to the current org.
var ErrConversationNotFound = errors.New("conversation session not found")

// ConversationRepository persists and queries cross-channel conversation
// sessions. The underlying conversation_sessions table (introduced by #257)
// is org-scoped and protected by Postgres RLS via db.WithOrgID.
type ConversationRepository struct {
	pool *pgxpool.Pool
}

// NewConversationRepository creates a new ConversationRepository.
func NewConversationRepository(pool *pgxpool.Pool) *ConversationRepository {
	return &ConversationRepository{pool: pool}
}

const conversationCols = `id, org_id, kb_id, user_id, channel,
	COALESCE(messages, '[]'::jsonb) AS messages,
	started_at, ended_at, summary`

func scanConversation(row pgx.Row) (*model.ConversationSession, error) {
	var s model.ConversationSession
	var msgRaw []byte
	var summary *string
	if err := row.Scan(
		&s.ID, &s.OrgID, &s.KBID, &s.UserID, &s.Channel,
		&msgRaw, &s.StartedAt, &s.EndedAt, &summary,
	); err != nil {
		return nil, err
	}
	turns, err := model.UnmarshalMessages(msgRaw)
	if err != nil {
		return nil, fmt.Errorf("decode messages: %w", err)
	}
	s.Messages = turns
	s.Summary = summary
	return &s, nil
}

// CreateSession inserts a new conversation session and returns the stored row.
// messages is the initial turn payload (may be empty).
func (r *ConversationRepository) CreateSession(
	ctx context.Context,
	orgID, kbID, userID, channel string,
	initialMessages []model.ConversationTurn,
) (*model.ConversationSession, error) {
	if orgID == "" || kbID == "" || userID == "" || channel == "" {
		return nil, fmt.Errorf("CreateSession: missing required identifiers")
	}

	msgsJSON, err := model.MarshalMessages(initialMessages)
	if err != nil {
		return nil, fmt.Errorf("CreateSession marshal: %w", err)
	}

	var session *model.ConversationSession
	err = db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`INSERT INTO conversation_sessions
				(org_id, kb_id, user_id, channel, messages)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING `+conversationCols,
			orgID, kbID, userID, channel, msgsJSON,
		)
		var e error
		session, e = scanConversation(row)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("ConversationRepository.CreateSession: %w", err)
	}
	return session, nil
}

// AppendMessage atomically pushes a new ConversationTurn onto the messages
// JSONB array. Returns the updated session. Uses jsonb_build_array +
// || concatenation so the update is a single SQL statement.
func (r *ConversationRepository) AppendMessage(
	ctx context.Context,
	orgID, sessionID string,
	turn model.ConversationTurn,
) (*model.ConversationSession, error) {
	turnJSON, err := json.Marshal(turn)
	if err != nil {
		return nil, fmt.Errorf("AppendMessage marshal: %w", err)
	}

	var session *model.ConversationSession
	err = db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`UPDATE conversation_sessions
			 SET messages = COALESCE(messages, '[]'::jsonb) || jsonb_build_array($3::jsonb)
			 WHERE id = $1 AND org_id = $2
			 RETURNING `+conversationCols,
			sessionID, orgID, string(turnJSON),
		)
		s, e := scanConversation(row)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrConversationNotFound
		}
		if e != nil {
			return e
		}
		session = s
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ConversationRepository.AppendMessage: %w", err)
	}
	return session, nil
}

// EndSession stamps ended_at = NOW() on the session. Idempotent: subsequent
// calls are a no-op.
func (r *ConversationRepository) EndSession(
	ctx context.Context,
	orgID, sessionID string,
) error {
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx,
			`UPDATE conversation_sessions
			 SET ended_at = NOW()
			 WHERE id = $1 AND org_id = $2 AND ended_at IS NULL`,
			sessionID, orgID,
		)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			// Either unknown session or already ended — treat as not found only
			// when the row truly doesn't exist.
			var exists bool
			if err := tx.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM conversation_sessions WHERE id = $1 AND org_id = $2)`,
				sessionID, orgID,
			).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrConversationNotFound
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("ConversationRepository.EndSession: %w", err)
	}
	return nil
}

// GetRecentByUser returns the most recent sessions for a given user on a KB
// ordered by started_at DESC. Limit <= 0 means use a default of 5.
func (r *ConversationRepository) GetRecentByUser(
	ctx context.Context,
	orgID, kbID, userID string,
	limit int,
) ([]model.ConversationSession, error) {
	if limit <= 0 {
		limit = model.MaxConversationHistoryTurns
	}

	var out []model.ConversationSession
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx,
			`SELECT `+conversationCols+`
			 FROM conversation_sessions
			 WHERE org_id = $1 AND kb_id = $2 AND user_id = $3
			 ORDER BY started_at DESC
			 LIMIT $4`,
			orgID, kbID, userID, limit,
		)
		if e != nil {
			return e
		}
		defer rows.Close()

		for rows.Next() {
			s, e2 := scanConversation(rows)
			if e2 != nil {
				return e2
			}
			out = append(out, *s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("ConversationRepository.GetRecentByUser: %w", err)
	}
	return out, nil
}

// GetByID fetches a single session and verifies the user owns it. Returns
// ErrConversationNotFound when the row is missing or owned by another user.
func (r *ConversationRepository) GetByID(
	ctx context.Context,
	orgID, sessionID, userID string,
) (*model.ConversationSession, error) {
	var session *model.ConversationSession
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT `+conversationCols+`
			 FROM conversation_sessions
			 WHERE id = $1 AND org_id = $2 AND user_id = $3`,
			sessionID, orgID, userID,
		)
		s, e := scanConversation(row)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrConversationNotFound
		}
		if e != nil {
			return e
		}
		session = s
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ConversationRepository.GetByID: %w", err)
	}
	return session, nil
}

// ListByUser returns paginated session summaries for a user on a KB.
// Uses jsonb_array_length to compute message count in-SQL.
func (r *ConversationRepository) ListByUser(
	ctx context.Context,
	orgID, kbID, userID string,
	limit, offset int,
) ([]model.ConversationSessionSummary, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var summaries []model.ConversationSessionSummary
	var total int

	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		if e := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM conversation_sessions
			 WHERE org_id = $1 AND kb_id = $2 AND user_id = $3`,
			orgID, kbID, userID,
		).Scan(&total); e != nil {
			return fmt.Errorf("count: %w", e)
		}

		rows, e := tx.Query(ctx,
			`SELECT id, org_id, kb_id, user_id, channel,
			        started_at, ended_at,
			        jsonb_array_length(COALESCE(messages, '[]'::jsonb)) AS message_count,
			        summary
			 FROM conversation_sessions
			 WHERE org_id = $1 AND kb_id = $2 AND user_id = $3
			 ORDER BY started_at DESC
			 LIMIT $4 OFFSET $5`,
			orgID, kbID, userID, limit, offset,
		)
		if e != nil {
			return fmt.Errorf("list query: %w", e)
		}
		defer rows.Close()

		for rows.Next() {
			var s model.ConversationSessionSummary
			var summary *string
			if e2 := rows.Scan(
				&s.ID, &s.OrgID, &s.KBID, &s.UserID, &s.Channel,
				&s.StartedAt, &s.EndedAt, &s.MessageCount, &summary,
			); e2 != nil {
				return fmt.Errorf("scan: %w", e2)
			}
			s.Summary = summary
			summaries = append(summaries, s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, fmt.Errorf("ConversationRepository.ListByUser: %w", err)
	}
	if summaries == nil {
		summaries = []model.ConversationSessionSummary{}
	}
	return summaries, total, nil
}
