package repository_test

import (
	"strings"
	"testing"

	"github.com/ravencloak-org/Raven/internal/repository"
)

// TestKBRepository_Constructor verifies the constructor returns a non-nil
// value even with a nil pool — matches the project's existing repository
// unit-test pattern (see chunk_test.go).
func TestKBRepository_Constructor(t *testing.T) {
	repo := repository.NewKBRepository(nil)
	if repo == nil {
		t.Fatal("NewKBRepository should return non-nil even with nil pool")
	}
}

// TestKBColumns_IncludesMarketplaceFields locks in the column list that
// scanKB depends on. Adding a struct field without updating the SELECT
// projection silently breaks RETURNING-based reads — this guard catches
// that drift before it reaches a live DB.
func TestKBColumns_IncludesMarketplaceFields(t *testing.T) {
	cols := repository.KBColumnsForTest()
	expected := []string{
		// pre-marketplace baseline
		"id", "org_id", "workspace_id", "name", "slug",
		"description", "settings", "status",
		"cache_enabled", "cache_similarity_threshold",
		// marketplace (issue #723, migration 00047)
		"visibility",
		"published_at",
		"published_by_user_id",
		"source_public_kb_id",
		"imported_from_revision_at",
		"last_modified_at",
		"license_spdx_id",
		"import_count",
		"preview_count",
		"created_at", "updated_at",
	}
	for _, col := range expected {
		if !strings.Contains(cols, col) {
			t.Errorf("kbColumns missing field %q; full constant:\n%s", col, cols)
		}
	}
}

// TestKBColumns_OmitsSearchTSV documents the deliberate choice to exclude
// search_tsv from the read shape — callers never need the raw tsvector,
// and Marketplace search reads it server-side via SQL functions.
func TestKBColumns_OmitsSearchTSV(t *testing.T) {
	if strings.Contains(repository.KBColumnsForTest(), "search_tsv") {
		t.Error("kbColumns should not select search_tsv; the tsvector is server-side only")
	}
}
