package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// OrgServicer is the interface the handler requires from the service layer.
type OrgServicer interface {
	Create(ctx context.Context, req model.CreateOrgRequest) (*model.Organization, error)
	GetByID(ctx context.Context, orgID string) (*model.Organization, error)
	Update(ctx context.Context, orgID string, req model.UpdateOrgRequest) (*model.Organization, error)
	Delete(ctx context.Context, orgID string) error
}

// OrgHandler handles HTTP requests for organisation management.
type OrgHandler struct {
	svc OrgServicer
}

// NewOrgHandler creates a new OrgHandler.
func NewOrgHandler(svc OrgServicer) *OrgHandler {
	return &OrgHandler{svc: svc}
}

// Create handles POST /api/v1/orgs.
func (h *OrgHandler) Create(c *gin.Context) {
	var req model.CreateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}
	org, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, org)
}

// Get handles GET /api/v1/orgs/:org_id.
func (h *OrgHandler) Get(c *gin.Context) {
	orgID := c.Param("org_id")
	org, err := h.svc.GetByID(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, org)
}

// Update handles PUT /api/v1/orgs/:org_id.
func (h *OrgHandler) Update(c *gin.Context) {
	orgID := c.Param("org_id")
	callerOrgID, _ := c.Get(string(middleware.ContextKeyOrgID))
	callerRole, _ := c.Get(string(middleware.ContextKeyOrgRole))
	if callerRole != "org_admin" && callerOrgID != orgID {
		c.AbortWithStatusJSON(http.StatusForbidden, apierror.AppError{
			Code:    http.StatusForbidden,
			Message: "Forbidden",
			Detail:  "cannot update another organisation",
		})
		return
	}

	var req model.UpdateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}
	org, err := h.svc.Update(c.Request.Context(), orgID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, org)
}

// Delete handles DELETE /api/v1/orgs/:org_id.
func (h *OrgHandler) Delete(c *gin.Context) {
	orgID := c.Param("org_id")
	if err := h.svc.Delete(c.Request.Context(), orgID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
