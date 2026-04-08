package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// WhatsAppRepository defines the persistence interface the WhatsAppService requires.
type WhatsAppRepository interface {
	CreatePhoneNumber(ctx context.Context, tx pgx.Tx, orgID string, req *model.CreateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error)
	GetPhoneNumber(ctx context.Context, tx pgx.Tx, orgID, phoneID string) (*model.WhatsAppPhoneNumber, error)
	UpdatePhoneNumber(ctx context.Context, tx pgx.Tx, orgID, phoneID string, req *model.UpdateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error)
	DeletePhoneNumber(ctx context.Context, tx pgx.Tx, orgID, phoneID string) error
	ListPhoneNumbers(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.WhatsAppPhoneNumber, int, error)

	CreateCall(ctx context.Context, tx pgx.Tx, orgID string, call *model.WhatsAppCall) (*model.WhatsAppCall, error)
	GetCall(ctx context.Context, tx pgx.Tx, orgID, callID string) (*model.WhatsAppCall, error)
	GetCallByCallID(ctx context.Context, tx pgx.Tx, orgID, metaCallID string) (*model.WhatsAppCall, error)
	UpdateCallState(ctx context.Context, tx pgx.Tx, orgID, callID string, state model.WhatsAppCallState) (*model.WhatsAppCall, error)
	ListCalls(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.WhatsAppCall, int, error)
}

// WhatsAppService contains business logic for WhatsApp phone number provisioning and call management.
type WhatsAppService struct {
	repo WhatsAppRepository
	pool *pgxpool.Pool
}

// NewWhatsAppService creates a new WhatsAppService.
func NewWhatsAppService(repo WhatsAppRepository, pool *pgxpool.Pool) *WhatsAppService {
	return &WhatsAppService{repo: repo, pool: pool}
}

// --- Phone number operations ---

// CreatePhoneNumber registers a new WhatsApp Business phone number for an org.
func (s *WhatsAppService) CreatePhoneNumber(ctx context.Context, orgID string, req *model.CreateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error) {
	if req == nil {
		return nil, apierror.NewBadRequest("request body must not be nil")
	}
	var phone *model.WhatsAppPhoneNumber
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		phone, e = s.repo.CreatePhoneNumber(ctx, tx, orgID, req)
		return e
	})
	if err != nil {
		if isWADuplicate(err) {
			return nil, apierror.NewConflict("phone number already registered for this organisation")
		}
		slog.ErrorContext(ctx, "WhatsAppService.CreatePhoneNumber db error", "error", err)
		return nil, apierror.NewInternal("failed to create phone number")
	}
	return phone, nil
}

// GetPhoneNumber retrieves a phone number by ID.
func (s *WhatsAppService) GetPhoneNumber(ctx context.Context, orgID, phoneID string) (*model.WhatsAppPhoneNumber, error) {
	var phone *model.WhatsAppPhoneNumber
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		phone, e = s.repo.GetPhoneNumber(ctx, tx, orgID, phoneID)
		return e
	})
	if err != nil {
		if isWANotFound(err) {
			return nil, apierror.NewNotFound("phone number not found")
		}
		slog.ErrorContext(ctx, "WhatsAppService.GetPhoneNumber db error", "error", err)
		return nil, apierror.NewInternal("failed to get phone number")
	}
	return phone, nil
}

// UpdatePhoneNumber updates mutable fields of a phone number.
func (s *WhatsAppService) UpdatePhoneNumber(ctx context.Context, orgID, phoneID string, req *model.UpdateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error) {
	if req == nil {
		return nil, apierror.NewBadRequest("request body must not be nil")
	}
	var phone *model.WhatsAppPhoneNumber
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		phone, e = s.repo.UpdatePhoneNumber(ctx, tx, orgID, phoneID, req)
		return e
	})
	if err != nil {
		if isWANotFound(err) {
			return nil, apierror.NewNotFound("phone number not found")
		}
		slog.ErrorContext(ctx, "WhatsAppService.UpdatePhoneNumber db error", "error", err)
		return nil, apierror.NewInternal("failed to update phone number")
	}
	return phone, nil
}

// DeletePhoneNumber removes a phone number by ID.
func (s *WhatsAppService) DeletePhoneNumber(ctx context.Context, orgID, phoneID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.DeletePhoneNumber(ctx, tx, orgID, phoneID)
	})
	if err != nil {
		if isWANotFound(err) {
			return apierror.NewNotFound("phone number not found")
		}
		slog.ErrorContext(ctx, "WhatsAppService.DeletePhoneNumber db error", "error", err)
		return apierror.NewInternal("failed to delete phone number")
	}
	return nil
}

