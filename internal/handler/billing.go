package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// BillingServicer is the interface the handler requires from the billing service layer.
type BillingServicer interface {
	GetPlans() []model.Plan
	CreateSubscription(ctx context.Context, orgID string, req model.CreateSubscriptionRequest) (*model.Subscription, error)
	CancelSubscription(ctx context.Context, orgID string, subscriptionID string) error
	CreatePaymentIntent(ctx context.Context, orgID string, req model.CreatePaymentIntentRequest) (*model.PaymentIntent, error)
	VerifyWebhookSignature(payload []byte, signature string) error
	HandleWebhook(ctx context.Context, event model.HyperswitchWebhookPayload) error
}

// BillingHandler handles HTTP requests for billing and subscription management.
type BillingHandler struct {
	svc BillingServicer
}

// NewBillingHandler creates a new BillingHandler.
func NewBillingHandler(svc BillingServicer) *BillingHandler {
	return &BillingHandler{svc: svc}
}

// GetPlans handles GET /api/v1/billing/plans.
//
// @Summary     List billing plans
// @Tags        billing
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} model.Plan
// @Router      /billing/plans [get]
func (h *BillingHandler) GetPlans(c *gin.Context) {
	plans := h.svc.GetPlans()
	c.JSON(http.StatusOK, plans)
}

// Subscribe handles POST /api/v1/billing/subscriptions.
//
// @Summary     Create subscription
// @Tags        billing
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body model.CreateSubscriptionRequest true "Subscription payload"
// @Success     201 {object} model.Subscription
// @Failure     422 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /billing/subscriptions [post]
func (h *BillingHandler) Subscribe(c *gin.Context) {
	orgID, exists := c.Get(string(middleware.ContextKeyOrgID))
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized",
			Detail:  "missing organisation context",
		})
		return
	}

	var req model.CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	sub, err := h.svc.CreateSubscription(c.Request.Context(), orgID.(string), req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, sub)
}

// Unsubscribe handles DELETE /api/v1/billing/subscriptions/:id.
//
// @Summary     Cancel subscription
// @Tags        billing
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Subscription ID to cancel"
// @Success     204
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /billing/subscriptions/{id} [delete]
func (h *BillingHandler) Unsubscribe(c *gin.Context) {
	orgID, exists := c.Get(string(middleware.ContextKeyOrgID))
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized",
			Detail:  "missing organisation context",
		})
		return
	}

	subscriptionID := c.Param("id")
	if subscriptionID == "" {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  "subscription id path parameter is required",
		})
		return
	}

	if err := h.svc.CancelSubscription(c.Request.Context(), orgID.(string), subscriptionID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// CreatePaymentIntent handles POST /api/v1/billing/payment-intents.
//
// @Summary     Create payment intent
// @Tags        billing
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body model.CreatePaymentIntentRequest true "Payment intent payload"
// @Success     201 {object} model.PaymentIntent
// @Failure     422 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /billing/payment-intents [post]
func (h *BillingHandler) CreatePaymentIntent(c *gin.Context) {
	orgID, exists := c.Get(string(middleware.ContextKeyOrgID))
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized",
			Detail:  "missing organisation context",
		})
		return
	}

	var req model.CreatePaymentIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	pi, err := h.svc.CreatePaymentIntent(c.Request.Context(), orgID.(string), req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, pi)
}

// Webhook handles POST /api/v1/billing/webhook.
// This endpoint is called by Hyperswitch to notify about payment events.
// It does NOT require JWT authentication; it uses webhook signature verification.
//
// @Summary     Hyperswitch webhook
// @Tags        billing
// @Accept      json
// @Produce     json
// @Param       X-Webhook-Signature header string true "HMAC-SHA256 signature"
// @Success     200 {object} map[string]string
// @Failure     401 {object} apierror.AppError
// @Failure     400 {object} apierror.AppError
// @Router      /billing/webhook [post]
func (h *BillingHandler) Webhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  "failed to read request body",
		})
		return
	}

	signature := c.GetHeader("X-Webhook-Signature")
	if err := h.svc.VerifyWebhookSignature(payload, signature); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	// Since we already consumed the body for signature verification,
	// unmarshal from the raw payload bytes directly.
	var event model.HyperswitchWebhookPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  "invalid webhook payload: " + err.Error(),
		})
		return
	}

	if err := h.svc.HandleWebhook(c.Request.Context(), event); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
