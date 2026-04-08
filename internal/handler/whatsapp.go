package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// WhatsAppServicer is the interface the handler requires from the service layer.
type WhatsAppServicer interface {
	CreatePhoneNumber(ctx context.Context, orgID string, req *model.CreateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error)
	GetPhoneNumber(ctx context.Context, orgID, phoneID string) (*model.WhatsAppPhoneNumber, error)
	UpdatePhoneNumber(ctx context.Context, orgID, phoneID string, req *model.UpdateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error)
	DeletePhoneNumber(ctx context.Context, orgID, phoneID string) error
	ListPhoneNumbers(ctx context.Context, orgID string, limit, offset int) (*model.WhatsAppPhoneNumberListResponse, error)

	InitiateCall(ctx context.Context, orgID string, req *model.InitiateWhatsAppCallRequest) (*model.WhatsAppCall, error)
	GetCall(ctx context.Context, orgID, callID string) (*model.WhatsAppCall, error)
	UpdateCallState(ctx context.Context, orgID, callID string, state model.WhatsAppCallState) (*model.WhatsAppCall, error)
	ListCalls(ctx context.Context, orgID string, limit, offset int) (*model.WhatsAppCallListResponse, error)
}

// WhatsAppHandler handles HTTP requests for WhatsApp phone number and call management.
type WhatsAppHandler struct {
	svc WhatsAppServicer
}

// NewWhatsAppHandler creates a new WhatsAppHandler.
func NewWhatsAppHandler(svc WhatsAppServicer) *WhatsAppHandler {
	return &WhatsAppHandler{svc: svc}
}

// --- Phone number endpoints ---

// CreatePhoneNumber handles POST /v1/orgs/:org_id/whatsapp/phone-numbers.
//
// @Summary     Register WhatsApp phone number
// @Description Registers a new WhatsApp Business phone number for the organisation.
// @Tags        whatsapp
// @Accept      json
// @Produce     json
// @Param       org_id  path   string                                  true "Organisation ID"
// @Param       request body   model.CreateWhatsAppPhoneNumberRequest  true "Phone number details"
// @Success     201 {object} model.WhatsAppPhoneNumber
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     409 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/phone-numbers [post]
func (h *WhatsAppHandler) CreatePhoneNumber(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	var req model.CreateWhatsAppPhoneNumberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	phone, err := h.svc.CreatePhoneNumber(c.Request.Context(), orgID, &req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, phone)
}

// GetPhoneNumber handles GET /v1/orgs/:org_id/whatsapp/phone-numbers/:phone_id.
//
// @Summary     Get WhatsApp phone number
// @Description Returns a WhatsApp phone number by ID.
// @Tags        whatsapp
// @Produce     json
// @Param       org_id   path string true "Organisation ID"
// @Param       phone_id path string true "Phone number ID"
// @Success     200 {object} model.WhatsAppPhoneNumber
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/phone-numbers/{phone_id} [get]
func (h *WhatsAppHandler) GetPhoneNumber(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	phoneID := c.Param("phone_id")
	phone, err := h.svc.GetPhoneNumber(c.Request.Context(), orgID, phoneID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, phone)
}

// UpdatePhoneNumber handles PUT /v1/orgs/:org_id/whatsapp/phone-numbers/:phone_id.
//
// @Summary     Update WhatsApp phone number
// @Description Updates display name or verified status of a phone number.
// @Tags        whatsapp
// @Accept      json
// @Produce     json
// @Param       org_id   path   string                                  true "Organisation ID"
// @Param       phone_id path   string                                  true "Phone number ID"
// @Param       request  body   model.UpdateWhatsAppPhoneNumberRequest  true "Update fields"
// @Success     200 {object} model.WhatsAppPhoneNumber
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/phone-numbers/{phone_id} [put]
func (h *WhatsAppHandler) UpdatePhoneNumber(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	phoneID := c.Param("phone_id")

	var req model.UpdateWhatsAppPhoneNumberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	phone, err := h.svc.UpdatePhoneNumber(c.Request.Context(), orgID, phoneID, &req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, phone)
}

