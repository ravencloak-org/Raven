package handler_test

import (
	"context"
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

// mockSearchService implements handler.SearchServicer for unit tests.
type mockSearchService struct {
	textSearchFn            func(ctx context.Context, orgID, kbID, query string, limit int) (*model.SearchResponse, error)
	textSearchWithFiltersFn func(ctx context.Context, orgID, kbID, query string, docIDs []string, limit int) (*model.SearchResponse, error)
}

func (m *mockSearchService) TextSearch(ctx context.Context, orgID, kbID, query string, limit int) (*model.SearchResponse, error) {
	return m.textSearchFn(ctx, orgID, kbID, query, limit)
}

func (m *mockSearchService) TextSearchWithFilters(ctx context.Context, orgID, kbID, query string, docIDs []string, limit int) (*model.SearchResponse, error) {
	return m.textSearchWithFiltersFn(ctx, orgID, kbID, query, docIDs, limit)
}

func newSearchRouter(svc handler.SearchServicer) *gin.Engine {
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
	h := handler.NewSearchHandler(svc)
	const base = "/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/search"
	r.GET(base, h.Search)
	return r
}

func TestSearch_Success(t *testing.T) {
	heading := "Introduction"
	svc := &mockSearchService{
		textSearchFn: func(_ context.Context, orgID, kbID, query string, limit int) (*model.SearchResponse, error) {
			return &model.SearchResponse{
				Results: []model.ChunkWithRank{
					{
						ID:              "chunk-1",
						OrgID:           orgID,
						KnowledgeBaseID: kbID,
						Content:         "test content about " + query,
						ChunkIndex:      0,
						ChunkType:       "text",
						Heading:         &heading,
						CreatedAt:       time.Now(),
						Rank:            0.5,
						Highlight:       "<b>test</b> content",
					},
				},
				Total: 1,
			}, nil
		},
	}
	r := newSearchRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/search?q=test&limit=5", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearch_MissingQuery_Returns400(t *testing.T) {
	svc := &mockSearchService{}
	r := newSearchRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/search", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearch_EmptyQuery_Returns400(t *testing.T) {
	svc := &mockSearchService{}
	r := newSearchRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/search?q=", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearch_InvalidLimit_Returns400(t *testing.T) {
	svc := &mockSearchService{}
	r := newSearchRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/search?q=test&limit=abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearch_NegativeLimit_Returns400(t *testing.T) {
	svc := &mockSearchService{}
	r := newSearchRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/search?q=test&limit=-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearch_ServiceError_Returns500(t *testing.T) {
	svc := &mockSearchService{
		textSearchFn: func(_ context.Context, _, _, _ string, _ int) (*model.SearchResponse, error) {
			return nil, apierror.NewInternal("database error")
		},
	}
	r := newSearchRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/search?q=test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearch_WithDocIDs_CallsFilteredSearch(t *testing.T) {
	called := false
	svc := &mockSearchService{
		textSearchWithFiltersFn: func(_ context.Context, _, _, _ string, docIDs []string, _ int) (*model.SearchResponse, error) {
			called = true
			if len(docIDs) != 2 {
				t.Errorf("expected 2 doc_ids, got %d", len(docIDs))
			}
			return &model.SearchResponse{Results: []model.ChunkWithRank{}, Total: 0}, nil
		},
	}
	r := newSearchRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/search?q=test&doc_ids=doc-1&doc_ids=doc-2", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected TextSearchWithFilters to be called")
	}
}

func TestSearch_DefaultLimit_Success(t *testing.T) {
	var receivedLimit int
	svc := &mockSearchService{
		textSearchFn: func(_ context.Context, _, _, _ string, limit int) (*model.SearchResponse, error) {
			receivedLimit = limit
			return &model.SearchResponse{Results: []model.ChunkWithRank{}, Total: 0}, nil
		},
	}
	r := newSearchRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/search?q=test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	// The handler passes limit=0 when not specified; the service should handle default.
	if receivedLimit != 0 {
		t.Errorf("expected limit 0 (for service to apply default), got %d", receivedLimit)
	}
}
