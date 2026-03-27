package service

import (
	"context"
	"strings"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// UserService contains business logic for user management.
type UserService struct {
	repo *repository.UserRepository
}

// NewUserService creates a new UserService.
func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// GetMe fetches the current user by their Keycloak subject (from JWT).
func (s *UserService) GetMe(ctx context.Context, keycloakSub string) (*model.User, error) {
	u, err := s.repo.GetByKeycloakSub(ctx, keycloakSub)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("user not found")
		}
		return nil, apierror.NewInternal("failed to fetch user: " + err.Error())
	}
	return u, nil
}

// UpdateMe applies a partial update to the current user's profile.
func (s *UserService) UpdateMe(ctx context.Context, userID string, req model.UpdateUserRequest) (*model.User, error) {
	u, err := s.repo.UpdateDisplayName(ctx, userID, req.DisplayName)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("user not found")
		}
		return nil, apierror.NewInternal("failed to update user: " + err.Error())
	}
	return u, nil
}

// GetByID fetches a user by their primary key ID.
func (s *UserService) GetByID(ctx context.Context, userID string) (*model.User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("user not found")
		}
		return nil, apierror.NewInternal("failed to fetch user: " + err.Error())
	}
	return u, nil
}

// HandleKeycloakEvent processes an incoming Keycloak webhook event.
// Supported event types: REGISTER, UPDATE_PROFILE, DELETE_ACCOUNT.
// Unknown event types are silently ignored (future-proof).
func (s *UserService) HandleKeycloakEvent(ctx context.Context, event model.KeycloakWebhookEvent) error {
	switch event.Type {
	case "REGISTER", "UPDATE_PROFILE":
		displayName := event.Attributes["firstName"] + " " + event.Attributes["lastName"]
		displayName = strings.TrimSpace(displayName)
		_, err := s.repo.UpsertByKeycloakSub(ctx, event.OrgID, event.UserID, event.Email, displayName)
		if err != nil {
			return apierror.NewInternal("keycloak sync failed: " + err.Error())
		}
	case "DELETE_ACCOUNT":
		// Soft-delete: find user by keycloak_sub first, then disable.
		u, err := s.repo.GetByKeycloakSub(ctx, event.UserID)
		if err != nil {
			// If not found, treat as idempotent success.
			if strings.Contains(err.Error(), "no rows") {
				return nil
			}
			return apierror.NewInternal("user lookup failed: " + err.Error())
		}
		if err := s.repo.SoftDelete(ctx, u.ID); err != nil {
			return apierror.NewInternal("user delete failed: " + err.Error())
		}
	default:
		// Ignore unknown event types — forward compatibility.
	}
	return nil
}

// DeleteMe soft-deletes the current user (GDPR right to erasure).
func (s *UserService) DeleteMe(ctx context.Context, userID string) error {
	if err := s.repo.SoftDelete(ctx, userID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("user not found")
		}
		return apierror.NewInternal("failed to delete user: " + err.Error())
	}
	return nil
}
