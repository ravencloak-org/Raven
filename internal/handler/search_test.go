package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	hybridSearchFn          func(ctx context.Context, orgID, kbID, query string, embedding []float32, topK int) (*model.HybridSearchResponse, error)
	defaultLimit            int
	maxLimit                int
}

func (m *mockSearchService) TextSearch(ctx context.Context, orgID, kbID, query string, limit int) (*model.SearchResponse, error) {
	return m.textSearchFn(ctx, orgID, kbID, query, limit)
}

func (m *mockSearchService) TextSearchWithFilters(ctx context.Context, orgID, kbID, query string, docIDs []string, limit int) (*model.SearchResponse, error) {
	return m.textSearchWithFiltersFn(ctx, orgID, kbID, query, docIDs, limit)
}

func (m *mockSearchService) HybridSearch(ctx context.Context, orgID, kbID, query string, embedding []float32, topK int) (*model.HybridSearchResponse, error) {
	if m.hybridSearchFn == nil {
		return &model.HybridSearchResponse{Results: []model.HybridSearchResult{}, Total: 0}, nil
	}
	return m.hybridSearchFn(ctx, orgID, kbID, query, embedding, topK)
}

func (m *mockSearchService) RetrievalLimits() (int, int) {
	def := m.defaultLimit
	if def == 0 {
		def = 10
	}
	maxL := m.maxLimit
	if maxL == 0 {
		maxL = 100
	}
	return def, maxL
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
	r.POST("/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/hybrid-search", h.HybridSearch)
	return r
}

// newSearchRouterWithRBAC simulates the production RBAC middleware chain: if
// the caller's org-id (set by upstream auth middleware) does not match the
// :org_id URL parameter, the request is rejected with 403 before reaching
// the handler. This mirrors what RequireOrgRole + ResolveWorkspaceRole do
// when the requested org is not one the caller belongs to.
func newSearchRouterWithRBAC(svc handler.SearchServicer, callerOrgID, callerWSRole string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Set(string(middleware.ContextKeyOrgID), callerOrgID)
		c.Set(string(middleware.ContextKeyWorkspaceRole), callerWSRole)
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		pathOrg := c.Param("org_id")
		if pathOrg != "" && pathOrg != callerOrgID {
			_ = c.Error(&apierror.AppError{
				Code:    http.StatusForbidden,
				Message: "Forbidden",
				Detail:  "caller is not a member of this organisation",
			})
			c.Abort()
			return
		}
		c.Next()
	})
	h := handler.NewSearchHandler(svc)
	r.POST("/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/hybrid-search", h.HybridSearch)
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

// ---------------------------------------------------------------------------
// Hybrid search (POST .../hybrid-search) tests
// ---------------------------------------------------------------------------

const hybridSearchPath = "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/hybrid-search"

