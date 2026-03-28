package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// APIKeyServicer is the interface the handler requires from the service layer.
type APIKeyServicer interface {
	Create(ctx context.Context, orgID, wsID, kbID, userID string, req model.CreateAPIKeyRequest) (*model.CreateAPIKeyResponse, error)
	List(ctx context.Context, orgID, kbID string) ([]model.APIKey, error)
	Revoke(ctx context.Context, orgID, id string) error
}

// APIKeyHandler handles HTTP requests for API key management.
type APIKeyHandler struct {
	svc APIKeyServicer
}

// NewAPIKeyHandler creates a new APIKeyHandler.
func NewAPIKeyHandler(svc APIKeyServicer) *APIKeyHandler {
	return &APIKeyHandler{svc: svc}
}

// Create handles POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/api-keys.
//
// @Summary     Create API key
// @Tags        api-keys
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       ws_id   path string true "Workspace ID"
// @Param       kb_id   path string true "Knowledge base ID"
// @Param       request body model.CreateAPIKeyRequest true "API key payload"
// @Success     201 {object} model.CreateAPIKeyResponse
// @Failure     422 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/api-keys [post]
func (h *APIKeyHandler) Create(c *gin.Context) {
	orgID := c.Param("org_id")
	wsID := c.Param("ws_id")
	kbID := c.Param("kb_id")
	userID, _ := c.Get(string(middleware.ContextKeyUserID))

	var req model.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	uid, _ := userID.(string)
	resp, err := h.svc.Create(c.Request.Context(), orgID, wsID, kbID, uid, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// List handles GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/api-keys.
//
// @Summary     List API keys for a knowledge base
// @Tags        api-keys
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       ws_id  path string true "Workspace ID"
// @Param       kb_id  path string true "Knowledge base ID"
// @Success     200 {array} model.APIKey
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/api-keys [get]
func (h *APIKeyHandler) List(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")

	keys, err := h.svc.List(c.Request.Context(), orgID, kbID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	if keys == nil {
		keys = []model.APIKey{}
	}
	c.JSON(http.StatusOK, keys)
}

// Revoke handles DELETE /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/api-keys/:key_id.
//
// @Summary     Revoke an API key
// @Tags        api-keys
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       ws_id  path string true "Workspace ID"
// @Param       kb_id  path string true "Knowledge base ID"
// @Param       key_id path string true "API key ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/api-keys/{key_id} [delete]
func (h *APIKeyHandler) Revoke(c *gin.Context) {
	orgID := c.Param("org_id")
	keyID := c.Param("key_id")

	if err := h.svc.Revoke(c.Request.Context(), orgID, keyID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
