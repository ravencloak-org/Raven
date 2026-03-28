package model

import "time"

// ApiKeyStatus represents the lifecycle state of an API key.
type ApiKeyStatus string

const (
	ApiKeyStatusActive  ApiKeyStatus = "active"
	ApiKeyStatusRevoked ApiKeyStatus = "revoked"
)

// ApiKey is a publishable API key scoped to a knowledge base.
type ApiKey struct {
	ID              string       `json:"id"`
	OrgID           string       `json:"org_id"`
	WorkspaceID     string       `json:"workspace_id,omitempty"`
	KnowledgeBaseID string       `json:"knowledge_base_id"`
	Name            string       `json:"name"`
	KeyHash         string       `json:"-"`
	KeyPrefix       string       `json:"key_prefix"`
	AllowedDomains  []string     `json:"allowed_domains"`
	RateLimit       int          `json:"rate_limit"`
	Status          ApiKeyStatus `json:"status"`
	CreatedBy       string       `json:"created_by,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	ExpiresAt       *time.Time   `json:"expires_at,omitempty"`
}

// CreateApiKeyRequest is the payload for POST .../api-keys.
type CreateApiKeyRequest struct {
	Name           string   `json:"name" binding:"required,min=2,max=255"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	RateLimit      *int     `json:"rate_limit,omitempty"`
}

// CreateApiKeyResponse is the response returned when a new API key is created.
// The RawKey field contains the full API key and is shown only once.
type CreateApiKeyResponse struct {
	ApiKey
	RawKey string `json:"raw_key"`
}
