package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// WorkspaceRepository handles database operations for workspaces.
// All mutating methods must be called inside a db.WithOrgID transaction to
// satisfy RLS — the caller is responsible for supplying the correct pgx.Tx.
type WorkspaceRepository struct {
	pool *pgxpool.Pool
}

// NewWorkspaceRepository creates a new WorkspaceRepository.
func NewWorkspaceRepository(pool *pgxpool.Pool) *WorkspaceRepository {
	return &WorkspaceRepository{pool: pool}
}

const wsColumns = `id, org_id, name, slug, settings, created_at, updated_at`

func scanWorkspace(row pgx.Row) (*model.Workspace, error) {
	var ws model.Workspace
	err := row.Scan(
		&ws.ID,
		&ws.OrgID,
		&ws.Name,
		&ws.Slug,
		&ws.Settings,
		&ws.CreatedAt,
		&ws.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ws, nil
}

// Create inserts a new workspace using the provided transaction (must have org_id set for RLS).
func (r *WorkspaceRepository) Create(ctx context.Context, tx pgx.Tx, orgID, name, slug string) (*model.Workspace, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO workspaces (org_id, name, slug)
		 VALUES ($1, $2, $3)
		 RETURNING `+wsColumns,
		orgID, name, slug,
	)
	ws, err := scanWorkspace(row)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceRepository.Create: %w", err)
	}
	return ws, nil
}

// GetByOrgAndID fetches a workspace belonging to the given org.
func (r *WorkspaceRepository) GetByOrgAndID(ctx context.Context, tx pgx.Tx, orgID, wsID string) (*model.Workspace, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+wsColumns+` FROM workspaces WHERE id = $1 AND org_id = $2`,
		wsID, orgID,
	)
	ws, err := scanWorkspace(row)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceRepository.GetByOrgAndID: %w", err)
	}
	return ws, nil
}

// ListByOrg returns all workspaces for an organisation.
func (r *WorkspaceRepository) ListByOrg(ctx context.Context, tx pgx.Tx, orgID string) ([]model.Workspace, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+wsColumns+` FROM workspaces WHERE org_id = $1 ORDER BY created_at`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceRepository.ListByOrg: %w", err)
	}
	defer rows.Close()

	var workspaces []model.Workspace
	for rows.Next() {
		var ws model.Workspace
		if err := rows.Scan(&ws.ID, &ws.OrgID, &ws.Name, &ws.Slug, &ws.Settings, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			return nil, fmt.Errorf("WorkspaceRepository.ListByOrg scan: %w", err)
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, rows.Err()
}

// Update applies partial updates to a workspace.
func (r *WorkspaceRepository) Update(ctx context.Context, tx pgx.Tx, orgID, wsID string, name *string, settings map[string]any) (*model.Workspace, error) {
	row := tx.QueryRow(ctx,
		`UPDATE workspaces
		 SET
		   name     = COALESCE($3, name),
		   settings = CASE WHEN $4::jsonb IS NOT NULL THEN $4::jsonb ELSE settings END
		 WHERE id = $1 AND org_id = $2
		 RETURNING `+wsColumns,
		wsID, orgID, name, settings,
	)
	ws, err := scanWorkspace(row)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceRepository.Update: %w", err)
	}
	return ws, nil
}

// SoftDelete deletes a workspace (hard delete; workspace data cascades).
func (r *WorkspaceRepository) Delete(ctx context.Context, tx pgx.Tx, orgID, wsID string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM workspaces WHERE id = $1 AND org_id = $2`,
		wsID, orgID,
	)
	if err != nil {
		return fmt.Errorf("WorkspaceRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("WorkspaceRepository.Delete: workspace %s not found", wsID)
	}
	return nil
}

// AddMember adds a user to a workspace with the given role.
func (r *WorkspaceRepository) AddMember(ctx context.Context, tx pgx.Tx, orgID, wsID, userID, role string) (*model.WorkspaceMember, error) {
	var m model.WorkspaceMember
	err := tx.QueryRow(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role, org_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, workspace_id, user_id, org_id, role, created_at`,
		wsID, userID, role, orgID,
	).Scan(&m.ID, &m.WorkspaceID, &m.UserID, &m.OrgID, &m.Role, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceRepository.AddMember: %w", err)
	}
	return &m, nil
}

// UpdateMemberRole changes the role of an existing workspace member.
func (r *WorkspaceRepository) UpdateMemberRole(ctx context.Context, tx pgx.Tx, wsID, userID, role string) (*model.WorkspaceMember, error) {
	var m model.WorkspaceMember
	err := tx.QueryRow(ctx,
		`UPDATE workspace_members
		 SET role = $3
		 WHERE workspace_id = $1 AND user_id = $2
		 RETURNING id, workspace_id, user_id, org_id, role, created_at`,
		wsID, userID, role,
	).Scan(&m.ID, &m.WorkspaceID, &m.UserID, &m.OrgID, &m.Role, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceRepository.UpdateMemberRole: %w", err)
	}
	return &m, nil
}

// RemoveMember removes a user from a workspace.
func (r *WorkspaceRepository) RemoveMember(ctx context.Context, tx pgx.Tx, wsID, userID string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		wsID, userID,
	)
	if err != nil {
		return fmt.Errorf("WorkspaceRepository.RemoveMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("WorkspaceRepository.RemoveMember: membership not found")
	}
	return nil
}
