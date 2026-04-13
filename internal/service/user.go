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

// GetByExternalID fetches a user by their external IdP identifier.
func (s *UserService) GetByExternalID(ctx context.Context, externalID string) (*model.User, error) {
	u, err := s.repo.GetByExternalID(ctx, externalID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("user not found")
		}
		return nil, apierror.NewInternal("failed to fetch user: " + err.Error())
	}
	return u, nil
}

// Create creates a new user from external IdP data (called during first login).
func (s *UserService) Create(ctx context.Context, externalID, email, displayName string) (*model.User, error) {
	u, err := s.repo.UpsertByExternalID(ctx, externalID, email, displayName)
	if err != nil {
		return nil, apierror.NewInternal("failed to create user: " + err.Error())
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
