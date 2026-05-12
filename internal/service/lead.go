package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mapLeadDBError converts low-level pgx/pgconn errors to API errors.
func mapLeadDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apierror.NewNotFound("lead not found")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503": // foreign_key_violation
			return apierror.NewBadRequest("invalid reference (knowledge base not found)")
		case "22P02": // invalid_text_representation (bad UUID)
			return apierror.NewBadRequest("invalid id format")
		case "23505": // unique_violation
			return apierror.NewBadRequest("lead already exists with conflicting unique field")
		}
	}
	slog.Error("lead: unexpected database error", "error", err)
	return apierror.NewInternal("an unexpected error occurred")
}

// LeadWebhookDispatcher is the slice of WebhookService that LeadService needs
// in order to fire outbound `lead.generated` events. Defined as a local
// interface so tests can substitute a fake. A nil dispatcher is supported and
// means "do not emit webhooks".
type LeadWebhookDispatcher interface {
	Dispatch(ctx context.Context, orgID, eventType string, payload map[string]any) error
}

// LeadService contains business logic for lead profile management.
type LeadService struct {
	repo              *repository.LeadRepository
	webhookDispatcher LeadWebhookDispatcher
}

// NewLeadService creates a new LeadService.
func NewLeadService(repo *repository.LeadRepository) *LeadService {
	return &LeadService{repo: repo}
}

// WithWebhookDispatcher attaches a webhook dispatcher so that successful
// upserts fan out a `lead.generated` event. Chainable at wiring time.
func (s *LeadService) WithWebhookDispatcher(d LeadWebhookDispatcher) *LeadService {
	s.webhookDispatcher = d
	return s
}

// dispatchLeadGenerated fires a `lead.generated` webhook in a detached
// goroutine. Errors are logged and swallowed — webhook delivery never blocks
// or fails the producer's success path. Note that today the repo's Upsert
// merges by (org, email) so an event is emitted on both first capture and
// subsequent merges; receivers should be idempotent on `lead_id`.
func (s *LeadService) dispatchLeadGenerated(ctx context.Context, orgID string, lead *model.LeadProfile) {
	if s.webhookDispatcher == nil || lead == nil {
		return
	}
	detached := context.WithoutCancel(ctx)
	payload := map[string]any{
		"lead_id":           lead.ID,
		"email":             lead.Email,
		"name":              lead.Name,
		"knowledge_base_id": lead.KnowledgeBaseID,
		"session_ids":       lead.SessionIDs,
	}
	go func() {
		if err := s.webhookDispatcher.Dispatch(detached, orgID,
			string(model.WebhookEventLeadGenerated), payload); err != nil {
			slog.WarnContext(detached, "webhook dispatch failed",
				"event_type", string(model.WebhookEventLeadGenerated),
				"org_id", orgID, "lead_id", lead.ID, "error", err)
		}
	}()
}

// Upsert validates and persists a lead profile, creating or merging by org+email.
func (s *LeadService) Upsert(ctx context.Context, orgID string, req model.UpsertLeadRequest) (*model.LeadProfile, error) {
	lead, err := s.repo.Upsert(ctx, orgID, req)
	if err != nil {
		return nil, mapLeadDBError(err)
	}
	s.dispatchLeadGenerated(ctx, orgID, lead)
	return lead, nil
}

// GetByID retrieves a lead profile by ID within an org.
func (s *LeadService) GetByID(ctx context.Context, orgID, id string) (*model.LeadProfile, error) {
	lead, err := s.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, mapLeadDBError(err)
	}
	return lead, nil
}

// List returns a paginated list of lead profiles for an org.
func (s *LeadService) List(ctx context.Context, orgID string, minScore *float32, limit, offset int) (*model.LeadListResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	leads, total, err := s.repo.List(ctx, orgID, minScore, limit, offset)
	if err != nil {
		return nil, mapLeadDBError(err)
	}
	if leads == nil {
		leads = []model.LeadProfile{}
	}

	page := offset/limit + 1
	return &model.LeadListResponse{
		Data:     leads,
		Total:    total,
		Page:     page,
		PageSize: limit,
	}, nil
}

// Update validates and applies partial updates to a lead profile.
func (s *LeadService) Update(ctx context.Context, orgID, id string, req model.UpdateLeadRequest) (*model.LeadProfile, error) {
	lead, err := s.repo.Update(ctx, orgID, id, req)
	if err != nil {
		return nil, mapLeadDBError(err)
	}
	return lead, nil
}

// Delete permanently removes a lead profile.
func (s *LeadService) Delete(ctx context.Context, orgID, id string) error {
	if err := s.repo.Delete(ctx, orgID, id); err != nil {
		return mapLeadDBError(err)
	}
	return nil
}

// ExportCSV returns all lead profiles for an org for CRM CSV export.
func (s *LeadService) ExportCSV(ctx context.Context, orgID string) ([]model.LeadProfile, error) {
	leads, err := s.repo.ExportCSV(ctx, orgID)
	if err != nil {
		return nil, mapLeadDBError(err)
	}
	return leads, nil
}
