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
	LLMProviderOllama     LLMProvider = "ollama"
	LLMProviderCustom     LLMProvider = "custom"
)

// ValidLLMProviders is the set of valid LLM provider enum values.
// Must stay in sync with the llm_provider Postgres enum (see
// migrations/00045_llm_provider_add_ollama.sql) and the
// `_SUPPORTED_PROVIDERS` set in ai-worker/raven_worker/providers/registry.py.
var ValidLLMProviders = map[LLMProvider]bool{
	LLMProviderOpenAI:      true,
	LLMProviderAnthropic:   true,
	LLMProviderCohere:      true,
	LLMProviderGoogle:      true,
	LLMProviderAzureOpenAI: true,
	LLMProviderOllama:      true,
	LLMProviderCustom:      true,
}

// embeddingCapableProviders is the set of providers that expose a usable
// embedding API. The ai-worker's provider registry (see
// ai-worker/raven_worker/providers/registry.py and anthropic_provider.py)
// is the canonical source of truth — Anthropic ships an embedding stub
// that always raises NotImplementedError, and the registry currently only
// instantiates real embedding providers for the four entries below. Custom
// is intentionally omitted because the ai-worker registry has no embedding
// handler for it today; if that changes, flip the entry to true here and
// register the matching provider class in registry.py.
var embeddingCapableProviders = map[LLMProvider]bool{
	LLMProviderOpenAI: true,
	LLMProviderCohere: true,
	LLMProviderOllama: true,
}

// SupportsEmbeddings reports whether the given provider can serve the
// embedding sub-call of RAG / document ingestion. Used by the embedding
// provider resolver so an org whose default LLM provider is Anthropic
// (chat-only) can still run RAG by routing the embed sub-call to a
// sibling provider that does support embeddings.
func SupportsEmbeddings(p LLMProvider) bool {
	return embeddingCapableProviders[p]
}

// EmbeddingProviderPriority is the resolver's tie-break order when the org's
// default LLM provider cannot serve embeddings. Ollama wins because it's
// the local / edge-friendly default (matches the chat path's literal
// fallback). OpenAI and Cohere follow as the supported cloud embedders.
//
// Keep the order in sync with the resolver doc-comment in
// service.ResolveEmbeddingProvider — both reference this same priority list.
var EmbeddingProviderPriority = []LLMProvider{
	LLMProviderOllama,
	LLMProviderOpenAI,
	LLMProviderCohere,
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

// TestProviderRequest is the payload for POST .../llm-providers/test.
//
// Two shapes are accepted:
//
//  1. Inline credentials — used before a provider row exists (e.g. the Create
//     dialog's "Test connection" button): {provider, base_url?, api_key}.
//  2. Stored credentials — used after a provider row exists (e.g. the Edit
//     dialog's "Test connection" re-gate without re-entering the secret):
//     {provider_id}. The handler loads the row, decrypts the stored key, and
//     probes the vendor with it.
//
// At least one of {provider+api_key, provider_id} must be present. When
// provider_id is set, the inline fields are ignored.
type TestProviderRequest struct {
	ProviderID *string      `json:"provider_id,omitempty" binding:"omitempty,uuid"`
	Provider   *LLMProvider `json:"provider,omitempty"`
	BaseURL    *string      `json:"base_url,omitempty"`
	APIKey     *string      `json:"api_key,omitempty"`
}

// TestConnectionResult is the response shape for POST .../llm-providers/test.
type TestConnectionResult struct {
	OK       bool   `json:"ok"`
	Provider string `json:"provider"`
	Detail   string `json:"detail,omitempty"`
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
