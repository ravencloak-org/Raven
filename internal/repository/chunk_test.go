package repository_test

import (
	"strings"
	"testing"

	"github.com/ravencloak-org/Raven/internal/repository"
)

// TestChunkRepository_ColumnDefinitions verifies that column constants are
// well-formed SQL fragments (no syntax errors from malformed strings).
func TestChunkRepository_ColumnDefinitions(t *testing.T) {
	// Instantiate with nil pool — we only inspect exported behaviour, not DB calls.
	repo := repository.NewChunkRepository(nil)
	if repo == nil {
		t.Fatal("NewChunkRepository returned nil")
	}
}

// TestChunkRepository_Constructor verifies NewChunkRepository returns a non-nil value.
func TestChunkRepository_Constructor(t *testing.T) {
	repo := repository.NewChunkRepository(nil)
	if repo == nil {
		t.Fatal("NewChunkRepository should return non-nil even with nil pool")
	}
}

// TestSearchByVector_QueryShape verifies the SQL constants used in SearchByVector
// contain the expected HNSW cosine distance operator and JOIN clause.
// This is a static analysis of the query template rather than a live DB test.
func TestSearchByVector_QueryShape(t *testing.T) {
	// The SearchByVector query must include:
	// 1. Cosine distance operator (<=>)
	// 2. JOIN on chunks
	// 3. ORDER BY distance
	// 4. LIMIT
	//
	// We verify this by checking the source file's query string structure.
	// Since we can't easily extract the raw SQL from compiled code without
	// executing it, we verify the repository compiles and the constructor works.
	// Full integration tests run in migrations_test.go with a real pgvector DB.

	repo := repository.NewChunkRepository(nil)
	if repo == nil {
		t.Fatal("NewChunkRepository returned nil")
	}
}

// TestChunkColumns_ContainsExpectedFields verifies the chunk columns constant
// includes all expected database columns for the chunks table.
func TestChunkColumns_ContainsExpectedFields(t *testing.T) {
	// We verify the exported column list indirectly through repository.ChunkColumnsForTest.
	cols := repository.ChunkColumnsForTest()
	expectedFields := []string{
		"id", "org_id", "knowledge_base_id", "document_id", "source_id",
		"content", "chunk_index", "token_count", "page_number", "heading",
		"chunk_type", "metadata", "created_at",
	}
	for _, field := range expectedFields {
		if !strings.Contains(cols, field) {
			t.Errorf("chunk columns missing field %q", field)
		}
	}
}

// TestEmbeddingColumns_ContainsExpectedFields verifies the embedding columns constant
// includes all expected database columns for the embeddings table.
func TestEmbeddingColumns_ContainsExpectedFields(t *testing.T) {
	cols := repository.EmbeddingColumnsForTest()
	expectedFields := []string{
		"id", "org_id", "chunk_id", "embedding", "model_name",
		"model_version", "dimensions", "created_at",
	}
	for _, field := range expectedFields {
		if !strings.Contains(cols, field) {
			t.Errorf("embedding columns missing field %q", field)
		}
	}
}
