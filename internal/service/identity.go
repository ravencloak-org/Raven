package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/posthog"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// IdentityService resolves cross-channel user identities and syncs with PostHog.
type IdentityService struct {
	repo    *repository.IdentityRepository
	posthog *posthog.Client
}

// NewIdentityService creates a new IdentityService.
func NewIdentityService(repo *repository.IdentityRepository, phClient *posthog.Client) *IdentityService {
	return &IdentityService{
		repo:    repo,
		posthog: phClient,
	}
}

// Identify upserts a user identity record and, when a user_id is present, sends
// PostHog Alias and Identify events to link the anonymous session to the user.
func (s *IdentityService) Identify(ctx context.Context, orgID string, req model.IdentifyRequest) (*model.UserIdentity, error) {
	identity, err := s.repo.Upsert(ctx, orgID, req)
	if err != nil {
		if errors.Is(err, repository.ErrIdentityNotFound) {
			return nil, apierror.NewNotFound("identity not found")
		}
		return nil, apierror.NewInternal("failed to upsert identity: " + err.Error())
	}

	// When a user_id is now set, merge the anonymous→identified user in PostHog.
	if req.UserID != "" {
		if err := s.posthog.Alias(ctx, req.UserID, req.AnonymousID); err != nil {
			slog.WarnContext(ctx, "posthog alias failed",
				slog.String("user_id", req.UserID),
				slog.String("anonymous_id", req.AnonymousID),
				slog.String("error", err.Error()),
			)
		}
		if err := s.posthog.Identify(ctx, req.UserID, map[string]any{
			"channel": string(req.Channel),
		}); err != nil {
			slog.WarnContext(ctx, "posthog identify failed",
				slog.String("user_id", req.UserID),
				slog.String("channel", string(req.Channel)),
				slog.String("error", err.Error()),
			)
		}
	}

	return identity, nil
}

// Track sends a custom event to PostHog on behalf of the given identity.
// It uses user_id as the distinct ID when set, otherwise falls back to anonymous_id.
func (s *IdentityService) Track(ctx context.Context, orgID string, req model.TrackEventRequest) error {
	distinctID := req.UserID
	if distinctID == "" {
		distinctID = req.AnonymousID
	}
	if distinctID == "" {
		return apierror.NewBadRequest("one of user_id or anonymous_id is required")
	}
	if err := s.posthog.Capture(ctx, distinctID, req.Event, req.Properties); err != nil {
		return apierror.NewInternal("failed to track event: " + err.Error())
	}
	return nil
}

// List returns a paginated set of user identities for an org.
func (s *IdentityService) List(ctx context.Context, orgID string, limit, offset int) (*model.IdentityListResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	identities, total, err := s.repo.List(ctx, orgID, limit, offset)
	if err != nil {
		return nil, apierror.NewInternal("failed to list identities: " + err.Error())
	}
	return &model.IdentityListResponse{Identities: identities, Total: total}, nil
}

// Delete removes a user identity record by ID.
func (s *IdentityService) Delete(ctx context.Context, orgID, id string) error {
	if err := s.repo.Delete(ctx, orgID, id); err != nil {
		if errors.Is(err, repository.ErrIdentityNotFound) {
			return apierror.NewNotFound("identity not found")
		}
		return apierror.NewInternal("failed to delete identity: " + err.Error())
	}
	return nil
}
