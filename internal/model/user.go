package model

import "time"

// UserStatus represents the lifecycle state of a user.
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

// User represents a Raven user scoped to an organisation.
type User struct {
	ID            string     `json:"id"`
	OrgID         string     `json:"org_id"`
	Email         string     `json:"email"`
	DisplayName   string     `json:"display_name,omitempty"`
	KeycloakSub   string     `json:"keycloak_sub,omitempty"`
	Status        UserStatus `json:"status"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// UpdateUserRequest is the payload for PUT /api/v1/me or admin user updates.
type UpdateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty" binding:"omitempty,max=255"`
}

// KeycloakWebhookEvent is the payload sent by the Keycloak SPI webhook.
// See: ravencloak SPI event format.
type KeycloakWebhookEvent struct {
	Type      string            `json:"type"`       // e.g. "REGISTER", "UPDATE_PROFILE", "DELETE_ACCOUNT"
	RealmID   string            `json:"realmId"`
	UserID    string            `json:"userId"`
	OrgID     string            `json:"orgId"`
	Email     string            `json:"email"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Timestamp  int64             `json:"timestamp"`
}
