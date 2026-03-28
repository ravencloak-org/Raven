package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// LLMProviderRepository handles database operations for LLM provider configs.
// All methods receive a pgx.Tx that must already have app.current_org_id set
// for RLS enforcement.
type LLMProviderRepository struct {
	pool *pgxpool.Pool
}

// NewLLMProviderRepository creates a new LLMProviderRepository.
func NewLLMProviderRepository(pool *pgxpool.Pool) *LLMProviderRepository {
	return &LLMProviderRepository{pool: pool}
}

const llmProviderColumns = `id, org_id, provider, display_name, api_key_encrypted, api_key_iv, api_key_hint, base_url, config, is_default, status, created_by, created_at, updated_at`

func scanLLMProvider(row pgx.Row) (*model.LLMProviderConfig, error) {
	var cfg model.LLMProviderConfig
	err := row.Scan(
		&cfg.ID,
		&cfg.OrgID,
		&cfg.Provider,
		&cfg.DisplayName,
		&cfg.APIKeyEncrypted,
		&cfg.APIKeyIV,
		&cfg.APIKeyHint,
		&cfg.BaseURL,
		&cfg.Config,
		&cfg.IsDefault,
		&cfg.Status,
		&cfg.CreatedBy,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Create inserts a new LLM provider config.
func (r *LLMProviderRepository) Create(ctx context.Context, tx pgx.Tx, cfg *model.LLMProviderConfig) (*model.LLMProviderConfig, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO llm_provider_configs (org_id, provider, display_name, api_key_encrypted, api_key_iv, api_key_hint, base_url, config, is_default, status, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING `+llmProviderColumns,
		cfg.OrgID,
		cfg.Provider,
		cfg.DisplayName,
		cfg.APIKeyEncrypted,
		cfg.APIKeyIV,
		cfg.APIKeyHint,
		cfg.BaseURL,
		cfg.Config,
		cfg.IsDefault,
		cfg.Status,
		cfg.CreatedBy,
	)
	result, err := scanLLMProvider(row)
	if err != nil {
		return nil, fmt.Errorf("LLMProviderRepository.Create: %w", err)
	}
	return result, nil
}

// GetByID fetches a single LLM provider config by its ID within an org.
func (r *LLMProviderRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, configID string) (*model.LLMProviderConfig, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+llmProviderColumns+` FROM llm_provider_configs WHERE id = $1 AND org_id = $2`,
		configID, orgID,
	)
	cfg, err := scanLLMProvider(row)
	if err != nil {
		return nil, fmt.Errorf("LLMProviderRepository.GetByID: %w", err)
	}
	return cfg, nil
}

// List returns all LLM provider configs for an organisation.
func (r *LLMProviderRepository) List(ctx context.Context, tx pgx.Tx, orgID string) ([]model.LLMProviderConfig, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+llmProviderColumns+` FROM llm_provider_configs WHERE org_id = $1 ORDER BY created_at`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("LLMProviderRepository.List: %w", err)
	}
	defer rows.Close()

	var configs []model.LLMProviderConfig
	for rows.Next() {
		var cfg model.LLMProviderConfig
		if err := rows.Scan(
			&cfg.ID,
			&cfg.OrgID,
			&cfg.Provider,
			&cfg.DisplayName,
			&cfg.APIKeyEncrypted,
			&cfg.APIKeyIV,
			&cfg.APIKeyHint,
			&cfg.BaseURL,
			&cfg.Config,
			&cfg.IsDefault,
			&cfg.Status,
			&cfg.CreatedBy,
			&cfg.CreatedAt,
			&cfg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("LLMProviderRepository.List scan: %w", err)
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

// Update applies partial updates to an LLM provider config.
func (r *LLMProviderRepository) Update(ctx context.Context, tx pgx.Tx, orgID, configID string, displayName *string, apiKeyEncrypted []byte, apiKeyIV []byte, apiKeyHint *string, baseURL *string, config map[string]any, status *model.ProviderStatus) (*model.LLMProviderConfig, error) {
	row := tx.QueryRow(ctx,
		`UPDATE llm_provider_configs
		 SET
		   display_name     = COALESCE($3, display_name),
		   api_key_encrypted = COALESCE($4, api_key_encrypted),
		   api_key_iv        = COALESCE($5, api_key_iv),
		   api_key_hint      = COALESCE($6, api_key_hint),
		   base_url          = COALESCE($7, base_url),
		   config            = CASE WHEN $8::jsonb IS NOT NULL THEN $8::jsonb ELSE config END,
		   status            = COALESCE($9, status)
		 WHERE id = $1 AND org_id = $2
		 RETURNING `+llmProviderColumns,
		configID, orgID, displayName, apiKeyEncrypted, apiKeyIV, apiKeyHint, baseURL, config, status,
	)
	cfg, err := scanLLMProvider(row)
	if err != nil {
		return nil, fmt.Errorf("LLMProviderRepository.Update: %w", err)
	}
	return cfg, nil
}

// Delete removes an LLM provider config.
func (r *LLMProviderRepository) Delete(ctx context.Context, tx pgx.Tx, orgID, configID string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM llm_provider_configs WHERE id = $1 AND org_id = $2`,
		configID, orgID,
	)
	if err != nil {
		return fmt.Errorf("LLMProviderRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("LLMProviderRepository.Delete: provider config %s not found", configID)
	}
	return nil
}

// GetDefault returns the default LLM provider config for an org (is_default = true).
func (r *LLMProviderRepository) GetDefault(ctx context.Context, tx pgx.Tx, orgID string) (*model.LLMProviderConfig, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+llmProviderColumns+` FROM llm_provider_configs WHERE org_id = $1 AND is_default = true LIMIT 1`,
		orgID,
	)
	cfg, err := scanLLMProvider(row)
	if err != nil {
		return nil, fmt.Errorf("LLMProviderRepository.GetDefault: %w", err)
	}
	return cfg, nil
}

// SetDefault sets a provider config as the default (unsetting all others first).
func (r *LLMProviderRepository) SetDefault(ctx context.Context, tx pgx.Tx, orgID, configID string) error {
	// Unset all existing defaults for this org.
	if _, err := tx.Exec(ctx,
		`UPDATE llm_provider_configs SET is_default = false WHERE org_id = $1 AND is_default = true`,
		orgID,
	); err != nil {
		return fmt.Errorf("LLMProviderRepository.SetDefault unset: %w", err)
	}
	// Set the requested config as default.
	tag, err := tx.Exec(ctx,
		`UPDATE llm_provider_configs SET is_default = true WHERE id = $1 AND org_id = $2`,
		configID, orgID,
	)
	if err != nil {
		return fmt.Errorf("LLMProviderRepository.SetDefault set: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("LLMProviderRepository.SetDefault: provider config %s not found", configID)
	}
	return nil
}
