package model

import "time"

// LeadProfile represents a visitor lead tracked from chatbot interactions.
type LeadProfile struct {
	ID              string         `json:"id"`
	OrgID           string         `json:"org_id"`
	KnowledgeBaseID string         `json:"knowledge_base_id,omitempty"`
	SessionIDs      []string       `json:"session_ids"`
	Email           string         `json:"email,omitempty"`
	Name            string         `json:"name,omitempty"`
	Phone           string         `json:"phone,omitempty"`
	Company         string         `json:"company,omitempty"`
	Metadata        map[string]any `json:"metadata"`
	EngagementScore float32        `json:"engagement_score"`
	TotalMessages   int            `json:"total_messages"`
	TotalSessions   int            `json:"total_sessions"`
	FirstSeenAt     time.Time      `json:"first_seen_at"`
	LastSeenAt      time.Time      `json:"last_seen_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// UpsertLeadRequest is the payload for POST .../leads (create or update by org+email).
type UpsertLeadRequest struct {
	KnowledgeBaseID *string        `json:"knowledge_base_id,omitempty"`
	SessionIDs      []string       `json:"session_ids,omitempty"`
	Email           string         `json:"email,omitempty" binding:"omitempty,email"`
	Name            string         `json:"name,omitempty" binding:"omitempty,max=255"`
	Phone           string         `json:"phone,omitempty" binding:"omitempty,max=50"`
	Company         string         `json:"company,omitempty" binding:"omitempty,max=255"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	TotalMessages   *int           `json:"total_messages,omitempty"`
	TotalSessions   *int           `json:"total_sessions,omitempty"`
}

// UpdateLeadRequest is the payload for PUT .../leads/:id.
type UpdateLeadRequest struct {
	KnowledgeBaseID *string        `json:"knowledge_base_id,omitempty"`
	SessionIDs      []string       `json:"session_ids,omitempty"`
	Email           *string        `json:"email,omitempty" binding:"omitempty,email"`
	Name            *string        `json:"name,omitempty" binding:"omitempty,max=255"`
	Phone           *string        `json:"phone,omitempty" binding:"omitempty,max=50"`
	Company         *string        `json:"company,omitempty" binding:"omitempty,max=255"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	TotalMessages   *int           `json:"total_messages,omitempty"`
	TotalSessions   *int           `json:"total_sessions,omitempty"`
}

// LeadListResponse wraps a paginated list of lead profiles.
type LeadListResponse struct {
	Data     []LeadProfile `json:"data"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}
