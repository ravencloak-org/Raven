package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// SearchServicer is the interface the handler requires from the service layer.
type SearchServicer interface {
	TextSearch(ctx context.Context, orgID, kbID, query string, limit int) (*model.SearchResponse, error)
	TextSearchWithFilters(ctx context.Context, orgID, kbID, query string, docIDs []string, limit int) (*model.SearchResponse, error)
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
