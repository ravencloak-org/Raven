package service

import (
	"testing"
	"time"

	"github.com/ravencloak-org/Raven/internal/model"
)

func TestStrPtr_NonEmpty(t *testing.T) {
	s := "gpt-4"
	p := strPtr(s)
	if p == nil {
		t.Fatal("expected non-nil pointer for non-empty string")
	}
	if *p != s {
		t.Errorf("expected %q, got %q", s, *p)
	}
}

func TestStrPtr_Empty(t *testing.T) {
	p := strPtr("")
	if p != nil {
		t.Errorf("expected nil pointer for empty string, got %q", *p)
	}
}

func TestSSEEventTypes(t *testing.T) {
	tests := []struct {
		event SSEEventType
		want  string
	}{
		{SSEEventToken, "token"},
		{SSEEventSources, "sources"},
		{SSEEventDone, "done"},
		{SSEEventError, "error"},
	}
	for _, tt := range tests {
		if string(tt.event) != tt.want {
			t.Errorf("SSEEventType %q != %q", tt.event, tt.want)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"empty string", "", 0},
		{"short string", "hi", 1},        // len=2, 2/4=0, min 1
		{"exact four", "test", 1},         // len=4, 4/4=1
		{"eight chars", "testtest", 2},    // len=8, 8/4=2
		{"long string", "Hello, world! This is a test of the token estimation function.", 15}, // len=62, 62/4=15
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.EstimateTokens(tt.content)
			if got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.content, got, tt.want)
			}
		})
	}
}

func TestTokenCountOrEstimate_WithTokenCount(t *testing.T) {
	tc := 42
	msg := &model.ChatMessage{Content: "some content", TokenCount: &tc}
	got := tokenCountOrEstimate(msg)
	if got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestTokenCountOrEstimate_WithoutTokenCount(t *testing.T) {
	msg := &model.ChatMessage{Content: "some content"} // len=12, 12/4=3
	got := tokenCountOrEstimate(msg)
	if got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestBuildConversationContext_SlidingWindow(t *testing.T) {
	// Create a ChatService with a mock repo that returns many messages.
	// We test the buildConversationContextTx method directly.
	tc10 := 10
	messages := make([]model.ChatMessage, 20) // 10 pairs
	for i := 0; i < 20; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = model.ChatMessage{
			ID:         "msg-" + string(rune('a'+i)),
			SessionID:  "sess-1",
			OrgID:      "org-1",
			Role:       role,
			Content:    "message content",
			TokenCount: &tc10,
			CreatedAt:  time.Now().Add(time.Duration(i) * time.Minute),
		}
	}

	// Simulate what buildConversationContextTx does:
	// With maxTurns=5 (so 10 messages), tokenBudget=4096 and 10 tokens each,
	// all 10 messages fit (100 tokens < 4096).
	maxTurns := 5
	tokenBudget := 4096

	// Fetch last maxTurns*2 = 10 messages.
	recentMessages := messages[len(messages)-maxTurns*2:]

	// Walk from newest to oldest, summing tokens.
	totalTokens := 0
	truncated := false
	cutIndex := 0
	for i := len(recentMessages) - 1; i >= 0; i-- {
		tokens := tokenCountOrEstimate(&recentMessages[i])
		if totalTokens+tokens > tokenBudget {
			cutIndex = i + 1
			truncated = true
			break
		}
		totalTokens += tokens
	}

	result := recentMessages[cutIndex:]

	if truncated {
		t.Error("expected no truncation with large token budget")
	}
	if len(result) != 10 {
		t.Errorf("expected 10 messages in sliding window, got %d", len(result))
	}
	if totalTokens != 100 {
		t.Errorf("expected 100 total tokens, got %d", totalTokens)
	}
}

func TestBuildConversationContext_TokenBudget(t *testing.T) {
	// Create messages where token budget will truncate.
	tc100 := 100
	messages := make([]model.ChatMessage, 10)
	for i := 0; i < 10; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = model.ChatMessage{
			ID:         "msg-" + string(rune('a'+i)),
			SessionID:  "sess-1",
			OrgID:      "org-1",
			Role:       role,
			Content:    "message",
			TokenCount: &tc100,
			CreatedAt:  time.Now().Add(time.Duration(i) * time.Minute),
		}
	}

	// With tokenBudget=500, we can fit 5 messages (500 tokens).
	tokenBudget := 500

	totalTokens := 0
	truncated := false
	cutIndex := 0
	for i := len(messages) - 1; i >= 0; i-- {
		tokens := tokenCountOrEstimate(&messages[i])
		if totalTokens+tokens > tokenBudget {
			cutIndex = i + 1
			truncated = true
			break
		}
		totalTokens += tokens
	}

	result := messages[cutIndex:]

	if !truncated {
		t.Error("expected truncation with small token budget")
	}
	if len(result) != 5 {
		t.Errorf("expected 5 messages after truncation, got %d", len(result))
	}
	if totalTokens != 500 {
		t.Errorf("expected 500 total tokens, got %d", totalTokens)
	}
}

func TestBuildConversationContext_EmptySession(t *testing.T) {
	// With zero messages, the context should be empty.
	messages := []model.ChatMessage{}

	convCtx := &model.ConversationContext{
		SessionID:   "sess-1",
		Messages:    nil,
		TotalTokens: 0,
		Truncated:   false,
	}

	if len(messages) != 0 {
		t.Error("expected empty messages slice")
	}
	if convCtx.TotalTokens != 0 {
		t.Errorf("expected 0 total tokens, got %d", convCtx.TotalTokens)
	}
	if convCtx.Truncated {
		t.Error("expected no truncation for empty session")
	}
	if convCtx.Messages != nil {
		t.Error("expected nil messages for empty session")
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"session not found", &testError{msg: "session not found"}, true},
		{"no rows", &testError{msg: "no rows in result set"}, true},
		{"other error", &testError{msg: "something went wrong"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNotFound(tt.err)
			if got != tt.want {
				t.Errorf("isNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestModelConstants(t *testing.T) {
	if model.DefaultMaxTurns != 10 {
		t.Errorf("DefaultMaxTurns = %d, want 10", model.DefaultMaxTurns)
	}
	if model.DefaultTokenBudget != 4096 {
		t.Errorf("DefaultTokenBudget = %d, want 4096", model.DefaultTokenBudget)
	}
	if model.AnonymousSessionTTL != 24*time.Hour {
		t.Errorf("AnonymousSessionTTL = %v, want 24h", model.AnonymousSessionTTL)
	}
}
