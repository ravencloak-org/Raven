package service

import (
	"context"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// HybridRetrievalService performs parallel BM25 (PostgreSQL) and vector
// (ClickHouse QBit) searches, fusing results with Reciprocal Rank Fusion.
// It is used by enterprise-tier organisations that have migrated their
// vector embeddings from pgvector to ClickHouse.
type HybridRetrievalService struct {
	searchRepo *repository.SearchRepository
	chRepo     *repository.ClickHouseEmbeddingRepository
	pool       *pgxpool.Pool
	backend    model.VectorBackend
	threshold  int64
	logger     *slog.Logger
}

// NewHybridRetrievalService creates a new HybridRetrievalService.
// When chRepo is nil, the service falls back to pgvector for all vector searches.
func NewHybridRetrievalService(
	searchRepo *repository.SearchRepository,
	chRepo *repository.ClickHouseEmbeddingRepository,
	pool *pgxpool.Pool,
	backend model.VectorBackend,
	chunkThreshold int64,
) *HybridRetrievalService {
	return &HybridRetrievalService{
		searchRepo: searchRepo,
		chRepo:     chRepo,
		pool:       pool,
		backend:    backend,
		threshold:  chunkThreshold,
		logger:     slog.Default(),
	}
}

// ResolveBackend determines which vector backend to use for a given org.
// Enterprise orgs with ClickHouse configured and the backend set to "clickhouse"
// will use ClickHouse; all others fall back to pgvector.
func (s *HybridRetrievalService) ResolveBackend(_ string) model.VectorBackend {
	if s.chRepo == nil {
		return model.VectorBackendPgvector
	}
	if s.backend == model.VectorBackendClickHouse {
		return model.VectorBackendClickHouse
	}
	return model.VectorBackendPgvector
}

// HybridSearch performs a combined BM25 + vector search with RRF fusion.
// When the resolved backend is ClickHouse, the vector leg queries ClickHouse
// QBit columns in parallel with the PostgreSQL BM25 search.
// When the resolved backend is pgvector, it delegates to the existing
// SearchService.HybridSearch behaviour.
func (s *HybridRetrievalService) HybridSearch(
	ctx context.Context,
	orgID, kbID string,
	query string,
	embedding []float32,
	topK int,
) (*model.HybridSearchResponse, error) {
	q := sanitizeQuery(query)
	topK = clampLimit(topK)

	candidateK := topK * 3
	if candidateK > maxSearchLimit {
		candidateK = maxSearchLimit
	}

	backend := s.ResolveBackend(orgID)

	if backend == model.VectorBackendPgvector {
		return s.hybridSearchPgvector(ctx, orgID, kbID, q, embedding, topK, candidateK)
	}

	return s.hybridSearchClickHouse(ctx, orgID, kbID, q, embedding, topK, candidateK)
}

// hybridSearchPgvector runs both BM25 and pgvector searches in PostgreSQL
// within the same RLS-scoped transaction (existing behaviour).
func (s *HybridRetrievalService) hybridSearchPgvector(
	ctx context.Context,
	orgID, kbID, query string,
	embedding []float32,
	topK, candidateK int,
) (*model.HybridSearchResponse, error) {
	var vectorResults, bm25Results []model.HybridSearchResult

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var vErr, bErr error

		if len(embedding) > 0 {
			vectorResults, vErr = s.searchRepo.VectorSearch(ctx, tx, kbID, embedding, candidateK)
			if vErr != nil {
				return vErr
			}
		}
		if query != "" {
			bm25Results, bErr = s.searchRepo.BM25Search(ctx, tx, kbID, query, candidateK)
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

// hybridSearchClickHouse runs BM25 in PostgreSQL and vector search in
// ClickHouse in parallel using goroutines, then fuses with RRF.
func (s *HybridRetrievalService) hybridSearchClickHouse(
	ctx context.Context,
	orgID, kbID, query string,
	embedding []float32,
	topK, candidateK int,
) (*model.HybridSearchResponse, error) {
	var (
		wg                         sync.WaitGroup
		bm25Results                []model.HybridSearchResult
		clickhouseResults          []model.ClickHouseSearchResult
		bm25Err, clickhouseErr     error
	)

	// BM25 search in PostgreSQL.
	if query != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
				var err error
				bm25Results, err = s.searchRepo.BM25Search(ctx, tx, kbID, query, candidateK)
				return err
			})
			if err != nil {
				bm25Err = err
			}
		}()
	}

	// Vector search in ClickHouse.
	if len(embedding) > 0 && s.chRepo != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			clickhouseResults, err = s.chRepo.SearchSimilar(ctx, orgID, kbID, embedding, candidateK)
			if err != nil {
				clickhouseErr = err
			}
		}()
	}

	wg.Wait()

	if bm25Err != nil {
		s.logger.Error("BM25 search failed", "error", bm25Err)
		return nil, apierror.NewInternal("BM25 search failed: " + bm25Err.Error())
	}
	if clickhouseErr != nil {
		s.logger.Error("ClickHouse vector search failed", "error", clickhouseErr)
		return nil, apierror.NewInternal("ClickHouse vector search failed: " + clickhouseErr.Error())
	}

	// Convert ClickHouse results to the common HybridSearchResult format.
	vectorResults := make([]model.HybridSearchResult, len(clickhouseResults))
	for i, cr := range clickhouseResults {
		vectorResults[i] = model.HybridSearchResult{
			ChunkID:     cr.ChunkID,
			VectorScore: cr.Score,
		}
	}

	merged := fuseRRF(vectorResults, bm25Results, topK)

	// Enrich merged results with chunk content from PostgreSQL for any
	// ClickHouse-only results that lack content fields.
	if err := s.enrichChunkContent(ctx, orgID, merged); err != nil {
		s.logger.Warn("failed to enrich chunk content", "error", err)
		// Non-fatal: return results without content enrichment.
	}

	return &model.HybridSearchResponse{Results: merged, Total: len(merged)}, nil
}