func TestHybridSearch_Success(t *testing.T) {
	docID := "doc-1"
	heading := "Returns policy"
	svc := &mockSearchService{
		hybridSearchFn: func(_ context.Context, orgID, kbID, query string, embedding []float32, topK int) (*model.HybridSearchResponse, error) {
			if orgID != "org-abc" || kbID != "kb-1" {
				t.Errorf("unexpected scope: org=%s kb=%s", orgID, kbID)
			}
			if query != "refund window" {
				t.Errorf("expected sanitized query 'refund window', got %q", query)
			}
			if len(embedding) != 3 {
				t.Errorf("expected embedding len 3, got %d", len(embedding))
			}
			if topK != 5 {
				t.Errorf("expected topK 5, got %d", topK)
			}
			return &model.HybridSearchResponse{
				Results: []model.HybridSearchResult{
					{
						ChunkID:         "chunk-1",
						OrgID:           orgID,
						KnowledgeBaseID: kbID,
						DocumentID:      &docID,
						Content:         "Refunds within 14 days.",
						ChunkIndex:      0,
						Heading:         &heading,
						ChunkType:       "text",
						CreatedAt:       time.Now(),
						VectorScore:     0.85,
						BM25Score:       12.3,
						RRFScore:        0.0312,
						VectorRank:      1,
						BM25Rank:        3,
					},
				},
				Total: 1,
			}, nil
		},
	}
	r := newSearchRouter(svc)
	body := `{"query":"refund window","top_k":5,"embedding":[0.1,-0.2,0.3]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, hybridSearchPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp model.HybridSearchHTTPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v (body=%s)", err, w.Body.String())
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	got := resp.Results[0]
	if got.VectorScore == 0 || got.BM25Score == 0 || got.RRFScore == 0 {
		t.Errorf("score fields missing: %+v", got)
	}
	if got.VectorRank != 1 || got.BM25Rank != 3 {
		t.Errorf("rank fields missing: %+v", got)
	}
	if resp.Query != "refund window" {
		t.Errorf("expected echo query 'refund window', got %q", resp.Query)
	}
	if resp.TopK != 5 {
		t.Errorf("expected echo top_k 5, got %d", resp.TopK)
	}
}

func TestHybridSearch_EmptyQuery_400(t *testing.T) {
	svc := &mockSearchService{
		hybridSearchFn: func(context.Context, string, string, string, []float32, int) (*model.HybridSearchResponse, error) {
			t.Fatal("service should not be called when query is empty")
			return nil, nil
		},
	}
	r := newSearchRouter(svc)

	cases := map[string]string{
		"missing query": `{"top_k":5}`,
		"empty query":   `{"query":"","top_k":5}`,
		"blank query":   `{"query":"   ","top_k":5}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, hybridSearchPath, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHybridSearch_TopKClampedToMax(t *testing.T) {
	// Simulate the service's internal clamp by returning maxLimit results
	// when called with an over-cap topK. The handler should echo back the
	// clamped top_k via RetrievalLimits (50 here).
	const maxLimit = 50
	svc := &mockSearchService{
		defaultLimit: 10,
		maxLimit:     maxLimit,
		hybridSearchFn: func(_ context.Context, _, _, _ string, _ []float32, topK int) (*model.HybridSearchResponse, error) {
			// The handler passes the raw topK through; the service clamps.
			if topK != 999 {
				t.Errorf("expected raw topK 999 to reach service, got %d", topK)
			}
			results := make([]model.HybridSearchResult, maxLimit)
			for i := range results {
				results[i] = model.HybridSearchResult{ChunkID: "c", RRFScore: 0.01}
			}
			return &model.HybridSearchResponse{Results: results, Total: maxLimit}, nil
		},
	}
	r := newSearchRouter(svc)
	body := `{"query":"refund","top_k":999}`
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, hybridSearchPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp model.HybridSearchHTTPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != maxLimit {
		t.Errorf("expected %d results (clamped), got %d", maxLimit, len(resp.Results))
	}
	if resp.TopK != maxLimit {
		t.Errorf("expected echo top_k %d (clamped), got %d", maxLimit, resp.TopK)
	}
}

func TestHybridSearch_KBNotFound_404(t *testing.T) {
	svc := &mockSearchService{
		hybridSearchFn: func(context.Context, string, string, string, []float32, int) (*model.HybridSearchResponse, error) {
			return nil, apierror.NewNotFound("knowledge base not found")
		},
	}
	r := newSearchRouter(svc)
	body := `{"query":"refund","top_k":5}`
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, hybridSearchPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHybridSearch_UnauthorizedOrg_403(t *testing.T) {
	svc := &mockSearchService{
		hybridSearchFn: func(context.Context, string, string, string, []float32, int) (*model.HybridSearchResponse, error) {
			t.Fatal("service should not be called when RBAC rejects")
			return nil, nil
		},
	}
	// Caller belongs to org-other but is hitting org-abc.
	r := newSearchRouterWithRBAC(svc, "org-other", "member")
	body := `{"query":"refund"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, hybridSearchPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHybridSearch_InvalidJSON_400(t *testing.T) {
	svc := &mockSearchService{}
	r := newSearchRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, hybridSearchPath, strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHybridSearch_NegativeTopK_400(t *testing.T) {
	svc := &mockSearchService{}
	r := newSearchRouter(svc)
	body := `{"query":"refund","top_k":-3}`
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, hybridSearchPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