// ListPhoneNumbers returns org-scoped phone numbers with pagination.
func (s *WhatsAppService) ListPhoneNumbers(ctx context.Context, orgID string, limit, offset int) (*model.WhatsAppPhoneNumberListResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var phones []model.WhatsAppPhoneNumber
	var total int
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		phones, total, e = s.repo.ListPhoneNumbers(ctx, tx, orgID, limit, offset)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "WhatsAppService.ListPhoneNumbers db error", "error", err)
		return nil, apierror.NewInternal("failed to list phone numbers")
	}
	if phones == nil {
		phones = []model.WhatsAppPhoneNumber{}
	}
	return &model.WhatsAppPhoneNumberListResponse{
		PhoneNumbers: phones,
		Total:        total,
		Limit:        limit,
		Offset:       offset,
	}, nil
}

// --- Call operations ---

// InitiateCall creates a new outbound call record.
func (s *WhatsAppService) InitiateCall(ctx context.Context, orgID string, req *model.InitiateWhatsAppCallRequest) (*model.WhatsAppCall, error) {
	if req == nil {
		return nil, apierror.NewBadRequest("request body must not be nil")
	}
	var call *model.WhatsAppCall
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Verify the phone number exists and belongs to this org.
		phone, e := s.repo.GetPhoneNumber(ctx, tx, orgID, req.PhoneNumberID)
		if e != nil {
			return e
		}
		now := time.Now()
		newCall := &model.WhatsAppCall{
			CallID:        generateCallID(),
			PhoneNumberID: req.PhoneNumberID,
			Direction:     model.WhatsAppCallDirectionOutbound,
			State:         model.WhatsAppCallStateRinging,
			Caller:        phone.PhoneNumber,
			Callee:        req.Callee,
			StartedAt:     &now,
		}
		call, e = s.repo.CreateCall(ctx, tx, orgID, newCall)
		return e
	})
	if err != nil {
		if isWANotFound(err) {
			return nil, apierror.NewNotFound("phone number not found")
		}
		slog.ErrorContext(ctx, "WhatsAppService.InitiateCall db error", "error", err)
		return nil, apierror.NewInternal("failed to initiate call")
	}
	return call, nil
}

// GetCall retrieves a call by its internal ID.
func (s *WhatsAppService) GetCall(ctx context.Context, orgID, callID string) (*model.WhatsAppCall, error) {
	var call *model.WhatsAppCall
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		call, e = s.repo.GetCall(ctx, tx, orgID, callID)
		return e
	})
	if err != nil {
		if isWANotFound(err) {
			return nil, apierror.NewNotFound("call not found")
		}
		slog.ErrorContext(ctx, "WhatsAppService.GetCall db error", "error", err)
		return nil, apierror.NewInternal("failed to get call")
	}
	return call, nil
}

// UpdateCallState transitions a call to a new state.
func (s *WhatsAppService) UpdateCallState(ctx context.Context, orgID, callID string, state model.WhatsAppCallState) (*model.WhatsAppCall, error) {
	if state != model.WhatsAppCallStateConnected && state != model.WhatsAppCallStateEnded {
		return nil, apierror.NewBadRequest("state must be 'connected' or 'ended'")
	}

	var call *model.WhatsAppCall
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		call, e = s.repo.UpdateCallState(ctx, tx, orgID, callID, state)
		return e
	})
	if err != nil {
		if isWANotFound(err) {
			return nil, apierror.NewNotFound("call not found")
		}
		slog.ErrorContext(ctx, "WhatsAppService.UpdateCallState db error", "error", err)
		return nil, apierror.NewInternal("failed to update call state")
	}
	return call, nil
}

// ListCalls returns org-scoped calls with pagination.
func (s *WhatsAppService) ListCalls(ctx context.Context, orgID string, limit, offset int) (*model.WhatsAppCallListResponse, error) {
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
		slog.ErrorContext(ctx, "WhatsAppService.ListCalls db error", "error", err)
		return nil, apierror.NewInternal("failed to list calls")
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

// --- Helpers ---

// generateCallID creates a simple internal call ID for outbound calls.
// For inbound calls received via webhook, the Meta call_id is used instead.
func generateCallID() string {
	return "raven-" + time.Now().Format("20060102150405.000")
}

// isWANotFound checks if an error indicates a missing record.
func isWANotFound(err error) bool {
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

// isWADuplicate checks if an error is a unique constraint violation.
func isWADuplicate(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate key") ||
		strings.Contains(err.Error(), "unique constraint")
}
