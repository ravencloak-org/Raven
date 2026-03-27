package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// WorkspaceServicer is the interface the handler requires from the service layer.
type WorkspaceServicer interface {
	Create(ctx context.Context, orgID string, req model.CreateWorkspaceRequest) (*model.Workspace, error)
	GetByOrgAndID(ctx context.Context, orgID, wsID string) (*model.Workspace, error)
	ListByOrg(ctx context.Context, orgID string) ([]model.Workspace, error)
	Update(ctx context.Context, orgID, wsID string, req model.UpdateWorkspaceRequest) (*model.Workspace, error)
	Delete(ctx context.Context, orgID, wsID string) error
	AddMember(ctx context.Context, orgID, wsID string, req model.AddWorkspaceMemberRequest) (*model.WorkspaceMember, error)
	UpdateMemberRole(ctx context.Context, orgID, wsID string, req model.UpdateWorkspaceMemberRequest, userID string) (*model.WorkspaceMember, error)
	RemoveMember(ctx context.Context, orgID, wsID, userID string) error
}

// WorkspaceHandler handles HTTP requests for workspace management.
type WorkspaceHandler struct {
	svc WorkspaceServicer
}

// NewWorkspaceHandler creates a new WorkspaceHandler.
func NewWorkspaceHandler(svc WorkspaceServicer) *WorkspaceHandler {
	return &WorkspaceHandler{svc: svc}
}

// orgIDFromContext extracts the org_id path param; falls back to JWT claim for validation.
func orgIDFromContext(c *gin.Context) string {
	return c.Param("org_id")
}

// Create handles POST /api/v1/orgs/:org_id/workspaces.
func (h *WorkspaceHandler) Create(c *gin.Context) {
	orgID := orgIDFromContext(c)
	var req model.CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	ws, err := h.svc.Create(c.Request.Context(), orgID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, ws)
}

// Get handles GET /api/v1/orgs/:org_id/workspaces/:ws_id.
func (h *WorkspaceHandler) Get(c *gin.Context) {
	orgID := orgIDFromContext(c)
	wsID := c.Param("ws_id")
	ws, err := h.svc.GetByOrgAndID(c.Request.Context(), orgID, wsID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, ws)
}

// List handles GET /api/v1/orgs/:org_id/workspaces.
func (h *WorkspaceHandler) List(c *gin.Context) {
	orgID := orgIDFromContext(c)
	workspaces, err := h.svc.ListByOrg(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	if workspaces == nil {
		workspaces = []model.Workspace{}
	}
	c.JSON(http.StatusOK, workspaces)
}

// Update handles PUT /api/v1/orgs/:org_id/workspaces/:ws_id.
func (h *WorkspaceHandler) Update(c *gin.Context) {
	orgID := orgIDFromContext(c)
	wsID := c.Param("ws_id")
	var req model.UpdateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	ws, err := h.svc.Update(c.Request.Context(), orgID, wsID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, ws)
}

// Delete handles DELETE /api/v1/orgs/:org_id/workspaces/:ws_id.
func (h *WorkspaceHandler) Delete(c *gin.Context) {
	orgID := orgIDFromContext(c)
	wsID := c.Param("ws_id")
	if err := h.svc.Delete(c.Request.Context(), orgID, wsID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// AddMember handles POST /api/v1/orgs/:org_id/workspaces/:ws_id/members.
func (h *WorkspaceHandler) AddMember(c *gin.Context) {
	orgID := orgIDFromContext(c)
	wsID := c.Param("ws_id")
	var req model.AddWorkspaceMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	member, err := h.svc.AddMember(c.Request.Context(), orgID, wsID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, member)
}

// UpdateMember handles PUT /api/v1/orgs/:org_id/workspaces/:ws_id/members/:user_id.
func (h *WorkspaceHandler) UpdateMember(c *gin.Context) {
	orgID := orgIDFromContext(c)
	wsID := c.Param("ws_id")
	userID := c.Param("user_id")
	var req model.UpdateWorkspaceMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	member, err := h.svc.UpdateMemberRole(c.Request.Context(), orgID, wsID, req, userID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, member)
}

// RemoveMember handles DELETE /api/v1/orgs/:org_id/workspaces/:ws_id/members/:user_id.
func (h *WorkspaceHandler) RemoveMember(c *gin.Context) {
	orgID := orgIDFromContext(c)
	wsID := c.Param("ws_id")
	userID := c.Param("user_id")

	// Prevent a user from removing themselves (must go through org admin).
	callerID, _ := c.Get(string(middleware.ContextKeyUserID))
	if callerID == userID {
		_ = c.Error(apierror.NewBadRequest("use DELETE /api/v1/me to leave a workspace"))
		c.Abort()
		return
	}

	if err := h.svc.RemoveMember(c.Request.Context(), orgID, wsID, userID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
