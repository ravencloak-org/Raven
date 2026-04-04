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

// ── mocks ────────────────────────────────────────────────────────────────────

type mockStrangerService struct {
	listFn         func(ctx context.Context, orgID string, status *model.StrangerStatus, limit, offset int) ([]model.StrangerUser, int, error)
	getByIDFn      func(ctx context.Context, orgID, id string) (*model.StrangerUser, error)
	blockFn        func(ctx context.Context, orgID, id, blockedBy string, req model.BlockStrangerRequest) error
	unblockFn      func(ctx context.Context, orgID, id string) error
	setRateLimitFn func(ctx context.Context, orgID, id string, rpm *int) error
	deleteFn       func(ctx context.Context, orgID, id string) error
}

func (m *mockStrangerService) List(ctx context.Context, orgID string, status *model.StrangerStatus, limit, offset int) ([]model.StrangerUser, int, error) {
	return m.listFn(ctx, orgID, status, limit, offset)
}
func (m *mockStrangerService) GetByID(ctx context.Context, orgID, id string) (*model.StrangerUser, error) {
	return m.getByIDFn(ctx, orgID, id)
}
func (m *mockStrangerService) Block(ctx context.Context, orgID, id, blockedBy string, req model.BlockStrangerRequest) error {
	return m.blockFn(ctx, orgID, id, blockedBy, req)
}
func (m *mockStrangerService) Unblock(ctx context.Context, orgID, id string) error {
	return m.unblockFn(ctx, orgID, id)
}
func (m *mockStrangerService) SetRateLimit(ctx context.Context, orgID, id string, rpm *int) error {
	return m.setRateLimitFn(ctx, orgID, id, rpm)
}
func (m *mockStrangerService) Delete(ctx context.Context, orgID, id string) error {
	return m.deleteFn(ctx, orgID, id)
}

// mockTierChecker implements handler.OrgTierChecker for tests.
type mockTierChecker struct {
	allowed bool
	err     error
}

func (m *mockTierChecker) IsPerUserControlsAllowed(_ context.Context, _ string) (bool, error) {
	return m.allowed, m.err
}

// ── router helpers ────────────────────────────────────────────────────────────

func newStrangerRouter(svc handler.StrangerServicer, tier handler.OrgTierChecker) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Next()
	})
	var h *handler.StrangerHandler
	if tier != nil {
		h = handler.NewStrangerHandlerWithTier(svc, tier)
	} else {
		h = handler.NewStrangerHandler(svc)
	}
	const base = "/api/v1/orgs/:org_id/strangers"
	r.GET(base, h.List)
	r.GET(base+"/:id", h.Get)
	r.POST(base+"/:id/block", h.Block)
	r.POST(base+"/:id/unblock", h.Unblock)
	r.PUT(base+"/:id/rate-limit", h.SetRateLimit)
	r.DELETE(base+"/:id", h.Delete)
	return r
}

