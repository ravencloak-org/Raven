package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// WhatsAppWebhookServicer is the interface the handler requires from the service layer.
type WhatsAppWebhookServicer interface {
	VerifyWebhook(mode, token, challenge string) (string, error)
	ValidateSignature(payload []byte, signatureHeader string) bool
	HandleCallStarted(ctx context.Context, phoneNumberID, callID, from, to, sdpOffer string) (*model.WhatsAppCall, error)
	HandleCallConnected(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error)
	HandleCallEnded(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error)
	SetSDPAnswer(ctx context.Context, orgID, callID, sdpAnswer string) (*model.WhatsAppCall, error)
	GetCall(ctx context.Context, orgID, id string) (*model.WhatsAppCall, error)
	ListCalls(ctx context.Context, orgID string, limit, offset int) (*model.WhatsAppCallListResponse, error)
}

// WhatsAppWebhookHandler handles incoming Meta webhook requests for WhatsApp calling.
type WhatsAppWebhookHandler struct {
	svc WhatsAppWebhookServicer
}

// NewWhatsAppWebhookHandler creates a new WhatsAppWebhookHandler.
func NewWhatsAppWebhookHandler(svc WhatsAppWebhookServicer) *WhatsAppWebhookHandler {
	return &WhatsAppWebhookHandler{svc: svc}
}

// Verify handles GET /webhooks/whatsapp — Meta's webhook verification challenge.
//
// @Summary     Verify WhatsApp webhook
// @Description Responds to Meta's webhook verification challenge during setup.
// @Tags        whatsapp-webhooks
// @Produce     plain
// @Param       hub.mode         query string true "Must be 'subscribe'"
// @Param       hub.verify_token query string true "Verification token"
// @Param       hub.challenge    query string true "Challenge string to echo back"
// @Success     200 {string} string "The hub.challenge value"
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /webhooks/whatsapp [get]
func (h *WhatsAppWebhookHandler) Verify(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	result, err := h.svc.VerifyWebhook(mode, token, challenge)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	c.String(http.StatusOK, result)
}

// webhookPayload represents the top-level structure of a Meta webhook notification.
type webhookPayload struct {
	Object string         `json:"object"`
	Entry  []webhookEntry `json:"entry"`
}

// webhookEntry represents a single entry in the Meta webhook payload.
type webhookEntry struct {
	ID      string           `json:"id"`
	Changes []webhookChange  `json:"changes"`
}

// webhookChange represents a change object within a webhook entry.
type webhookChange struct {
	Field string          `json:"field"`
	Value webhookValue    `json:"value"`
}

// webhookValue contains the actual event data.
type webhookValue struct {
	MessagingProduct string                 `json:"messaging_product"`
	Metadata         webhookMetadata        `json:"metadata,omitempty"`
	Calls            []webhookCallEvent     `json:"calls,omitempty"`
}

// webhookMetadata contains metadata about the WhatsApp Business Account.
type webhookMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

// webhookCallEvent represents a single call event in the webhook.
type webhookCallEvent struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	SDPOffer  string `json:"sdp_offer,omitempty"`
}

// Receive handles POST /webhooks/whatsapp — incoming Meta webhook events.
//
// @Summary     Receive WhatsApp webhook events
// @Description Processes incoming webhook events from Meta (call events, etc).
// @Tags        whatsapp-webhooks
// @Accept      json
// @Produce     json
// @Success     200 {string} string "ok"
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /webhooks/whatsapp [post]
func (h *WhatsAppWebhookHandler) Receive(c *gin.Context) {
	// Read the raw body for signature verification.
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		_ = c.Error(apierror.NewBadRequest("failed to read request body"))
		c.Abort()
		return
	}

	// Validate HMAC-SHA256 signature.
	signature := c.GetHeader("X-Hub-Signature-256")
	if !h.svc.ValidateSignature(body, signature) {
		_ = c.Error(apierror.NewUnauthorized("invalid webhook signature"))
		c.Abort()
		return
	}

	// Parse the webhook payload from the raw bytes (body was already consumed above).
	var payload webhookPayload
	if err := parseJSON(body, &payload); err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid webhook payload"))
		c.Abort()
		return
	}

	// Process each entry.
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			h.processChange(c.Request.Context(), change)
		}
	}

	// Always respond with 200 to acknowledge receipt — Meta retries on non-2xx.
	c.String(http.StatusOK, "ok")
}

