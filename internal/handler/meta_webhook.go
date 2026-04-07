package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// WhatsAppCallServicer is the optional service interface for persisting WhatsApp call
// events received via the Meta Graph API webhook. It may be nil when the WhatsApp
// calling service is not yet wired (graceful degradation).
type WhatsAppCallServicer interface {
	HandleCallStarted(ctx context.Context, phoneNumberID, callID, from, to, sdpOffer string) (*model.WhatsAppCall, error)
	HandleCallConnected(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error)
	HandleCallEnded(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error)
}

// MetaWebhookHandler handles incoming Meta Graph API webhook requests.
//
// Routes registered:
//
//	GET  /webhooks/meta   — hub.challenge verification
//	POST /webhooks/meta   — event processing
type MetaWebhookHandler struct {
	appSecret   string              // used for HMAC-SHA256 signature verification
	verifyToken string              // validated against hub.verify_token query param
	callSvc     WhatsAppCallServicer // may be nil; events are logged even without it
}

// NewMetaWebhookHandler creates a MetaWebhookHandler.
// If appSecret or verifyToken are empty a warning is logged but no error is returned,
// allowing the server to start without Meta credentials in development.
func NewMetaWebhookHandler(appSecret, verifyToken string, callSvc WhatsAppCallServicer) *MetaWebhookHandler {
	if appSecret == "" {
		slog.Warn("meta webhook: META_APP_SECRET not set — HMAC verification will be skipped")
	}
	if verifyToken == "" {
		slog.Warn("meta webhook: META_WEBHOOK_TOKEN not set — hub verification will be insecure")
	}
	return &MetaWebhookHandler{
		appSecret:   appSecret,
		verifyToken: verifyToken,
		callSvc:     callSvc,
	}
}

// VerifyWebhook handles GET /webhooks/meta.
//
// Meta sends a verification request when you register the webhook URL. It passes
// hub.mode, hub.verify_token, and hub.challenge as query params. This handler
// echoes back hub.challenge if the verify_token matches.
//
// @Summary     Verify Meta Graph API webhook
// @Description Responds to Meta's hub.challenge verification request.
// @Tags        meta-webhooks
// @Produce     plain
// @Param       hub.mode         query string true "Must be 'subscribe'"
// @Param       hub.verify_token query string true "Configured webhook verify token"
// @Param       hub.challenge    query string true "Challenge string to echo back"
// @Success     200 {string} string "The hub.challenge value"
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /webhooks/meta [get]
func (h *MetaWebhookHandler) VerifyWebhook(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode != "subscribe" {
		_ = c.Error(apierror.NewBadRequest("invalid hub.mode: expected 'subscribe'"))
		c.Abort()
		return
	}
	if token != h.verifyToken {
		_ = c.Error(apierror.NewUnauthorized("verify token mismatch"))
		c.Abort()
		return
	}
	if challenge == "" {
		_ = c.Error(apierror.NewBadRequest("hub.challenge is required"))
		c.Abort()
		return
	}

	c.String(http.StatusOK, challenge)
}

// HandleEvent handles POST /webhooks/meta.
//
// Meta sends call events (ringing, answered, ended) to this endpoint. The handler:
//  1. Reads the raw body for HMAC-SHA256 signature verification.
//  2. Verifies the X-Hub-Signature-256 header (skipped when appSecret is empty).
//  3. Parses the MetaWebhookPayload.
//  4. Routes each call event to the WhatsAppCallServicer (if configured).
//
// Always returns 200 OK on success — Meta retries on non-2xx responses.
//
// @Summary     Handle Meta Graph API webhook events
// @Description Processes incoming call events from Meta (ringing, answered, ended).
// @Tags        meta-webhooks
// @Accept      json
// @Produce     json
// @Success     200 {string} string "ok"
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /webhooks/meta [post]
func (h *MetaWebhookHandler) HandleEvent(c *gin.Context) {
	// Read raw body — required for HMAC verification before JSON parsing.
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		_ = c.Error(apierror.NewBadRequest("failed to read request body"))
		c.Abort()
		return
	}

	// Verify HMAC-SHA256 signature when appSecret is configured.
	signature := c.GetHeader("X-Hub-Signature-256")
	if !verifyHMAC(h.appSecret, string(body), signature) {
		_ = c.Error(apierror.NewUnauthorized("invalid webhook signature"))
		c.Abort()
		return
	}

	// Parse the webhook payload.
	var payload model.MetaWebhookPayload
	if err := parseMetaJSON(body, &payload); err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid webhook payload"))
		c.Abort()
		return
	}

	slog.InfoContext(c.Request.Context(), "meta webhook received",
		"object", payload.Object,
		"entries", len(payload.Entry),
	)

	// Route each change event.
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			h.routeChange(c.Request.Context(), change)
		}
	}

	// Always acknowledge with 200 so Meta does not retry.
	c.String(http.StatusOK, "ok")
}

// routeChange dispatches a single MetaWebhookChange to the appropriate handler.
func (h *MetaWebhookHandler) routeChange(ctx context.Context, change model.MetaWebhookChange) {
	phoneNumberID := change.Value.Metadata.PhoneNumberID

	for _, call := range change.Value.Calls {
		slog.InfoContext(ctx, "meta call event",
			"call_id", call.ID,
			"status", call.Status,
			"phone_number_id", phoneNumberID,
		)

		if h.callSvc == nil {
			slog.WarnContext(ctx, "meta webhook: call service not wired — event logged only",
				"call_id", call.ID,
				"status", call.Status,
			)
			continue
		}

		switch call.Status {
		case "ringing":
			// Inbound call — create the WhatsApp call record.
			if _, err := h.callSvc.HandleCallStarted(ctx, phoneNumberID, call.ID, call.From, call.To, call.SDPOffer); err != nil {
				slog.ErrorContext(ctx, "meta webhook: HandleCallStarted failed",
					"call_id", call.ID,
					"error", err,
				)
			}
		case "answered":
			// Call was answered — update the call state.
			if _, err := h.callSvc.HandleCallConnected(ctx, phoneNumberID, call.ID); err != nil {
				slog.ErrorContext(ctx, "meta webhook: HandleCallConnected failed",
					"call_id", call.ID,
					"error", err,
				)
			}
		case "ended":
			// Call ended — mark as ended and clean up any bridge.
			if _, err := h.callSvc.HandleCallEnded(ctx, phoneNumberID, call.ID); err != nil {
				slog.ErrorContext(ctx, "meta webhook: HandleCallEnded failed",
					"call_id", call.ID,
					"error", err,
				)
			}
		default:
			slog.WarnContext(ctx, "meta webhook: unknown call status",
				"call_id", call.ID,
				"status", call.Status,
			)
		}
	}
}

// verifyHMAC verifies the X-Hub-Signature-256 header against the raw body.
// When appSecret is empty, verification is skipped (development mode) and true
// is returned so that the handler can still process events.
func verifyHMAC(appSecret, body string, signature string) bool {
	if appSecret == "" {
		slog.Warn("meta webhook: HMAC verification skipped — META_APP_SECRET not configured")
		return true
	}

	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(body))
	expected := prefix + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// parseMetaJSON unmarshals raw bytes into a value.
func parseMetaJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
