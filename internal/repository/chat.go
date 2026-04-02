package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// ChatRepository handles database operations for chat sessions and messages.
type ChatRepository struct {
	pool *pgxpool.Pool
}

// NewChatRepository creates a new ChatRepository.
func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{pool: pool}
}

const (
	queryChatSessionInsert = `INSERT INTO chat_sessions (org_id, knowledge_base_id, user_id, session_token, metadata, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, org_id, knowledge_base_id, user_id, session_token, metadata, created_at, expires_at`

	queryChatSessionByID = `SELECT id, org_id, knowledge_base_id, user_id, session_token, metadata, created_at, expires_at
		FROM chat_sessions WHERE id = $1 AND org_id = $2`

	queryChatMessageInsert = `INSERT INTO chat_messages (session_id, org_id, role, content, token_count, chunk_ids, model_name, latency_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, session_id, org_id, role, content, token_count, chunk_ids, model_name, latency_ms, created_at`

	queryChatMessageList = `SELECT id, session_id, org_id, role, content, token_count, chunk_ids, model_name, latency_ms, created_at
		FROM chat_messages WHERE session_id = $1 AND org_id = $2
		ORDER BY created_at ASC LIMIT $3`

	queryChatRecentMessages = `SELECT id, session_id, org_id, role, content, token_count, chunk_ids, model_name, latency_ms, created_at
		FROM chat_messages WHERE session_id = $1 AND org_id = $2
		ORDER BY created_at DESC LIMIT $3`

	queryChatCountMessages = `SELECT COUNT(*) FROM chat_messages WHERE session_id = $1 AND org_id = $2`

	queryChatDeleteExpired = `DELETE FROM chat_sessions WHERE expires_at IS NOT NULL AND expires_at < NOW()`

	queryChatUpdateExpiry = `UPDATE chat_sessions SET expires_at = NOW() + INTERVAL '24 hours' WHERE id = $1 AND org_id = $2`

	queryChatListSessions = `SELECT id, org_id, knowledge_base_id, user_id, session_token, metadata, created_at, expires_at
		FROM chat_sessions WHERE org_id = $1 AND knowledge_base_id = $2
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`

	queryChatDeleteSession = `DELETE FROM chat_sessions WHERE id = $1 AND org_id = $2`

	queryChatMessageListPaged = `SELECT id, session_id, org_id, role, content, token_count, chunk_ids, model_name, latency_ms, created_at
		FROM chat_messages WHERE session_id = $1 AND org_id = $2
		ORDER BY created_at ASC LIMIT $3 OFFSET $4`
)

func scanChatSession(row pgx.Row) (*model.ChatSession, error) {
	var s model.ChatSession
	var metadataJSON []byte
	err := row.Scan(
		&s.ID,
		&s.OrgID,
		&s.KnowledgeBaseID,
		&s.UserID,
		&s.SessionToken,
		&metadataJSON,
		&s.CreatedAt,
		&s.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &s.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal session metadata: %w", err)
		}
	}
	if s.Metadata == nil {
		s.Metadata = map[string]any{}
	}
	return &s, nil
}

// CreateSession inserts a new chat session.
func (r *ChatRepository) CreateSession(ctx context.Context, tx pgx.Tx, session *model.ChatSession) (*model.ChatSession, error) {
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return nil, fmt.Errorf("ChatRepository.CreateSession marshal metadata: %w", err)
	}
	row := tx.QueryRow(ctx, queryChatSessionInsert,
		session.OrgID,
		session.KnowledgeBaseID,
		session.UserID,
		session.SessionToken,
		metadataJSON,
		session.ExpiresAt,
	)
	s, err := scanChatSession(row)
	if err != nil {
		return nil, fmt.Errorf("ChatRepository.CreateSession: %w", err)
	}
	return s, nil
}