// processChange routes a single change to the appropriate call event handler.
func (h *WhatsAppWebhookHandler) processChange(ctx context.Context, change webhookChange) {
	phoneNumberID := change.Value.Metadata.PhoneNumberID

	for _, callEvent := range change.Value.Calls {
		switch callEvent.Status {
		case "ringing", "voice_call_started":
			if _, err := h.svc.HandleCallStarted(ctx, phoneNumberID, callEvent.ID, callEvent.From, callEvent.To, callEvent.SDPOffer); err != nil {
				slog.ErrorContext(ctx, "failed to handle call started",
					"call_id", callEvent.ID,
					"error", err,
				)
			}
		case "connected", "voice_call_connected":
			if _, err := h.svc.HandleCallConnected(ctx, phoneNumberID, callEvent.ID); err != nil {
				slog.ErrorContext(ctx, "failed to handle call connected",
					"call_id", callEvent.ID,
					"error", err,
				)
			}
		case "ended", "voice_call_ended":
			if _, err := h.svc.HandleCallEnded(ctx, phoneNumberID, callEvent.ID); err != nil {
				slog.ErrorContext(ctx, "failed to handle call ended",
					"call_id", callEvent.ID,
					"error", err,
				)
			}
		default:
			slog.WarnContext(ctx, "unknown WhatsApp call event status",
				"status", callEvent.Status,
				"call_id", callEvent.ID,
			)
		}
	}
}

// GetCall handles GET /orgs/:org_id/whatsapp/calls/:call_id.
//
// @Summary     Get WhatsApp call
// @Description Returns a WhatsApp call record by ID.
// @Tags        whatsapp-calls
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       call_id path string true "Call ID (internal UUID)"
// @Success     200 {object} model.WhatsAppCallResponse
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls/{call_id} [get]
func (h *WhatsAppWebhookHandler) GetCall(c *gin.Context) {
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
	c.JSON(http.StatusOK, model.WhatsAppCallResponse{Call: *call})
}

// ListCalls handles GET /orgs/:org_id/whatsapp/calls.
//
// @Summary     List WhatsApp calls
// @Description Returns org-scoped WhatsApp calls with pagination.
// @Tags        whatsapp-calls
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path  string true  "Organisation ID"
// @Param       limit  query int    false "Number of calls (default 20)"
// @Param       offset query int    false "Offset (default 0)"
// @Success     200 {object} model.WhatsAppCallListResponse
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls [get]
func (h *WhatsAppWebhookHandler) ListCalls(c *gin.Context) {
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

// SendSDPAnswer handles POST /orgs/:org_id/whatsapp/calls/:call_id/answer.
//
// @Summary     Send SDP answer for a WhatsApp call
// @Description Stores the SDP answer and (in a future version) sends it to Meta.
// @Tags        whatsapp-calls
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       call_id path string true "Meta's external call ID"
// @Param       request body model.SendSDPAnswerRequest true "SDP answer payload"
// @Success     200 {object} model.WhatsAppCallResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/whatsapp/calls/{call_id}/answer [post]
func (h *WhatsAppWebhookHandler) SendSDPAnswer(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}
	callID := c.Param("call_id")

	var req struct {
		SDPAnswer string `json:"sdp_answer" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	call, err := h.svc.SetSDPAnswer(c.Request.Context(), orgID, callID, req.SDPAnswer)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, call)
}

// parseJSON is a helper to parse JSON from raw bytes.
func parseJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
