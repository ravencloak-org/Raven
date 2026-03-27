package model

import "time"

// KBStatus represents the lifecycle state of a knowledge base.
type KBStatus string

const (
	KBStatusActive   KBStatus = "active"
	KBStatusArchived KBStatus = "archived"
)

// KnowledgeBase is a scoped document store within a workspace.
type KnowledgeBase struct {
	ID          string         `json:"id"`
	OrgID       string         `json:"org_id"`
	WorkspaceID string         `json:"workspace_id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Description string         `json:"description,omitempty"`
	Settings    map[string]any `json:"settings"`
	Status      KBStatus       `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// CreateKBRequest is the payload for POST .../knowledge-bases.
type CreateKBRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description,omitempty"`
}

// UpdateKBRequest is the payload for PUT .../knowledge-bases/:kb_id.
type UpdateKBRequest struct {
	Name        *string        `json:"name,omitempty" binding:"omitempty,min=2,max=255"`
	Description *string        `json:"description,omitempty"`
	Settings    map[string]any `json:"settings,omitempty"`
}