// GetSession retrieves a session by ID.
func (r *ChatRepository) GetSession(ctx context.Context, tx pgx.Tx, sessionID, orgID string) (*model.ChatSession, error) {
	row := tx.QueryRow(ctx, queryChatSessionByID, sessionID, orgID)
	s, err := scanChatSession(row)
	if err != nil {
		return nil, fmt.Errorf("ChatRepository.GetSession: %w", err)
	}
	return s, nil
}

// CreateMessage inserts a chat message.
func (r *ChatRepository) CreateMessage(ctx context.Context, tx pgx.Tx, msg *model.ChatMessage) (*model.ChatMessage, error) {
	row := tx.QueryRow(ctx, queryChatMessageInsert,
		msg.SessionID,
		msg.OrgID,
		msg.Role,
		msg.Content,
		msg.TokenCount,
		msg.ChunkIDs,
		msg.ModelName,
		msg.LatencyMs,
	)
	var m model.ChatMessage
	err := row.Scan(
		&m.ID,
		&m.SessionID,
		&m.OrgID,
		&m.Role,
		&m.Content,
		&m.TokenCount,
		&m.ChunkIDs,
		&m.ModelName,
		&m.LatencyMs,
		&m.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("ChatRepository.CreateMessage: %w", err)
	}
	return &m, nil
}

