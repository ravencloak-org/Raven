package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SemanticCacheRepository manages the response_cache table.
type SemanticCacheRepository struct {
	db *pgxpool.Pool
}

// NewSemanticCacheRepository creates a new SemanticCacheRepository.
func NewSemanticCacheRepository(db *pgxpool.Pool) *SemanticCacheRepository {
	return &SemanticCacheRepository{db: db}
}

// InvalidateKB deletes all cache entries for a knowledge base.
// It applies the org RLS GUC inside an explicit transaction and returns the number of rows deleted.
func (r *SemanticCacheRepository) InvalidateKB(ctx context.Context, orgID, kbID string) (int64, error) {
	conn, err := r.db.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("SemanticCacheRepository.InvalidateKB acquire: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("SemanticCacheRepository.InvalidateKB begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID)
	if err != nil {
		return 0, fmt.Errorf("SemanticCacheRepository.InvalidateKB set_config: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`DELETE FROM response_cache WHERE org_id = $1::uuid AND kb_id = $2::uuid`,
		orgID, kbID,
	)
	if err != nil {
		return 0, fmt.Errorf("SemanticCacheRepository.InvalidateKB delete: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("SemanticCacheRepository.InvalidateKB commit: %w", err)
	}

	return tag.RowsAffected(), nil
}

// Stats returns the active entry count and average hit count for a KB.
func (r *SemanticCacheRepository) Stats(ctx context.Context, orgID, kbID string) (count int64, avgHits float64, err error) {
	conn, err := r.db.Acquire(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("SemanticCacheRepository.Stats acquire: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("SemanticCacheRepository.Stats begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID)
	if err != nil {
		return 0, 0, fmt.Errorf("SemanticCacheRepository.Stats set_config: %w", err)
	}

	row := tx.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(AVG(hit_count), 0)
		 FROM response_cache
		 WHERE org_id = $1::uuid AND kb_id = $2::uuid AND expires_at > NOW()`,
		orgID, kbID,
	)

	if err = row.Scan(&count, &avgHits); err != nil {
		return 0, 0, fmt.Errorf("SemanticCacheRepository.Stats scan: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("SemanticCacheRepository.Stats commit: %w", err)
	}

	return count, avgHits, nil
}
