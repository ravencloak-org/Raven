package integration_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockSSEChatService provides a deterministic stub for SSE streaming tests.
type mockSSEChatService struct {
	streamFn func(ctx context.Context, orgID, kbID string, req *model.ChatCompletionRequest) (<-chan service.SSEEvent, error)
}

func (m *mockSSEChatService) StreamCompletion(ctx context.Context, orgID, kbID string, req *model.ChatCompletionRequest) (<-chan service.SSEEvent, error) {
	if m.streamFn != nil {
		return m.streamFn(ctx, orgID, kbID, req)
	}
	ch := make(chan service.SSEEvent, 2)
	ch <- service.SSEEvent{Event: service.SSEEventToken, Data: map[string]string{"text": "hello"}}
	ch <- service.SSEEvent{Event: service.SSEEventDone, Data: map[string]string{"session_id": "s1", "message_id": "m1"}}
	close(ch)
	return ch, nil
}

func (m *mockSSEChatService) GetHistory(_ context.Context, _, _ string, _, _ int) (*model.HistoryResponse, error) {
	return &model.HistoryResponse{}, nil
}

func (m *mockSSEChatService) ListSessions(_ context.Context, _, _ string, _, _ int) (*model.SessionListResponse, error) {
	return &model.SessionListResponse{}, nil
}

func (m *mockSSEChatService) DeleteSession(_ context.Context, _, _ string) error {
	return nil
}

// newSSERouter creates a minimal Gin router for SSE streaming tests.
func newSSERouter(svc handler.ChatServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-integration")
		c.Set(string(middleware.ContextKeyAPIKeyID), "key-1")
		c.Set(string(middleware.ContextKeyKBID), "kb-1")
		c.Next()
	})
	h := handler.NewChatHandler(svc)
	r.POST("/api/v1/chat/:kb_id/completions", h.StreamCompletion)
	return r
}

// TestSSEChat_StreamingResponse_TokenAndDone verifies that the streaming endpoint
// returns a valid SSE stream containing both a token event and a done event.
func TestSSEChat_StreamingResponse_TokenAndDone(t *testing.T) {
	svc := &mockSSEChatService{}
	r := newSSERouter(svc)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := `{"query":"hello world","stream":true}`
	req, err := http.NewRequest(http.MethodPost,
		ts.URL+"/api/v1/chat/kb-1/completions",
		strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")

	// Read SSE events using bufio.Scanner as the plan specifies.
	var assembled strings.Builder
	foundToken := false
	foundDone := false

	scanner := bufio.NewScanner(resp.Body)
	deadline := time.Now().Add(5 * time.Second)

	for scanner.Scan() && time.Now().Before(deadline) {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: token") {
			foundToken = true
		}
		if strings.HasPrefix(line, "event: done") {
			foundDone = true
		}
		if strings.HasPrefix(line, "data: ") {
			assembled.WriteString(strings.TrimPrefix(line, "data: "))
		}
	}

	assert.True(t, foundToken, "SSE stream must contain at least one token event")
	assert.True(t, foundDone, "SSE stream must contain a done event")
	assert.NotEmpty(t, assembled.String(), "assembled SSE data must not be empty")
}

// TestSSEChat_InvalidQuery_Returns400 verifies that a missing query field returns 400.
func TestSSEChat_InvalidQuery_Returns400(t *testing.T) {
	svc := &mockSSEChatService{}
	r := newSSERouter(svc)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// Send request with no query field (should fail validation).
	body := `{"stream":true}`
	req, err := http.NewRequest(http.MethodPost,
		ts.URL+"/api/v1/chat/kb-1/completions",
		strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestSSEChat_ServiceError_Returns500 verifies that a service-layer error
// results in HTTP 500 before the SSE stream begins.
func TestSSEChat_ServiceError_Returns500(t *testing.T) {
	svc := &mockSSEChatService{
		streamFn: func(_ context.Context, _, _ string, _ *model.ChatCompletionRequest) (<-chan service.SSEEvent, error) {
			return nil, apierror.NewInternal("ai worker unavailable")
		},
	}
	r := newSSERouter(svc)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := `{"query":"test error"}`
	req, err := http.NewRequest(http.MethodPost,
		ts.URL+"/api/v1/chat/kb-1/completions",
		strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
