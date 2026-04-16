package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SeedResult is the response from the seed endpoint.
type SeedResult struct {
	OrgID             string `json:"org_id"`
	WorkspaceID       string `json:"workspace_id"`
	KBID              string `json:"kb_id"`
	DocumentsEnqueued int    `json:"documents_enqueued"`
	PipelineStatus    string `json:"pipeline_status"`
}

// SeedServicer is the interface the seed handler requires.
type SeedServicer interface {
	SeedDemo(ctx context.Context, size string) (*SeedResult, error)
}

// SeedHandler handles the admin seed endpoint.
type SeedHandler struct {
	svc SeedServicer
}

// NewSeedHandler creates a new SeedHandler.
func NewSeedHandler(svc SeedServicer) *SeedHandler {
	return &SeedHandler{svc: svc}
}

// SeedDemo handles POST /api/v1/admin/seed-demo.
// Reads `size` query param (default "small").
func (h *SeedHandler) SeedDemo(c *gin.Context) {
	size := c.DefaultQuery("size", "small")

	result, err := h.svc.SeedDemo(c.Request.Context(), size)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, result)
}
