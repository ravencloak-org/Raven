package model

import "time"

// KBStatus represents the lifecycle state of a knowledge base.
type KBStatus string

// KBStatusActive and KBStatusArchived are the valid lifecycle states for a KnowledgeBase.
const (
	KBStatusActive   KBStatus = "active"
	KBStatusArchived KBStatus = "archived"
)

// KBVisibility represents the Marketplace publish state of a KB. Private
// KBs are tenant-scoped (the default); Public KBs are listed on the
// Marketplace and importable by other orgs. See ADR-0002 for the publish
// boundary and migration 00047 for the underlying enum.
type KBVisibility string

// KBVisibilityPrivate and KBVisibilityPublic are the valid values of the
// kb_visibility enum added in migration 00047 (issue #723).
const (
	KBVisibilityPrivate KBVisibility = "private"
	KBVisibilityPublic  KBVisibility = "public"
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
	CacheSimilarityThreshold float32 `json:"cache_similarity_threshold"`

	// Marketplace columns (migration 00047, issue #723). The zero values
	// here describe a brand-new private KB that has never been published —
	// the shape every existing row has after the migration runs.

	// Visibility is the Marketplace publish state. See ADR-0002.
	Visibility KBVisibility `json:"visibility"`
	// PublishedAt is set when the KB first transitions private → public,
	// and updated on each re-publish.
	PublishedAt *time.Time `json:"published_at,omitempty"`
	// PublishedByUserID records which org member ran the publish action.
	// Nullable so the FK can survive a user delete (ON DELETE SET NULL).
	PublishedByUserID *string `json:"published_by_user_id,omitempty"`
	// SourcePublicKBID is the Importer-side lineage pointer to the source
	// public KB this row was forked from (ADR-0001). Null for KBs that
	// were authored locally rather than imported.
	SourcePublicKBID *string `json:"source_public_kb_id,omitempty"`
	// ImportedFromRevisionAt captures the source KB's last_modified_at at
	// the moment of Import or Re-import, so the "stale relative to source"
	// signal in ADR-0003 is computable without a Marketplace round-trip.
	ImportedFromRevisionAt *time.Time `json:"imported_from_revision_at,omitempty"`
	// LastModifiedAt is bumped by triggers on documents/sources/chunks so
	// Marketplace listing can surface freshness without scanning children.
	// Distinct from UpdatedAt, which only tracks row-level changes to the
	// knowledge_bases tuple itself.
	LastModifiedAt time.Time `json:"last_modified_at"`
	// LicenseSPDXID is the SPDX identifier (e.g. "CC-BY-4.0") attached to
	// the published KB. Nullable here; the partial CHECK enforcing non-null
	// when visibility='public' lands in migration 00050 (ADR-0006).
	LicenseSPDXID *string `json:"license_spdx_id,omitempty"`
	// ImportCount and PreviewCount power the discovery surface (ADR-0008)
	// and are bumped by the Importer / Preview handlers in later issues.
	ImportCount  int `json:"import_count"`
	PreviewCount int `json:"preview_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
