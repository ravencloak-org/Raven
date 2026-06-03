package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/middleware"
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

// KBPublisher is the narrow slice of marketplace.PublishService the KB
// handler needs. Defined here so handler unit tests can pass a stub
// without dragging the full pgxpool wiring into the test harness.
type KBPublisher interface {
	PublishKB(ctx context.Context, orgID, kbID, userID, licenseSPDXID string) (*marketplace.PublishResult, error)
}

// PublishKBRequest is the payload for POST .../knowledge-bases/{id}/publish.
// The SPDX identifier is required and is validated against the canonical
// allow-list in marketplace.IsAllowedLicense — the binding tag only catches
// the empty-string case.
type PublishKBRequest struct {
	LicenseSPDXID string `json:"license_spdx_id" binding:"required"`
}

// KBHandler handles HTTP requests for knowledge base management.
type KBHandler struct {
	svc       KBServicer
	publisher KBPublisher
}

// NewKBHandler creates a new KBHandler. The publisher is optional and may
// be left nil in test wiring that does not exercise the publish path; the
// Publish handler short-circuits with 500 when it is nil so the missing
// dependency surfaces loudly rather than as a confusing 404.
func NewKBHandler(svc KBServicer) *KBHandler {
	return &KBHandler{svc: svc}
}

// WithPublisher wires the Marketplace publish service. Kept as an
// after-construction setter so existing callers of NewKBHandler don't
// have to be touched and so the handler tests can opt in to the publish
// path with a stub.
func (h *KBHandler) WithPublisher(p KBPublisher) *KBHandler {
	h.publisher = p
	return h
}

// Create handles POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases.
// Requires minimum workspace role "member" (enforced at route registration).
//
// @Summary     Create knowledge base
// @Tags        knowledge-bases
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       ws_id   path string true "Workspace ID"
// @Param       request body model.CreateKBRequest true "Knowledge base payload"
// @Success     201 {object} model.KnowledgeBase
// @Failure     422 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases [post]
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
//
// @Summary     List knowledge bases in a workspace
// @Tags        knowledge-bases
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       ws_id  path string true "Workspace ID"
// @Success     200 {array} model.KnowledgeBase
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases [get]
func (h *KBHandler) List(c *gin.Context) {
	orgID := c.Param("org_id")
	wsID := c.Param("ws_id")
	kbs, err := h.svc.ListByWorkspace(c.Request.Context(), orgID, wsID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	kbs = lo.Ternary(kbs == nil, []model.KnowledgeBase{}, kbs)
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

// Publish handles POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/publish.
// Flips a KB from private to public, validates the chosen SPDX license
// against marketplace.AllowedSPDXLicenses, and stamps the publish
// metadata. Requires workspace role "admin" (enforced at route
// registration), which is Raven's equivalent of the kb:publish
// permission until per-action ACLs land.
//
// @Summary     Publish knowledge base to Marketplace
// @Tags        knowledge-bases
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       ws_id   path string true "Workspace ID"
// @Param       kb_id   path string true "Knowledge base ID"
// @Param       request body PublishKBRequest true "Publish payload"
// @Success     200 {object} marketplace.PublishResult
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     409 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/publish [post]
func (h *KBHandler) Publish(c *gin.Context) {
	if h.publisher == nil {
		_ = c.Error(apierror.NewInternal("publish service not wired"))
		c.Abort()
		return
	}
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")
	userID, _ := c.Get(string(middleware.ContextKeyUserID))
	userIDStr, _ := userID.(string)
	if userIDStr == "" {
		// Auth middleware should have already short-circuited; if we ever
		// land here the request is unauthenticated. Surface 401 instead
		// of writing a NULL user id to the audit column.
		_ = c.Error(apierror.NewUnauthorized("user context missing"))
		c.Abort()
		return
	}

	var req PublishKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierror.NewUnprocessableEntity(err.Error()))
		c.Abort()
		return
	}

	result, err := h.publisher.PublishKB(c.Request.Context(), orgID, kbID, userIDStr, req.LicenseSPDXID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, result)
}

// Archive handles DELETE /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id.
// Requires workspace role "admin" (enforced at route registration).
//
// @Summary     Archive (soft-delete) knowledge base
// @Tags        knowledge-bases
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       ws_id  path string true "Workspace ID"
// @Param       kb_id  path string true "Knowledge base ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id} [delete]
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
