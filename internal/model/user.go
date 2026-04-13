package model

import "time"

// UserStatus represents the lifecycle state of a user.
type UserStatus string

// UserStatusActive and UserStatusDisabled are the valid lifecycle states for a User.
const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

// User represents a Raven user scoped to an organisation.
type User struct {
	ID           string     `json:"id"`
	OrgID        *string    `json:"org_id,omitempty"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"display_name,omitempty"`
	ExternalID   string     `json:"external_id,omitempty"`
	AuthProvider string     `json:"auth_provider"`
	Status       UserStatus `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// UpdateUserRequest is the payload for PUT /api/v1/me or admin user updates.
type UpdateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty" binding:"omitempty,max=255"`
}
