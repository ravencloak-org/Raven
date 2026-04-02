package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/samber/lo"

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
	return apierror.NewInternal(err.Error())
}

// LeadService contains business logic for lead profile management.
type LeadService struct {
	repo *repository.LeadRepository
}

// NewLeadService creates a new LeadService.
func NewLeadService(repo *repository.LeadRepository) *LeadService {
	return &LeadService{repo: repo}
}

// ComputeEngagementScore calculates a lead engagement score.
// Formula: messages*0.5 + sessions*2.0 + emailBonus (10 if email non-empty, else 0).
func ComputeEngagementScore(lead *model.LeadProfile) float32 {
	emailBonus := float32(0)
	if lead.Email != "" {
		emailBonus = 10
	}
	return float32(lead.TotalMessages)*0.5 + float32(lead.TotalSessions)*2.0 + emailBonus
}

// Upsert validates and persists a lead profile, creating or merging by org+email.
func (s *LeadService) Upsert(ctx context.Context, orgID string, req model.UpsertLeadRequest) (*model.LeadProfile, error) {
	lead, err := s.repo.Upsert(ctx, orgID, req)
	if err != nil {
		return nil, mapLeadDBError(err)
	}
	lead.EngagementScore = ComputeEngagementScore(lead)
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
		return nil, apierror.NewInternal("failed to list leads: " + err.Error())
	}
	leads = lo.Ternary(leads == nil, []model.LeadProfile{}, leads)

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
	lead.EngagementScore = ComputeEngagementScore(lead)
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
