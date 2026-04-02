package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// RoutingServicer is the interface the handler requires from the service layer.
type RoutingServicer interface {
	Create(ctx context.Context, orgID string, req model.CreateRoutingRuleRequest, createdBy string) (*model.RoutingRule, error)
	GetByID(ctx context.Context, orgID, ruleID string) (*model.RoutingRule, error)
	List(ctx context.Context, orgID string, page, pageSize int) (*model.RoutingRuleListResponse, error)
	Update(ctx context.Context, orgID, ruleID string, req model.UpdateRoutingRuleRequest) (*model.RoutingRule, error)
	Delete(ctx context.Context, orgID, ruleID string) error
	ResolveKBForDocument(ctx context.Context, orgID, sourceType, sourceIdentifier string, metadata map[string]any) (*model.ResolveRoutingResponse, error)
	ListCatalogMetadata(ctx context.Context, orgID, catalogType string) ([]model.CatalogMetadata, error)
}

// RoutingHandler handles HTTP requests for routing rule management.
type RoutingHandler struct {
	svc RoutingServicer
}

// NewRoutingHandler creates a new RoutingHandler.
func NewRoutingHandler(svc RoutingServicer) *RoutingHandler {
	return &RoutingHandler{svc: svc}
}

// Create handles POST /api/v1/orgs/:org_id/routing-rules.
//
// @Summary     Create routing rule
// @Description Create a new data classification and routing rule
// @Tags        routing-rules
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.CreateRoutingRuleRequest true "Routing rule payload"
// @Success     201 {object} model.RoutingRule
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/routing-rules [post]
func (h *RoutingHandler) Create(c *gin.Context) {
	orgID := c.Param("org_id")
	var req model.CreateRoutingRuleRequest
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
	rule, err := h.svc.Create(c.Request.Context(), orgID, req, createdByStr)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// List handles GET /api/v1/orgs/:org_id/routing-rules.
//
// @Summary     List routing rules
// @Description List all routing rules for an organisation
// @Tags        routing-rules
// @Produce     json
// @Security    BearerAuth
// @Param       org_id    path  string true  "Organisation ID"
// @Param       page      query int    false "Page number (default 1)"
// @Param       page_size query int    false "Page size (default 20, max 100)"
// @Success     200 {object} model.RoutingRuleListResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/routing-rules [get]
func (h *RoutingHandler) List(c *gin.Context) {
	orgID := c.Param("org_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.List(c.Request.Context(), orgID, page, pageSize)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Get handles GET /api/v1/orgs/:org_id/routing-rules/:rule_id.
//
// @Summary     Get routing rule by ID
// @Description Get a single routing rule by its ID
// @Tags        routing-rules
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       rule_id path string true "Routing rule ID"
// @Success     200 {object} model.RoutingRule
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/routing-rules/{rule_id} [get]
func (h *RoutingHandler) Get(c *gin.Context) {
	orgID := c.Param("org_id")
	ruleID := c.Param("rule_id")
	rule, err := h.svc.GetByID(c.Request.Context(), orgID, ruleID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, rule)
}

// Update handles PUT /api/v1/orgs/:org_id/routing-rules/:rule_id.
//
// @Summary     Update routing rule
// @Description Update an existing routing rule
// @Tags        routing-rules
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       rule_id path string true "Routing rule ID"
// @Param       request body model.UpdateRoutingRuleRequest true "Update payload"
// @Success     200 {object} model.RoutingRule
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/routing-rules/{rule_id} [put]
func (h *RoutingHandler) Update(c *gin.Context) {
	orgID := c.Param("org_id")
	ruleID := c.Param("rule_id")
	var req model.UpdateRoutingRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	rule, err := h.svc.Update(c.Request.Context(), orgID, ruleID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, rule)
}

// Delete handles DELETE /api/v1/orgs/:org_id/routing-rules/:rule_id.
//
// @Summary     Delete routing rule
// @Description Permanently remove a routing rule
// @Tags        routing-rules
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       rule_id path string true "Routing rule ID"
// @Success     204
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/routing-rules/{rule_id} [delete]
func (h *RoutingHandler) Delete(c *gin.Context) {
	orgID := c.Param("org_id")
	ruleID := c.Param("rule_id")
	if err := h.svc.Delete(c.Request.Context(), orgID, ruleID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// Resolve handles POST /api/v1/orgs/:org_id/routing-rules/resolve.
//
// @Summary     Resolve routing
// @Description Test resolution: given source metadata, determine which knowledge base to route to
// @Tags        routing-rules
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.ResolveRoutingRequest true "Resolution request"
// @Success     200 {object} model.ResolveRoutingResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/routing-rules/resolve [post]
func (h *RoutingHandler) Resolve(c *gin.Context) {
	orgID := c.Param("org_id")
	var req model.ResolveRoutingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	resp, err := h.svc.ResolveKBForDocument(c.Request.Context(), orgID, req.SourceType, req.SourceIdentifier, req.Metadata)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListCatalog handles GET /api/v1/orgs/:org_id/catalog.
//
// @Summary     List catalog metadata
// @Description List discovered catalog metadata from external data sources
// @Tags        routing-rules
// @Produce     json
// @Security    BearerAuth
// @Param       org_id       path  string true  "Organisation ID"
// @Param       catalog_type query string false "Filter by catalog type"
// @Success     200 {array} model.CatalogMetadata
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/catalog [get]
func (h *RoutingHandler) ListCatalog(c *gin.Context) {
	orgID := c.Param("org_id")
	catalogType := c.Query("catalog_type")
	items, err := h.svc.ListCatalogMetadata(c.Request.Context(), orgID, catalogType)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	items = lo.Ternary(items == nil, []model.CatalogMetadata{}, items)
	c.JSON(http.StatusOK, items)
}
