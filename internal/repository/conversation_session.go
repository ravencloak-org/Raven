package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
)

// ConversationSessionRepository provides CRUD access to conversation_sessions
// (migration 00037). All mutating calls set app.current_org_id for RLS.
type ConversationSessionRepository struct {
	pool *pgxpool.Pool
}

// NewConversationSessionRepository returns a new repository.
func NewConversationSessionRepository(pool *pgxpool.Pool) *ConversationSessionRepository {
	return &ConversationSessionRepository{pool: pool}
}

const sqlInsertConversationSession = `
INSERT INTO conversation_sessions (org_id, kb_id, user_id, channel, messages, started_at, ended_at, summary)
VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8)
RETURNING id, org_id, kb_id, user_id, channel, messages, started_at, ended_at, summary
`

const sqlGetConversationSessionByID = `
SELECT id, org_id, kb_id, user_id, channel, messages, started_at, ended_at, summary
FROM conversation_sessions
WHERE id = $1
`

const sqlUpdateConversationSummary = `
UPDATE conversation_sessions
SET summary = $2, ended_at = COALESCE(ended_at, NOW())
WHERE id = $1
`

// Create inserts a new conversation session row under the given org's RLS scope.
func (r *ConversationSessionRepository) Create(ctx context.Context, s *model.ConversationSession) error {
	msgs, err := s.MessagesJSON()
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}
	return db.WithOrgID(ctx, r.pool, s.OrgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, sqlInsertConversationSession,
			s.OrgID, s.KBID, s.UserID, string(s.Channel), string(msgs),
			s.StartedAt, s.EndedAt, s.Summary,
		)
		var raw []byte
		return scanConversationSession(row, s, &raw)
	})
}

// GetByID loads a conversation session by id under the given org's RLS scope.
func (r *ConversationSessionRepository) GetByID(ctx context.Context, orgID, id string) (*model.ConversationSession, error) {
	var s model.ConversationSession
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, sqlGetConversationSessionByID, id)
		var raw []byte
		return scanConversationSession(row, &s, &raw)
	})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// SetSummary records the generated summary and marks the session ended if not already.
func (r *ConversationSessionRepository) SetSummary(ctx context.Context, orgID, id, summary string) error {
	return db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, sqlUpdateConversationSummary, id, summary)
		return err
	})
}

// scanConversationSession scans a row into s. raw is reused as a scratch buffer
// for the JSONB messages column so callers don't allocate on every call.
func scanConversationSession(row pgx.Row, s *model.ConversationSession, raw *[]byte) error {
	var channel string
	var summary *string
	if err := row.Scan(&s.ID, &s.OrgID, &s.KBID, &s.UserID, &channel, raw,
		&s.StartedAt, &s.EndedAt, &summary); err != nil {
		return fmt.Errorf("scan conversation_session: %w", err)
	}
	s.Channel = model.ConversationChannel(channel)
	s.Summary = summary
	if len(*raw) > 0 {
		if err := json.Unmarshal(*raw, &s.Messages); err != nil {
			return fmt.Errorf("unmarshal messages: %w", err)
		}
	}
	return nil
}