// enrichChunkContent fetches chunk content from PostgreSQL for results
// that only have a ChunkID (from ClickHouse) but no content.
func (s *HybridRetrievalService) enrichChunkContent(ctx context.Context, orgID string, results []model.HybridSearchResult) error {
	// Collect chunk IDs that need content.
	var needContent []string
	for _, r := range results {
		if r.Content == "" && r.ChunkID != "" {
			needContent = append(needContent, r.ChunkID)
		}
	}
	if len(needContent) == 0 {
		return nil
	}

	// Fetch chunk content from PostgreSQL.
	type chunkInfo struct {
		Content         string
		OrgID           string
		KnowledgeBaseID string
		DocumentID      *string
		SourceID        *string
		ChunkIndex      int
		TokenCount      *int
		PageNumber      *int
		Heading         *string
		ChunkType       string
	}
	chunkMap := make(map[string]chunkInfo)

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id, org_id, knowledge_base_id, document_id, source_id,
				content, chunk_index, token_count, page_number, heading, chunk_type
			 FROM chunks WHERE id = ANY($1)`, needContent)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var ci chunkInfo
			if err := rows.Scan(&id, &ci.OrgID, &ci.KnowledgeBaseID, &ci.DocumentID,
				&ci.SourceID, &ci.Content, &ci.ChunkIndex, &ci.TokenCount,
				&ci.PageNumber, &ci.Heading, &ci.ChunkType); err != nil {
				return err
			}
			chunkMap[id] = ci
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}

	// Fill in missing content.
	for i := range results {
		if ci, ok := chunkMap[results[i].ChunkID]; ok && results[i].Content == "" {
			results[i].Content = ci.Content
			results[i].OrgID = ci.OrgID
			results[i].KnowledgeBaseID = ci.KnowledgeBaseID
			results[i].DocumentID = ci.DocumentID
			results[i].SourceID = ci.SourceID
			results[i].ChunkIndex = ci.ChunkIndex
			results[i].TokenCount = ci.TokenCount
			results[i].PageNumber = ci.PageNumber
			results[i].Heading = ci.Heading
			results[i].ChunkType = ci.ChunkType
		}
	}

	return nil
}

