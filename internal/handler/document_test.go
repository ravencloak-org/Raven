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

// mockDocumentService implements handler.DocumentServicer for unit tests.
type mockDocumentService struct {
	getByIDFn      func(ctx context.Context, orgID, docID string) (*model.Document, error)
	listFn         func(ctx context.Context, orgID, kbID string, page, pageSize int) (*model.DocumentListResponse, error)
	updateFn       func(ctx context.Context, orgID, docID string, req model.UpdateDocumentRequest) (*model.Document, error)
	deleteFn       func(ctx context.Context, orgID, docID string) error
	updateStatusFn func(ctx context.Context, orgID, docID string, newStatus model.ProcessingStatus, errorMsg string) error
}

func (m *mockDocumentService) GetByID(ctx context.Context, orgID, docID string) (*model.Document, error) {
	return m.getByIDFn(ctx, orgID, docID)
}

func (m *mockDocumentService) List(ctx context.Context, orgID, kbID string, page, pageSize int) (*model.DocumentListResponse, error) {
	return m.listFn(ctx, orgID, kbID, page, pageSize)
}

func (m *mockDocumentService) Update(ctx context.Context, orgID, docID string, req model.UpdateDocumentRequest) (*model.Document, error) {
	return m.updateFn(ctx, orgID, docID, req)
}

func (m *mockDocumentService) Delete(ctx context.Context, orgID, docID string) error {
	return m.deleteFn(ctx, orgID, docID)
}

func (m *mockDocumentService) UpdateStatus(ctx context.Context, orgID, docID string, newStatus model.ProcessingStatus, errorMsg string) error {
	return m.updateStatusFn(ctx, orgID, docID, newStatus, errorMsg)
}

func newDocRouter(svc handler.DocumentServicer) *gin.Engine {
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
	h := handler.NewDocumentHandler(svc)
	const base = "/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents"
	r.GET(base, h.List)
	r.GET(base+"/:doc_id", h.Get)
	r.PUT(base+"/:doc_id", h.Update)
	r.DELETE(base+"/:doc_id", h.Delete)
	return r
}

func TestListDocuments_Success(t *testing.T) {
	svc := &mockDocumentService{
		listFn: func(_ context.Context, orgID, kbID string, page, pageSize int) (*model.DocumentListResponse, error) {
			return &model.DocumentListResponse{
				Documents: []model.Document{{ID: "doc-1", OrgID: orgID, KnowledgeBaseID: kbID, FileName: "test.pdf"}},
				Total:     1,
				Page:      page,
				PageSize:  pageSize,
			}, nil
		},
	}
	r := newDocRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp model.DocumentListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Documents) != 1 {
		t.Errorf("expected 1 document, got %d", len(resp.Documents))
	}
}

func TestListDocuments_EmptyReturnsArray(t *testing.T) {
	svc := &mockDocumentService{
		listFn: func(_ context.Context, _, _ string, page, pageSize int) (*model.DocumentListResponse, error) {
			return &model.DocumentListResponse{
				Documents: []model.Document{},
				Total:     0,
				Page:      page,
				PageSize:  pageSize,
			}, nil
		},
	}
	r := newDocRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetDocument_Success(t *testing.T) {
	svc := &mockDocumentService{
		getByIDFn: func(_ context.Context, orgID, docID string) (*model.Document, error) {
			return &model.Document{ID: docID, OrgID: orgID, FileName: "test.pdf"}, nil
		},
	}
	r := newDocRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/doc-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDocument_NotFound_Returns404(t *testing.T) {
	svc := &mockDocumentService{
		getByIDFn: func(_ context.Context, _, _ string) (*model.Document, error) {
			return nil, apierror.NewNotFound("document not found")
		},
	}
	r := newDocRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateDocument_Success(t *testing.T) {
	title := "Updated Title"
	svc := &mockDocumentService{
		updateFn: func(_ context.Context, orgID, docID string, _ model.UpdateDocumentRequest) (*model.Document, error) {
			return &model.Document{ID: docID, OrgID: orgID, FileName: "test.pdf", Title: title}, nil
		},
	}
	r := newDocRouter(svc)
	body, _ := json.Marshal(map[string]string{"title": title})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/doc-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateDocument_NotFound_Returns404(t *testing.T) {
	svc := &mockDocumentService{
		updateFn: func(_ context.Context, _, _ string, _ model.UpdateDocumentRequest) (*model.Document, error) {
			return nil, apierror.NewNotFound("document not found")
		},
	}
	r := newDocRouter(svc)
	body, _ := json.Marshal(map[string]string{"title": "X"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/bad-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteDocument_Success(t *testing.T) {
	svc := &mockDocumentService{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newDocRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/doc-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestDeleteDocument_NotFound_Returns404(t *testing.T) {
	svc := &mockDocumentService{
		deleteFn: func(_ context.Context, _, _ string) error {
			return apierror.NewNotFound("document not found")
		},
	}
	r := newDocRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListDocuments_WithPagination(t *testing.T) {
	svc := &mockDocumentService{
		listFn: func(_ context.Context, _, _ string, page, pageSize int) (*model.DocumentListResponse, error) {
			if page != 2 || pageSize != 10 {
				t.Errorf("expected page=2 pageSize=10, got page=%d pageSize=%d", page, pageSize)
			}
			return &model.DocumentListResponse{
				Documents: []model.Document{},
				Total:     0,
				Page:      page,
				PageSize:  pageSize,
			}, nil
		},
	}
	r := newDocRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents?page=2&page_size=10", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
