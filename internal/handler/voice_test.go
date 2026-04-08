package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockVoiceService implements handler.VoiceServicer for unit tests.
type mockVoiceService struct {
	createSessionFn      func(ctx context.Context, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error)
	getSessionFn         func(ctx context.Context, orgID, sessionID string) (*model.VoiceSession, error)
	updateSessionStateFn func(ctx context.Context, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error)
	listSessionsFn       func(ctx context.Context, orgID string, limit, offset int) (*model.VoiceSessionListResponse, error)
	appendTurnFn         func(ctx context.Context, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error)
	listTurnsFn          func(ctx context.Context, orgID, sessionID string) (*model.VoiceTurnListResponse, error)
	generateTokenFn      func(ctx context.Context, orgID, sessionID, identity string) (*model.VoiceTokenResponse, error)
}

func (m *mockVoiceService) CreateSession(ctx context.Context, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error) {
	if m.createSessionFn != nil {
		return m.createSessionFn(ctx, orgID, req)
	}
	return nil, nil
}

func (m *mockVoiceService) GetSession(ctx context.Context, orgID, sessionID string) (*model.VoiceSession, error) {
	if m.getSessionFn != nil {
		return m.getSessionFn(ctx, orgID, sessionID)
	}
	return nil, nil
}

func (m *mockVoiceService) UpdateSessionState(ctx context.Context, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error) {
	if m.updateSessionStateFn != nil {
		return m.updateSessionStateFn(ctx, orgID, sessionID, state)
	}
	return nil, nil
}

func (m *mockVoiceService) ListSessions(ctx context.Context, orgID string, limit, offset int) (*model.VoiceSessionListResponse, error) {
	if m.listSessionsFn != nil {
		return m.listSessionsFn(ctx, orgID, limit, offset)
	}
	return nil, nil
}

func (m *mockVoiceService) AppendTurn(ctx context.Context, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error) {
	if m.appendTurnFn != nil {
		return m.appendTurnFn(ctx, orgID, sessionID, req)
	}
	return nil, nil
}

func (m *mockVoiceService) ListTurns(ctx context.Context, orgID, sessionID string) (*model.VoiceTurnListResponse, error) {
	if m.listTurnsFn != nil {
		return m.listTurnsFn(ctx, orgID, sessionID)
	}
	return nil, nil
}

func (m *mockVoiceService) GenerateToken(ctx context.Context, orgID, sessionID, identity string) (*model.VoiceTokenResponse, error) {
	if m.generateTokenFn != nil {
		return m.generateTokenFn(ctx, orgID, sessionID, identity)
	}
	panic("unexpected GenerateToken call")
}

func newVoiceRouter(svc handler.VoiceServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-test")
		c.Next()
	})
	h := handler.NewVoiceHandler(svc)
	r.POST("/api/v1/orgs/:org_id/voice-sessions", h.CreateSession)
	r.GET("/api/v1/orgs/:org_id/voice-sessions", h.ListSessions)
	r.GET("/api/v1/orgs/:org_id/voice-sessions/:session_id", h.GetSession)
	r.PATCH("/api/v1/orgs/:org_id/voice-sessions/:session_id", h.UpdateSessionState)
	r.POST("/api/v1/orgs/:org_id/voice-sessions/:session_id/turns", h.AppendTurn)
	r.GET("/api/v1/orgs/:org_id/voice-sessions/:session_id/turns", h.ListTurns)
	return r
}

// --- CreateSession ---

