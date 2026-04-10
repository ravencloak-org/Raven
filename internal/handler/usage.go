package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// UsageServicer defines what the usage handler needs from the service layer.
type UsageServicer interface {
	GetUsage(ctx context.Context, orgID string) (*model.UsageResponse, error)
}

// UsageHandler handles HTTP requests for billing usage.
type UsageHandler struct {
	svc UsageServicer
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler(svc UsageServicer) *UsageHandler {
	return &UsageHandler{svc: svc}
}

// GetUsage handles GET /api/v1/billing/usage.
func (h *UsageHandler) GetUsage(c *gin.Context) {
	orgID, exists := c.Get(string(middleware.ContextKeyOrgID))
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized",
			Detail:  "missing organisation context",
		})
		return
	}

	usage, err := h.svc.GetUsage(c.Request.Context(), orgID.(string))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, usage)
}
