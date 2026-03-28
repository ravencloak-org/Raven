package service

import (
	"math"
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
)

func TestSanitizeQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trims whitespace", "  hello world  ", "hello world"},
		{"collapses spaces", "hello    world", "hello world"},
		{"handles tabs and newlines", "hello\t\n  world", "hello world"},
		{"empty string", "", ""},
		{"only whitespace", "   ", ""},
		{"single word", "test", "test"},
		{"preserves inner content", "multi word search query", "multi word search query"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeQuery(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"zero returns default", 0, defaultSearchLimit},
		{"negative returns default", -5, defaultSearchLimit},
		{"within range", 25, 25},
		{"at max", maxSearchLimit, maxSearchLimit},
		{"over max returns max", 200, maxSearchLimit},
		{"one is valid", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampLimit(tt.input)
			if got != tt.want {
				t.Errorf("clampLimit(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// almostEqual checks whether two float64 values are within epsilon.
func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestFuseRRF_BothRetrieversOverlap(t *testing.T) {
	// Chunk A appears at rank 1 in both lists.
	// Chunk B appears at rank 2 in vector, rank 3 in BM25.
	// Chunk C appears only in vector at rank 3.
	// Chunk D appears only in BM25 at rank 2.
	vectorResults := []model.HybridSearchResult{
		{ChunkID: "A", Content: "chunk A", VectorScore: 0.95},
		{ChunkID: "B", Content: "chunk B", VectorScore: 0.85},
		{ChunkID: "C", Content: "chunk C", VectorScore: 0.75},
	}
	bm25Results := []model.HybridSearchResult{
		{ChunkID: "A", Content: "chunk A", BM25Score: 3.2},
		{ChunkID: "D", Content: "chunk D", BM25Score: 2.8},
		{ChunkID: "B", Content: "chunk B", BM25Score: 1.5},
	}

	results := fuseRRF(vectorResults, bm25Results, 10)

	// Expected RRF scores (k=60):
	// A: 1/(60+1) + 1/(60+1) = 2/61 ~= 0.032787
	// B: 1/(60+2) + 1/(60+3) = 1/62 + 1/63 ~= 0.032014
	// D: 1/(60+2) = 1/62 ~= 0.016129
	// C: 1/(60+3) = 1/63 ~= 0.015873

	const eps = 1e-6

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Verify ordering: A > B > D > C
	expectedOrder := []string{"A", "B", "D", "C"}
	for i, want := range expectedOrder {
		if results[i].ChunkID != want {
			t.Errorf("result[%d].ChunkID = %q, want %q", i, results[i].ChunkID, want)
		}
	}

	// Verify A's RRF score.
	expectedA := 2.0 / 61.0
	if !almostEqual(results[0].RRFScore, expectedA, eps) {
		t.Errorf("A RRF score = %f, want %f", results[0].RRFScore, expectedA)
	}

	// Verify A retains both vector and BM25 metadata.
	if results[0].VectorRank != 1 {
		t.Errorf("A VectorRank = %d, want 1", results[0].VectorRank)
	}
	if results[0].BM25Rank != 1 {
		t.Errorf("A BM25Rank = %d, want 1", results[0].BM25Rank)
	}

	// Verify B's RRF score.
	expectedB := 1.0/62.0 + 1.0/63.0
	if !almostEqual(results[1].RRFScore, expectedB, eps) {
		t.Errorf("B RRF score = %f, want %f", results[1].RRFScore, expectedB)
	}

	// Verify D (BM25-only) has no vector rank.
	if results[2].ChunkID != "D" {
		t.Fatalf("expected D at position 2, got %q", results[2].ChunkID)
	}
	if results[2].VectorRank != 0 {
		t.Errorf("D VectorRank = %d, want 0 (not in vector results)", results[2].VectorRank)
	}
	if results[2].BM25Rank != 2 {
		t.Errorf("D BM25Rank = %d, want 2", results[2].BM25Rank)
	}

	// Verify C (vector-only) has no BM25 rank.
	if results[3].ChunkID != "C" {
		t.Fatalf("expected C at position 3, got %q", results[3].ChunkID)
	}
	if results[3].BM25Rank != 0 {
		t.Errorf("C BM25Rank = %d, want 0 (not in BM25 results)", results[3].BM25Rank)
	}
	if results[3].VectorRank != 3 {
		t.Errorf("C VectorRank = %d, want 3", results[3].VectorRank)
	}
}

func TestFuseRRF_TopKTruncation(t *testing.T) {
	vectorResults := []model.HybridSearchResult{
		{ChunkID: "A", VectorScore: 0.9},
		{ChunkID: "B", VectorScore: 0.8},
		{ChunkID: "C", VectorScore: 0.7},
		{ChunkID: "D", VectorScore: 0.6},
		{ChunkID: "E", VectorScore: 0.5},
	}

	results := fuseRRF(vectorResults, nil, 3)

	if len(results) != 3 {
		t.Fatalf("expected 3 results after topK truncation, got %d", len(results))
	}

	// Highest RRF scores should be the first three by rank.
	for i, r := range results {
		expectedScore := 1.0 / float64(rrfK+i+1)
		if !almostEqual(r.RRFScore, expectedScore, 1e-9) {
			t.Errorf("result[%d] RRF score = %f, want %f", i, r.RRFScore, expectedScore)
		}
	}
}

func TestFuseRRF_EmptyInputs(t *testing.T) {
	tests := []struct {
		name    string
		vector  []model.HybridSearchResult
		bm25    []model.HybridSearchResult
		wantLen int
	}{
		{"both empty", nil, nil, 0},
		{"vector empty", nil, []model.HybridSearchResult{{ChunkID: "A"}}, 1},
		{"bm25 empty", []model.HybridSearchResult{{ChunkID: "A"}}, nil, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := fuseRRF(tt.vector, tt.bm25, 10)
			if len(results) != tt.wantLen {
				t.Errorf("got %d results, want %d", len(results), tt.wantLen)
			}
		})
	}
}

func TestFuseRRF_VectorOnlyResults(t *testing.T) {
	vectorResults := []model.HybridSearchResult{
		{ChunkID: "X", VectorScore: 0.99},
		{ChunkID: "Y", VectorScore: 0.88},
	}

	results := fuseRRF(vectorResults, nil, 10)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// X should rank higher (rank 1 vs rank 2).
	if results[0].ChunkID != "X" {
		t.Errorf("expected X first, got %q", results[0].ChunkID)
	}
	if results[0].VectorRank != 1 {
		t.Errorf("X VectorRank = %d, want 1", results[0].VectorRank)
	}
	if results[0].BM25Rank != 0 {
		t.Errorf("X BM25Rank = %d, want 0", results[0].BM25Rank)
	}

	expectedX := 1.0 / float64(rrfK+1)
	if !almostEqual(results[0].RRFScore, expectedX, 1e-9) {
		t.Errorf("X RRF score = %f, want %f", results[0].RRFScore, expectedX)
	}
}

func TestFuseRRF_BM25OnlyResults(t *testing.T) {
	bm25Results := []model.HybridSearchResult{
		{ChunkID: "P", BM25Score: 5.0},
		{ChunkID: "Q", BM25Score: 3.0},
	}

	results := fuseRRF(nil, bm25Results, 10)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].ChunkID != "P" {
		t.Errorf("expected P first, got %q", results[0].ChunkID)
	}
	if results[0].BM25Rank != 1 {
		t.Errorf("P BM25Rank = %d, want 1", results[0].BM25Rank)
	}
	if results[0].VectorRank != 0 {
		t.Errorf("P VectorRank = %d, want 0", results[0].VectorRank)
	}
}

func TestFuseRRF_IdenticalRanksDifferentRetrievers(t *testing.T) {
	// Two chunks that each appear at rank 1 in different retrievers.
	// They should have equal RRF scores since each has one contribution at rank 1.
	vectorResults := []model.HybridSearchResult{
		{ChunkID: "A", VectorScore: 0.95},
	}
	bm25Results := []model.HybridSearchResult{
		{ChunkID: "B", BM25Score: 4.0},
	}

	results := fuseRRF(vectorResults, bm25Results, 10)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Both should have identical RRF scores: 1/(60+1).
	expectedScore := 1.0 / float64(rrfK+1)
	if !almostEqual(results[0].RRFScore, expectedScore, 1e-9) {
		t.Errorf("result[0] RRF = %f, want %f", results[0].RRFScore, expectedScore)
	}
	if !almostEqual(results[1].RRFScore, expectedScore, 1e-9) {
		t.Errorf("result[1] RRF = %f, want %f", results[1].RRFScore, expectedScore)
	}
}

func TestFuseRRF_PreservesScoreMetadata(t *testing.T) {
	vectorResults := []model.HybridSearchResult{
		{ChunkID: "A", VectorScore: 0.92, Content: "vector content A"},
	}
	bm25Results := []model.HybridSearchResult{
		{ChunkID: "A", BM25Score: 3.5, Content: "bm25 content A"},
	}

	results := fuseRRF(vectorResults, bm25Results, 10)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.VectorScore != 0.92 {
		t.Errorf("VectorScore = %f, want 0.92", r.VectorScore)
	}
	if r.BM25Score != 3.5 {
		t.Errorf("BM25Score = %f, want 3.5", r.BM25Score)
	}
	if r.VectorRank != 1 {
		t.Errorf("VectorRank = %d, want 1", r.VectorRank)
	}
	if r.BM25Rank != 1 {
		t.Errorf("BM25Rank = %d, want 1", r.BM25Rank)
	}

	// Both at rank 1: RRF = 2/(60+1).
	expectedRRF := 2.0 / float64(rrfK+1)
	if !almostEqual(r.RRFScore, expectedRRF, 1e-9) {
		t.Errorf("RRF score = %f, want %f", r.RRFScore, expectedRRF)
	}
}

func TestRRFKConstant(t *testing.T) {
	// Verify the RRF constant matches the standard value from the paper.
	if rrfK != 60 {
		t.Errorf("rrfK = %d, want 60 (standard RRF constant)", rrfK)
	}
}