func TestVoiceCreateSession_Success(t *testing.T) {
	now := time.Now()
	svc := &mockVoiceService{
		createSessionFn: func(_ context.Context, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error) {
			if orgID != "org-test" {
				t.Errorf("expected orgID=org-test, got %s", orgID)
			}
			if req.LiveKitRoom != "room-abc" {
				t.Errorf("expected livekit_room=room-abc, got %s", req.LiveKitRoom)
			}
			return &model.VoiceSession{
				ID:          "sess-1",
				OrgID:       orgID,
				LiveKitRoom: req.LiveKitRoom,
				State:       model.VoiceSessionStateCreated,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	r := newVoiceRouter(svc)
	body := `{"livekit_room":"room-abc"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/voice-sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.VoiceSession
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "sess-1" {
		t.Errorf("id = %q, want 'sess-1'", resp.ID)
	}
	if resp.State != model.VoiceSessionStateCreated {
		t.Errorf("state = %q, want 'created'", resp.State)
	}
}

func TestVoiceCreateSession_EmptyBody_Returns201(t *testing.T) {
	// LiveKitRoom is auto-generated by the service, so an empty body is valid.
	svc := &mockVoiceService{}
	r := newVoiceRouter(svc)
	body := `{}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/voice-sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoiceCreateSession_MissingOrgID_Returns401(t *testing.T) {
	svc := &mockVoiceService{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewVoiceHandler(svc)
	r.POST("/api/v1/orgs/:org_id/voice-sessions", h.CreateSession)

	body := `{"livekit_room":"room-abc"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/voice-sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoiceCreateSession_ServiceError_Returns500(t *testing.T) {
	svc := &mockVoiceService{
		createSessionFn: func(_ context.Context, _ string, _ *model.CreateVoiceSessionRequest) (*model.VoiceSession, error) {
			return nil, apierror.NewInternal("db error")
		},
	}
	r := newVoiceRouter(svc)
	body := `{"livekit_room":"room-abc"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/voice-sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetSession ---

func TestVoiceGetSession_Success(t *testing.T) {
	now := time.Now()
	svc := &mockVoiceService{
		getSessionFn: func(_ context.Context, orgID, sessionID string) (*model.VoiceSession, error) {
			if orgID != "org-test" {
				t.Errorf("expected orgID=org-test, got %s", orgID)
			}
			if sessionID != "sess-abc" {
				t.Errorf("expected sessionID=sess-abc, got %s", sessionID)
			}
			return &model.VoiceSession{
				ID:          sessionID,
				OrgID:       orgID,
				LiveKitRoom: "room-1",
				State:       model.VoiceSessionStateActive,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	r := newVoiceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/voice-sessions/sess-abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.VoiceSession
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "sess-abc" {
		t.Errorf("id = %q, want 'sess-abc'", resp.ID)
	}
	if resp.State != model.VoiceSessionStateActive {
		t.Errorf("state = %q, want 'active'", resp.State)
	}
}

func TestVoiceGetSession_NotFound(t *testing.T) {
	svc := &mockVoiceService{
		getSessionFn: func(_ context.Context, _, _ string) (*model.VoiceSession, error) {
			return nil, apierror.NewNotFound("voice session not found")
		},
	}
	r := newVoiceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/voice-sessions/nonexistent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpdateSessionState ---

func TestVoiceUpdateSessionState_ToActive(t *testing.T) {
	now := time.Now()
	svc := &mockVoiceService{
		updateSessionStateFn: func(_ context.Context, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error) {
			if state != model.VoiceSessionStateActive {
				t.Errorf("expected state=active, got %s", state)
			}
			return &model.VoiceSession{
				ID:          sessionID,
				OrgID:       orgID,
				LiveKitRoom: "room-1",
				State:       state,
				StartedAt:   &now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	r := newVoiceRouter(svc)
	body := `{"state":"active"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/orgs/org-test/voice-sessions/sess-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.VoiceSession
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.State != model.VoiceSessionStateActive {
		t.Errorf("state = %q, want 'active'", resp.State)
	}
}

func TestVoiceUpdateSessionState_ToEnded(t *testing.T) {
	now := time.Now()
	dur := 60
	svc := &mockVoiceService{
		updateSessionStateFn: func(_ context.Context, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error) {
			return &model.VoiceSession{
				ID:                  sessionID,
				OrgID:               orgID,
				LiveKitRoom:         "room-1",
				State:               state,
				StartedAt:           &now,
				EndedAt:             &now,
				CallDurationSeconds: &dur,
				CreatedAt:           now,
				UpdatedAt:           now,
			}, nil
		},
	}

	r := newVoiceRouter(svc)
	body := `{"state":"ended"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/orgs/org-test/voice-sessions/sess-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.VoiceSession
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.State != model.VoiceSessionStateEnded {
		t.Errorf("state = %q, want 'ended'", resp.State)
	}
	if resp.CallDurationSeconds == nil {
		t.Error("expected call_duration_seconds to be set")
	} else if *resp.CallDurationSeconds != 60 {
		t.Errorf("call_duration_seconds = %d, want 60", *resp.CallDurationSeconds)
	}
}

func TestVoiceUpdateSessionState_MissingState_Returns400(t *testing.T) {
	svc := &mockVoiceService{}
	r := newVoiceRouter(svc)
	body := `{}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/orgs/org-test/voice-sessions/sess-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoiceUpdateSessionState_NotFound(t *testing.T) {
	svc := &mockVoiceService{
		updateSessionStateFn: func(_ context.Context, _, _ string, _ model.VoiceSessionState) (*model.VoiceSession, error) {
			return nil, apierror.NewNotFound("voice session not found")
		},
	}
	r := newVoiceRouter(svc)
	body := `{"state":"active"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/orgs/org-test/voice-sessions/missing", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListSessions ---

func TestVoiceListSessions_Success(t *testing.T) {
	now := time.Now()
	svc := &mockVoiceService{
		listSessionsFn: func(_ context.Context, orgID string, limit, offset int) (*model.VoiceSessionListResponse, error) {
			if orgID != "org-test" {
				t.Errorf("expected orgID=org-test, got %s", orgID)
			}
			return &model.VoiceSessionListResponse{
				Sessions: []model.VoiceSession{
					{ID: "sess-1", OrgID: orgID, LiveKitRoom: "room-1", State: model.VoiceSessionStateCreated, CreatedAt: now, UpdatedAt: now},
					{ID: "sess-2", OrgID: orgID, LiveKitRoom: "room-2", State: model.VoiceSessionStateEnded, CreatedAt: now, UpdatedAt: now},
				},
				Total:  2,
				Limit:  limit,
				Offset: offset,
			}, nil
		},
	}

	r := newVoiceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/voice-sessions", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.VoiceSessionListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(resp.Sessions))
	}
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Total)
	}
}

func TestVoiceListSessions_CustomPagination(t *testing.T) {
	svc := &mockVoiceService{
		listSessionsFn: func(_ context.Context, _ string, limit, offset int) (*model.VoiceSessionListResponse, error) {
			if limit != 5 {
				t.Errorf("expected limit=5, got %d", limit)
			}
			if offset != 10 {
				t.Errorf("expected offset=10, got %d", offset)
			}
			return &model.VoiceSessionListResponse{
				Sessions: []model.VoiceSession{},
				Total:    0,
				Limit:    limit,
				Offset:   offset,
			}, nil
		},
	}

	r := newVoiceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/voice-sessions?limit=5&offset=10", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- AppendTurn ---

func TestVoiceAppendTurn_Success(t *testing.T) {
	now := time.Now()
	svc := &mockVoiceService{
		appendTurnFn: func(_ context.Context, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error) {
			if orgID != "org-test" {
				t.Errorf("expected orgID=org-test, got %s", orgID)
			}
			if sessionID != "sess-1" {
				t.Errorf("expected sessionID=sess-1, got %s", sessionID)
			}
			if req.Speaker != model.VoiceSpeakerUser {
				t.Errorf("expected speaker=user, got %s", req.Speaker)
			}
			return &model.VoiceTurn{
				ID:         "turn-1",
				SessionID:  sessionID,
				OrgID:      orgID,
				Speaker:    req.Speaker,
				Transcript: req.Transcript,
				StartedAt:  req.StartedAt,
				CreatedAt:  now,
			}, nil
		},
	}

	r := newVoiceRouter(svc)
	body := `{"speaker":"user","transcript":"Hello there","started_at":"2026-04-04T10:00:00Z"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/voice-sessions/sess-1/turns", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.VoiceTurn
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.ID != "turn-1" {
		t.Errorf("id = %q, want 'turn-1'", resp.ID)
	}
	if resp.Speaker != model.VoiceSpeakerUser {
		t.Errorf("speaker = %q, want 'user'", resp.Speaker)
	}
}

func TestVoiceAppendTurn_MissingFields_Returns400(t *testing.T) {
	svc := &mockVoiceService{}
	r := newVoiceRouter(svc)
	body := `{"speaker":"agent"}` // missing transcript and started_at
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/voice-sessions/sess-1/turns", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoiceAppendTurn_SessionNotFound_Returns404(t *testing.T) {
	svc := &mockVoiceService{
		appendTurnFn: func(_ context.Context, _, _ string, _ *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error) {
			return nil, apierror.NewNotFound("voice session not found")
		},
	}
	r := newVoiceRouter(svc)
	body := `{"speaker":"agent","transcript":"Hi","started_at":"2026-04-04T10:00:00Z"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/voice-sessions/nonexistent/turns", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListTurns ---

func TestVoiceListTurns_Success(t *testing.T) {
	now := time.Now()
	svc := &mockVoiceService{
		listTurnsFn: func(_ context.Context, orgID, sessionID string) (*model.VoiceTurnListResponse, error) {
			if orgID != "org-test" {
				t.Errorf("expected orgID=org-test, got %s", orgID)
			}
			if sessionID != "sess-1" {
				t.Errorf("expected sessionID=sess-1, got %s", sessionID)
			}
			return &model.VoiceTurnListResponse{
				SessionID: sessionID,
				Turns: []model.VoiceTurn{
					{ID: "turn-1", SessionID: sessionID, OrgID: orgID, Speaker: model.VoiceSpeakerAgent, Transcript: "Hello", StartedAt: now, CreatedAt: now},
					{ID: "turn-2", SessionID: sessionID, OrgID: orgID, Speaker: model.VoiceSpeakerUser, Transcript: "Hi agent", StartedAt: now, CreatedAt: now},
				},
			}, nil
		},
	}

	r := newVoiceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/voice-sessions/sess-1/turns", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.VoiceTurnListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.SessionID != "sess-1" {
		t.Errorf("session_id = %q, want 'sess-1'", resp.SessionID)
	}
	if len(resp.Turns) != 2 {
		t.Errorf("expected 2 turns, got %d", len(resp.Turns))
	}
	if resp.Turns[0].Speaker != model.VoiceSpeakerAgent {
		t.Errorf("first turn speaker = %q, want 'agent'", resp.Turns[0].Speaker)
	}
}

func TestVoiceListTurns_SessionNotFound_Returns404(t *testing.T) {
	svc := &mockVoiceService{
		listTurnsFn: func(_ context.Context, _, _ string) (*model.VoiceTurnListResponse, error) {
			return nil, apierror.NewNotFound("voice session not found")
		},
	}
	r := newVoiceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/voice-sessions/nonexistent/turns", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoiceListTurns_MissingOrgID_Returns401(t *testing.T) {
	svc := &mockVoiceService{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewVoiceHandler(svc)
	r.GET("/api/v1/orgs/:org_id/voice-sessions/:session_id/turns", h.ListTurns)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/voice-sessions/sess-1/turns", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}
