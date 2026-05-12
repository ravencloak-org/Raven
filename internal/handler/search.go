package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// SearchServicer is the interface the handler requires from the service layer.
//
// HybridSearch fuses pgvector cosine similarity with BM25 keyword ranking via
// Reciprocal Rank Fusion (see `internal/service/search.go`). The embedding
// argument is optional — if empty the vector leg is skipped and the response
// degrades cleanly to BM25-only, mirroring the GET /search behaviour.
type SearchServicer interface {
	TextSearch(ctx context.Context, orgID, kbID, query string, limit int) (*model.SearchResponse, error)
	TextSearchWithFilters(ctx context.Context, orgID, kbID, query string, docIDs []string, limit int) (*model.SearchResponse, error)
	HybridSearch(ctx context.Context, orgID, kbID, query string, embedding []float32, topK int) (*model.HybridSearchResponse, error)
	RetrievalLimits() (defaultLimit, maxLimit int)
}

// SearchHandler handles HTTP requests for full-text search.
type SearchHandler struct {
	svc SearchServicer
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(svc SearchServicer) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// Search handles GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/search.
//
// @Summary     Full-text search within a knowledge base
// @Description Searches chunks by content and heading using PostgreSQL tsvector with BM25-style ranking.
// @Tags        search
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path   string true  "Organisation ID"
// @Param       ws_id  path   string true  "Workspace ID"
// @Param       kb_id  path   string true  "Knowledge base ID"
// @Param       q      query  string true  "Search query"
// @Param       limit  query  int    false "Maximum results (default 10, max 100)"
// @Param       doc_ids query []string false "Filter by document IDs"
// @Success     200 {object} model.SearchResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")

	query := c.Query("q")
	if query == "" {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  "query parameter 'q' is required",
		})
		c.Abort()
		return
	}

	limit := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 0 {
			_ = c.Error(&apierror.AppError{
				Code:    http.StatusBadRequest,
				Message: "Bad Request",
				Detail:  "limit must be a non-negative integer",
			})
			c.Abort()
			return
		}
		limit = parsed
	}

	docIDs := c.QueryArray("doc_ids")

	var (
		resp *model.SearchResponse
		err  error
	)
	if len(docIDs) > 0 {
		resp, err = h.svc.TextSearchWithFilters(c.Request.Context(), orgID, kbID, query, docIDs, limit)
	} else {
		resp, err = h.svc.TextSearch(c.Request.Context(), orgID, kbID, query, limit)
	}
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// HybridSearch handles POST
// /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/hybrid-search.
//
// Body shape:
//
//	{
//	  "query":     "string, required, non-empty",
//	  "top_k":     10,                              // optional, server-clamped to RetrievalConfig.MaxLimit
//	  "filters":   {"rerank": "cohere"},            // optional, free-form bag
//	  "doc_ids":   ["uuid", ...],                   // optional, currently informational (RRF leg ignores)
//	  "embedding": [0.01, -0.02, ...]               // optional, pre-computed query embedding
//	}
//
// If `embedding` is omitted the response is a BM25-only RRF ranking — the
// vector leg is silently skipped inside `SearchService.HybridSearch`.
//
// @Summary     Hybrid (vector + BM25) search within a knowledge base
// @Description Fuses pgvector cosine similarity with PostgreSQL tsvector BM25 ranking via Reciprocal Rank Fusion (k = retrieval.rrf_k). Caller must supply a pre-computed query embedding for the vector leg; omitting it returns BM25-only results.
// @Tags        search
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string                       true "Organisation ID"
// @Param       ws_id  path string                       true "Workspace ID"
// @Param       kb_id  path string                       true "Knowledge base ID"
// @Param       body   body model.HybridSearchRequest    true "Hybrid search request"
// @Success     200 {object} model.HybridSearchHTTPResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/hybrid-search [post]
func (h *SearchHandler) HybridSearch(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")

	var req model.HybridSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid JSON body: " + err.Error()))
		c.Abort()
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		_ = c.Error(apierror.NewBadRequest("field 'query' is required and must be non-empty"))
		c.Abort()
		return
	}

	if req.TopK < 0 {
		_ = c.Error(apierror.NewBadRequest("field 'top_k' must be a non-negative integer"))
		c.Abort()
		return
	}

	// Compute the effective top_k that the service will use, so we can echo
	// it back in the response. Server-side clamp (0 → DefaultLimit,
	// > MaxLimit → MaxLimit) is applied inside SearchService.HybridSearch
	// via clampLimitWith — we mirror it here purely for the response echo.
	defaultLimit, maxLimit := h.svc.RetrievalLimits()
	effectiveTopK := req.TopK
	switch {
	case effectiveTopK <= 0:
		effectiveTopK = defaultLimit
	case effectiveTopK > maxLimit:
		effectiveTopK = maxLimit
	}

	resp, err := h.svc.HybridSearch(c.Request.Context(), orgID, kbID, query, req.Embedding, req.TopK)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	results := resp.Results
	if results == nil {
		results = []model.HybridSearchResult{}
	}
	c.JSON(http.StatusOK, model.HybridSearchHTTPResponse{
		Results: results,
		Query:   query,
		TopK:    effectiveTopK,
	})
}
