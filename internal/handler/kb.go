package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// KBServicer is the interface the handler requires from the service layer.
type KBServicer interface {
	Create(ctx context.Context, orgID, wsID string, req model.CreateKBRequest) (*model.KnowledgeBase, error)
	GetByID(ctx context.Context, orgID, kbID string) (*model.KnowledgeBase, error)
	ListByWorkspace(ctx context.Context, orgID, wsID string) ([]model.KnowledgeBase, error)
	Update(ctx context.Context, orgID, kbID string, req model.UpdateKBRequest) (*model.KnowledgeBase, error)
	Archive(ctx context.Context, orgID, kbID string) error
}

// KBHandler handles HTTP requests for knowledge base management.
type KBHandler struct {
	svc KBServicer
}

// NewKBHandler creates a new KBHandler.
func NewKBHandler(svc KBServicer) *KBHandler {
	return &KBHandler{svc: svc}
}

// Create handles POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases.
// Requires minimum workspace role "member" (enforced at route registration).
func (h *KBHandler) Create(c *gin.Context) {
	orgID := c.Param("org_id")
	wsID := c.Param("ws_id")
	var req model.CreateKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	kb, err := h.svc.Create(c.Request.Context(), orgID, wsID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, kb)
}

// Get handles GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id.
func (h *KBHandler) Get(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")
	kb, err := h.svc.GetByID(c.Request.Context(), orgID, kbID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, kb)
}

// List handles GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases.
func (h *KBHandler) List(c *gin.Context) {
	orgID := c.Param("org_id")
	wsID := c.Param("ws_id")
	kbs, err := h.svc.ListByWorkspace(c.Request.Context(), orgID, wsID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	if kbs == nil {
		kbs = []model.KnowledgeBase{}
	}
	c.JSON(http.StatusOK, kbs)
}

// Update handles PUT /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id.
func (h *KBHandler) Update(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")
	var req model.UpdateKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	kb, err := h.svc.Update(c.Request.Context(), orgID, kbID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, kb)
}

// Archive handles DELETE /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id.
// Requires workspace role "admin" (enforced at route registration).
func (h *KBHandler) Archive(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")
	if err := h.svc.Archive(c.Request.Context(), orgID, kbID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
