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

// mockWhatsAppBridgeService implements handler.WhatsAppBridgeServicer for unit tests.
type mockWhatsAppBridgeService struct {
	createBridgeFn      func(ctx context.Context, orgID, callID string, req *model.CreateBridgeRequest) (*model.CreateBridgeResponse, error)
	getBridgeFn         func(ctx context.Context, orgID, callID string) (*model.WhatsAppBridge, error)
	teardownBridgeFn    func(ctx context.Context, orgID, callID string) (*model.WhatsAppBridge, error)
	listActiveBridgesFn func(ctx context.Context, orgID string) ([]model.WhatsAppBridge, error)
}

func (m *mockWhatsAppBridgeService) CreateBridge(ctx context.Context, orgID, callID string, req *model.CreateBridgeRequest) (*model.CreateBridgeResponse, error) {
	if m.createBridgeFn != nil {
		return m.createBridgeFn(ctx, orgID, callID, req)
	}
	return nil, nil
}

func (m *mockWhatsAppBridgeService) GetBridge(ctx context.Context, orgID, callID string) (*model.WhatsAppBridge, error) {
	if m.getBridgeFn != nil {
		return m.getBridgeFn(ctx, orgID, callID)
	}
	return nil, nil
}

func (m *mockWhatsAppBridgeService) TeardownBridge(ctx context.Context, orgID, callID string) (*model.WhatsAppBridge, error) {
	if m.teardownBridgeFn != nil {
		return m.teardownBridgeFn(ctx, orgID, callID)
	}
	return nil, nil
}

func (m *mockWhatsAppBridgeService) ListActiveBridges(ctx context.Context, orgID string) ([]model.WhatsAppBridge, error) {
	if m.listActiveBridgesFn != nil {
		return m.listActiveBridgesFn(ctx, orgID)
	}
	return nil, nil
}

func newWhatsAppBridgeRouter(svc handler.WhatsAppBridgeServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-test")
		c.Next()
	})
	h := handler.NewWhatsAppBridgeHandler(svc)
	r.POST("/api/v1/orgs/:org_id/whatsapp/calls/:call_id/bridge", h.CreateBridge)
	r.GET("/api/v1/orgs/:org_id/whatsapp/calls/:call_id/bridge", h.GetBridge)
	r.DELETE("/api/v1/orgs/:org_id/whatsapp/calls/:call_id/bridge", h.TeardownBridge)
	r.GET("/api/v1/orgs/:org_id/whatsapp/bridges", h.ListActiveBridges)
	return r
}

// --- CreateBridge ---

