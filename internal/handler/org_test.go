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

// mockOrgService implements handler.OrgServicer for unit tests.
type mockOrgService struct {
	createFn func(ctx context.Context, req model.CreateOrgRequest) (*model.Organization, error)
	getFn    func(ctx context.Context, orgID string) (*model.Organization, error)
	updateFn func(ctx context.Context, orgID string, req model.UpdateOrgRequest) (*model.Organization, error)
	deleteFn func(ctx context.Context, orgID string) error
}

func (m *mockOrgService) Create(ctx context.Context, req model.CreateOrgRequest) (*model.Organization, error) {
	return m.createFn(ctx, req)
}
func (m *mockOrgService) GetByID(ctx context.Context, orgID string) (*model.Organization, error) {
	return m.getFn(ctx, orgID)
}
func (m *mockOrgService) Update(ctx context.Context, orgID string, req model.UpdateOrgRequest) (*model.Organization, error) {
	return m.updateFn(ctx, orgID, req)
}
func (m *mockOrgService) Delete(ctx context.Context, orgID string) error {
	return m.deleteFn(ctx, orgID)
}

func newOrgRouter(svc handler.OrgServicer, orgAdminSetup bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	if orgAdminSetup {
		r.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUserID), "user-123")
			c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
			c.Set(string(middleware.ContextKeyOrgID), "org-123")
			c.Next()
		})
	}
	h := handler.NewOrgHandler(svc)
	r.POST("/api/v1/orgs", h.Create)
	r.GET("/api/v1/orgs/:org_id", h.Get)
	r.PUT("/api/v1/orgs/:org_id", h.Update)
	r.DELETE("/api/v1/orgs/:org_id", h.Delete)
	return r
}

func TestCreateOrg_Success(t *testing.T) {
	svc := &mockOrgService{
		createFn: func(_ context.Context, req model.CreateOrgRequest) (*model.Organization, error) {
			return &model.Organization{ID: "new-id", Name: req.Name, Slug: "test-org"}, nil
		},
	}
	r := newOrgRouter(svc, true)

	body, _ := json.Marshal(map[string]string{"name": "Test Org"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp model.Organization
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp.Name != "Test Org" {
		t.Errorf("expected name 'Test Org', got %q", resp.Name)
	}
}

func TestCreateOrg_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockOrgService{}
	r := newOrgRouter(svc, false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs", bytes.NewBufferString(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateOrg_MissingName_Returns422(t *testing.T) {
	svc := &mockOrgService{}
	r := newOrgRouter(svc, false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestGetOrg_Success(t *testing.T) {
	svc := &mockOrgService{
		getFn: func(_ context.Context, orgID string) (*model.Organization, error) {
			return &model.Organization{ID: orgID, Name: "Acme", Slug: "acme"}, nil
		},
	}
	r := newOrgRouter(svc, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetOrg_NotFound_Returns404(t *testing.T) {
	svc := &mockOrgService{
		getFn: func(_ context.Context, _ string) (*model.Organization, error) {
			return nil, apierror.NewNotFound("organisation not found")
		},
	}
	r := newOrgRouter(svc, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/bad-id", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteOrg_Success(t *testing.T) {
	svc := &mockOrgService{
		deleteFn: func(_ context.Context, _ string) error { return nil },
	}
	r := newOrgRouter(svc, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}
