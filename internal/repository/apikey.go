package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// APIKeyRepository handles database operations for API keys.
type APIKeyRepository struct {
	pool *pgxpool.Pool
}

// NewAPIKeyRepository creates a new APIKeyRepository.
func NewAPIKeyRepository(pool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{pool: pool}
}

const apiKeySelectCols = `id, org_id, COALESCE(workspace_id::text, '') AS workspace_id,
	knowledge_base_id, name, key_hash, key_prefix,
	COALESCE(allowed_domains, '{}') AS allowed_domains,
	COALESCE(rate_limit, 60) AS rate_limit, status,
	COALESCE(created_by::text, '') AS created_by,
	created_at, expires_at`

const (
	queryAPIKeyInsert      = `INSERT INTO api_keys (org_id, workspace_id, knowledge_base_id, name, key_hash, key_prefix, allowed_domains, rate_limit, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING ` + apiKeySelectCols
	queryAPIKeyByHash      = `SELECT ` + apiKeySelectCols + ` FROM api_keys WHERE key_hash = $1 AND status = 'active'`
	queryAPIKeyByID        = `SELECT ` + apiKeySelectCols + ` FROM api_keys WHERE id = $1 AND org_id = $2`
	queryAPIKeyListByKB    = `SELECT ` + apiKeySelectCols + ` FROM api_keys WHERE org_id = $1 AND knowledge_base_id = $2 ORDER BY created_at DESC`
	queryAPIKeyRevoke      = `UPDATE api_keys SET status = 'revoked' WHERE id = $1 AND org_id = $2 AND status = 'active'`
)

func scanAPIKey(row pgx.Row) (*model.APIKey, error) {
	var ak model.APIKey
	err := row.Scan(
		&ak.ID,
		&ak.OrgID,
		&ak.WorkspaceID,
		&ak.KnowledgeBaseID,
		&ak.Name,
		&ak.KeyHash,
		&ak.KeyPrefix,
		&ak.AllowedDomains,
		&ak.RateLimit,
		&ak.Status,
		&ak.CreatedBy,
		&ak.CreatedAt,
		&ak.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	if ak.AllowedDomains == nil {
		ak.AllowedDomains = []string{}
	}
	return &ak, nil
}

// Create inserts a new API key record.
func (r *APIKeyRepository) Create(ctx context.Context, tx pgx.Tx, orgID, wsID, kbID, name, keyHash, keyPrefix, createdBy string, allowedDomains []string, rateLimit int) (*model.APIKey, error) {
	if allowedDomains == nil {
		allowedDomains = []string{}
	}
	row := tx.QueryRow(ctx, queryAPIKeyInsert,
		orgID, wsID, kbID, name, keyHash, keyPrefix, allowedDomains, rateLimit, createdBy,
	)
	ak, err := scanAPIKey(row)
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepository.Create: %w", err)
	}
	return ak, nil
}

// GetByKeyHash looks up an active API key by its SHA-256 hash.
func (r *APIKeyRepository) GetByKeyHash(ctx context.Context, tx pgx.Tx, keyHash string) (*model.APIKey, error) {
	row := tx.QueryRow(ctx, queryAPIKeyByHash, keyHash)
	ak, err := scanAPIKey(row)
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepository.GetByKeyHash: %w", err)
	}
	return ak, nil
}

// GetByID fetches an API key by primary key within an org.
func (r *APIKeyRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, id string) (*model.APIKey, error) {
	row := tx.QueryRow(ctx, queryAPIKeyByID, id, orgID)
	ak, err := scanAPIKey(row)
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepository.GetByID: %w", err)
	}
	return ak, nil
}

// ListByKB returns all API keys for a knowledge base.
func (r *APIKeyRepository) ListByKB(ctx context.Context, tx pgx.Tx, orgID, kbID string) ([]model.APIKey, error) {
	rows, err := tx.Query(ctx, queryAPIKeyListByKB, orgID, kbID)
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepository.ListByKB: %w", err)
	}
	defer rows.Close()

	var keys []model.APIKey
	for rows.Next() {
		var ak model.APIKey
		if err := rows.Scan(
			&ak.ID, &ak.OrgID, &ak.WorkspaceID,
			&ak.KnowledgeBaseID, &ak.Name, &ak.KeyHash, &ak.KeyPrefix,
			&ak.AllowedDomains, &ak.RateLimit, &ak.Status,
			&ak.CreatedBy, &ak.CreatedAt, &ak.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("APIKeyRepository.ListByKB scan: %w", err)
		}
		if ak.AllowedDomains == nil {
			ak.AllowedDomains = []string{}
		}
		keys = append(keys, ak)
	}
	return keys, rows.Err()
}

// Revoke sets an API key status to 'revoked'.
func (r *APIKeyRepository) Revoke(ctx context.Context, tx pgx.Tx, orgID, id string) error {
	tag, err := tx.Exec(ctx, queryAPIKeyRevoke, id, orgID)
	if err != nil {
		return fmt.Errorf("APIKeyRepository.Revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("APIKeyRepository.Revoke: api key %s not found or already revoked", id)
	}
	return nil
}

// GetByKeyHashNoTx looks up an active API key by its SHA-256 hash without
// requiring a caller-provided transaction. It acquires its own short-lived
// transaction to ensure the connection is in a clean, properly scoped state
// (no stale app.current_org_id GUC from a previous request). Designed for use
// by the auth middleware where no existing transaction context is available.
func (r *APIKeyRepository) GetByKeyHashNoTx(ctx context.Context, keyHash string) (*model.APIKey, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepository.GetByKeyHashNoTx begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	ak, err := scanAPIKey(tx.QueryRow(ctx, queryAPIKeyByHash, keyHash))
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepository.GetByKeyHashNoTx: %w", err)
	}

	// Set the org GUC so RLS is properly scoped for the duration of this tx.
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_org_id', $1, true)`, ak.OrgID); err != nil {
		return nil, fmt.Errorf("APIKeyRepository.GetByKeyHashNoTx set_config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("APIKeyRepository.GetByKeyHashNoTx commit: %w", err)
	}
	return ak, nil
}
