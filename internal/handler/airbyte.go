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

// AirbyteServicer is the interface the handler requires from the service layer.
type AirbyteServicer interface {
	Create(ctx context.Context, orgID, userID string, req model.CreateConnectorRequest) (*model.ConnectorResponse, error)
	GetByID(ctx context.Context, orgID, connectorID string) (*model.ConnectorResponse, error)
	List(ctx context.Context, orgID string, page, pageSize int) (*model.ConnectorListResponse, error)
	Update(ctx context.Context, orgID, connectorID string, req model.UpdateConnectorRequest) (*model.ConnectorResponse, error)
	Delete(ctx context.Context, orgID, connectorID string) error
	TriggerSync(ctx context.Context, orgID, connectorID string) error
	GetSyncHistory(ctx context.Context, orgID, connectorID string, limit int) ([]model.SyncHistoryResponse, error)
}

// AirbyteHandler handles HTTP requests for Airbyte connector management.
type AirbyteHandler struct {
	svc AirbyteServicer
}

// NewAirbyteHandler creates a new AirbyteHandler.
func NewAirbyteHandler(svc AirbyteServicer) *AirbyteHandler {
	return &AirbyteHandler{svc: svc}
}

// Create handles POST /api/v1/orgs/:org_id/connectors.
//
// @Summary     Create Airbyte connector
// @Tags        connectors
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.CreateConnectorRequest true "Connector payload"
// @Success     201 {object} model.ConnectorResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/connectors [post]
func (h *AirbyteHandler) Create(c *gin.Context) {
	orgID := c.Param("org_id")
	userID, _ := c.Get(string(middleware.ContextKeyUserID))
	userIDStr, _ := userID.(string)

	var req model.CreateConnectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	resp, err := h.svc.Create(c.Request.Context(), orgID, userIDStr, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// List handles GET /api/v1/orgs/:org_id/connectors.
//
// @Summary     List Airbyte connectors
// @Tags        connectors
// @Produce     json
// @Security    BearerAuth
// @Param       org_id    path  string true  "Organisation ID"
// @Param       page      query int    false "Page number (default 1)"
// @Param       page_size query int    false "Page size (default 20, max 100)"
// @Success     200 {object} model.ConnectorListResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/connectors [get]
func (h *AirbyteHandler) List(c *gin.Context) {
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

// Get handles GET /api/v1/orgs/:org_id/connectors/:connector_id.
//
// @Summary     Get Airbyte connector by ID
// @Tags        connectors
// @Produce     json
// @Security    BearerAuth
// @Param       org_id       path string true "Organisation ID"
// @Param       connector_id path string true "Connector ID"
// @Success     200 {object} model.ConnectorResponse
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/connectors/{connector_id} [get]
func (h *AirbyteHandler) Get(c *gin.Context) {
	orgID := c.Param("org_id")
	connectorID := c.Param("connector_id")
	resp, err := h.svc.GetByID(c.Request.Context(), orgID, connectorID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Update handles PUT /api/v1/orgs/:org_id/connectors/:connector_id.
//
// @Summary     Update Airbyte connector
// @Tags        connectors
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id       path string true "Organisation ID"
// @Param       connector_id path string true "Connector ID"
// @Param       request      body model.UpdateConnectorRequest true "Connector update payload"
// @Success     200 {object} model.ConnectorResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/connectors/{connector_id} [put]
func (h *AirbyteHandler) Update(c *gin.Context) {
	orgID := c.Param("org_id")
	connectorID := c.Param("connector_id")

	var req model.UpdateConnectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	resp, err := h.svc.Update(c.Request.Context(), orgID, connectorID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Delete handles DELETE /api/v1/orgs/:org_id/connectors/:connector_id.
//
// @Summary     Delete Airbyte connector
// @Tags        connectors
// @Security    BearerAuth
// @Param       org_id       path string true "Organisation ID"
// @Param       connector_id path string true "Connector ID"
// @Success     204
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/connectors/{connector_id} [delete]
func (h *AirbyteHandler) Delete(c *gin.Context) {
	orgID := c.Param("org_id")
	connectorID := c.Param("connector_id")
	if err := h.svc.Delete(c.Request.Context(), orgID, connectorID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// TriggerSync handles POST /api/v1/orgs/:org_id/connectors/:connector_id/sync.
//
// @Summary     Trigger Airbyte connector sync
// @Tags        connectors
// @Security    BearerAuth
// @Param       org_id       path string true "Organisation ID"
// @Param       connector_id path string true "Connector ID"
// @Success     202
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/connectors/{connector_id}/sync [post]
func (h *AirbyteHandler) TriggerSync(c *gin.Context) {
	orgID := c.Param("org_id")
	connectorID := c.Param("connector_id")
	if err := h.svc.TriggerSync(c.Request.Context(), orgID, connectorID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "sync job enqueued"})
}

// GetSyncHistory handles GET /api/v1/orgs/:org_id/connectors/:connector_id/history.
//
// @Summary     Get Airbyte connector sync history
// @Tags        connectors
// @Produce     json
// @Security    BearerAuth
// @Param       org_id       path  string true  "Organisation ID"
// @Param       connector_id path  string true  "Connector ID"
// @Param       limit        query int    false "Max records to return (default 20, max 100)"
// @Success     200 {array} model.SyncHistoryResponse
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/connectors/{connector_id}/history [get]
func (h *AirbyteHandler) GetSyncHistory(c *gin.Context) {
	orgID := c.Param("org_id")
	connectorID := c.Param("connector_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	history, err := h.svc.GetSyncHistory(c.Request.Context(), orgID, connectorID, limit)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, history)
}
