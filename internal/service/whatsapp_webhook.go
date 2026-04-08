package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
)

// Sentinel errors for WhatsApp webhook operations.
// Callers (e.g. HTTP handlers) should use errors.Is to translate these into
// transport-layer responses; the service layer must not import apierror.
var (
	ErrWebhookInvalidMode      = errors.New("invalid hub.mode: expected 'subscribe'")
	ErrWebhookTokenMismatch    = errors.New("verify token mismatch")
	ErrWebhookChallengeMissing = errors.New("hub.challenge is required")
	ErrWebhookCallNotFound     = errors.New("call not found")
	ErrWebhookInternal         = errors.New("internal error")
)

// WhatsAppCallRepository defines the persistence interface for WhatsApp calls.
type WhatsAppCallRepository interface {
	UpsertCall(ctx context.Context, tx pgx.Tx, orgID string, call *model.WhatsAppCall) (*model.WhatsAppCall, error)
	GetCallByID(ctx context.Context, tx pgx.Tx, orgID, id string) (*model.WhatsAppCall, error)
	GetCallByCallID(ctx context.Context, tx pgx.Tx, orgID, callID string) (*model.WhatsAppCall, error)
	UpdateCallState(ctx context.Context, tx pgx.Tx, orgID, callID string, state model.WhatsAppCallState) (*model.WhatsAppCall, error)
	SetSDPAnswer(ctx context.Context, tx pgx.Tx, orgID, callID, sdpAnswer string) (*model.WhatsAppCall, error)
	ListCalls(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.WhatsAppCall, int, error)
	LookupPhoneNumber(ctx context.Context, tx pgx.Tx, phoneNumberID string) (*model.WhatsAppPhoneNumber, error)
}

// WhatsAppWebhookService handles Meta webhook verification, signature validation, and call events.
type WhatsAppWebhookService struct {
	repo        WhatsAppCallRepository
	pool        *pgxpool.Pool
	verifyToken string
	appSecret   string
}

// NewWhatsAppWebhookService creates a new WhatsAppWebhookService.
func NewWhatsAppWebhookService(repo WhatsAppCallRepository, pool *pgxpool.Pool, verifyToken, appSecret string) *WhatsAppWebhookService {
	return &WhatsAppWebhookService{
		repo:        repo,
		pool:        pool,
		verifyToken: verifyToken,
		appSecret:   appSecret,
	}
}

// VerifyWebhook validates Meta's webhook verification challenge.
// Returns the hub.challenge value if the verify_token matches.
func (s *WhatsAppWebhookService) VerifyWebhook(mode, token, challenge string) (string, error) {
	if mode != "subscribe" {
		return "", ErrWebhookInvalidMode
	}
	if token != s.verifyToken {
		return "", ErrWebhookTokenMismatch
	}
	if challenge == "" {
		return "", ErrWebhookChallengeMissing
	}
	return challenge, nil
}

// ValidateSignature verifies the HMAC-SHA256 signature of an incoming webhook payload.
// The signature header format is "sha256=<hex>".
func (s *WhatsAppWebhookService) ValidateSignature(payload []byte, signatureHeader string) bool {
	if s.appSecret == "" {
		// If no app secret configured, skip validation (development mode).
		slog.Warn("WhatsApp webhook signature validation skipped: app secret not configured")
		return true
	}

	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	signatureHex := signatureHeader[len(prefix):]

	mac := hmac.New(sha256.New, []byte(s.appSecret))
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)

	receivedMAC, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}

	return hmac.Equal(expectedMAC, receivedMAC)
}

// HandleCallStarted processes a voice_call_started webhook event.
// It creates or updates the call record with the SDP offer.
func (s *WhatsAppWebhookService) HandleCallStarted(ctx context.Context, phoneNumberID, callID, from, to, sdpOffer string) (*model.WhatsAppCall, error) {
	// Look up the org_id from the registered phone number.
	orgID, err := s.lookupOrgByPhone(ctx, phoneNumberID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	call := &model.WhatsAppCall{
		PhoneNumberID: phoneNumberID,
		CallID:        callID,
		Caller:        from,
		Callee:        to,
		Direction:     model.WhatsAppCallDirectionInbound,
		State:         model.WhatsAppCallStateRinging,
		StartedAt:     &now,
	}
	if sdpOffer != "" {
		call.SDPOffer = &sdpOffer
	}

	var result *model.WhatsAppCall
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.UpsertCall(ctx, tx, orgID, call)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "WhatsAppWebhookService.HandleCallStarted db error", "error", err, "call_id", callID)
		return nil, fmt.Errorf("failed to create call record: %w", ErrWebhookInternal)
	}

	maskedFrom := "***" + from[max(0, len(from)-4):]
	slog.InfoContext(ctx, "WhatsApp call started",
		"call_id", callID,
		"caller", maskedFrom,
		"phone_number_id", phoneNumberID,
		"org_id", orgID,
	)
	return result, nil
}

