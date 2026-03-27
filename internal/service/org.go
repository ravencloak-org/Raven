package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// OrgService contains business logic for organisation management.
type OrgService struct {
	repo *repository.OrgRepository
}

// NewOrgService creates a new OrgService.
func NewOrgService(repo *repository.OrgRepository) *OrgService {
	return &OrgService{repo: repo}
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// toSlug converts a human-readable name to a URL-safe slug.
func toSlug(name string) string {
	s := strings.ToLower(name)
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// Create validates the request, derives a slug, and persists a new organisation.
func (s *OrgService) Create(ctx context.Context, req model.CreateOrgRequest) (*model.Organization, error) {
	slug := toSlug(req.Name)
	if slug == "" {
		return nil, apierror.NewBadRequest("organisation name produces an empty slug")
	}
	org, err := s.repo.Create(ctx, req.Name, slug)
	if err != nil {
		// A unique-constraint violation means the slug is already taken.
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apierror.NewBadRequest("organisation slug already taken: " + slug)
		}
		return nil, apierror.NewInternal("failed to create organisation: " + err.Error())
	}
	return org, nil
}

// GetByID retrieves an active organisation by ID.
func (s *OrgService) GetByID(ctx context.Context, orgID string) (*model.Organization, error) {
	org, err := s.repo.GetByID(ctx, orgID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("organisation not found")
		}
		return nil, apierror.NewInternal("failed to fetch organisation: " + err.Error())
	}
	return org, nil
}

// Update applies a partial update to an organisation.
func (s *OrgService) Update(ctx context.Context, orgID string, req model.UpdateOrgRequest) (*model.Organization, error) {
	org, err := s.repo.Update(ctx, orgID, req.Name, req.Settings)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("organisation not found")
		}
		return nil, apierror.NewInternal("failed to update organisation: " + err.Error())
	}
	return org, nil
}

// Delete soft-deletes an organisation by setting its status to 'deactivated'.
func (s *OrgService) Delete(ctx context.Context, orgID string) error {
	if err := s.repo.SoftDelete(ctx, orgID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("organisation not found")
		}
		return apierror.NewInternal("failed to delete organisation: " + err.Error())
	}
	return nil
}
