package handler_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockChatService implements handler.ChatServicer for unit tests.
type mockChatService struct {
	streamFn        func(ctx context.Context, orgID, kbID string, req *model.ChatCompletionRequest) (<-chan service.SSEEvent, error)
	getHistoryFn    func(ctx context.Context, orgID, sessionID string, limit, offset int) (*model.HistoryResponse, error)
	listSessionsFn  func(ctx context.Context, orgID, kbID string, limit, offset int) (*model.SessionListResponse, error)
	deleteSessionFn func(ctx context.Context, orgID, sessionID string) error
}

func (m *mockChatService) StreamCompletion(ctx context.Context, orgID, kbID string, req *model.ChatCompletionRequest) (<-chan service.SSEEvent, error) {
	return m.streamFn(ctx, orgID, kbID, req)
}

func (m *mockChatService) GetHistory(ctx context.Context, orgID, sessionID string, limit, offset int) (*model.HistoryResponse, error) {
	if m.getHistoryFn != nil {
		return m.getHistoryFn(ctx, orgID, sessionID, limit, offset)
	}
	return nil, nil
}

func (m *mockChatService) ListSessions(ctx context.Context, orgID, kbID string, limit, offset int) (*model.SessionListResponse, error) {
	if m.listSessionsFn != nil {
		return m.listSessionsFn(ctx, orgID, kbID, limit, offset)
	}
	return nil, nil
}

func (m *mockChatService) DeleteSession(ctx context.Context, orgID, sessionID string) error {
	if m.deleteSessionFn != nil {
		return m.deleteSessionFn(ctx, orgID, sessionID)
	}
	return nil
}

func newChatRouter(svc handler.ChatServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Set(string(middleware.ContextKeyAPIKeyID), "key-1")
		c.Set(string(middleware.ContextKeyKBID), "kb-1")
		c.Next()
	})
	h := handler.NewChatHandler(svc)
	r.POST("/api/v1/chat/:kb_id/completions", h.StreamCompletion)
	r.GET("/api/v1/chat/:kb_id/sessions", h.ListSessions)
	r.GET("/api/v1/chat/:kb_id/sessions/:session_id/history", h.GetHistory)
	r.DELETE("/api/v1/chat/:kb_id/sessions/:session_id", h.DeleteSession)
	return r
}

