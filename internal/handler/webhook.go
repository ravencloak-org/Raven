package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// supportedWebhookEvents is the set of valid event names accepted for webhook subscriptions.
var supportedWebhookEvents = map[string]struct{}{
	string(model.WebhookEventLeadGenerated):         {},
	string(model.WebhookEventConversationEscalated): {},
	string(model.WebhookEventDocumentProcessed):     {},
	string(model.WebhookEventSyncCompleted):         {},
}

// maxAllowedRetries is the upper bound for max_retries to prevent abuse.
const maxAllowedRetries = 20

// validateUUID checks that s is a valid UUID and returns an error string if not.
func validateUUID(s, name string) error {
	if _, err := uuid.Parse(s); err != nil {
		return fmt.Errorf("%s is not a valid UUID", name)
	}
	return nil
}

// validateEvents checks that all events in the slice are supported.
func validateEvents(events []string) error {
	for _, e := range events {
		if _, ok := supportedWebhookEvents[e]; !ok {
			return fmt.Errorf("unsupported event type: %s", e)
		}
	}
	return nil
}

// validateMaxRetries checks that max_retries is within acceptable bounds.
func validateMaxRetries(v *int) error {
	if v != nil && (*v < 0 || *v > maxAllowedRetries) {
		return fmt.Errorf("max_retries must be between 0 and %d", maxAllowedRetries)
	}
	return nil
}

// WebhookServicer is the interface the handler requires from the service layer.
type WebhookServicer interface {
	Create(ctx context.Context, orgID, userID string, req model.CreateWebhookRequest) (*model.WebhookConfig, error)
	GetByID(ctx context.Context, orgID, id string) (*model.WebhookConfig, error)
	List(ctx context.Context, orgID string) ([]model.WebhookConfig, error)
	Update(ctx context.Context, orgID, id string, req model.UpdateWebhookRequest) (*model.WebhookConfig, error)
	Delete(ctx context.Context, orgID, id string) error
	ListDeliveries(ctx context.Context, orgID, webhookID string, limit int) ([]model.WebhookDelivery, error)
}

// WebhookHandler handles HTTP requests for webhook management.
type WebhookHandler struct {
	svc WebhookServicer
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(svc WebhookServicer) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

// Create handles POST /orgs/:org_id/webhooks.
//
// @Summary     Create webhook
// @Tags        webhooks
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.CreateWebhookRequest true "Webhook payload"
// @Success     201 {object} model.WebhookConfig
// @Failure     400 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/webhooks [post]
func (h *WebhookHandler) Create(c *gin.Context) {
	orgID := c.Param("org_id")
	if err := validateUUID(orgID, "org_id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}

	userIDVal, exists := c.Get(string(middleware.ContextKeyUserID))
	if !exists {
		_ = c.Error(apierror.NewUnauthorized("user authentication required"))
		c.Abort()
		return
	}
	userID, _ := userIDVal.(string)

	var req model.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	if err := validateEvents(req.Events); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}
	if err := validateMaxRetries(req.MaxRetries); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}

	hook, err := h.svc.Create(c.Request.Context(), orgID, userID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, hook)
}

// List handles GET /orgs/:org_id/webhooks.
//
// @Summary     List webhooks
// @Tags        webhooks
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Success     200 {array} model.WebhookConfig
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/webhooks [get]
func (h *WebhookHandler) List(c *gin.Context) {
	orgID := c.Param("org_id")
	if err := validateUUID(orgID, "org_id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}

	hooks, err := h.svc.List(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	if hooks == nil {
		hooks = []model.WebhookConfig{}
	}
	c.JSON(http.StatusOK, hooks)
}

// Get handles GET /orgs/:org_id/webhooks/:id.
//
// @Summary     Get webhook
// @Tags        webhooks
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       id     path string true "Webhook ID"
// @Success     200 {object} model.WebhookConfig
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/webhooks/{id} [get]
func (h *WebhookHandler) Get(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")
	if err := validateUUID(orgID, "org_id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}
	if err := validateUUID(id, "id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}

	hook, err := h.svc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, hook)
}

// Update handles PUT /orgs/:org_id/webhooks/:id.
//
// @Summary     Update webhook
// @Tags        webhooks
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       id      path string true "Webhook ID"
// @Param       request body model.UpdateWebhookRequest true "Update payload"
// @Success     200 {object} model.WebhookConfig
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/webhooks/{id} [put]
func (h *WebhookHandler) Update(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")
	if err := validateUUID(orgID, "org_id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}
	if err := validateUUID(id, "id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}

	var req model.UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	if len(req.Events) > 0 {
		if err := validateEvents(req.Events); err != nil {
			_ = c.Error(apierror.NewBadRequest(err.Error()))
			c.Abort()
			return
		}
	}
	if err := validateMaxRetries(req.MaxRetries); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}

	hook, err := h.svc.Update(c.Request.Context(), orgID, id, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, hook)
}

// Delete handles DELETE /orgs/:org_id/webhooks/:id.
//
// @Summary     Delete webhook
// @Tags        webhooks
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       id     path string true "Webhook ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/webhooks/{id} [delete]
func (h *WebhookHandler) Delete(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")
	if err := validateUUID(orgID, "org_id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}
	if err := validateUUID(id, "id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}

	if err := h.svc.Delete(c.Request.Context(), orgID, id); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// ListDeliveries handles GET /orgs/:org_id/webhooks/:id/deliveries.
//
// @Summary     List webhook deliveries
// @Tags        webhooks
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       id     path string true "Webhook ID"
// @Success     200 {array} model.WebhookDelivery
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/webhooks/{id}/deliveries [get]
func (h *WebhookHandler) ListDeliveries(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")
	if err := validateUUID(orgID, "org_id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}
	if err := validateUUID(id, "id"); err != nil {
		_ = c.Error(apierror.NewBadRequest(err.Error()))
		c.Abort()
		return
	}

	deliveries, err := h.svc.ListDeliveries(c.Request.Context(), orgID, id, 50)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, deliveries)
}
