package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// NotificationServicer is the interface the handler requires from the service layer.
type NotificationServicer interface {
	CreateConfig(ctx context.Context, orgID string, req model.CreateNotificationConfigRequest) (*model.NotificationConfig, error)
	GetConfig(ctx context.Context, orgID, id string) (*model.NotificationConfig, error)
	ListConfigs(ctx context.Context, orgID string) ([]model.NotificationConfig, error)
	UpdateConfig(ctx context.Context, orgID, id string, req model.UpdateNotificationConfigRequest) (*model.NotificationConfig, error)
	DeleteConfig(ctx context.Context, orgID, id string) error
	ListLogs(ctx context.Context, orgID string, limit int) ([]model.NotificationLog, error)
}

// NotificationHandler handles HTTP requests for notification config management.
type NotificationHandler struct {
	svc NotificationServicer
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(svc NotificationServicer) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// CreateConfig handles POST /api/v1/orgs/:org_id/notifications/configs.
//
// @Summary     Create notification config
// @Tags        notifications
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.CreateNotificationConfigRequest true "Notification config payload"
// @Success     201 {object} model.NotificationConfig
// @Failure     400 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/notifications/configs [post]
func (h *NotificationHandler) CreateConfig(c *gin.Context) {
	orgID := c.Param("org_id")

	var req model.CreateNotificationConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	cfg, err := h.svc.CreateConfig(c.Request.Context(), orgID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, cfg)
}

// ListConfigs handles GET /api/v1/orgs/:org_id/notifications/configs.
//
// @Summary     List notification configs
// @Tags        notifications
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Success     200 {array} model.NotificationConfig
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/notifications/configs [get]
func (h *NotificationHandler) ListConfigs(c *gin.Context) {
	orgID := c.Param("org_id")

	configs, err := h.svc.ListConfigs(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, configs)
}

// UpdateConfig handles PUT /api/v1/orgs/:org_id/notifications/configs/:id.
//
// @Summary     Update notification config
// @Tags        notifications
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       id      path string true "Config ID"
// @Param       request body model.UpdateNotificationConfigRequest true "Update payload"
// @Success     200 {object} model.NotificationConfig
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/notifications/configs/{id} [put]
func (h *NotificationHandler) UpdateConfig(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  "id must be a valid UUID",
		})
		return
	}

	var req model.UpdateNotificationConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	cfg, err := h.svc.UpdateConfig(c.Request.Context(), orgID, id, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// DeleteConfig handles DELETE /api/v1/orgs/:org_id/notifications/configs/:id.
//
// @Summary     Delete notification config
// @Tags        notifications
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       id      path string true "Config ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/notifications/configs/{id} [delete]
func (h *NotificationHandler) DeleteConfig(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  "id must be a valid UUID",
		})
		return
	}

	if err := h.svc.DeleteConfig(c.Request.Context(), orgID, id); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// ListLogs handles GET /api/v1/orgs/:org_id/notifications/logs.
//
// @Summary     List recent notification delivery logs
// @Tags        notifications
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Success     200 {array} model.NotificationLog
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/notifications/logs [get]
func (h *NotificationHandler) ListLogs(c *gin.Context) {
	orgID := c.Param("org_id")

	logs, err := h.svc.ListLogs(c.Request.Context(), orgID, 50)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, logs)
}
