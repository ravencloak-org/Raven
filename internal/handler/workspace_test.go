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

// mockWorkspaceService implements handler.WorkspaceServicer for unit tests.
type mockWorkspaceService struct {
	createFn           func(ctx context.Context, orgID string, req model.CreateWorkspaceRequest) (*model.Workspace, error)
	getFn              func(ctx context.Context, orgID, wsID string) (*model.Workspace, error)
	listFn             func(ctx context.Context, orgID string) ([]model.Workspace, error)
	updateFn           func(ctx context.Context, orgID, wsID string, req model.UpdateWorkspaceRequest) (*model.Workspace, error)
	deleteFn           func(ctx context.Context, orgID, wsID string) error
	addMemberFn        func(ctx context.Context, orgID, wsID string, req model.AddWorkspaceMemberRequest) (*model.WorkspaceMember, error)
	updateMemberRoleFn func(ctx context.Context, orgID, wsID string, req model.UpdateWorkspaceMemberRequest, userID string) (*model.WorkspaceMember, error)
	removeMemberFn     func(ctx context.Context, orgID, wsID, userID string) error
}

func (m *mockWorkspaceService) Create(ctx context.Context, orgID string, req model.CreateWorkspaceRequest) (*model.Workspace, error) {
	return m.createFn(ctx, orgID, req)
}
func (m *mockWorkspaceService) GetByOrgAndID(ctx context.Context, orgID, wsID string) (*model.Workspace, error) {
	return m.getFn(ctx, orgID, wsID)
}
func (m *mockWorkspaceService) ListByOrg(ctx context.Context, orgID string) ([]model.Workspace, error) {
	return m.listFn(ctx, orgID)
}
func (m *mockWorkspaceService) Update(ctx context.Context, orgID, wsID string, req model.UpdateWorkspaceRequest) (*model.Workspace, error) {
	return m.updateFn(ctx, orgID, wsID, req)
}
func (m *mockWorkspaceService) Delete(ctx context.Context, orgID, wsID string) error {
	return m.deleteFn(ctx, orgID, wsID)
}
func (m *mockWorkspaceService) AddMember(ctx context.Context, orgID, wsID string, req model.AddWorkspaceMemberRequest) (*model.WorkspaceMember, error) {
	return m.addMemberFn(ctx, orgID, wsID, req)
}
func (m *mockWorkspaceService) UpdateMemberRole(ctx context.Context, orgID, wsID string, req model.UpdateWorkspaceMemberRequest, userID string) (*model.WorkspaceMember, error) {
	return m.updateMemberRoleFn(ctx, orgID, wsID, req, userID)
}
func (m *mockWorkspaceService) RemoveMember(ctx context.Context, orgID, wsID, userID string) error {
	return m.removeMemberFn(ctx, orgID, wsID, userID)
}

func newWSRouter(svc handler.WorkspaceServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "caller-user")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Next()
	})
	h := handler.NewWorkspaceHandler(svc)
	r.POST("/api/v1/orgs/:org_id/workspaces", h.Create)
	r.GET("/api/v1/orgs/:org_id/workspaces", h.List)
	r.GET("/api/v1/orgs/:org_id/workspaces/:ws_id", h.Get)
	r.PUT("/api/v1/orgs/:org_id/workspaces/:ws_id", h.Update)
	r.DELETE("/api/v1/orgs/:org_id/workspaces/:ws_id", h.Delete)
	r.POST("/api/v1/orgs/:org_id/workspaces/:ws_id/members", h.AddMember)
	r.PUT("/api/v1/orgs/:org_id/workspaces/:ws_id/members/:user_id", h.UpdateMember)
	r.DELETE("/api/v1/orgs/:org_id/workspaces/:ws_id/members/:user_id", h.RemoveMember)
	return r
}

func TestCreateWorkspace_Success(t *testing.T) {
	svc := &mockWorkspaceService{
		createFn: func(_ context.Context, orgID string, req model.CreateWorkspaceRequest) (*model.Workspace, error) {
			return &model.Workspace{ID: "ws-1", OrgID: orgID, Name: req.Name, Slug: "eng"}, nil
		},
	}
	r := newWSRouter(svc)
	body, _ := json.Marshal(map[string]string{"name": "Engineering"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWorkspace_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockWorkspaceService{}
	r := newWSRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces", bytes.NewBufferString(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestGetWorkspace_Success(t *testing.T) {
	svc := &mockWorkspaceService{
		getFn: func(_ context.Context, orgID, wsID string) (*model.Workspace, error) {
			return &model.Workspace{ID: wsID, OrgID: orgID, Name: "Eng", Slug: "eng"}, nil
		},
	}
	r := newWSRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetWorkspace_NotFound_Returns404(t *testing.T) {
	svc := &mockWorkspaceService{
		getFn: func(_ context.Context, _, _ string) (*model.Workspace, error) {
			return nil, apierror.NewNotFound("workspace not found")
		},
	}
	r := newWSRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListWorkspaces_ReturnsEmptyArray(t *testing.T) {
	svc := &mockWorkspaceService{
		listFn: func(_ context.Context, _ string) ([]model.Workspace, error) {
			return nil, nil
		},
	}
	r := newWSRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "[]" {
		t.Errorf("expected empty array, got %s", w.Body.String())
	}
}

func TestDeleteWorkspace_Success(t *testing.T) {
	svc := &mockWorkspaceService{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newWSRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/workspaces/ws-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestAddMember_Success(t *testing.T) {
	svc := &mockWorkspaceService{
		addMemberFn: func(_ context.Context, orgID, wsID string, req model.AddWorkspaceMemberRequest) (*model.WorkspaceMember, error) {
			return &model.WorkspaceMember{ID: "mem-1", WorkspaceID: wsID, UserID: req.UserID, Role: req.Role, OrgID: orgID}, nil
		},
	}
	r := newWSRouter(svc)
	body, _ := json.Marshal(map[string]string{"user_id": "user-x", "role": "member"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/members", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRemoveMember_CannotRemoveSelf(t *testing.T) {
	svc := &mockWorkspaceService{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "self-user")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Next()
	})
	h := handler.NewWorkspaceHandler(svc)
	r.DELETE("/api/v1/orgs/:org_id/workspaces/:ws_id/members/:user_id", h.RemoveMember)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/workspaces/ws-1/members/self-user", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when removing self, got %d", w.Code)
	}
}
