package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// SearchRepository handles full-text search queries against the chunks table.
// All operations use a pgx.Tx with org_id set for RLS enforcement.
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
