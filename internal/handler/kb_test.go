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
	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockKBService implements handler.KBServicer for unit tests.
type mockKBService struct {
	createFn          func(ctx context.Context, orgID, wsID string, req model.CreateKBRequest) (*model.KnowledgeBase, error)
	getFn             func(ctx context.Context, orgID, kbID string) (*model.KnowledgeBase, error)
	listFn            func(ctx context.Context, orgID, wsID string) ([]model.KnowledgeBase, error)
	updateFn          func(ctx context.Context, orgID, kbID string, req model.UpdateKBRequest) (*model.KnowledgeBase, error)
	archiveFn         func(ctx context.Context, orgID, kbID string) error
}

func (m *mockKBService) Create(ctx context.Context, orgID, wsID string, req model.CreateKBRequest) (*model.KnowledgeBase, error) {
	return m.createFn(ctx, orgID, wsID, req)
}
func (m *mockKBService) GetByID(ctx context.Context, orgID, kbID string) (*model.KnowledgeBase, error) {
	return m.getFn(ctx, orgID, kbID)
}
func (m *mockKBService) ListByWorkspace(ctx context.Context, orgID, wsID string) ([]model.KnowledgeBase, error) {
	return m.listFn(ctx, orgID, wsID)
}
func (m *mockKBService) Update(ctx context.Context, orgID, kbID string, req model.UpdateKBRequest) (*model.KnowledgeBase, error) {
	return m.updateFn(ctx, orgID, kbID, req)
}
func (m *mockKBService) Archive(ctx context.Context, orgID, kbID string) error {
	return m.archiveFn(ctx, orgID, kbID)
}

func newKBRouter(svc handler.KBServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "admin")
		c.Next()
	})
	h := handler.NewKBHandler(svc)
	const base = "/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases"
	r.POST(base, h.Create)
	r.GET(base, h.List)
	r.GET(base+"/:kb_id", h.Get)
	r.PUT(base+"/:kb_id", h.Update)
	r.DELETE(base+"/:kb_id", h.Archive)
	return r
}

