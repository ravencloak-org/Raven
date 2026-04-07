package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// WhatsAppBridgeServicer is the interface the handler requires from the service layer.
type WhatsAppBridgeServicer interface {
	CreateBridge(ctx context.Context, orgID, callID string, req *model.CreateBridgeRequest) (*model.CreateBridgeResponse, error)
	GetBridge(ctx context.Context, orgID, callID string) (*model.WhatsAppBridge, error)
	TeardownBridge(ctx context.Context, orgID, callID string) (*model.WhatsAppBridge, error)
	ListActiveBridges(ctx context.Context, orgID string) ([]model.WhatsAppBridge, error)
}

// WhatsAppBridgeHandler handles HTTP requests for WhatsApp-LiveKit bridge management.
type WhatsAppBridgeHandler struct {
	svc WhatsAppBridgeServicer
}

// NewWhatsAppBridgeHandler creates a new WhatsAppBridgeHandler.
func NewWhatsAppBridgeHandler(svc WhatsAppBridgeServicer) *WhatsAppBridgeHandler {
	return &WhatsAppBridgeHandler{svc: svc}
}

// CreateBridge handles POST /orgs/:org_id/whatsapp/calls/:call_id/bridge.
//
// @Summary     Bridge WhatsApp call to LiveKit
// @Description Creates a LiveKit room and bridges a WhatsApp WebRTC call into it.
// @Tags        whatsapp-bridge
// @Accept      json
// @Produce     json
// @Param       org_id  path   string                   true "Organisation ID"
// @Param       call_id path   string                   true "WhatsApp Call ID"
// @Param       request body   model.CreateBridgeRequest true "Bridge request with SDP offer"
// @Success     201 {object} model.CreateBridgeResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls/{call_id}/bridge [post]
func (h *WhatsAppBridgeHandler) CreateBridge(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	callID := c.Param("call_id")
	if callID == "" {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  "call_id is required",
		})
		c.Abort()
		return
	}

	var req model.CreateBridgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	resp, err := h.svc.CreateBridge(c.Request.Context(), orgID, callID, &req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// GetBridge handles GET /orgs/:org_id/whatsapp/calls/:call_id/bridge.
//
// @Summary     Get bridge status
// @Description Returns the bridge status for a WhatsApp call.
// @Tags        whatsapp-bridge
// @Produce     json
// @Param       org_id  path string true "Organisation ID"
// @Param       call_id path string true "WhatsApp Call ID"
// @Success     200 {object} model.BridgeStatusResponse
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls/{call_id}/bridge [get]
func (h *WhatsAppBridgeHandler) GetBridge(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	callID := c.Param("call_id")
	bridge, err := h.svc.GetBridge(c.Request.Context(), orgID, callID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, &model.BridgeStatusResponse{Bridge: *bridge})
}

// TeardownBridge handles DELETE /orgs/:org_id/whatsapp/calls/:call_id/bridge.
//
// @Summary     Tear down bridge
// @Description Ends the bridge between a WhatsApp call and LiveKit room.
// @Tags        whatsapp-bridge
// @Produce     json
// @Param       org_id  path string true "Organisation ID"
// @Param       call_id path string true "WhatsApp Call ID"
// @Success     200 {object} model.BridgeStatusResponse
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls/{call_id}/bridge [delete]
func (h *WhatsAppBridgeHandler) TeardownBridge(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	callID := c.Param("call_id")
	bridge, err := h.svc.TeardownBridge(c.Request.Context(), orgID, callID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, &model.BridgeStatusResponse{Bridge: *bridge})
}

// ListActiveBridges handles GET /orgs/:org_id/whatsapp/bridges.
//
// @Summary     List active bridges
// @Description Returns all active WhatsApp-LiveKit bridges for an org.
// @Tags        whatsapp-bridge
// @Produce     json
// @Param       org_id path string true "Organisation ID"
// @Success     200 {array} model.WhatsAppBridge
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/bridges [get]
func (h *WhatsAppBridgeHandler) ListActiveBridges(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	bridges, err := h.svc.ListActiveBridges(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, bridges)
}