// DeletePhoneNumber handles DELETE /v1/orgs/:org_id/whatsapp/phone-numbers/:phone_id.
//
// @Summary     Delete WhatsApp phone number
// @Description Removes a WhatsApp phone number from the organisation.
// @Tags        whatsapp
// @Produce     json
// @Param       org_id   path string true "Organisation ID"
// @Param       phone_id path string true "Phone number ID"
// @Success     204
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/phone-numbers/{phone_id} [delete]
func (h *WhatsAppHandler) DeletePhoneNumber(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	phoneID := c.Param("phone_id")
	if err := h.svc.DeletePhoneNumber(c.Request.Context(), orgID, phoneID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// ListPhoneNumbers handles GET /v1/orgs/:org_id/whatsapp/phone-numbers.
//
// @Summary     List WhatsApp phone numbers
// @Description Returns org-scoped WhatsApp phone numbers with pagination.
// @Tags        whatsapp
// @Produce     json
// @Param       org_id path  string true  "Organisation ID"
// @Param       limit  query int    false "Number of results (default 20)"
// @Param       offset query int    false "Offset (default 0)"
// @Success     200 {object} model.WhatsAppPhoneNumberListResponse
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/phone-numbers [get]
func (h *WhatsAppHandler) ListPhoneNumbers(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	resp, err := h.svc.ListPhoneNumbers(c.Request.Context(), orgID, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// --- Call endpoints ---

// InitiateCall handles POST /v1/orgs/:org_id/whatsapp/calls.
//
// @Summary     Initiate WhatsApp call
// @Description Creates a new outbound WhatsApp call via the Business Calling API.
// @Tags        whatsapp
// @Accept      json
// @Produce     json
// @Param       org_id  path   string                             true "Organisation ID"
// @Param       request body   model.InitiateWhatsAppCallRequest  true "Call details"
// @Success     201 {object} model.WhatsAppCall
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls [post]
func (h *WhatsAppHandler) InitiateCall(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	var req model.InitiateWhatsAppCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	call, err := h.svc.InitiateCall(c.Request.Context(), orgID, &req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, call)
}

// GetCall handles GET /v1/orgs/:org_id/whatsapp/calls/:call_id.
//
// @Summary     Get WhatsApp call
// @Description Returns a WhatsApp call by ID.
// @Tags        whatsapp
// @Produce     json
// @Param       org_id  path string true "Organisation ID"
// @Param       call_id path string true "Call ID"
// @Success     200 {object} model.WhatsAppCall
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls/{call_id} [get]
func (h *WhatsAppHandler) GetCall(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	callID := c.Param("call_id")
	call, err := h.svc.GetCall(c.Request.Context(), orgID, callID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, call)
}

// UpdateCallState handles PATCH /v1/orgs/:org_id/whatsapp/calls/:call_id.
//
// @Summary     Update WhatsApp call state
// @Description Transitions a WhatsApp call to 'connected' or 'ended'.
// @Tags        whatsapp
// @Accept      json
// @Produce     json
// @Param       org_id  path   string                              true "Organisation ID"
// @Param       call_id path   string                              true "Call ID"
// @Param       request body   model.UpdateWhatsAppCallStateRequest true "State transition"
// @Success     200 {object} model.WhatsAppCall
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls/{call_id} [patch]
func (h *WhatsAppHandler) UpdateCallState(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	callID := c.Param("call_id")

	var req model.UpdateWhatsAppCallStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	call, err := h.svc.UpdateCallState(c.Request.Context(), orgID, callID, req.State)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, call)
}

// ListCalls handles GET /v1/orgs/:org_id/whatsapp/calls.
//
// @Summary     List WhatsApp calls
// @Description Returns org-scoped WhatsApp calls with pagination.
// @Tags        whatsapp
// @Produce     json
// @Param       org_id path  string true  "Organisation ID"
// @Param       limit  query int    false "Number of results (default 20)"
// @Param       offset query int    false "Offset (default 0)"
// @Success     200 {object} model.WhatsAppCallListResponse
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls [get]
func (h *WhatsAppHandler) ListCalls(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	resp, err := h.svc.ListCalls(c.Request.Context(), orgID, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}
