package model

import "time"

// Workspace is a collaboration space within an organisation.
type Workspace struct {
	ID        string         `json:"id"`
	OrgID     string         `json:"org_id"`
	Name      string         `json:"name"`
	Slug      string         `json:"slug"`
	Settings  map[string]any `json:"settings"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// CreateWorkspaceRequest is the payload for POST /api/v1/orgs/:org_id/workspaces.
type CreateWorkspaceRequest struct {
	Name string `json:"name" binding:"required,min=2,max=255"`
}

// UpdateWorkspaceRequest is the payload for PUT /api/v1/orgs/:org_id/workspaces/:ws_id.
type UpdateWorkspaceRequest struct {
	Name     *string        `json:"name,omitempty" binding:"omitempty,min=2,max=255"`
	Settings map[string]any `json:"settings,omitempty"`
}

// WorkspaceMember represents a user's membership in a workspace.
type WorkspaceMember struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	OrgID       string    `json:"org_id"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// AddWorkspaceMemberRequest is the payload for POST /api/v1/orgs/:org_id/workspaces/:ws_id/members.
type AddWorkspaceMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required,oneof=owner admin member viewer"`
}

// UpdateWorkspaceMemberRequest is the payload for PUT .../members/:user_id.
type UpdateWorkspaceMemberRequest struct {
	Role string `json:"role" binding:"required,oneof=owner admin member viewer"`
}