func TestStreamCompletion_SSEFormat(t *testing.T) {
	svc := &mockChatService{
		streamFn: func(_ context.Context, orgID, kbID string, req *model.ChatCompletionRequest) (<-chan service.SSEEvent, error) {
			if orgID != "org-abc" {
				t.Errorf("expected orgID=org-abc, got %s", orgID)
			}
			if kbID != "kb-1" {
				t.Errorf("expected kbID=kb-1, got %s", kbID)
			}
			if req.Query != "hello" {
				t.Errorf("expected query=hello, got %s", req.Query)
			}

			ch := make(chan service.SSEEvent, 3)
			ch <- service.SSEEvent{Event: service.SSEEventToken, Data: map[string]string{"text": "Hello"}}
			ch <- service.SSEEvent{Event: service.SSEEventToken, Data: map[string]string{"text": " world"}}
			ch <- service.SSEEvent{Event: service.SSEEventDone, Data: map[string]string{
				"session_id": "sess-1",
				"message_id": "msg-1",
			}}
			close(ch)
			return ch, nil
		},
	}

	r := newChatRouter(svc)
	body := `{"query":"hello","stream":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/kb-1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify Content-Type header.
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	// Parse SSE events.
	events := parseSSEEvents(t, w.Body.Bytes())

	if len(events) < 3 {
		t.Fatalf("expected at least 3 SSE events, got %d", len(events))
	}

	// Verify first token event.
	if events[0].eventType != "token" {
		t.Errorf("event[0] type = %q, want 'token'", events[0].eventType)
	}
	var tokenData map[string]string
	if err := json.Unmarshal([]byte(events[0].data), &tokenData); err != nil {
		t.Fatalf("failed to parse token event data: %v", err)
	}
	if tokenData["text"] != "Hello" {
		t.Errorf("token text = %q, want 'Hello'", tokenData["text"])
	}

	// Verify done event (last one).
	last := events[len(events)-1]
	if last.eventType != "done" {
		t.Errorf("last event type = %q, want 'done'", last.eventType)
	}
	var doneData map[string]string
	if err := json.Unmarshal([]byte(last.data), &doneData); err != nil {
		t.Fatalf("failed to parse done event data: %v", err)
	}
	if doneData["session_id"] != "sess-1" {
		t.Errorf("done session_id = %q, want 'sess-1'", doneData["session_id"])
	}
	if doneData["message_id"] != "msg-1" {
		t.Errorf("done message_id = %q, want 'msg-1'", doneData["message_id"])
	}
}

func TestStreamCompletion_SourcesEvent(t *testing.T) {
	svc := &mockChatService{
		streamFn: func(_ context.Context, _, _ string, _ *model.ChatCompletionRequest) (<-chan service.SSEEvent, error) {
			ch := make(chan service.SSEEvent, 2)
			ch <- service.SSEEvent{Event: service.SSEEventSources, Data: map[string]any{
				"sources": []model.ChatSource{
					{DocumentID: "doc-1", DocumentName: "test.pdf", ChunkText: "sample", Score: 0.95},
				},
			}}
			ch <- service.SSEEvent{Event: service.SSEEventDone, Data: map[string]string{
				"session_id": "sess-1",
				"message_id": "msg-1",
			}}
			close(ch)
			return ch, nil
		},
	}

	r := newChatRouter(svc)
	body := `{"query":"what is raven?"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/kb-1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	events := parseSSEEvents(t, w.Body.Bytes())
	found := false
	for _, e := range events {
		if e.eventType == "sources" {
			found = true
			// Verify sources data is valid JSON.
			var sourcesData map[string]any
			if err := json.Unmarshal([]byte(e.data), &sourcesData); err != nil {
				t.Fatalf("failed to parse sources event data: %v", err)
			}
			srcs, ok := sourcesData["sources"]
			if !ok {
				t.Fatal("sources event missing 'sources' key")
			}
			srcList, ok := srcs.([]any)
			if !ok {
				t.Fatal("sources is not an array")
			}
			if len(srcList) != 1 {
				t.Errorf("expected 1 source, got %d", len(srcList))
			}
		}
	}
	if !found {
		t.Error("expected a sources event but none found")
	}
}

func TestStreamCompletion_MissingQuery_Returns400(t *testing.T) {
	svc := &mockChatService{}
	r := newChatRouter(svc)
	body := `{"stream":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/kb-1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStreamCompletion_InvalidJSON_Returns400(t *testing.T) {
	svc := &mockChatService{}
	r := newChatRouter(svc)
	body := `not json`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/kb-1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStreamCompletion_ServiceError_Returns500(t *testing.T) {
	svc := &mockChatService{
		streamFn: func(_ context.Context, _, _ string, _ *model.ChatCompletionRequest) (<-chan service.SSEEvent, error) {
			return nil, apierror.NewInternal("ai worker unavailable")
		},
	}

	r := newChatRouter(svc)
	body := `{"query":"test"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/kb-1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStreamCompletion_ErrorEvent(t *testing.T) {
	svc := &mockChatService{
		streamFn: func(_ context.Context, _, _ string, _ *model.ChatCompletionRequest) (<-chan service.SSEEvent, error) {
			ch := make(chan service.SSEEvent, 2)
			ch <- service.SSEEvent{Event: service.SSEEventToken, Data: map[string]string{"text": "partial"}}
			ch <- service.SSEEvent{Event: service.SSEEventError, Data: map[string]string{"error": "stream interrupted"}}
			close(ch)
			return ch, nil
		},
	}

	r := newChatRouter(svc)
	body := `{"query":"test"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/kb-1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (SSE errors are in-stream), got %d", w.Code)
	}

	events := parseSSEEvents(t, w.Body.Bytes())
	foundError := false
	for _, e := range events {
		if e.eventType == "error" {
			foundError = true
			var errData map[string]string
			if err := json.Unmarshal([]byte(e.data), &errData); err != nil {
				t.Fatalf("failed to parse error event: %v", err)
			}
			if errData["error"] != "stream interrupted" {
				t.Errorf("error message = %q, want 'stream interrupted'", errData["error"])
			}
		}
	}
	if !foundError {
		t.Error("expected an error SSE event but none found")
	}
}

func TestStreamCompletion_MissingOrgID_Returns401(t *testing.T) {
	svc := &mockChatService{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	// Intentionally do NOT set ContextKeyOrgID
	h := handler.NewChatHandler(svc)
	r.POST("/api/v1/chat/:kb_id/completions", h.StreamCompletion)

	body := `{"query":"hello"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/kb-1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Conversation History Tests ---

func TestGetHistory_Success(t *testing.T) {
	now := time.Now()
	tc10 := 10
	svc := &mockChatService{
		getHistoryFn: func(_ context.Context, orgID, sessionID string, limit, offset int) (*model.HistoryResponse, error) {
			if orgID != "org-abc" {
				t.Errorf("expected orgID=org-abc, got %s", orgID)
			}
			if sessionID != "sess-123" {
				t.Errorf("expected sessionID=sess-123, got %s", sessionID)
			}
			if limit != 50 {
				t.Errorf("expected limit=50, got %d", limit)
			}
			if offset != 0 {
				t.Errorf("expected offset=0, got %d", offset)
			}
			return &model.HistoryResponse{
				SessionID: sessionID,
				Messages: []model.ChatMessage{
					{ID: "msg-1", SessionID: sessionID, OrgID: orgID, Role: "user", Content: "hello", TokenCount: &tc10, CreatedAt: now},
					{ID: "msg-2", SessionID: sessionID, OrgID: orgID, Role: "assistant", Content: "hi there", TokenCount: &tc10, CreatedAt: now},
				},
				TotalCount: 2,
				Limit:      50,
				Offset:     0,
			}, nil
		},
	}

	r := newChatRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/kb-1/sessions/sess-123/history", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.HistoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.SessionID != "sess-123" {
		t.Errorf("session_id = %q, want 'sess-123'", resp.SessionID)
	}
	if len(resp.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(resp.Messages))
	}
	if resp.TotalCount != 2 {
		t.Errorf("total_count = %d, want 2", resp.TotalCount)
	}
}

func TestGetHistory_NotFound(t *testing.T) {
	svc := &mockChatService{
		getHistoryFn: func(_ context.Context, _, _ string, _, _ int) (*model.HistoryResponse, error) {
			return nil, apierror.NewNotFound("session not found")
		},
	}

	r := newChatRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/kb-1/sessions/nonexistent/history", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetHistory_CustomPagination(t *testing.T) {
	svc := &mockChatService{
		getHistoryFn: func(_ context.Context, _, sessionID string, limit, offset int) (*model.HistoryResponse, error) {
			if limit != 10 {
				t.Errorf("expected limit=10, got %d", limit)
			}
			if offset != 5 {
				t.Errorf("expected offset=5, got %d", offset)
			}
			return &model.HistoryResponse{
				SessionID:  sessionID,
				Messages:   []model.ChatMessage{},
				TotalCount: 20,
				Limit:      limit,
				Offset:     offset,
			}, nil
		},
	}

	r := newChatRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/kb-1/sessions/sess-1/history?limit=10&offset=5", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListSessions_Success(t *testing.T) {
	now := time.Now()
	svc := &mockChatService{
		listSessionsFn: func(_ context.Context, orgID, kbID string, limit, offset int) (*model.SessionListResponse, error) {
			if orgID != "org-abc" {
				t.Errorf("expected orgID=org-abc, got %s", orgID)
			}
			if kbID != "kb-1" {
				t.Errorf("expected kbID=kb-1, got %s", kbID)
			}
			return &model.SessionListResponse{
				Sessions: []model.ChatSession{
					{ID: "sess-1", OrgID: orgID, KnowledgeBaseID: kbID, SessionToken: "tok-1", Metadata: map[string]any{}, CreatedAt: now},
					{ID: "sess-2", OrgID: orgID, KnowledgeBaseID: kbID, SessionToken: "tok-2", Metadata: map[string]any{}, CreatedAt: now},
				},
				Limit:  limit,
				Offset: offset,
			}, nil
		},
	}

	r := newChatRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/kb-1/sessions", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.SessionListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(resp.Sessions))
	}
}

func TestDeleteSession_Success(t *testing.T) {
	deleteCalled := false
	svc := &mockChatService{
		deleteSessionFn: func(_ context.Context, orgID, sessionID string) error {
			deleteCalled = true
			if orgID != "org-abc" {
				t.Errorf("expected orgID=org-abc, got %s", orgID)
			}
			if sessionID != "sess-to-delete" {
				t.Errorf("expected sessionID=sess-to-delete, got %s", sessionID)
			}
			return nil
		},
	}

	r := newChatRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/chat/kb-1/sessions/sess-to-delete", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if !deleteCalled {
		t.Error("expected delete to be called")
	}
}

func TestDeleteSession_NotFound(t *testing.T) {
	svc := &mockChatService{
		deleteSessionFn: func(_ context.Context, _, _ string) error {
			return apierror.NewNotFound("session not found")
		},
	}

	r := newChatRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/chat/kb-1/sessions/nonexistent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListSessions_MissingOrgID_Returns401(t *testing.T) {
	svc := &mockChatService{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	// Intentionally do NOT set ContextKeyOrgID
	h := handler.NewChatHandler(svc)
	r.GET("/api/v1/chat/:kb_id/sessions", h.ListSessions)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/kb-1/sessions", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// sseEvent is a parsed SSE event for test assertions.
type sseEvent struct {
	eventType string
	data      string
}

// parseSSEEvents parses raw SSE byte output into structured events.
func parseSSEEvents(t *testing.T, raw []byte) []sseEvent {
	t.Helper()
	var events []sseEvent
	scanner := bufio.NewScanner(bytes.NewReader(raw))

	var currentEvent sseEvent
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			currentEvent.eventType = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			currentEvent.data = strings.TrimPrefix(line, "data: ")
		case line == "":
			// Empty line marks end of an SSE event.
			if currentEvent.eventType != "" || currentEvent.data != "" {
				events = append(events, currentEvent)
				currentEvent = sseEvent{}
			}
		}
	}
	// Handle last event if not terminated with empty line.
	if currentEvent.eventType != "" || currentEvent.data != "" {
		events = append(events, currentEvent)
	}

	return events
}
