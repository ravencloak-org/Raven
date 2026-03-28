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

const apiKeyColumns = `id, org_id, COALESCE(workspace_id::text, '') AS workspace_id,
	knowledge_base_id, name, key_hash, key_prefix,
	COALESCE(allowed_domains, '{}') AS allowed_domains,
	COALESCE(rate_limit, 60) AS rate_limit, status,
	COALESCE(created_by::text, '') AS created_by,
	created_at, expires_at`

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
	return &ak, nil
}

// Create inserts a new API key record.
func (r *APIKeyRepository) Create(ctx context.Context, tx pgx.Tx, orgID, wsID, kbID, name, keyHash, keyPrefix, createdBy string, allowedDomains []string, rateLimit int) (*model.APIKey, error) {
	if allowedDomains == nil {
		allowedDomains = []string{}
	}
	row := tx.QueryRow(ctx,
		`INSERT INTO api_keys (org_id, workspace_id, knowledge_base_id, name, key_hash, key_prefix, allowed_domains, rate_limit, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING `+apiKeyColumns,
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
	row := tx.QueryRow(ctx,
		`SELECT `+apiKeyColumns+`
		 FROM api_keys
		 WHERE key_hash = $1 AND status = 'active'`,
		keyHash,
	)
	ak, err := scanAPIKey(row)
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepository.GetByKeyHash: %w", err)
	}
	return ak, nil
}

// GetByID fetches an API key by primary key within an org.
func (r *APIKeyRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, id string) (*model.APIKey, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+apiKeyColumns+`
		 FROM api_keys
		 WHERE id = $1 AND org_id = $2`,
		id, orgID,
	)
	ak, err := scanAPIKey(row)
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepository.GetByID: %w", err)
	}
	return ak, nil
}

// ListByKB returns all API keys for a knowledge base.
func (r *APIKeyRepository) ListByKB(ctx context.Context, tx pgx.Tx, orgID, kbID string) ([]model.APIKey, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+apiKeyColumns+`
		 FROM api_keys
		 WHERE org_id = $1 AND knowledge_base_id = $2
		 ORDER BY created_at DESC`,
		orgID, kbID,
	)
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
		keys = append(keys, ak)
	}
	return keys, rows.Err()
}

// Revoke sets an API key status to 'revoked'.
func (r *APIKeyRepository) Revoke(ctx context.Context, tx pgx.Tx, orgID, id string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE api_keys SET status = 'revoked' WHERE id = $1 AND org_id = $2 AND status = 'active'`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("APIKeyRepository.Revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("APIKeyRepository.Revoke: api key %s not found or already revoked", id)
	}
	return nil
}

// GetByKeyHashNoTx looks up an active API key by its SHA-256 hash without
// requiring a caller-provided transaction. It acquires its own connection
// from the pool. This is designed for use by the auth middleware where there
// is no existing transaction context.
func (r *APIKeyRepository) GetByKeyHashNoTx(ctx context.Context, keyHash string) (*model.APIKey, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+apiKeyColumns+`
		 FROM api_keys
		 WHERE key_hash = $1 AND status = 'active'`,
		keyHash,
	)
	ak, err := scanAPIKey(row)
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepository.GetByKeyHashNoTx: %w", err)
	}
	return ak, nil
}
