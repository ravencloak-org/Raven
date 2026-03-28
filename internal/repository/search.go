package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"github.com/ravencloak-org/Raven/internal/model"
)

// SearchRepository handles full-text and vector search queries against the
// chunks and embeddings tables. All operations use a pgx.Tx with org_id set
// for RLS enforcement.
type SearchRepository struct {
	pool *pgxpool.Pool
}

// NewSearchRepository creates a new SearchRepository.
func NewSearchRepository(pool *pgxpool.Pool) *SearchRepository {
	return &SearchRepository{pool: pool}
}

const searchColumns = `c.id, c.org_id, c.knowledge_base_id, c.document_id, c.source_id,
	c.content, c.chunk_index, c.token_count, c.page_number, c.heading, c.chunk_type, c.created_at`

func scanChunkWithRank(rows pgx.Rows) ([]model.ChunkWithRank, error) {
	var results []model.ChunkWithRank
	for rows.Next() {
		var cr model.ChunkWithRank
		if err := rows.Scan(
			&cr.ID, &cr.OrgID, &cr.KnowledgeBaseID, &cr.DocumentID, &cr.SourceID,
			&cr.Content, &cr.ChunkIndex, &cr.TokenCount, &cr.PageNumber, &cr.Heading,
			&cr.ChunkType, &cr.CreatedAt, &cr.Rank, &cr.Highlight,
		); err != nil {
			return nil, fmt.Errorf("scanChunkWithRank: %w", err)
		}
		results = append(results, cr)
	}
	return results, rows.Err()
}

// TextSearch performs a full-text search across content and heading using
// ts_rank_cd for relevance ranking and ts_headline for snippet generation.
func (r *SearchRepository) TextSearch(ctx context.Context, tx pgx.Tx, orgID, kbID, query string, limit int) ([]model.ChunkWithRank, error) {
	sql := `SELECT ` + searchColumns + `,
		ts_rank_cd(to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content), q) AS rank,
		ts_headline('english', c.content, q,
			'MaxWords=35, MinWords=15, ShortWord=3, HighlightAll=FALSE') AS highlight
	FROM chunks c, plainto_tsquery('english', $1) q
	WHERE c.org_id = $2 AND c.knowledge_base_id = $3
		AND to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content) @@ q
	ORDER BY rank DESC
	LIMIT $4`

	rows, err := tx.Query(ctx, sql, query, orgID, kbID, limit)
	if err != nil {
		return nil, fmt.Errorf("SearchRepository.TextSearch: %w", err)
	}
	defer rows.Close()

	results, err := scanChunkWithRank(rows)
	if err != nil {
		return nil, fmt.Errorf("SearchRepository.TextSearch: %w", err)
	}
	return results, nil
}

// TextSearchWithFilters performs a full-text search restricted to a set of document IDs.
func (r *SearchRepository) TextSearchWithFilters(ctx context.Context, tx pgx.Tx, orgID, kbID, query string, docIDs []string, limit int) ([]model.ChunkWithRank, error) {
	sql := `SELECT ` + searchColumns + `,
		ts_rank_cd(to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content), q) AS rank,
		ts_headline('english', c.content, q,
			'MaxWords=35, MinWords=15, ShortWord=3, HighlightAll=FALSE') AS highlight
	FROM chunks c, plainto_tsquery('english', $1) q
	WHERE c.org_id = $2 AND c.knowledge_base_id = $3
		AND c.document_id = ANY($4)
		AND to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content) @@ q
	ORDER BY rank DESC
	LIMIT $5`

	rows, err := tx.Query(ctx, sql, query, orgID, kbID, docIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("SearchRepository.TextSearchWithFilters: %w", err)
	}
	defer rows.Close()

	results, err := scanChunkWithRank(rows)
	if err != nil {
		return nil, fmt.Errorf("SearchRepository.TextSearchWithFilters: %w", err)
	}
	return results, nil
}

// VectorSearch performs a cosine similarity search against the embeddings table
// using the HNSW index. It returns up to topK chunks ordered by similarity
// (highest score first). The score is computed as 1 - cosine_distance.
func (r *SearchRepository) VectorSearch(ctx context.Context, tx pgx.Tx, kbID string, embedding []float32, topK int) ([]model.HybridSearchResult, error) {
	sql := `SELECT c.id, c.org_id, c.knowledge_base_id, c.document_id, c.source_id,
			c.content, c.chunk_index, c.token_count, c.page_number, c.heading,
			c.chunk_type, c.created_at,
			1 - (e.embedding <=> $2) AS score
		FROM embeddings e
		JOIN chunks c ON c.id = e.chunk_id
		WHERE c.knowledge_base_id = $1
		ORDER BY e.embedding <=> $2
		LIMIT $3`

	rows, err := tx.Query(ctx, sql, kbID, pgvector.NewVector(embedding), topK)
	if err != nil {
		return nil, fmt.Errorf("SearchRepository.VectorSearch: %w", err)
	}
	defer rows.Close()

	var results []model.HybridSearchResult
	for rows.Next() {
		var r model.HybridSearchResult
		if err := rows.Scan(
			&r.ChunkID, &r.OrgID, &r.KnowledgeBaseID, &r.DocumentID, &r.SourceID,
			&r.Content, &r.ChunkIndex, &r.TokenCount, &r.PageNumber, &r.Heading,
			&r.ChunkType, &r.CreatedAt,
			&r.VectorScore,
		); err != nil {
			return nil, fmt.Errorf("SearchRepository.VectorSearch scan: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// BM25Search performs a full-text search using ts_rank_cd for BM25-style
// relevance ranking across content and heading columns. It returns up to topK
// chunks ordered by rank (highest first). RLS filters by org_id set on the tx.
func (r *SearchRepository) BM25Search(ctx context.Context, tx pgx.Tx, kbID, query string, topK int) ([]model.HybridSearchResult, error) {
	sql := `SELECT c.id, c.org_id, c.knowledge_base_id, c.document_id, c.source_id,
			c.content, c.chunk_index, c.token_count, c.page_number, c.heading,
			c.chunk_type, c.created_at,
			ts_rank_cd(to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content), q) AS rank
		FROM chunks c, plainto_tsquery('english', $1) q
		WHERE c.knowledge_base_id = $2
			AND to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content) @@ q
		ORDER BY rank DESC
		LIMIT $3`

	rows, err := tx.Query(ctx, sql, query, kbID, topK)
	if err != nil {
		return nil, fmt.Errorf("SearchRepository.BM25Search: %w", err)
	}
	defer rows.Close()

	var results []model.HybridSearchResult
	for rows.Next() {
		var r model.HybridSearchResult
		if err := rows.Scan(
			&r.ChunkID, &r.OrgID, &r.KnowledgeBaseID, &r.DocumentID, &r.SourceID,
			&r.Content, &r.ChunkIndex, &r.TokenCount, &r.PageNumber, &r.Heading,
			&r.ChunkType, &r.CreatedAt,
			&r.BM25Score,
		); err != nil {
			return nil, fmt.Errorf("SearchRepository.BM25Search scan: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
