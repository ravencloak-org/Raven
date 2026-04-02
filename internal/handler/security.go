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

// SecurityServicer is the interface the handler requires from the service layer.
type SecurityServicer interface {
	Create(ctx context.Context, orgID, userID string, req model.CreateSecurityRuleRequest) (*model.SecurityRule, error)
	GetByID(ctx context.Context, orgID, ruleID string) (*model.SecurityRule, error)
	List(ctx context.Context, orgID string) ([]model.SecurityRule, error)
	Update(ctx context.Context, orgID, ruleID string, req model.UpdateSecurityRuleRequest) (*model.SecurityRule, error)
	Delete(ctx context.Context, orgID, ruleID string) error
	ListEvents(ctx context.Context, orgID string, limit, offset int) (*model.SecurityEventResponse, error)
	InvalidateCache(ctx context.Context, orgID string)
}

// SecurityHandler handles HTTP requests for security rule management.
type SecurityHandler struct {
	svc SecurityServicer
}

// NewSecurityHandler creates a new SecurityHandler.
func NewSecurityHandler(svc SecurityServicer) *SecurityHandler {
	return &SecurityHandler{svc: svc}
}

// CreateRule handles POST /api/v1/orgs/:org_id/security/rules.
//
// @Summary     Create security rule
// @Tags        security
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.CreateSecurityRuleRequest true "Security rule payload"
// @Success     201 {object} model.SecurityRule
// @Failure     400 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /orgs/{org_id}/security/rules [post]
func (h *SecurityHandler) CreateRule(c *gin.Context) {
	orgID := c.Param("org_id")
	userIDVal, exists := c.Get(string(middleware.ContextKeyUserID))
	if !exists {
		_ = c.Error(apierror.NewUnauthorized("user authentication required"))
		c.Abort()
		return
	}
	userID, _ := userIDVal.(string)

	var req model.CreateSecurityRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	rule, err := h.svc.Create(c.Request.Context(), orgID, userID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// ListRules handles GET /api/v1/orgs/:org_id/security/rules.
//
// @Summary     List security rules
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Success     200 {array} model.SecurityRule
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/security/rules [get]
func (h *SecurityHandler) ListRules(c *gin.Context) {
	orgID := c.Param("org_id")

	rules, err := h.svc.List(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	if rules == nil {
		rules = []model.SecurityRule{}
	}
	c.JSON(http.StatusOK, rules)
}

// GetRule handles GET /api/v1/orgs/:org_id/security/rules/:rule_id.
//
// @Summary     Get security rule
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       rule_id path string true "Security rule ID"
// @Success     200 {object} model.SecurityRule
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/security/rules/{rule_id} [get]
func (h *SecurityHandler) GetRule(c *gin.Context) {
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

// UpdateRule handles PUT /api/v1/orgs/:org_id/security/rules/:rule_id.
//
// @Summary     Update security rule
// @Tags        security
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       rule_id path string true "Security rule ID"
// @Param       request body model.UpdateSecurityRuleRequest true "Update payload"
// @Success     200 {object} model.SecurityRule
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/security/rules/{rule_id} [put]
func (h *SecurityHandler) UpdateRule(c *gin.Context) {
	orgID := c.Param("org_id")
	ruleID := c.Param("rule_id")

	var req model.UpdateSecurityRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
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

// DeleteRule handles DELETE /api/v1/orgs/:org_id/security/rules/:rule_id.
//
// @Summary     Delete security rule
// @Tags        security
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       rule_id path string true "Security rule ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/security/rules/{rule_id} [delete]
func (h *SecurityHandler) DeleteRule(c *gin.Context) {
	orgID := c.Param("org_id")
	ruleID := c.Param("rule_id")

	if err := h.svc.Delete(c.Request.Context(), orgID, ruleID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// ListEvents handles GET /api/v1/orgs/:org_id/security/events.
//
// @Summary     List security events (audit log)
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       limit  query int false "Results per page (default 50, max 200)"
// @Param       offset query int false "Offset for pagination"
// @Success     200 {object} model.SecurityEventResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/security/events [get]
func (h *SecurityHandler) ListEvents(c *gin.Context) {
	orgID := c.Param("org_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	resp, err := h.svc.ListEvents(c.Request.Context(), orgID, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// InvalidateRuleCache handles POST /api/v1/orgs/:org_id/security/rules/invalidate-cache.
//
// @Summary     Invalidate security rules cache
// @Tags        security
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Success     204
// @Router      /orgs/{org_id}/security/rules/invalidate-cache [post]
func (h *SecurityHandler) InvalidateRuleCache(c *gin.Context) {
	orgID := c.Param("org_id")
	h.svc.InvalidateCache(c.Request.Context(), orgID)
	c.Status(http.StatusNoContent)
}