func TestCreateKB_Success(t *testing.T) {
	svc := &mockKBService{
		createFn: func(_ context.Context, orgID, wsID string, req model.CreateKBRequest) (*model.KnowledgeBase, error) {
			return &model.KnowledgeBase{ID: "kb-1", OrgID: orgID, WorkspaceID: wsID, Name: req.Name, Slug: "docs"}, nil
		},
	}
	r := newKBRouter(svc)
	body, _ := json.Marshal(map[string]string{"name": "Docs"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateKB_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockKBService{}
	r := newKBRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases", bytes.NewBufferString(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestGetKB_Success(t *testing.T) {
	svc := &mockKBService{
		getFn: func(_ context.Context, orgID, kbID string) (*model.KnowledgeBase, error) {
			return &model.KnowledgeBase{ID: kbID, OrgID: orgID, Name: "Docs"}, nil
		},
	}
	r := newKBRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetKB_NotFound_Returns404(t *testing.T) {
	svc := &mockKBService{
		getFn: func(_ context.Context, _, _ string) (*model.KnowledgeBase, error) {
			return nil, apierror.NewNotFound("knowledge base not found")
		},
	}
	r := newKBRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListKBs_ReturnsEmptyArray(t *testing.T) {
	svc := &mockKBService{
		listFn: func(_ context.Context, _, _ string) ([]model.KnowledgeBase, error) {
			return nil, nil
		},
	}
	r := newKBRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestArchiveKB_Success(t *testing.T) {
	svc := &mockKBService{
		archiveFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newKBRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

// TestUpdateKB_AcceptsCacheKnobs verifies the KB update endpoint accepts the
// M9 semantic-cache knobs (cache_enabled and cache_similarity_threshold).
// Issue #256.
func TestUpdateKB_AcceptsCacheKnobs(t *testing.T) {
	threshold := float32(0.88)
	enabled := false
	var received model.UpdateKBRequest
	svc := &mockKBService{
		updateFn: func(_ context.Context, orgID, kbID string, req model.UpdateKBRequest) (*model.KnowledgeBase, error) {
			received = req
			return &model.KnowledgeBase{
				ID:                       kbID,
				OrgID:                    orgID,
				Name:                     "KB",
				CacheEnabled:             false,
				CacheSimilarityThreshold: threshold,
			}, nil
		},
	}
	r := newKBRouter(svc)
	body, _ := json.Marshal(map[string]any{
		"cache_enabled":              enabled,
		"cache_similarity_threshold": threshold,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.CacheEnabled == nil || *received.CacheEnabled != false {
		t.Errorf("cache_enabled not propagated to service: %+v", received.CacheEnabled)
	}
	if received.CacheSimilarityThreshold == nil || *received.CacheSimilarityThreshold != threshold {
		t.Errorf("cache_similarity_threshold not propagated: %+v", received.CacheSimilarityThreshold)
	}
}

// ---------------------------------------------------------------------------
// Publish endpoint tests (issue #726).
// ---------------------------------------------------------------------------

// mockPublisher implements handler.KBPublisher for unit tests covering the
// /publish route. The publishFn lets each test inject the exact result
// or *apierror.AppError it wants the handler to render.
type mockPublisher struct {
	publishFn func(ctx context.Context, orgID, kbID, userID, licenseSPDXID string) (*marketplace.PublishResult, error)
	// lastCall records the most recent invocation so assertions can verify
	// the handler propagated path params and the user id from the auth
	// context.
	lastCall struct {
		orgID, kbID, userID, license string
	}
}

func (m *mockPublisher) PublishKB(ctx context.Context, orgID, kbID, userID, licenseSPDXID string) (*marketplace.PublishResult, error) {
	m.lastCall.orgID = orgID
	m.lastCall.kbID = kbID
	m.lastCall.userID = userID
	m.lastCall.license = licenseSPDXID
	if m.publishFn == nil {
		return nil, apierror.NewInternal("mock publishFn not set")
	}
	return m.publishFn(ctx, orgID, kbID, userID, licenseSPDXID)
}

// newKBRouterWithPublisher wires the same harness as newKBRouter and
// additionally registers the publish route with a stub publisher and
// optional userID override. Pass userID="" to simulate the unauthenticated
// case (the handler must still surface 401 — the auth middleware would
// normally have short-circuited, but the defence-in-depth check belongs
// to the handler).
func newKBRouterWithPublisher(svc handler.KBServicer, pub handler.KBPublisher, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set(string(middleware.ContextKeyUserID), userID)
		}
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "admin")
		c.Next()
	})
	h := handler.NewKBHandler(svc).WithPublisher(pub)
	const base = "/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases"
	r.POST(base+"/:kb_id/publish", h.Publish)
	return r
}

func TestPublishKB_Success(t *testing.T) {
	publishedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	pub := &mockPublisher{
		publishFn: func(_ context.Context, _, _, _, license string) (*marketplace.PublishResult, error) {
			return &marketplace.PublishResult{
				Visibility:     model.KBVisibilityPublic,
				PublishedAt:    publishedAt,
				MarketplaceURL: marketplace.URL("acme", "support-docs"),
			}, nil
		},
	}
	r := newKBRouterWithPublisher(&mockKBService{}, pub, "user-1")

	body, _ := json.Marshal(map[string]string{"license_spdx_id": "MIT"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/publish",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if pub.lastCall.orgID != "org-abc" || pub.lastCall.kbID != "kb-1" {
		t.Errorf("path params not propagated: %+v", pub.lastCall)
	}
	if pub.lastCall.userID != "user-1" {
		t.Errorf("expected user id from context, got %q", pub.lastCall.userID)
	}
	if pub.lastCall.license != "MIT" {
		t.Errorf("license not propagated: %q", pub.lastCall.license)
	}

	var resp marketplace.PublishResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Visibility != model.KBVisibilityPublic {
		t.Errorf("visibility: got %q want public", resp.Visibility)
	}
	if resp.MarketplaceURL != "https://raven.ravencloak.org/marketplace/acme/support-docs" {
		t.Errorf("marketplace_url drift: %q", resp.MarketplaceURL)
	}
}

// TestPublishKB_MissingUserID_Returns401 exercises the defence-in-depth
// check inside the handler: a request that somehow reached us without the
// auth middleware setting a user id must be refused with 401 rather than
// writing a NULL into published_by_user_id.
func TestPublishKB_MissingUserID_Returns401(t *testing.T) {
	pub := &mockPublisher{}
	r := newKBRouterWithPublisher(&mockKBService{}, pub, "")

	body, _ := json.Marshal(map[string]string{"license_spdx_id": "MIT"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/publish",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	if pub.lastCall.userID != "" {
		t.Error("publisher must not be called when user context is missing")
	}
}

// TestPublishKB_KBFrozen_Returns409 covers the path where the publish
// service returns the canonical kb_frozen 409 — surfaced when the KB sits
// in read_only_private (Free Plan freeze, ADR-0004). The handler must
// pass it through with the correct status code.
func TestPublishKB_KBFrozen_Returns409(t *testing.T) {
	pub := &mockPublisher{
		publishFn: func(_ context.Context, _, _, _, _ string) (*marketplace.PublishResult, error) {
			return nil, apierror.NewKBFrozen("frozen on Free Plan")
		},
	}
	r := newKBRouterWithPublisher(&mockKBService{}, pub, "user-1")
	body, _ := json.Marshal(map[string]string{"license_spdx_id": "MIT"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/publish",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublishKB_NotFound_Returns404(t *testing.T) {
	pub := &mockPublisher{
		publishFn: func(_ context.Context, _, _, _, _ string) (*marketplace.PublishResult, error) {
			return nil, apierror.NewNotFound("knowledge base not found")
		},
	}
	r := newKBRouterWithPublisher(&mockKBService{}, pub, "user-1")
	body, _ := json.Marshal(map[string]string{"license_spdx_id": "MIT"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/missing/publish",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestPublishKB_BadLicense_Returns422 confirms the handler renders the
// allow-list rejection from the service as 422 (not 400). The license is
// shape-valid but semantically rejected — exactly what 422 is for.
func TestPublishKB_BadLicense_Returns422(t *testing.T) {
	pub := &mockPublisher{
		publishFn: func(_ context.Context, _, _, _, _ string) (*marketplace.PublishResult, error) {
			return nil, apierror.NewUnprocessableEntity("license_spdx_id \"BSD-3-Clause\" is not in the allow-list")
		},
	}
	r := newKBRouterWithPublisher(&mockKBService{}, pub, "user-1")
	body, _ := json.Marshal(map[string]string{"license_spdx_id": "BSD-3-Clause"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/publish",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

// TestPublishKB_EmptyLicensePayload_Returns422 covers the request-binding
// level: the JSON parses cleanly but license_spdx_id is missing /
// empty-string, which the `binding:"required"` tag rejects with 422
// before the service is ever consulted.
func TestPublishKB_EmptyLicensePayload_Returns422(t *testing.T) {
	pub := &mockPublisher{
		publishFn: func(_ context.Context, _, _, _, _ string) (*marketplace.PublishResult, error) {
			t.Fatal("service must not be called when payload is invalid")
			return nil, nil
		},
	}
	r := newKBRouterWithPublisher(&mockKBService{}, pub, "user-1")
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/publish",
		bytes.NewBufferString(`{}`),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

// TestUpdateKB_RejectsOutOfRangeThreshold confirms the validator rejects
// thresholds outside the 0.80 – 0.99 band documented in the spec.
func TestUpdateKB_RejectsOutOfRangeThreshold(t *testing.T) {
	svc := &mockKBService{}
	r := newKBRouter(svc)
	body, _ := json.Marshal(map[string]any{"cache_similarity_threshold": 1.2})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for bad threshold, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Unpublish endpoint tests (issue #727).
// ---------------------------------------------------------------------------

// mockUnpublisher implements handler.KBUnpublisher for unit tests
// covering the /unpublish route. Symmetric with mockPublisher.
type mockUnpublisher struct {
	unpublishFn func(ctx context.Context, orgID, wsID, kbID string) (*marketplace.UnpublishResult, error)
	lastCall    struct {
		orgID, wsID, kbID string
	}
}

func (m *mockUnpublisher) UnpublishKB(ctx context.Context, orgID, wsID, kbID string) (*marketplace.UnpublishResult, error) {
	m.lastCall.orgID = orgID
	m.lastCall.wsID = wsID
	m.lastCall.kbID = kbID
	if m.unpublishFn == nil {
		return nil, apierror.NewInternal("mock unpublishFn not set")
	}
	return m.unpublishFn(ctx, orgID, wsID, kbID)
}

// newKBRouterWithUnpublisher mirrors newKBRouterWithPublisher for the
// /unpublish route. Pass userID="" to simulate the unauthenticated
// path so the defence-in-depth 401 inside the handler is exercised.
func newKBRouterWithUnpublisher(svc handler.KBServicer, unpub handler.KBUnpublisher, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set(string(middleware.ContextKeyUserID), userID)
		}
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "admin")
		c.Next()
	})
	h := handler.NewKBHandler(svc).WithUnpublisher(unpub)
	const base = "/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases"
	r.POST(base+"/:kb_id/unpublish", h.Unpublish)
	return r
}

func TestUnpublishKB_Success(t *testing.T) {
	heldUntil := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	unpub := &mockUnpublisher{
		unpublishFn: func(_ context.Context, _, _, _ string) (*marketplace.UnpublishResult, error) {
			return &marketplace.UnpublishResult{
				Visibility:    model.KBVisibilityPrivate,
				SlugHeldUntil: heldUntil,
			}, nil
		},
	}
	r := newKBRouterWithUnpublisher(&mockKBService{}, unpub, "user-1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/041b0ae6-ae25-404b-8b46-06857a874222/workspaces/83311cb6-f03a-4ebd-a1d8-4a382a7edff7/knowledge-bases/6ceb1402-876b-4998-aaed-72e891e1c56e/unpublish",
		http.NoBody,
	)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if unpub.lastCall.orgID != "041b0ae6-ae25-404b-8b46-06857a874222" ||
		unpub.lastCall.wsID != "83311cb6-f03a-4ebd-a1d8-4a382a7edff7" ||
		unpub.lastCall.kbID != "6ceb1402-876b-4998-aaed-72e891e1c56e" {
		t.Errorf("path params not propagated: %+v", unpub.lastCall)
	}

	var resp marketplace.UnpublishResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Visibility != model.KBVisibilityPrivate {
		t.Errorf("visibility: got %q want private", resp.Visibility)
	}
	if !resp.SlugHeldUntil.Equal(heldUntil) {
		t.Errorf("slug_held_until drift: got %v want %v", resp.SlugHeldUntil, heldUntil)
	}
}

func TestUnpublishKB_MissingUserID_Returns401(t *testing.T) {
	unpub := &mockUnpublisher{}
	r := newKBRouterWithUnpublisher(&mockKBService{}, unpub, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/041b0ae6-ae25-404b-8b46-06857a874222/workspaces/83311cb6-f03a-4ebd-a1d8-4a382a7edff7/knowledge-bases/6ceb1402-876b-4998-aaed-72e891e1c56e/unpublish",
		http.NoBody,
	)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	if unpub.lastCall.kbID != "" {
		t.Error("unpublisher must not be called when user context is missing")
	}
}

func TestUnpublishKB_NotFound_Returns404(t *testing.T) {
	unpub := &mockUnpublisher{
		unpublishFn: func(_ context.Context, _, _, _ string) (*marketplace.UnpublishResult, error) {
			return nil, apierror.NewNotFound("knowledge base not found")
		},
	}
	r := newKBRouterWithUnpublisher(&mockKBService{}, unpub, "user-1")
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/041b0ae6-ae25-404b-8b46-06857a874222/workspaces/83311cb6-f03a-4ebd-a1d8-4a382a7edff7/knowledge-bases/00000000-0000-0000-0000-000000000000/unpublish",
		http.NoBody,
	)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUnpublishKB_KBFrozen_Returns409(t *testing.T) {
	unpub := &mockUnpublisher{
		unpublishFn: func(_ context.Context, _, _, _ string) (*marketplace.UnpublishResult, error) {
			return nil, apierror.NewKBFrozen("frozen on Free Plan")
		},
	}
	r := newKBRouterWithUnpublisher(&mockKBService{}, unpub, "user-1")
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/041b0ae6-ae25-404b-8b46-06857a874222/workspaces/83311cb6-f03a-4ebd-a1d8-4a382a7edff7/knowledge-bases/6ceb1402-876b-4998-aaed-72e891e1c56e/unpublish",
		http.NoBody,
	)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

// TestUnpublishKB_ServiceNotWired_Returns500 confirms the
// defence-in-depth check fires when KBHandler was constructed without
// WithUnpublisher. Production main.go always wires it; the loud 500
// surfaces a missing dependency rather than a confusing 404.
func TestUnpublishKB_ServiceNotWired_Returns500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Next()
	})
	h := handler.NewKBHandler(&mockKBService{}) // no WithUnpublisher
	const base = "/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases"
	r.POST(base+"/:kb_id/unpublish", h.Unpublish)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/orgs/041b0ae6-ae25-404b-8b46-06857a874222/workspaces/83311cb6-f03a-4ebd-a1d8-4a382a7edff7/knowledge-bases/6ceb1402-876b-4998-aaed-72e891e1c56e/unpublish",
		http.NoBody,
	)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when unpublisher is nil, got %d", w.Code)
	}
}
