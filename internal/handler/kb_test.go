package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
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
