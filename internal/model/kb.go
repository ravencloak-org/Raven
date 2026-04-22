package model

import "time"

// KBStatus represents the lifecycle state of a knowledge base.
type KBStatus string

// KBStatusActive and KBStatusArchived are the valid lifecycle states for a KnowledgeBase.
const (
	KBStatusActive   KBStatus = "active"
	KBStatusArchived KBStatus = "archived"
)

// KnowledgeBase is a scoped document store within a workspace.
type KnowledgeBase struct {
	ID                       string         `json:"id"`
	OrgID                    string         `json:"org_id"`
	WorkspaceID              string         `json:"workspace_id"`
	Name                     string         `json:"name"`
	Slug                     string         `json:"slug"`
	Description              string         `json:"description,omitempty"`
	Settings                 map[string]any `json:"settings"`
	Status                   KBStatus       `json:"status"`
	// CacheEnabled toggles the semantic response cache for this KB. See #256.
	CacheEnabled bool `json:"cache_enabled"`
	// CacheSimilarityThreshold is the cosine-similarity floor (0.80–0.99)
	// above which a stored cache entry is considered a HIT for an incoming
	// query. Stricter values yield fewer hits but higher answer accuracy.
	CacheSimilarityThreshold float32   `json:"cache_similarity_threshold"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// CreateKBRequest is the payload for POST .../knowledge-bases.
type CreateKBRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Description string `json:"description,omitempty"`
}

// UpdateKBRequest is the payload for PUT/PATCH .../knowledge-bases/:kb_id.
//
// All fields are optional — only non-nil fields are applied. The semantic
// cache knobs (#256) live here so operators can toggle caching and tune the
// similarity threshold from a single endpoint.
type UpdateKBRequest struct {
	Name                     *string        `json:"name,omitempty" binding:"omitempty,min=2,max=255"`
	Description              *string        `json:"description,omitempty"`
	Settings                 map[string]any `json:"settings,omitempty"`
	CacheEnabled             *bool          `json:"cache_enabled,omitempty"`
	CacheSimilarityThreshold *float32       `json:"cache_similarity_threshold,omitempty" binding:"omitempty,min=0.80,max=0.99"`
}
