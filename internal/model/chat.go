package model

import "time"

// ChatSession represents an active or historical chat session scoped to a
// knowledge base and organisation.
type ChatSession struct {
	ID              string         `json:"id"`
	OrgID           string         `json:"org_id"`
	KnowledgeBaseID string         `json:"knowledge_base_id"`
	UserID          *string        `json:"user_id,omitempty"`
	SessionToken    string         `json:"session_token"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
	ExpiresAt       *time.Time     `json:"expires_at,omitempty"`
}

// ChatMessage represents a single message within a chat session.
type ChatMessage struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	OrgID      string    `json:"org_id"`
	Role       string    `json:"role"` // "user", "assistant", "system"
	Content    string    `json:"content"`
	TokenCount *int      `json:"token_count,omitempty"`
	ChunkIDs   []string  `json:"chunk_ids,omitempty"`
	ModelName  *string   `json:"model_name,omitempty"`
	LatencyMs  *int      `json:"latency_ms,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ChatCompletionRequest is the request body for POST /v1/chat/:kb_id/completions.
type ChatCompletionRequest struct {
	Query    string            `json:"query" binding:"required"`
	SessionID string           `json:"session_id,omitempty"`
	Model    string            `json:"model,omitempty"`
	Provider string            `json:"provider,omitempty"`
	Filters  map[string]string `json:"filters,omitempty"`
	Stream   bool              `json:"stream"`
}

// ChatSource describes a source chunk that contributed to the AI response.
type ChatSource struct {
	DocumentID   string  `json:"document_id"`
	DocumentName string  `json:"document_name"`
	ChunkText    string  `json:"chunk_text"`
	Score        float32 `json:"score"`
}
