package service

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// WorkspaceService contains business logic for workspace management.
type WorkspaceService struct {
	repo  *repository.WorkspaceRepository
	pool  *pgxpool.Pool
	quota QuotaCheckerI
}

// NewWorkspaceService creates a new WorkspaceService.
func NewWorkspaceService(repo *repository.WorkspaceRepository, pool *pgxpool.Pool, quota QuotaCheckerI) *WorkspaceService {
	return &WorkspaceService{repo: repo, pool: pool, quota: quota}
}

// Create validates and creates a new workspace within an organisation.
func (s *WorkspaceService) Create(ctx context.Context, orgID string, req model.CreateWorkspaceRequest) (*model.Workspace, error) {
	slug := toSlug(req.Name)
	if slug == "" {
		return nil, apierror.NewBadRequest("workspace name produces an empty slug")
	}
	var ws *model.Workspace
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		ws, err = s.repo.Create(ctx, tx, orgID, req.Name, slug)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apierror.NewBadRequest("workspace slug already taken in this organisation: " + slug)
		}
		return nil, apierror.NewInternal("failed to create workspace: " + err.Error())
	}
	return ws, nil
}

// GetByOrgAndID retrieves a workspace by org and workspace ID.
func (s *WorkspaceService) GetByOrgAndID(ctx context.Context, orgID, wsID string) (*model.Workspace, error) {
	var ws *model.Workspace
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		ws, err = s.repo.GetByOrgAndID(ctx, tx, orgID, wsID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("workspace not found")
		}
		return nil, apierror.NewInternal("failed to fetch workspace: " + err.Error())
	}
	return ws, nil
}

// ListByOrg returns all workspaces for an organisation.
func (s *WorkspaceService) ListByOrg(ctx context.Context, orgID string) ([]model.Workspace, error) {
	var workspaces []model.Workspace
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		workspaces, err = s.repo.ListByOrg(ctx, tx, orgID)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list workspaces: " + err.Error())
	}
	return workspaces, nil
}

// Update applies partial updates to a workspace.
func (s *WorkspaceService) Update(ctx context.Context, orgID, wsID string, req model.UpdateWorkspaceRequest) (*model.Workspace, error) {
	var ws *model.Workspace
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		ws, err = s.repo.Update(ctx, tx, orgID, wsID, req.Name, req.Settings)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("workspace not found")
		}
		return nil, apierror.NewInternal("failed to update workspace: " + err.Error())
	}
	return ws, nil
}

// Delete removes a workspace from an organisation.
func (s *WorkspaceService) Delete(ctx context.Context, orgID, wsID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, orgID, wsID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("workspace not found")
		}
		return apierror.NewInternal("failed to delete workspace: " + err.Error())
	}
	return nil
}

// AddMember adds a user to a workspace.
func (s *WorkspaceService) AddMember(ctx context.Context, orgID, wsID string, req model.AddWorkspaceMemberRequest) (*model.WorkspaceMember, error) {
	if s.quota != nil {
		if err := s.quota.CheckSeatQuota(ctx, orgID); err != nil {
			return nil, err
		}
	}

	var member *model.WorkspaceMember
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		member, err = s.repo.AddMember(ctx, tx, orgID, wsID, req.UserID, req.Role)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apierror.NewBadRequest("user is already a member of this workspace")
		}
		return nil, apierror.NewInternal("failed to add member: " + err.Error())
	}
	return member, nil
}

// UpdateMemberRole changes a workspace member's role.
func (s *WorkspaceService) UpdateMemberRole(ctx context.Context, orgID, wsID string, req model.UpdateWorkspaceMemberRequest, userID string) (*model.WorkspaceMember, error) {
	var member *model.WorkspaceMember
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		member, err = s.repo.UpdateMemberRole(ctx, tx, wsID, userID, req.Role)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("workspace member not found")
		}
		return nil, apierror.NewInternal("failed to update member role: " + err.Error())
	}
	return member, nil
}

// RemoveMember removes a user from a workspace.
func (s *WorkspaceService) RemoveMember(ctx context.Context, orgID, wsID, userID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.RemoveMember(ctx, tx, wsID, userID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("workspace member not found")
		}
		return apierror.NewInternal("failed to remove member: " + err.Error())
	}
	return nil
}
