package model

import "time"

// StrangerStatus enumerates the lifecycle states of an anonymous chat user.
type StrangerStatus string

// Supported stranger user statuses.
const (
	StrangerStatusActive    StrangerStatus = "active"
	StrangerStatusThrottled StrangerStatus = "throttled"
	StrangerStatusBlocked   StrangerStatus = "blocked"
	StrangerStatusBanned    StrangerStatus = "banned"
)

// StrangerUser represents an anonymous chat participant tracked by session ID.
type StrangerUser struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"org_id"`
	SessionID    string         `json:"session_id"`
	IPAddress    *string        `json:"ip_address,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	Status       StrangerStatus `json:"status"`
	BlockReason  string         `json:"block_reason,omitempty"`
	MessageCount int            `json:"message_count"`
	RateLimitRPM *int           `json:"rate_limit_rpm,omitempty"`
	LastActiveAt time.Time      `json:"last_active_at"`
	BlockedAt    *time.Time     `json:"blocked_at,omitempty"`
	BlockedBy    string         `json:"blocked_by,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// UpsertStrangerRequest is the payload for tracking an anonymous session.
type UpsertStrangerRequest struct {
	SessionID      string  `json:"session_id" binding:"required"`
	IPAddress      *string `json:"ip_address,omitempty"`
	UserAgent      string  `json:"user_agent,omitempty"`
	IncrementCount bool    `json:"-"` // true only for message-producing requests (e.g. completions)
}

// BlockStrangerRequest is the payload for blocking or banning a user.
type BlockStrangerRequest struct {
	Status StrangerStatus `json:"status" binding:"required"`
	Reason string         `json:"reason" binding:"required,min=3,max=500"`
}

// SetRateLimitRequest overrides the per-session message rate limit.
type SetRateLimitRequest struct {
	RPM *int `json:"rpm"`
}
