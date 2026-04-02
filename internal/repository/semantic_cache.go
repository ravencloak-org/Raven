package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
)

// SemanticCacheRepository manages the response_cache table.
type SemanticCacheRepository struct {
	pool *pgxpool.Pool
}

// NewSemanticCacheRepository creates a new SemanticCacheRepository.
func NewSemanticCacheRepository(pool *pgxpool.Pool) *SemanticCacheRepository {
	return &SemanticCacheRepository{pool: pool}
}

// InvalidateKB deletes all cache entries for a knowledge base.
// It applies the org RLS GUC inside an explicit transaction and returns the number of rows deleted.
func (r *SemanticCacheRepository) InvalidateKB(ctx context.Context, orgID, kbID string) (int64, error) {
	var rowsAffected int64
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM response_cache WHERE org_id = $1::uuid AND kb_id = $2::uuid`,
			orgID, kbID,
		)
		if err != nil {
			return fmt.Errorf("delete: %w", err)
		}
		rowsAffected = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("SemanticCacheRepository.InvalidateKB: %w", err)
	}
	return rowsAffected, nil
}

// Stats returns the active entry count and average hit count for a KB.
func (r *SemanticCacheRepository) Stats(ctx context.Context, orgID, kbID string) (count int64, avgHits float64, err error) {
	err = db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT COUNT(*), COALESCE(AVG(hit_count), 0)
			 FROM response_cache
			 WHERE org_id = $1::uuid AND kb_id = $2::uuid AND expires_at > NOW()`,
			orgID, kbID,
		)
		if scanErr := row.Scan(&count, &avgHits); scanErr != nil {
			return fmt.Errorf("scan: %w", scanErr)
		}
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("SemanticCacheRepository.Stats: %w", err)
	}
	return count, avgHits, nil
}