// HandleCallConnected processes a voice_call_connected webhook event.
func (s *WhatsAppWebhookService) HandleCallConnected(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error) {
	orgID, err := s.lookupOrgByPhone(ctx, phoneNumberID)
	if err != nil {
		return nil, err
	}

	var result *model.WhatsAppCall
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.UpdateCallState(ctx, tx, orgID, callID, model.WhatsAppCallStateConnected)
		return e
	})
	if err != nil {
		if isWhatsAppNotFound(err) {
			return nil, ErrWebhookCallNotFound
		}
		slog.ErrorContext(ctx, "WhatsAppWebhookService.HandleCallConnected db error", "error", err, "call_id", callID)
		return nil, fmt.Errorf("failed to update call state: %w", ErrWebhookInternal)
	}

	slog.InfoContext(ctx, "WhatsApp call connected", "call_id", callID, "org_id", orgID)
	return result, nil
}

// HandleCallEnded processes a voice_call_ended webhook event.
func (s *WhatsAppWebhookService) HandleCallEnded(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error) {
	orgID, err := s.lookupOrgByPhone(ctx, phoneNumberID)
	if err != nil {
		return nil, err
	}

	var result *model.WhatsAppCall
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.UpdateCallState(ctx, tx, orgID, callID, model.WhatsAppCallStateEnded)
		return e
	})
	if err != nil {
		if isWhatsAppNotFound(err) {
			return nil, ErrWebhookCallNotFound
		}
		slog.ErrorContext(ctx, "WhatsAppWebhookService.HandleCallEnded db error", "error", err, "call_id", callID)
		return nil, fmt.Errorf("failed to update call state: %w", ErrWebhookInternal)
	}

	slog.InfoContext(ctx, "WhatsApp call ended", "call_id", callID, "org_id", orgID)
	return result, nil
}

// SetSDPAnswer stores an SDP answer on a call record.
func (s *WhatsAppWebhookService) SetSDPAnswer(ctx context.Context, orgID, callID, sdpAnswer string) (*model.WhatsAppCall, error) {
	var result *model.WhatsAppCall
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.SetSDPAnswer(ctx, tx, orgID, callID, sdpAnswer)
		return e
	})
	if err != nil {
		if isWhatsAppNotFound(err) {
			return nil, ErrWebhookCallNotFound
		}
		slog.ErrorContext(ctx, "WhatsAppWebhookService.SetSDPAnswer db error", "error", err, "call_id", callID)
		return nil, fmt.Errorf("failed to set SDP answer: %w", ErrWebhookInternal)
	}
	return result, nil
}

// GetCall retrieves a call by its internal UUID.
func (s *WhatsAppWebhookService) GetCall(ctx context.Context, orgID, id string) (*model.WhatsAppCall, error) {
	var result *model.WhatsAppCall
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.GetCallByID(ctx, tx, orgID, id)
		return e
	})
	if err != nil {
		if isWhatsAppNotFound(err) {
			return nil, ErrWebhookCallNotFound
		}
		slog.ErrorContext(ctx, "WhatsAppWebhookService.GetCall db error", "error", err)
		return nil, fmt.Errorf("failed to get call: %w", ErrWebhookInternal)
	}
	return result, nil
}

// ListCalls returns org-scoped WhatsApp calls with pagination.
func (s *WhatsAppWebhookService) ListCalls(ctx context.Context, orgID string, limit, offset int) (*model.WhatsAppCallListResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var calls []model.WhatsAppCall
	var total int
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		calls, total, e = s.repo.ListCalls(ctx, tx, orgID, limit, offset)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "WhatsAppWebhookService.ListCalls db error", "error", err)
		return nil, fmt.Errorf("failed to list calls: %w", ErrWebhookInternal)
	}
	if calls == nil {
		calls = []model.WhatsAppCall{}
	}
	return &model.WhatsAppCallListResponse{
		Calls:  calls,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// lookupOrgByPhone finds the org_id associated with a phone_number_id.
// It uses a dedicated pool connection (not within an org-scoped tx) because
// we need to discover the org_id before we can set RLS.
func (s *WhatsAppWebhookService) lookupOrgByPhone(ctx context.Context, phoneNumberID string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("lookupOrgByPhone begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Set admin role to bypass RLS for phone number lookup.
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE raven_admin"); err != nil {
		return "", fmt.Errorf("lookupOrgByPhone set role: %w", err)
	}

	phone, err := s.repo.LookupPhoneNumber(ctx, tx, phoneNumberID)
	if err != nil {
		if isWhatsAppNotFound(err) {
			slog.WarnContext(ctx, "unregistered WhatsApp phone number", "phone_number_id", phoneNumberID)
			return "", ErrWebhookCallNotFound
		}
		return "", fmt.Errorf("lookupOrgByPhone: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("lookupOrgByPhone commit: %w", err)
	}
	return phone.OrgID, nil
}

// isWhatsAppNotFound checks if an error indicates a missing record.
func isWhatsAppNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "no rows in result set") ||
		strings.HasSuffix(msg, "not found")
}