func doStrangerRequest(r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// sampleStranger returns a minimal StrangerUser for test assertions.
func sampleStranger(id string) *model.StrangerUser {
	ip := "127.0.0.1"
	return &model.StrangerUser{
		ID:           id,
		OrgID:        "org-abc",
		SessionID:    "sess-1",
		IPAddress:    &ip,
		Status:       model.StrangerStatusActive,
		MessageCount: 0,
		LastActiveAt: time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// ── List ─────────────────────────────────────────────────────────────────────

func TestStrangerList_Success(t *testing.T) {
	svc := &mockStrangerService{
		listFn: func(_ context.Context, orgID string, _ *model.StrangerStatus, limit, offset int) ([]model.StrangerUser, int, error) {
			if orgID != "org-abc" {
				t.Errorf("expected org-abc, got %s", orgID)
			}
			return []model.StrangerUser{*sampleStranger("s1")}, 1, nil
		},
	}
	r := newStrangerRouter(svc, nil)
	w := doStrangerRequest(r, http.MethodGet, "/api/v1/orgs/org-abc/strangers", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp handler.StrangerListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Total)
	}
	if len(resp.Strangers) != 1 {
		t.Errorf("expected 1 stranger, got %d", len(resp.Strangers))
	}
}

func TestStrangerList_InvalidStatus(t *testing.T) {
	r := newStrangerRouter(&mockStrangerService{}, nil)
	w := doStrangerRequest(r, http.MethodGet, "/api/v1/orgs/org-abc/strangers?status=invalid", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ── Get ──────────────────────────────────────────────────────────────────────

func TestStrangerGet_Success(t *testing.T) {
	svc := &mockStrangerService{
		getByIDFn: func(_ context.Context, orgID, id string) (*model.StrangerUser, error) {
			return sampleStranger(id), nil
		},
	}
	r := newStrangerRouter(svc, nil)
	w := doStrangerRequest(r, http.MethodGet, "/api/v1/orgs/org-abc/strangers/s1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStrangerGet_NotFound(t *testing.T) {
	svc := &mockStrangerService{
		getByIDFn: func(_ context.Context, _, _ string) (*model.StrangerUser, error) {
			return nil, apierror.NewNotFound("stranger not found")
		},
	}
	r := newStrangerRouter(svc, nil)
	w := doStrangerRequest(r, http.MethodGet, "/api/v1/orgs/org-abc/strangers/missing", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ── Block ─────────────────────────────────────────────────────────────────────

func TestStrangerBlock_Success(t *testing.T) {
	svc := &mockStrangerService{
		blockFn: func(_ context.Context, orgID, id, blockedBy string, req model.BlockStrangerRequest) error {
			if orgID != "org-abc" {
				t.Errorf("wrong org: %s", orgID)
			}
			if blockedBy != "user-1" {
				t.Errorf("wrong blockedBy: %s", blockedBy)
			}
			return nil
		},
	}
	tier := &mockTierChecker{allowed: true}
	r := newStrangerRouter(svc, tier)
	body := model.BlockStrangerRequest{
		Status: model.StrangerStatusBlocked,
		Reason: "spamming the chat",
	}
	w := doStrangerRequest(r, http.MethodPost, "/api/v1/orgs/org-abc/strangers/s1/block", body)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStrangerBlock_FreeTierDenied(t *testing.T) {
	svc := &mockStrangerService{
		blockFn: func(_ context.Context, _, _, _ string, _ model.BlockStrangerRequest) error {
			return nil
		},
	}
	tier := &mockTierChecker{allowed: false}
	r := newStrangerRouter(svc, tier)
	body := model.BlockStrangerRequest{
		Status: model.StrangerStatusBlocked,
		Reason: "spamming",
	}
	w := doStrangerRequest(r, http.MethodPost, "/api/v1/orgs/org-abc/strangers/s1/block", body)
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStrangerBlock_InvalidStatus(t *testing.T) {
	tier := &mockTierChecker{allowed: true}
	r := newStrangerRouter(&mockStrangerService{}, tier)
	body := map[string]string{"status": "active", "reason": "some reason"}
	w := doStrangerRequest(r, http.MethodPost, "/api/v1/orgs/org-abc/strangers/s1/block", body)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStrangerBlock_NoTierChecker_Allowed(t *testing.T) {
	// When no tierChecker is wired (e.g. dev mode), Block should succeed.
	svc := &mockStrangerService{
		blockFn: func(_ context.Context, _, _, _ string, _ model.BlockStrangerRequest) error {
			return nil
		},
	}
	r := newStrangerRouter(svc, nil)
	body := model.BlockStrangerRequest{Status: model.StrangerStatusBanned, Reason: "test ban reason"}
	w := doStrangerRequest(r, http.MethodPost, "/api/v1/orgs/org-abc/strangers/s1/block", body)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Unblock ──────────────────────────────────────────────────────────────────

func TestStrangerUnblock_Success(t *testing.T) {
	svc := &mockStrangerService{
		unblockFn: func(_ context.Context, orgID, id string) error {
			if orgID != "org-abc" || id != "s1" {
				t.Errorf("wrong args: orgID=%s id=%s", orgID, id)
			}
			return nil
		},
	}
	r := newStrangerRouter(svc, nil)
	w := doStrangerRequest(r, http.MethodPost, "/api/v1/orgs/org-abc/strangers/s1/unblock", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

// ── SetRateLimit ──────────────────────────────────────────────────────────────

func TestStrangerSetRateLimit_Success(t *testing.T) {
	svc := &mockStrangerService{
		setRateLimitFn: func(_ context.Context, orgID, id string, rpm *int) error {
			if *rpm != 30 {
				t.Errorf("expected rpm=30, got %d", *rpm)
			}
			return nil
		},
	}
	tier := &mockTierChecker{allowed: true}
	r := newStrangerRouter(svc, tier)
	rpm := 30
	body := model.SetRateLimitRequest{RPM: &rpm}
	w := doStrangerRequest(r, http.MethodPut, "/api/v1/orgs/org-abc/strangers/s1/rate-limit", body)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStrangerSetRateLimit_FreeTierDenied(t *testing.T) {
	tier := &mockTierChecker{allowed: false}
	r := newStrangerRouter(&mockStrangerService{}, tier)
	rpm := 10
	body := model.SetRateLimitRequest{RPM: &rpm}
	w := doStrangerRequest(r, http.MethodPut, "/api/v1/orgs/org-abc/strangers/s1/rate-limit", body)
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStrangerSetRateLimit_NegativeRPM(t *testing.T) {
	tier := &mockTierChecker{allowed: true}
	r := newStrangerRouter(&mockStrangerService{}, tier)
	rpm := -1
	body := model.SetRateLimitRequest{RPM: &rpm}
	w := doStrangerRequest(r, http.MethodPut, "/api/v1/orgs/org-abc/strangers/s1/rate-limit", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStrangerSetRateLimit_ClearOverride(t *testing.T) {
	// rpm=null clears the override; should succeed even with tier check.
	svc := &mockStrangerService{
		setRateLimitFn: func(_ context.Context, _, _ string, rpm *int) error {
			if rpm != nil {
				t.Errorf("expected nil rpm, got %v", *rpm)
			}
			return nil
		},
	}
	tier := &mockTierChecker{allowed: true}
	r := newStrangerRouter(svc, tier)
	body := model.SetRateLimitRequest{RPM: nil}
	w := doStrangerRequest(r, http.MethodPut, "/api/v1/orgs/org-abc/strangers/s1/rate-limit", body)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Delete ─────────────────────────────────────────────────────────────────────

func TestStrangerDelete_Success(t *testing.T) {
	svc := &mockStrangerService{
		deleteFn: func(_ context.Context, orgID, id string) error {
			if orgID != "org-abc" || id != "s1" {
				t.Errorf("wrong args: orgID=%s id=%s", orgID, id)
			}
			return nil
		},
	}
	r := newStrangerRouter(svc, nil)
	w := doStrangerRequest(r, http.MethodDelete, "/api/v1/orgs/org-abc/strangers/s1", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStrangerDelete_NotFound(t *testing.T) {
	svc := &mockStrangerService{
		deleteFn: func(_ context.Context, _, _ string) error {
			return apierror.NewNotFound("stranger not found")
		},
	}
	r := newStrangerRouter(svc, nil)
	w := doStrangerRequest(r, http.MethodDelete, "/api/v1/orgs/org-abc/strangers/missing", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
