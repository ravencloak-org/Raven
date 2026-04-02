package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// SemanticCacheRepositorier defines the repository operations needed by the handler.
type SemanticCacheRepositorier interface {
	InvalidateKB(ctx context.Context, orgID, kbID string) (int64, error)
	Stats(ctx context.Context, orgID, kbID string) (count int64, avgHits float64, err error)
}

// SemanticCacheHandler handles HTTP requests for semantic cache management.
type SemanticCacheHandler struct {
	repo SemanticCacheRepositorier
}

// NewSemanticCacheHandler creates a new SemanticCacheHandler.
func NewSemanticCacheHandler(repo SemanticCacheRepositorier) *SemanticCacheHandler {
	return &SemanticCacheHandler{repo: repo}
}

// InvalidateKBCache handles DELETE /orgs/:org_id/kbs/:kb_id/cache.
//
// @Summary     Invalidate semantic cache for a knowledge base
// @Tags        cache
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       kb_id  path string true "Knowledge Base ID"
// @Success     200 {object} map[string]int64
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/kbs/{kb_id}/cache [delete]
func (h *SemanticCacheHandler) InvalidateKBCache(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")

	if _, err := uuid.Parse(orgID); err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid org_id"))
		c.Abort()
		return
	}
	if _, err := uuid.Parse(kbID); err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid kb_id"))
		c.Abort()
		return
	}

	deleted, err := h.repo.InvalidateKB(c.Request.Context(), orgID, kbID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to invalidate KB cache",
			"error", err, "org_id", orgID, "kb_id", kbID)
		_ = c.Error(apierror.NewInternal("failed to invalidate cache"))
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

// GetCacheStats handles GET /orgs/:org_id/kbs/:kb_id/cache/stats.
//
// @Summary     Get semantic cache statistics for a knowledge base
// @Tags        cache
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       kb_id  path string true "Knowledge Base ID"
// @Success     200 {object} map[string]interface{}
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/kbs/{kb_id}/cache/stats [get]
func (h *SemanticCacheHandler) GetCacheStats(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")

	if _, err := uuid.Parse(orgID); err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid org_id"))
		c.Abort()
		return
	}
	if _, err := uuid.Parse(kbID); err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid kb_id"))
		c.Abort()
		return
	}

	count, avgHits, err := h.repo.Stats(c.Request.Context(), orgID, kbID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to fetch cache stats",
			"error", err, "org_id", orgID, "kb_id", kbID)
		_ = c.Error(apierror.NewInternal("failed to fetch cache stats"))
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count":    count,
		"avg_hits": avgHits,
	})
}
