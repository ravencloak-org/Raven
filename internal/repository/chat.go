package repository

import (
	"context"
	"encoding/json"
	"fmt"

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
	queryChatSessionInsert = `INSERT INTO chat_sessions (org_id, knowledge_base_id, user_id, session_token, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, org_id, knowledge_base_id, user_id, session_token, metadata, created_at, expires_at`

	queryChatSessionByID = `SELECT id, org_id, knowledge_base_id, user_id, session_token, metadata, created_at, expires_at
		FROM chat_sessions WHERE id = $1 AND org_id = $2`

	queryChatMessageInsert = `INSERT INTO chat_messages (session_id, org_id, role, content, token_count, chunk_ids, model_name, latency_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, session_id, org_id, role, content, token_count, chunk_ids, model_name, latency_ms, created_at`

	queryChatMessageList = `SELECT id, session_id, org_id, role, content, token_count, chunk_ids, model_name, latency_ms, created_at
		FROM chat_messages WHERE session_id = $1 AND org_id = $2
		ORDER BY created_at ASC LIMIT $3`
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
