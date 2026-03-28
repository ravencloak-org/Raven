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

// mockSourceService implements handler.SourceServicer for unit tests.
type mockSourceService struct {
	createFn func(ctx context.Context, orgID, kbID string, req model.CreateSourceRequest, createdBy string) (*model.Source, error)
	getFn    func(ctx context.Context, orgID, sourceID string) (*model.Source, error)
	listFn   func(ctx context.Context, orgID, kbID string, page, pageSize int) (*model.SourceListResponse, error)
	updateFn func(ctx context.Context, orgID, sourceID string, req model.UpdateSourceRequest) (*model.Source, error)
	deleteFn func(ctx context.Context, orgID, sourceID string) error
}

func (m *mockSourceService) Create(ctx context.Context, orgID, kbID string, req model.CreateSourceRequest, createdBy string) (*model.Source, error) {
	return m.createFn(ctx, orgID, kbID, req, createdBy)
}
func (m *mockSourceService) GetByID(ctx context.Context, orgID, sourceID string) (*model.Source, error) {
	return m.getFn(ctx, orgID, sourceID)
}
func (m *mockSourceService) List(ctx context.Context, orgID, kbID string, page, pageSize int) (*model.SourceListResponse, error) {
	return m.listFn(ctx, orgID, kbID, page, pageSize)
}
func (m *mockSourceService) Update(ctx context.Context, orgID, sourceID string, req model.UpdateSourceRequest) (*model.Source, error) {
	return m.updateFn(ctx, orgID, sourceID, req)
}
func (m *mockSourceService) Delete(ctx context.Context, orgID, sourceID string) error {
	return m.deleteFn(ctx, orgID, sourceID)
}

func newSourceRouter(svc handler.SourceServicer) *gin.Engine {
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
	h := handler.NewSourceHandler(svc)
	const base = "/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/sources"
	r.POST(base, h.Create)
	r.GET(base, h.List)
	r.GET(base+"/:source_id", h.Get)
	r.PUT(base+"/:source_id", h.Update)
	r.DELETE(base+"/:source_id", h.Delete)
	return r
}

func TestCreateSource_Success(t *testing.T) {
	svc := &mockSourceService{
		createFn: func(_ context.Context, orgID, kbID string, req model.CreateSourceRequest, createdBy string) (*model.Source, error) {
			return &model.Source{
				ID:              "src-1",
				OrgID:           orgID,
				KnowledgeBaseID: kbID,
				SourceType:      req.SourceType,
				URL:             req.URL,
				CrawlDepth:      1,
				CrawlFrequency:  model.CrawlFrequencyManual,
				CreatedBy:       createdBy,
			}, nil
		},
	}
	r := newSourceRouter(svc)
	body, _ := json.Marshal(map[string]string{
		"source_type": "web_page",
		"url":         "https://example.com",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSource_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockSourceService{}
	r := newSourceRouter(svc)
	w := httptest.NewRecorder()
	// Missing required fields
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestCreateSource_ServiceError_Returns400(t *testing.T) {
	svc := &mockSourceService{
		createFn: func(_ context.Context, _, _ string, _ model.CreateSourceRequest, _ string) (*model.Source, error) {
			return nil, apierror.NewBadRequest("invalid URL: missing scheme")
		},
	}
	r := newSourceRouter(svc)
	body, _ := json.Marshal(map[string]string{
		"source_type": "web_page",
		"url":         "https://example.com",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSource_Success(t *testing.T) {
	svc := &mockSourceService{
		getFn: func(_ context.Context, orgID, sourceID string) (*model.Source, error) {
			return &model.Source{ID: sourceID, OrgID: orgID, URL: "https://example.com"}, nil
		},
	}
	r := newSourceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources/src-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetSource_NotFound_Returns404(t *testing.T) {
	svc := &mockSourceService{
		getFn: func(_ context.Context, _, _ string) (*model.Source, error) {
			return nil, apierror.NewNotFound("source not found")
		},
	}
	r := newSourceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListSources_Success(t *testing.T) {
	svc := &mockSourceService{
		listFn: func(_ context.Context, _, _ string, page, pageSize int) (*model.SourceListResponse, error) {
			return &model.SourceListResponse{
				Data:     []model.Source{},
				Total:    0,
				Page:     page,
				PageSize: pageSize,
			}, nil
		},
	}
	r := newSourceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp model.SourceListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil empty array")
	}
}

func TestListSources_WithPagination(t *testing.T) {
	svc := &mockSourceService{
		listFn: func(_ context.Context, _, _ string, page, pageSize int) (*model.SourceListResponse, error) {
			return &model.SourceListResponse{
				Data:     []model.Source{},
				Total:    50,
				Page:     page,
				PageSize: pageSize,
			}, nil
		},
	}
	r := newSourceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources?page=2&page_size=10", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp model.SourceListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Page != 2 {
		t.Errorf("expected page 2, got %d", resp.Page)
	}
	if resp.PageSize != 10 {
		t.Errorf("expected page_size 10, got %d", resp.PageSize)
	}
}

func TestUpdateSource_Success(t *testing.T) {
	newURL := "https://updated.example.com"
	svc := &mockSourceService{
		updateFn: func(_ context.Context, orgID, sourceID string, _ model.UpdateSourceRequest) (*model.Source, error) {
			return &model.Source{ID: sourceID, OrgID: orgID, URL: newURL}, nil
		},
	}
	r := newSourceRouter(svc)
	body, _ := json.Marshal(map[string]string{"url": newURL})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources/src-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSource_NotFound_Returns404(t *testing.T) {
	svc := &mockSourceService{
		updateFn: func(_ context.Context, _, _ string, _ model.UpdateSourceRequest) (*model.Source, error) {
			return nil, apierror.NewNotFound("source not found")
		},
	}
	r := newSourceRouter(svc)
	body, _ := json.Marshal(map[string]string{"url": "https://example.com"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources/bad-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteSource_Success(t *testing.T) {
	svc := &mockSourceService{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newSourceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources/src-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestDeleteSource_NotFound_Returns404(t *testing.T) {
	svc := &mockSourceService{
		deleteFn: func(_ context.Context, _, _ string) error {
			return apierror.NewNotFound("source not found")
		},
	}
	r := newSourceRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/sources/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
