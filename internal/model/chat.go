package model

import "time"

const (
	// DefaultMaxTurns is the sliding window size: last 10 turns (5 user + 5 assistant pairs).
	DefaultMaxTurns = 10
	// DefaultTokenBudget is the maximum token count for conversation context history.
	DefaultTokenBudget = 4096
	// AnonymousSessionTTL is the time-to-live for anonymous (unauthenticated) chat sessions.
	AnonymousSessionTTL = 24 * time.Hour
)

// ConversationContext represents the assembled history for a RAG query.
type ConversationContext struct {
	SessionID   string        `json:"session_id"`
	Messages    []ChatMessage `json:"messages"`
	TotalTokens int           `json:"total_tokens"`
	Truncated   bool          `json:"truncated"`
}

// ChatCompletionResponse is the non-streaming response (for future use).
type ChatCompletionResponse struct {
	SessionID string       `json:"session_id"`
	MessageID string       `json:"message_id"`
	Text      string       `json:"text"`
	Sources   []ChatSource `json:"sources"`
}

// HistoryResponse is the paginated message history returned by GET /sessions/:session_id/history.
type HistoryResponse struct {
	SessionID    string        `json:"session_id"`
	Messages     []ChatMessage `json:"messages"`
	TotalCount   int           `json:"total_count"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
}

// SessionListResponse is the list of sessions returned by GET /sessions.
type SessionListResponse struct {
	Sessions []ChatSession `json:"sessions"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
}

// EstimateTokens returns a rough token estimate for a string (approx 4 chars per token for English).
func EstimateTokens(content string) int {
	n := len(content) / 4
	if n == 0 && len(content) > 0 {
		n = 1
	}
	return n
}

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
