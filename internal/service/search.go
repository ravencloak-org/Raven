package service

import (
	"context"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

const (
	defaultSearchLimit = 10
	maxSearchLimit     = 100

	// rrfK is the constant used in the Reciprocal Rank Fusion formula.
	// score = sum(1 / (k + rank_i)) for each retriever where the document appears.
	// k=60 is the standard value from the original RRF paper (Cormack et al., 2009).
	rrfK = 60
)

// SearchService contains business logic for full-text and hybrid search operations.
type SearchService struct {
	repo *repository.SearchRepository
	pool *pgxpool.Pool
}

// NewSearchService creates a new SearchService.
func NewSearchService(repo *repository.SearchRepository, pool *pgxpool.Pool) *SearchService {
	return &SearchService{repo: repo, pool: pool}
}

// sanitizeQuery trims whitespace and collapses multiple spaces.
func sanitizeQuery(q string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(q)), " ")
}

// clampLimit ensures limit is within [1, maxSearchLimit].
func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchLimit
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

// TextSearch performs a full-text search across chunks in a knowledge base.
func (s *SearchService) TextSearch(ctx context.Context, orgID, kbID, query string, limit int) (*model.SearchResponse, error) {
	q := sanitizeQuery(query)
	if q == "" {
		return &model.SearchResponse{Results: []model.ChunkWithRank{}, Total: 0}, nil
	}
	limit = clampLimit(limit)

	var results []model.ChunkWithRank
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		results, err = s.repo.TextSearch(ctx, tx, orgID, kbID, q, limit)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to search chunks: " + err.Error())
	}
	results = lo.Ternary(results == nil, []model.ChunkWithRank{}, results)
	return &model.SearchResponse{Results: results, Total: len(results)}, nil
}

// TextSearchWithFilters performs a full-text search restricted to specific documents.
func (s *SearchService) TextSearchWithFilters(ctx context.Context, orgID, kbID, query string, docIDs []string, limit int) (*model.SearchResponse, error) {
	q := sanitizeQuery(query)
	if q == "" {
		return &model.SearchResponse{Results: []model.ChunkWithRank{}, Total: 0}, nil
	}
	limit = clampLimit(limit)

	if len(docIDs) == 0 {
		return s.TextSearch(ctx, orgID, kbID, q, limit)
	}

	var results []model.ChunkWithRank
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		results, err = s.repo.TextSearchWithFilters(ctx, tx, orgID, kbID, q, docIDs, limit)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to search chunks: " + err.Error())
	}
	results = lo.Ternary(results == nil, []model.ChunkWithRank{}, results)
	return &model.SearchResponse{Results: results, Total: len(results)}, nil
}

// HybridSearch performs a combined vector similarity + BM25 keyword search,
// merging results via Reciprocal Rank Fusion (RRF). The embedding parameter
// should be pre-computed by the caller (e.g., via an embedding API).
//
// Both retrieval strategies run inside the same RLS-scoped transaction.
// Results are ranked by fused RRF score in descending order.
func (s *SearchService) HybridSearch(ctx context.Context, orgID, kbID string, query string, embedding []float32, topK int) (*model.HybridSearchResponse, error) {
	q := sanitizeQuery(query)
	topK = clampLimit(topK)

	// Both retrievers use an expanded candidate set so RRF has enough signal.
	candidateK := topK * 3
	if candidateK > maxSearchLimit {
		candidateK = maxSearchLimit
	}

	var vectorResults, bm25Results []model.HybridSearchResult

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var vErr, bErr error

		// Vector search (always run if embedding is provided).
		if len(embedding) > 0 {
			vectorResults, vErr = s.repo.VectorSearch(ctx, tx, kbID, embedding, candidateK)
			if vErr != nil {
				return vErr
			}
		}

		// BM25 search (skip if query is empty after sanitisation).
		if q != "" {
			bm25Results, bErr = s.repo.BM25Search(ctx, tx, kbID, q, candidateK)
			if bErr != nil {
				return bErr
			}
		}

		return nil
	})
	if err != nil {
		return nil, apierror.NewInternal("hybrid search failed: " + err.Error())
	}

	merged := fuseRRF(vectorResults, bm25Results, topK)

	return &model.HybridSearchResponse{Results: merged, Total: len(merged)}, nil
}

// fuseRRF merges vector and BM25 result lists using Reciprocal Rank Fusion.
//
// For each document appearing in either list, the RRF score is computed as:
//
//	score = sum(1 / (k + rank_i))
//
// where k = rrfK (60) and rank_i is the 1-based position in each retriever's
// ranked list. Documents appearing in only one list receive a single RRF
// contribution. The output is sorted by descending RRF score and truncated to
// topK results.
func fuseRRF(vectorResults, bm25Results []model.HybridSearchResult, topK int) []model.HybridSearchResult {
	type fusedEntry struct {
		result   model.HybridSearchResult
		rrfScore float64
	}

	index := make(map[string]*fusedEntry)

	// Process vector results (1-based ranking).
	for rank, vr := range vectorResults {
		entry, exists := index[vr.ChunkID]
		if !exists {
			entry = &fusedEntry{result: vr}
			index[vr.ChunkID] = entry
		}
		entry.result.VectorScore = vr.VectorScore
		entry.result.VectorRank = rank + 1
		entry.rrfScore += 1.0 / float64(rrfK+rank+1)
	}

	// Process BM25 results (1-based ranking).
	for rank, br := range bm25Results {
		entry, exists := index[br.ChunkID]
		if !exists {
			entry = &fusedEntry{result: br}
			index[br.ChunkID] = entry
		}
		entry.result.BM25Score = br.BM25Score
		entry.result.BM25Rank = rank + 1
		entry.rrfScore += 1.0 / float64(rrfK+rank+1)
	}

	// Collect and sort by RRF score descending.
	fused := make([]fusedEntry, 0, len(index))
	for _, entry := range index {
		fused = append(fused, *entry)
	}
	sort.Slice(fused, func(i, j int) bool {
		return fused[i].rrfScore > fused[j].rrfScore
	})

	// Truncate to topK and set final RRF scores.
	if len(fused) > topK {
		fused = fused[:topK]
	}

	results := make([]model.HybridSearchResult, len(fused))
	for i, f := range fused {
		results[i] = f.result
		results[i].RRFScore = f.rrfScore
	}

	return results
}

// TODO: Rerank placeholder — a future enhancement can apply a cross-encoder
// or other reranking model on top of the RRF-fused results before returning.
