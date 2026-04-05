package service

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
)

func TestResolveBackend_NilClickHouse(t *testing.T) {
	svc := NewHybridRetrievalService(nil, nil, nil, model.VectorBackendClickHouse, 5000000)
	got := svc.ResolveBackend("org-1")
	if got != model.VectorBackendPgvector {
		t.Errorf("ResolveBackend with nil chRepo = %q, want %q", got, model.VectorBackendPgvector)
	}
}

func TestResolveBackend_PgvectorDefault(t *testing.T) {
	svc := NewHybridRetrievalService(nil, nil, nil, model.VectorBackendPgvector, 5000000)
	got := svc.ResolveBackend("org-1")
	if got != model.VectorBackendPgvector {
		t.Errorf("ResolveBackend = %q, want %q", got, model.VectorBackendPgvector)
	}
}

func TestFuseRRFWithSort_ParallelResults(t *testing.T) {
	// Simulate results from ClickHouse vector + PostgreSQL BM25.
	vectorResults := []model.HybridSearchResult{
		{ChunkID: "A", VectorScore: 0.95},
		{ChunkID: "B", VectorScore: 0.85},
	}
	bm25Results := []model.HybridSearchResult{
		{ChunkID: "A", BM25Score: 3.0},
		{ChunkID: "C", BM25Score: 2.5},
	}

	results := fuseRRF(vectorResults, bm25Results, 10)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// A should be first (appears in both lists).
	if results[0].ChunkID != "A" {
		t.Errorf("expected A first, got %q", results[0].ChunkID)
	}

	expectedA := 2.0 / 61.0
	if !almostEqual(results[0].RRFScore, expectedA, 1e-6) {
		t.Errorf("A RRF score = %f, want %f", results[0].RRFScore, expectedA)
	}

	// Verify A has both vector and BM25 metadata.
	if results[0].VectorRank != 1 {
		t.Errorf("A VectorRank = %d, want 1", results[0].VectorRank)
	}
	if results[0].BM25Rank != 1 {
		t.Errorf("A BM25Rank = %d, want 1", results[0].BM25Rank)
	}
}

func TestFuseRRFWithSort_EmptyInputs(t *testing.T) {
	results := fuseRRF(nil, nil, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil inputs, got %d", len(results))
	}
}

func TestFuseRRFWithSort_TopKTruncation(t *testing.T) {
	var vector []model.HybridSearchResult
	for i := 0; i < 20; i++ {
		vector = append(vector, model.HybridSearchResult{
			ChunkID:     "chunk-" + string(rune('A'+i)),
			VectorScore: float64(20-i) / 20.0,
		})
	}

	results := fuseRRF(vector, nil, 5)
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

func TestVectorBackendConstants(t *testing.T) {
	if model.VectorBackendPgvector != "pgvector" {
		t.Errorf("VectorBackendPgvector = %q, want %q", model.VectorBackendPgvector, "pgvector")
	}
	if model.VectorBackendClickHouse != "clickhouse" {
		t.Errorf("VectorBackendClickHouse = %q, want %q", model.VectorBackendClickHouse, "clickhouse")
	}
}
