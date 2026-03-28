package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// SourceRepository handles database operations for sources.
// All operations use a pgx.Tx with org_id set for RLS enforcement.
type SourceRepository struct {
	pool *pgxpool.Pool
}

// NewSourceRepository creates a new SourceRepository.
func NewSourceRepository(pool *pgxpool.Pool) *SourceRepository {
	return &SourceRepository{pool: pool}
}

const sourceColumns = `id, org_id, knowledge_base_id, source_type, url,
	crawl_depth, crawl_frequency, processing_status,
	COALESCE(processing_error, '') AS processing_error,
	COALESCE(title, '') AS title,
	pages_crawled, COALESCE(metadata, '{}') AS metadata,
	COALESCE(created_by::text, '') AS created_by,
	created_at, updated_at`

func scanSource(row pgx.Row) (*model.Source, error) {
	var s model.Source
	err := row.Scan(
		&s.ID,
		&s.OrgID,
		&s.KnowledgeBaseID,
		&s.SourceType,
		&s.URL,
		&s.CrawlDepth,
		&s.CrawlFrequency,
		&s.ProcessingStatus,
		&s.ProcessingError,
		&s.Title,
		&s.PagesCrawled,
		&s.Metadata,
		&s.CreatedBy,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Create inserts a new source within a transaction.
func (r *SourceRepository) Create(ctx context.Context, tx pgx.Tx, orgID, kbID string, req model.CreateSourceRequest, createdBy string) (*model.Source, error) {
	crawlDepth := 1
	if req.CrawlDepth != nil {
		crawlDepth = *req.CrawlDepth
	}
	crawlFrequency := model.CrawlFrequencyManual
	if req.CrawlFrequency != "" {
		crawlFrequency = req.CrawlFrequency
	}

	row := tx.QueryRow(ctx,
		`INSERT INTO sources (org_id, knowledge_base_id, source_type, url, crawl_depth, crawl_frequency, title, metadata, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), COALESCE($8::jsonb, '{}'), NULLIF($9, '')::uuid)
		 RETURNING `+sourceColumns,
		orgID, kbID, req.SourceType, req.URL, crawlDepth, crawlFrequency, req.Title, req.Metadata, createdBy,
	)
	s, err := scanSource(row)
	if err != nil {
		return nil, fmt.Errorf("SourceRepository.Create: %w", err)
	}
	return s, nil
}

// GetByID fetches a source by its primary key within an org.
func (r *SourceRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, sourceID string) (*model.Source, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+sourceColumns+`
		 FROM sources
		 WHERE id = $1 AND org_id = $2`,
		sourceID, orgID,
	)
	s, err := scanSource(row)
	if err != nil {
		return nil, fmt.Errorf("SourceRepository.GetByID: %w", err)
	}
	return s, nil
}

// List returns a paginated list of sources for a knowledge base.
func (r *SourceRepository) List(ctx context.Context, tx pgx.Tx, orgID, kbID string, page, pageSize int) ([]model.Source, int, error) {
	var total int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM sources WHERE org_id = $1 AND knowledge_base_id = $2`,
		orgID, kbID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("SourceRepository.List count: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := tx.Query(ctx,
		`SELECT `+sourceColumns+`
		 FROM sources
		 WHERE org_id = $1 AND knowledge_base_id = $2
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4`,
		orgID, kbID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("SourceRepository.List query: %w", err)
	}
	defer rows.Close()

	var sources []model.Source
	for rows.Next() {
		var s model.Source
		if err := rows.Scan(
			&s.ID, &s.OrgID, &s.KnowledgeBaseID, &s.SourceType, &s.URL,
			&s.CrawlDepth, &s.CrawlFrequency, &s.ProcessingStatus,
			&s.ProcessingError, &s.Title, &s.PagesCrawled, &s.Metadata,
			&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("SourceRepository.List scan: %w", err)
		}
		sources = append(sources, s)
	}
	return sources, total, rows.Err()
}

// Update applies partial updates to a source.
func (r *SourceRepository) Update(ctx context.Context, tx pgx.Tx, orgID, sourceID string, req model.UpdateSourceRequest) (*model.Source, error) {
	row := tx.QueryRow(ctx,
		`UPDATE sources
		 SET
		   url             = COALESCE($3, url),
		   crawl_depth     = COALESCE($4, crawl_depth),
		   crawl_frequency = COALESCE($5, crawl_frequency),
		   title           = COALESCE($6, title),
		   metadata        = CASE WHEN $7::jsonb IS NOT NULL THEN $7::jsonb ELSE metadata END,
		   updated_at      = NOW()
		 WHERE id = $1 AND org_id = $2
		 RETURNING `+sourceColumns,
		sourceID, orgID, req.URL, req.CrawlDepth, req.CrawlFrequency, req.Title, req.Metadata,
	)
	s, err := scanSource(row)
	if err != nil {
		return nil, fmt.Errorf("SourceRepository.Update: %w", err)
	}
	return s, nil
}

// Delete permanently removes a source.
func (r *SourceRepository) Delete(ctx context.Context, tx pgx.Tx, orgID, sourceID string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM sources WHERE id = $1 AND org_id = $2`,
		sourceID, orgID,
	)
	if err != nil {
		return fmt.Errorf("SourceRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("SourceRepository.Delete: source %s not found", sourceID)
	}
	return nil
}
