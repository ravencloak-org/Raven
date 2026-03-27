package model

import "time"

// OrgStatus represents the lifecycle state of an organisation.
type OrgStatus string

const (
	OrgStatusActive      OrgStatus = "active"
	OrgStatusSuspended   OrgStatus = "suspended"
	OrgStatusDeactivated OrgStatus = "deactivated"
)

// Organization is the top-level tenant entity.
type Organization struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	Status        OrgStatus      `json:"status"`
	Settings      map[string]any `json:"settings"`
	KeycloakRealm string         `json:"keycloak_realm,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// CreateOrgRequest is the payload for POST /api/v1/orgs.
type CreateOrgRequest struct {
	Name string `json:"name" binding:"required,min=2,max=255"`
}

// UpdateOrgRequest is the payload for PUT /api/v1/orgs/:org_id.
type UpdateOrgRequest struct {
	Name     *string        `json:"name,omitempty" binding:"omitempty,min=2,max=255"`
	Settings map[string]any `json:"settings,omitempty"`
}

// OrgMember is returned by GET /api/v1/orgs/:org_id/members.
type OrgMember struct {
	UserID   string    `json:"user_id"`
	Email    string    `json:"email"`
	OrgRole  string    `json:"org_role"`
	JoinedAt time.Time `json:"joined_at"`
}
