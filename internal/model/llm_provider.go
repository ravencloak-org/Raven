package model

import "time"

// LLMProvider represents the supported LLM provider types.
type LLMProvider string

// Supported LLM provider identifiers.
const (
	LLMProviderOpenAI     LLMProvider = "openai"
	LLMProviderAnthropic  LLMProvider = "anthropic"
	LLMProviderCohere     LLMProvider = "cohere"
	LLMProviderGoogle     LLMProvider = "google"
	LLMProviderAzureOpenAI LLMProvider = "azure_openai"
	LLMProviderCustom     LLMProvider = "custom"
)

// ValidLLMProviders is the set of valid LLM provider enum values.
var ValidLLMProviders = map[LLMProvider]bool{
	LLMProviderOpenAI:      true,
	LLMProviderAnthropic:   true,
	LLMProviderCohere:      true,
	LLMProviderGoogle:      true,
	LLMProviderAzureOpenAI: true,
	LLMProviderCustom:      true,
}

// ProviderStatus represents the lifecycle state of an LLM provider config.
type ProviderStatus string

// Provider status values for the lifecycle of an LLM provider config.
const (
	ProviderStatusActive  ProviderStatus = "active"
	ProviderStatusRevoked ProviderStatus = "revoked"
	ProviderStatusExpired ProviderStatus = "expired"
)

// ValidProviderStatuses is the set of valid provider status enum values.
var ValidProviderStatuses = map[ProviderStatus]bool{
	ProviderStatusActive:  true,
	ProviderStatusRevoked: true,
	ProviderStatusExpired: true,
}

// LLMProviderConfig represents a stored LLM provider configuration row.
type LLMProviderConfig struct {
	ID              string         `json:"id"`
	OrgID           string         `json:"org_id"`
	Provider        LLMProvider    `json:"provider"`
	DisplayName     string         `json:"display_name"`
	APIKeyEncrypted []byte         `json:"-"` // never serialised
	APIKeyIV        []byte         `json:"-"` // never serialised
	APIKeyHint      string         `json:"api_key_hint,omitempty"`
	BaseURL         *string        `json:"base_url,omitempty"`
	Config          map[string]any `json:"config,omitempty"`
	IsDefault       bool           `json:"is_default"`
	Status          ProviderStatus `json:"status"`
	CreatedBy       *string        `json:"created_by,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// CreateLLMProviderRequest is the payload for POST .../llm-providers.
type CreateLLMProviderRequest struct {
	Provider    LLMProvider    `json:"provider" binding:"required"`
	DisplayName string         `json:"display_name" binding:"required,min=1,max=255"`
	APIKey      string         `json:"api_key" binding:"required,min=1"`
	BaseURL     *string        `json:"base_url,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	IsDefault   bool           `json:"is_default,omitempty"`
}

// UpdateLLMProviderRequest is the payload for PUT .../llm-providers/:provider_id.
type UpdateLLMProviderRequest struct {
	DisplayName *string        `json:"display_name,omitempty" binding:"omitempty,min=1,max=255"`
	APIKey      *string        `json:"api_key,omitempty" binding:"omitempty,min=1"`
	BaseURL     *string        `json:"base_url,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	Status      *ProviderStatus `json:"status,omitempty"`
}

// LLMProviderResponse is the API response DTO — never contains encrypted key data.
type LLMProviderResponse struct {
	ID          string         `json:"id"`
	OrgID       string         `json:"org_id"`
	Provider    LLMProvider    `json:"provider"`
	DisplayName string         `json:"display_name"`
	APIKeyHint  string         `json:"api_key_hint,omitempty"`
	BaseURL     *string        `json:"base_url,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	IsDefault   bool           `json:"is_default"`
	Status      ProviderStatus `json:"status"`
	CreatedBy   *string        `json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ToResponse converts an LLMProviderConfig (internal) to an LLMProviderResponse (API-safe).
func (c *LLMProviderConfig) ToResponse() *LLMProviderResponse {
	return &LLMProviderResponse{
		ID:          c.ID,
		OrgID:       c.OrgID,
		Provider:    c.Provider,
		DisplayName: c.DisplayName,
		APIKeyHint:  c.APIKeyHint,
		BaseURL:     c.BaseURL,
		Config:      c.Config,
		IsDefault:   c.IsDefault,
		Status:      c.Status,
		CreatedBy:   c.CreatedBy,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
