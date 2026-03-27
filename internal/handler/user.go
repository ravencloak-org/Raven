package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// UserServicer is the interface the handler requires from the service layer.
type UserServicer interface {
	GetMe(ctx context.Context, keycloakSub string) (*model.User, error)
	UpdateMe(ctx context.Context, userID string, req model.UpdateUserRequest) (*model.User, error)
	GetByID(ctx context.Context, userID string) (*model.User, error)
	HandleKeycloakEvent(ctx context.Context, event model.KeycloakWebhookEvent) error
	DeleteMe(ctx context.Context, userID string) error
}

// UserHandler handles HTTP requests for user management.
type UserHandler struct {
	svc UserServicer
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(svc UserServicer) *UserHandler {
	return &UserHandler{svc: svc}
}

// GetMe handles GET /api/v1/me.
//
// @Summary     Get current user profile
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} model.User
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /me [get]
func (h *UserHandler) GetMe(c *gin.Context) {
	sub, _ := c.Get(string(middleware.ContextKeyUserID))
	subStr, _ := sub.(string)
	if subStr == "" {
		_ = c.Error(apierror.NewUnauthorized("missing user identity"))
		c.Abort()
		return
	}
	user, err := h.svc.GetMe(c.Request.Context(), subStr)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateMe handles PUT /api/v1/me.
//
// @Summary     Update current user profile
// @Tags        users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body model.UpdateUserRequest true "Update payload"
// @Success     200 {object} model.User
// @Failure     422 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /me [put]
func (h *UserHandler) UpdateMe(c *gin.Context) {
	userID, _ := c.Get(string(middleware.ContextKeyUserID))
	userIDStr, _ := userID.(string)

	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	user, err := h.svc.UpdateMe(c.Request.Context(), userIDStr, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, user)
}

// DeleteMe handles DELETE /api/v1/me (GDPR right to erasure).
//
// @Summary     Delete current user account (GDPR)
// @Tags        users
// @Security    BearerAuth
// @Success     204
// @Failure     401 {object} apierror.AppError
// @Router      /me [delete]
func (h *UserHandler) DeleteMe(c *gin.Context) {
	userID, _ := c.Get(string(middleware.ContextKeyUserID))
	userIDStr, _ := userID.(string)
	if err := h.svc.DeleteMe(c.Request.Context(), userIDStr); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// GetUser handles GET /api/v1/users/:user_id (admin only).
//
// @Summary     Get user by ID (org_admin only)
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Param       user_id path string true "User ID"
// @Success     200 {object} model.User
// @Failure     404 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /users/{user_id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("user_id")
	user, err := h.svc.GetByID(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, user)
}

// KeycloakWebhook handles POST /api/v1/internal/keycloak-webhook.
// This endpoint is internal-only and must only be reachable from the
// compose-network (not externally). Enforce via network policy / firewall,
// NOT by authentication header — the SPI does not send a Bearer token.
//
// NOTE: Live integration test deferred until Keycloak SPI emits events in the
// local environment. Unit tests use mocked payloads (see user_test.go).
func (h *UserHandler) KeycloakWebhook(c *gin.Context) {
	var event model.KeycloakWebhookEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  "invalid webhook payload: " + err.Error(),
		})
		c.Abort()
		return
	}
	if err := h.svc.HandleKeycloakEvent(c.Request.Context(), event); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
