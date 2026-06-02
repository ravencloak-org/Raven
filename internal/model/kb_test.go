package model

import (
	"encoding/json"
	"testing"
	"time"
)

// TestKBVisibilityConstants pins the enum values to the strings stored in
// Postgres so a typo here would surface at compile/test time rather than
// when a query silently returns zero rows.
func TestKBVisibilityConstants(t *testing.T) {
	cases := map[KBVisibility]string{
		KBVisibilityPrivate: "private",
		KBVisibilityPublic:  "public",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("KBVisibility constant: want %q, got %q", want, got)
		}
	}
}

// TestKnowledgeBase_MarketplaceFields_JSONShape verifies the JSON tags for
// the Marketplace columns added in migration 00047 — Publish/Import handlers
// in later issues rely on this exact wire shape.
func TestKnowledgeBase_MarketplaceFields_JSONShape(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	userID := "11111111-1111-1111-1111-111111111111"
	sourceKBID := "22222222-2222-2222-2222-222222222222"
	license := "CC-BY-4.0"

	kb := KnowledgeBase{
		ID:                       "33333333-3333-3333-3333-333333333333",
		OrgID:                    "44444444-4444-4444-4444-444444444444",
		WorkspaceID:              "55555555-5555-5555-5555-555555555555",
		Name:                     "Test KB",
		Slug:                     "test-kb",
		Settings:                 map[string]any{},
		Status:                   KBStatusActive,
		CacheEnabled:             true,
		CacheSimilarityThreshold: 0.92,
		Visibility:               KBVisibilityPublic,
		PublishedAt:              &now,
		PublishedByUserID:        &userID,
		SourcePublicKBID:         &sourceKBID,
		ImportedFromRevisionAt:   &now,
		LastModifiedAt:           now,
		LicenseSPDXID:            &license,
		ImportCount:              3,
		PreviewCount:             7,
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	raw, err := json.Marshal(kb)
	if err != nil {
		t.Fatalf("marshal KnowledgeBase: %v", err)
	}

	var roundtripped map[string]any
	if err := json.Unmarshal(raw, &roundtripped); err != nil {
		t.Fatalf("unmarshal KnowledgeBase: %v", err)
	}

	expectedKeys := []string{
		"visibility",
		"published_at",
		"published_by_user_id",
		"source_public_kb_id",
		"imported_from_revision_at",
		"last_modified_at",
		"license_spdx_id",
		"import_count",
		"preview_count",
	}
	for _, key := range expectedKeys {
		if _, ok := roundtripped[key]; !ok {
			t.Errorf("missing JSON key %q in marshalled KnowledgeBase", key)
		}
	}

	if roundtripped["visibility"] != "public" {
		t.Errorf("visibility field: want \"public\", got %v", roundtripped["visibility"])
	}
	if got, ok := roundtripped["import_count"].(float64); !ok || got != 3 {
		t.Errorf("import_count: want 3, got %v", roundtripped["import_count"])
	}
}

// TestKnowledgeBase_NilOptionalFields_OmitEmpty ensures the omitempty tags
// keep the wire payload tidy for the default (never-published) row shape.
func TestKnowledgeBase_NilOptionalFields_OmitEmpty(t *testing.T) {
	kb := KnowledgeBase{
		ID:         "id",
		Visibility: KBVisibilityPrivate,
	}
	raw, err := json.Marshal(kb)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"published_at",
		"published_by_user_id",
		"source_public_kb_id",
		"imported_from_revision_at",
		"license_spdx_id",
	} {
		if _, present := out[key]; present {
			t.Errorf("expected %q to be omitted when nil, but it was present", key)
		}
	}
	// Non-pointer scalars must serialise even at their zero values so
	// clients can rely on a stable shape.
	for _, key := range []string{"visibility", "import_count", "preview_count", "last_modified_at"} {
		if _, present := out[key]; !present {
			t.Errorf("expected %q to always be present, but it was omitted", key)
		}
	}
}