func TestWhatsAppBridgeCreateBridge_Success(t *testing.T) {
	now := time.Now()
	sessionID := "vs-1"
	svc := &mockWhatsAppBridgeService{
		createBridgeFn: func(_ context.Context, orgID, callID string, req *model.CreateBridgeRequest) (*model.CreateBridgeResponse, error) {
			if orgID != "org-test" {
				t.Errorf("expected orgID=org-test, got %s", orgID)
			}
			if callID != "call-abc" {
				t.Errorf("expected callID=call-abc, got %s", callID)
			}
			return &model.CreateBridgeResponse{
				Bridge: model.WhatsAppBridge{
					ID:             "bridge-1",
					OrgID:          orgID,
					CallID:         callID,
					LiveKitRoom:    "wa-org-test-call-abc",
					BridgeState:    model.BridgeStateActive,
					VoiceSessionID: &sessionID,
					SDPOffer:       req.SDPOffer,
					SDPAnswer:      "v=0\r\n",
					CreatedAt:      now,
					UpdatedAt:      now,
				},
				SDPAnswer: "v=0\r\n",
			}, nil
		},
	}

	r := newWhatsAppBridgeRouter(svc)
	body := `{"sdp_offer":"v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\n"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/whatsapp/calls/call-abc/bridge", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.CreateBridgeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Bridge.ID != "bridge-1" {
		t.Errorf("bridge id = %q, want 'bridge-1'", resp.Bridge.ID)
	}
	if resp.Bridge.BridgeState != model.BridgeStateActive {
		t.Errorf("bridge state = %q, want 'active'", resp.Bridge.BridgeState)
	}
	if resp.SDPAnswer == "" {
		t.Error("expected non-empty SDP answer")
	}
}

func TestWhatsAppBridgeCreateBridge_MissingSDP_Returns400(t *testing.T) {
	svc := &mockWhatsAppBridgeService{}
	r := newWhatsAppBridgeRouter(svc)
	body := `{}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/whatsapp/calls/call-abc/bridge", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWhatsAppBridgeCreateBridge_ServiceError_Returns500(t *testing.T) {
	svc := &mockWhatsAppBridgeService{
		createBridgeFn: func(_ context.Context, _, _ string, _ *model.CreateBridgeRequest) (*model.CreateBridgeResponse, error) {
			return nil, apierror.NewInternal("db error")
		},
	}
	r := newWhatsAppBridgeRouter(svc)
	body := `{"sdp_offer":"v=0\r\n"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/whatsapp/calls/call-abc/bridge", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWhatsAppBridgeCreateBridge_MissingOrgID_Returns401(t *testing.T) {
	svc := &mockWhatsAppBridgeService{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewWhatsAppBridgeHandler(svc)
	r.POST("/api/v1/orgs/:org_id/whatsapp/calls/:call_id/bridge", h.CreateBridge)

	body := `{"sdp_offer":"v=0\r\n"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/whatsapp/calls/call-abc/bridge", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetBridge ---

func TestWhatsAppBridgeGetBridge_Success(t *testing.T) {
	now := time.Now()
	svc := &mockWhatsAppBridgeService{
		getBridgeFn: func(_ context.Context, orgID, callID string) (*model.WhatsAppBridge, error) {
			return &model.WhatsAppBridge{
				ID:          "bridge-1",
				OrgID:       orgID,
				CallID:      callID,
				LiveKitRoom: "wa-room-1",
				BridgeState: model.BridgeStateActive,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	r := newWhatsAppBridgeRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/whatsapp/calls/call-abc/bridge", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.BridgeStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Bridge.CallID != "call-abc" {
		t.Errorf("call_id = %q, want 'call-abc'", resp.Bridge.CallID)
	}
}

func TestWhatsAppBridgeGetBridge_NotFound(t *testing.T) {
	svc := &mockWhatsAppBridgeService{
		getBridgeFn: func(_ context.Context, _, _ string) (*model.WhatsAppBridge, error) {
			return nil, apierror.NewNotFound("bridge not found")
		},
	}
	r := newWhatsAppBridgeRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/whatsapp/calls/nonexistent/bridge", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- TeardownBridge ---

func TestWhatsAppBridgeTeardownBridge_Success(t *testing.T) {
	now := time.Now()
	svc := &mockWhatsAppBridgeService{
		teardownBridgeFn: func(_ context.Context, orgID, callID string) (*model.WhatsAppBridge, error) {
			return &model.WhatsAppBridge{
				ID:          "bridge-1",
				OrgID:       orgID,
				CallID:      callID,
				LiveKitRoom: "wa-room-1",
				BridgeState: model.BridgeStateClosed,
				ClosedAt:    &now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	r := newWhatsAppBridgeRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-test/whatsapp/calls/call-abc/bridge", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.BridgeStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Bridge.BridgeState != model.BridgeStateClosed {
		t.Errorf("bridge state = %q, want 'closed'", resp.Bridge.BridgeState)
	}
	if resp.Bridge.ClosedAt == nil {
		t.Error("expected closed_at to be set")
	}
}

func TestWhatsAppBridgeTeardownBridge_NotFound(t *testing.T) {
	svc := &mockWhatsAppBridgeService{
		teardownBridgeFn: func(_ context.Context, _, _ string) (*model.WhatsAppBridge, error) {
			return nil, apierror.NewNotFound("bridge not found")
		},
	}
	r := newWhatsAppBridgeRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-test/whatsapp/calls/nonexistent/bridge", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListActiveBridges ---

func TestWhatsAppBridgeListActiveBridges_Success(t *testing.T) {
	now := time.Now()
	svc := &mockWhatsAppBridgeService{
		listActiveBridgesFn: func(_ context.Context, orgID string) ([]model.WhatsAppBridge, error) {
			return []model.WhatsAppBridge{
				{ID: "bridge-1", OrgID: orgID, CallID: "call-1", LiveKitRoom: "wa-room-1", BridgeState: model.BridgeStateActive, CreatedAt: now, UpdatedAt: now},
				{ID: "bridge-2", OrgID: orgID, CallID: "call-2", LiveKitRoom: "wa-room-2", BridgeState: model.BridgeStateInitializing, CreatedAt: now, UpdatedAt: now},
			}, nil
		},
	}

	r := newWhatsAppBridgeRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/whatsapp/bridges", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []model.WhatsAppBridge
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 bridges, got %d", len(resp))
	}
}

func TestWhatsAppBridgeListActiveBridges_Empty(t *testing.T) {
	svc := &mockWhatsAppBridgeService{
		listActiveBridgesFn: func(_ context.Context, _ string) ([]model.WhatsAppBridge, error) {
			return []model.WhatsAppBridge{}, nil
		},
	}

	r := newWhatsAppBridgeRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/whatsapp/bridges", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []model.WhatsAppBridge
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected 0 bridges, got %d", len(resp))
	}
}
