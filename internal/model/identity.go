package model

import "time"

// IdentityChannel enumerates supported communication channels.
type IdentityChannel string

// Supported identity channels.
const (
	IdentityChannelChat   IdentityChannel = "chat"
	IdentityChannelWidget IdentityChannel = "widget"
	IdentityChannelAPI    IdentityChannel = "api"
)

// UserIdentity represents a cross-channel user identity tracked via PostHog.
type UserIdentity struct {
	ID                string          `json:"id"`
	OrgID             string          `json:"org_id"`
	AnonymousID       string          `json:"anonymous_id"`
	UserID            *string         `json:"user_id,omitempty"`
	PosthogDistinctID *string         `json:"posthog_distinct_id,omitempty"`
	Channel           IdentityChannel `json:"channel"`
	FirstSeenAt       time.Time       `json:"first_seen_at"`
	LastSeenAt        time.Time       `json:"last_seen_at"`
	SessionCount      int             `json:"session_count"`
	Metadata          map[string]any  `json:"metadata"`
	CreatedAt         time.Time       `json:"created_at"`
}

// IdentifyRequest is the payload for linking an anonymous session to an identified user.
type IdentifyRequest struct {
	AnonymousID       string          `json:"anonymous_id" binding:"required,min=1,max=255"`
	UserID            string          `json:"user_id,omitempty"`
	PosthogDistinctID string          `json:"posthog_distinct_id,omitempty"`
	Channel           IdentityChannel `json:"channel" binding:"required"`
	Metadata          map[string]any  `json:"metadata,omitempty"`
}

// TrackEventRequest is the payload for tracking a custom PostHog event.
type TrackEventRequest struct {
	AnonymousID string         `json:"anonymous_id,omitempty"`
	UserID      string         `json:"user_id,omitempty"`
	Event       string         `json:"event" binding:"required,min=1,max=255"`
	Properties  map[string]any `json:"properties,omitempty"`
}

// IdentityListResponse is a paginated list of user identities.
type IdentityListResponse struct {
	Identities []UserIdentity `json:"identities"`
	Total      int            `json:"total"`
}
