package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// SourceServicer is the interface the handler requires from the service layer.
type SourceServicer interface {
	Create(ctx context.Context, orgID, kbID string, req model.CreateSourceRequest, createdBy string) (*model.Source, error)
	GetByID(ctx context.Context, orgID, sourceID string) (*model.Source, error)
	List(ctx context.Context, orgID, kbID string, page, pageSize int) (*model.SourceListResponse, error)
	Update(ctx context.Context, orgID, sourceID string, req model.UpdateSourceRequest) (*model.Source, error)
	Delete(ctx context.Context, orgID, sourceID string) error
}

// SourceHandler handles HTTP requests for source management.
type SourceHandler struct {
	svc SourceServicer
}

// NewSourceHandler creates a new SourceHandler.
func NewSourceHandler(svc SourceServicer) *SourceHandler {
	return &SourceHandler{svc: svc}
}

// Create handles POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/sources.
// Requires minimum workspace role "member" (enforced at route registration).
//
// @Summary     Create source
// @Tags        sources
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       ws_id   path string true "Workspace ID"
// @Param       kb_id   path string true "Knowledge base ID"
// @Param       request body model.CreateSourceRequest true "Source payload"
// @Success     201 {object} model.Source
// @Failure     400 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/sources [post]
func (h *SourceHandler) Create(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")
	var req model.CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	createdBy, _ := c.Get(string(middleware.ContextKeyUserID))
	createdByStr, _ := createdBy.(string)
	src, err := h.svc.Create(c.Request.Context(), orgID, kbID, req, createdByStr)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, src)
}

// Get handles GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/sources/:source_id.
//
// @Summary     Get source by ID
// @Tags        sources
// @Produce     json
// @Security    BearerAuth
// @Param       org_id    path string true "Organisation ID"
// @Param       ws_id     path string true "Workspace ID"
// @Param       kb_id     path string true "Knowledge base ID"
// @Param       source_id path string true "Source ID"
// @Success     200 {object} model.Source
// @Failure     404 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/sources/{source_id} [get]
func (h *SourceHandler) Get(c *gin.Context) {
	orgID := c.Param("org_id")
	sourceID := c.Param("source_id")
	src, err := h.svc.GetByID(c.Request.Context(), orgID, sourceID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, src)
}

// List handles GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/sources.
//
// @Summary     List sources in a knowledge base
// @Tags        sources
// @Produce     json
// @Security    BearerAuth
// @Param       org_id    path  string true  "Organisation ID"
// @Param       ws_id     path  string true  "Workspace ID"
// @Param       kb_id     path  string true  "Knowledge base ID"
// @Param       page      query int    false "Page number (default 1)"
// @Param       page_size query int    false "Page size (default 20, max 100)"
// @Success     200 {object} model.SourceListResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/sources [get]
func (h *SourceHandler) List(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.List(c.Request.Context(), orgID, kbID, page, pageSize)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Update handles PUT /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/sources/:source_id.
//
// @Summary     Update source
// @Tags        sources
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id    path string true "Organisation ID"
// @Param       ws_id     path string true "Workspace ID"
// @Param       kb_id     path string true "Knowledge base ID"
// @Param       source_id path string true "Source ID"
// @Param       request   body model.UpdateSourceRequest true "Source update payload"
// @Success     200 {object} model.Source
// @Failure     400 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/sources/{source_id} [put]
func (h *SourceHandler) Update(c *gin.Context) {
	orgID := c.Param("org_id")
	sourceID := c.Param("source_id")
	var req model.UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	src, err := h.svc.Update(c.Request.Context(), orgID, sourceID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, src)
}

// Delete handles DELETE /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/sources/:source_id.
// Requires workspace role "admin" (enforced at route registration).
//
// @Summary     Delete source
// @Tags        sources
// @Security    BearerAuth
// @Param       org_id    path string true "Organisation ID"
// @Param       ws_id     path string true "Workspace ID"
// @Param       kb_id     path string true "Knowledge base ID"
// @Param       source_id path string true "Source ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/sources/{source_id} [delete]
func (h *SourceHandler) Delete(c *gin.Context) {
	orgID := c.Param("org_id")
	sourceID := c.Param("source_id")
	if err := h.svc.Delete(c.Request.Context(), orgID, sourceID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