// ListMessages returns messages for a session ordered by created_at.
func (r *ChatRepository) ListMessages(ctx context.Context, tx pgx.Tx, sessionID, orgID string, limit int) ([]model.ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := tx.Query(ctx, queryChatMessageList, sessionID, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("ChatRepository.ListMessages: %w", err)
	}
	defer rows.Close()

	var messages []model.ChatMessage
	for rows.Next() {
		var m model.ChatMessage
		if err := rows.Scan(
			&m.ID,
			&m.SessionID,
			&m.OrgID,
			&m.Role,
			&m.Content,
			&m.TokenCount,
			&m.ChunkIDs,
			&m.ModelName,
			&m.LatencyMs,
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ChatRepository.ListMessages scan: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// GetOrCreateSession retrieves an existing session by ID, or creates a new one.
// For anonymous sessions (no user_id), sets expires_at = now + 24h.
// For authenticated sessions (user_id present), no expiry.
// Returns the session, a boolean indicating if a new session was created, and an error.
func (r *ChatRepository) GetOrCreateSession(ctx context.Context, tx pgx.Tx, orgID, kbID, sessionID string, userID *string) (*model.ChatSession, bool, error) {
	if sessionID != "" {
		row := tx.QueryRow(ctx, queryChatSessionByID, sessionID, orgID)
		s, err := scanChatSession(row)
		if err == nil {
			return s, false, nil
		}
		// If session not found, fall through to create a new one.
		if err.Error() != "no rows in result set" && !isNoRows(err) {
			return nil, false, fmt.Errorf("ChatRepository.GetOrCreateSession lookup: %w", err)
		}
	}

	// Create a new session.
	session := &model.ChatSession{
		OrgID:           orgID,
		KnowledgeBaseID: kbID,
		UserID:          userID,
		SessionToken:    sessionID, // re-use caller's ID as token if provided
		Metadata:        map[string]any{},
	}
	// Anonymous sessions get a 24h TTL.
	if userID == nil {
		expiry := time.Now().Add(model.AnonymousSessionTTL)
		session.ExpiresAt = &expiry
	}

	created, err := r.CreateSession(ctx, tx, session)
	if err != nil {
		return nil, false, fmt.Errorf("ChatRepository.GetOrCreateSession create: %w", err)
	}
	return created, true, nil
}

// isNoRows checks whether an error indicates zero rows returned.
func isNoRows(err error) bool {
	return err != nil && err.Error() == "no rows in result set"
}

// ListRecentMessages returns the last N messages for a session, ordered by created_at ASC.
// The query fetches in DESC order and the result is reversed so the oldest message is first.
func (r *ChatRepository) ListRecentMessages(ctx context.Context, tx pgx.Tx, sessionID, orgID string, limit int) ([]model.ChatMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := tx.Query(ctx, queryChatRecentMessages, sessionID, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("ChatRepository.ListRecentMessages: %w", err)
	}
	defer rows.Close()

	var messages []model.ChatMessage
	for rows.Next() {
		var m model.ChatMessage
		if err := rows.Scan(
			&m.ID,
			&m.SessionID,
			&m.OrgID,
			&m.Role,
			&m.Content,
			&m.TokenCount,
			&m.ChunkIDs,
			&m.ModelName,
			&m.LatencyMs,
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ChatRepository.ListRecentMessages scan: %w", err)
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to get chronological order (oldest first).
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// CountSessionMessages returns total message count for a session.
func (r *ChatRepository) CountSessionMessages(ctx context.Context, tx pgx.Tx, sessionID, orgID string) (int, error) {
	var count int
	err := tx.QueryRow(ctx, queryChatCountMessages, sessionID, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ChatRepository.CountSessionMessages: %w", err)
	}
	return count, nil
}

// DeleteExpiredSessions removes anonymous sessions past their expires_at.
// Returns the number of deleted sessions.
func (r *ChatRepository) DeleteExpiredSessions(ctx context.Context, tx pgx.Tx) (int64, error) {
	tag, err := tx.Exec(ctx, queryChatDeleteExpired)
	if err != nil {
		return 0, fmt.Errorf("ChatRepository.DeleteExpiredSessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// UpdateSessionExpiry extends the expiry of an anonymous session by 24h from now.
func (r *ChatRepository) UpdateSessionExpiry(ctx context.Context, tx pgx.Tx, sessionID, orgID string) error {
	_, err := tx.Exec(ctx, queryChatUpdateExpiry, sessionID, orgID)
	if err != nil {
		return fmt.Errorf("ChatRepository.UpdateSessionExpiry: %w", err)
	}
	return nil
}

// ListSessions returns active sessions for a KB, ordered by created_at DESC.
func (r *ChatRepository) ListSessions(ctx context.Context, tx pgx.Tx, orgID, kbID string, limit, offset int) ([]model.ChatSession, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := tx.Query(ctx, queryChatListSessions, orgID, kbID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ChatRepository.ListSessions: %w", err)
	}
	defer rows.Close()

	var sessions []model.ChatSession
	for rows.Next() {
		var s model.ChatSession
		var metadataJSON []byte
		if err := rows.Scan(
			&s.ID,
			&s.OrgID,
			&s.KnowledgeBaseID,
			&s.UserID,
			&s.SessionToken,
			&metadataJSON,
			&s.CreatedAt,
			&s.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("ChatRepository.ListSessions scan: %w", err)
		}
		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &s.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal session metadata: %w", err)
			}
		}
		if s.Metadata == nil {
			s.Metadata = map[string]any{}
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// DeleteSession removes a session (cascades to messages via FK ON DELETE CASCADE).
func (r *ChatRepository) DeleteSession(ctx context.Context, tx pgx.Tx, sessionID, orgID string) error {
	tag, err := tx.Exec(ctx, queryChatDeleteSession, sessionID, orgID)
	if err != nil {
		return fmt.Errorf("ChatRepository.DeleteSession: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}

// ListMessagesPaged returns messages for a session with pagination.
func (r *ChatRepository) ListMessagesPaged(ctx context.Context, tx pgx.Tx, sessionID, orgID string, limit, offset int) ([]model.ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := tx.Query(ctx, queryChatMessageListPaged, sessionID, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ChatRepository.ListMessagesPaged: %w", err)
	}
	defer rows.Close()

	var messages []model.ChatMessage
	for rows.Next() {
		var m model.ChatMessage
		if err := rows.Scan(
			&m.ID,
			&m.SessionID,
			&m.OrgID,
			&m.Role,
			&m.Content,
			&m.TokenCount,
			&m.ChunkIDs,
			&m.ModelName,
			&m.LatencyMs,
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ChatRepository.ListMessagesPaged scan: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
