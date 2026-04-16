package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/model"
)

// SeedServicer is the interface the seed handler requires.
type SeedServicer interface {
	SeedDemo(ctx context.Context, size string) (*model.SeedResult, error)
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
// Reads `size` query param (default "small"). Only "small" is supported.
func (h *SeedHandler) SeedDemo(c *gin.Context) {
	size := c.DefaultQuery("size", "small")
	if size != "small" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "only size=small is supported"})
		return
	}

	result, err := h.svc.SeedDemo(c.Request.Context(), size)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, result)
}
