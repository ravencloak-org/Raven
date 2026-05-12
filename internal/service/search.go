package service

import (
	"context"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// Compile-time defaults used by the package-level helpers (`clampLimit`,
// `fuseRRF`) and as the fallback when a SearchService is constructed without
// an explicit RetrievalConfig. The runtime-effective values come from
// `config.RetrievalConfig`, which is wired in `cmd/api/main.go` from the
// RAVEN_RETRIEVAL_* env vars — see internal/config/config.go.
const (
	defaultSearchLimit = 10
	maxSearchLimit     = 100

	// rrfK is the constant used in the Reciprocal Rank Fusion formula.
	// score = sum(1 / (k + rank_i)) for each retriever where the document appears.
	// k=60 is the standard value from the original RRF paper (Cormack et al., 2009).
	rrfK = 60

	// defaultCandidateMultiplier mirrors the historical `candidateK = topK * 3`
	// behaviour from before retrieval tuning was made env-configurable.
	defaultCandidateMultiplier = 3
)

// defaultRetrievalConfig returns the RetrievalConfig used when a
// SearchService is constructed without an explicit config (e.g. integration
// tests). It must stay in sync with the package-level constants above and
// with the `retrieval.*` defaults in config.Load.
func defaultRetrievalConfig() config.RetrievalConfig {
	return config.RetrievalConfig{
		DefaultLimit:        defaultSearchLimit,
		MaxLimit:            maxSearchLimit,
		RRFK:                rrfK,
		CandidateMultiplier: defaultCandidateMultiplier,
		HybridVectorWeight:  1.0,
		HybridBM25Weight:    1.0,
	}
}

// SearchService contains business logic for full-text and hybrid search operations.
type SearchService struct {
	repo *repository.SearchRepository
	pool *pgxpool.Pool
	cfg  config.RetrievalConfig
}

// NewSearchService creates a new SearchService. The retrieval cfg controls
// topK clamping, the RRF smoothing constant `k`, candidate-set expansion, and
// per-leg hybrid weights — see config.RetrievalConfig. Zero-valued fields are
// replaced with the package defaults (defaultSearchLimit, maxSearchLimit,
// rrfK, defaultCandidateMultiplier, 1.0, 1.0) so callers that haven't been
// updated still get the historical behaviour.
func NewSearchService(repo *repository.SearchRepository, pool *pgxpool.Pool, cfg config.RetrievalConfig) *SearchService {
	return &SearchService{repo: repo, pool: pool, cfg: normaliseRetrievalConfig(cfg)}
}

// RetrievalLimits exposes the effective DefaultLimit and MaxLimit so callers
// (e.g. the hybrid-search HTTP handler) can echo the clamped top_k back to
// clients without duplicating the clamp logic. Returns the post-normalised
// values, i.e. the same numbers HybridSearch will actually use.
func (s *SearchService) RetrievalLimits() (defaultLimit, maxLimit int) {
	return s.cfg.DefaultLimit, s.cfg.MaxLimit
}

// normaliseRetrievalConfig backfills zero-valued fields with the historical
// defaults. Negative weights are clamped to zero — this matches the
// config-time validation but keeps tests that construct a SearchService
// directly (without going through config.Load) safe.
func normaliseRetrievalConfig(cfg config.RetrievalConfig) config.RetrievalConfig {
	d := defaultRetrievalConfig()
	if cfg.DefaultLimit <= 0 {
		cfg.DefaultLimit = d.DefaultLimit
	}
	if cfg.MaxLimit <= 0 {
		cfg.MaxLimit = d.MaxLimit
	}
	if cfg.MaxLimit < cfg.DefaultLimit {
		cfg.MaxLimit = cfg.DefaultLimit
	}
	if cfg.RRFK <= 0 {
		cfg.RRFK = d.RRFK
	}
	if cfg.CandidateMultiplier <= 0 {
		cfg.CandidateMultiplier = d.CandidateMultiplier
	}
	if cfg.HybridVectorWeight < 0 {
		cfg.HybridVectorWeight = 0
	}
	if cfg.HybridBM25Weight < 0 {
		cfg.HybridBM25Weight = 0
	}
	// A zero-valued weight (caller intentionally disabling a leg) is left
	// alone; we only refuse negatives.
	if cfg.HybridVectorWeight == 0 && cfg.HybridBM25Weight == 0 {
		// Both zero would null out every fused score; restore historical
		// equal-weight behaviour rather than returning empty results.
		cfg.HybridVectorWeight = d.HybridVectorWeight
		cfg.HybridBM25Weight = d.HybridBM25Weight
	}
	return cfg
}

// sanitizeQuery trims whitespace and collapses multiple spaces.
func sanitizeQuery(q string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(q)), " ")
}

// clampLimit ensures limit is within [1, maxSearchLimit] using the package
// defaults. Used by the TextSearch* methods, which never had a per-instance
// override, and by the existing unit tests.
func clampLimit(limit int) int {
	return clampLimitWith(limit, defaultSearchLimit, maxSearchLimit)
}

// clampLimitWith is the configurable equivalent of clampLimit. Used by
// HybridSearch so the limits respect the RAVEN_RETRIEVAL_* env vars.
func clampLimitWith(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
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
	topK = clampLimitWith(topK, s.cfg.DefaultLimit, s.cfg.MaxLimit)

	// Both retrievers use an expanded candidate set so RRF has enough signal.
	candidateK := topK * s.cfg.CandidateMultiplier
	if candidateK > s.cfg.MaxLimit {
		candidateK = s.cfg.MaxLimit
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

	merged := fuseRRFWith(vectorResults, bm25Results, topK, s.cfg.RRFK, s.cfg.HybridVectorWeight, s.cfg.HybridBM25Weight)

	return &model.HybridSearchResponse{Results: merged, Total: len(merged)}, nil
}

// fuseRRF merges vector and BM25 result lists using Reciprocal Rank Fusion
// with the package-level defaults (k = rrfK, equal leg weights of 1.0).
//
// It is a thin wrapper over fuseRRFWith kept for the existing unit tests in
// search_test.go and for any caller that does not have a RetrievalConfig.
// New code in this package should call fuseRRFWith directly with explicit
// `k`, `vectorWeight`, and `bm25Weight` values so RAVEN_RETRIEVAL_* env vars
// flow through end-to-end.
func fuseRRF(vectorResults, bm25Results []model.HybridSearchResult, topK int) []model.HybridSearchResult {
	return fuseRRFWith(vectorResults, bm25Results, topK, rrfK, 1.0, 1.0)
}

// fuseRRFWith is the parametrised RRF implementation. For each document
// appearing in either list, the RRF score is computed as:
//
//	score = vectorWeight * 1/(k + rank_vec) + bm25Weight * 1/(k + rank_bm25)
//
// where rank_* is the 1-based position in the respective retriever's ranked
// list, k is the RRF smoothing constant, and a missing rank contributes
// nothing. The output is sorted by descending fused score and truncated to
// topK results.
func fuseRRFWith(vectorResults, bm25Results []model.HybridSearchResult, topK, k int, vectorWeight, bm25Weight float64) []model.HybridSearchResult {
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
		entry.rrfScore += vectorWeight / float64(k+rank+1)
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
		entry.rrfScore += bm25Weight / float64(k+rank+1)
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
