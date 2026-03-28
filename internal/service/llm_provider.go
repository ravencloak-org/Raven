package service

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/crypto"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// LLMProviderService contains business logic for LLM provider config management.
type LLMProviderService struct {
	repo   *repository.LLMProviderRepository
	pool   *pgxpool.Pool
	aesKey []byte
}

// NewLLMProviderService creates a new LLMProviderService.
// aesKeyHex must be a 64-character hex string representing a 32-byte key.
func NewLLMProviderService(repo *repository.LLMProviderRepository, pool *pgxpool.Pool, aesKeyHex string) (*LLMProviderService, error) {
	key, err := hex.DecodeString(aesKeyHex)
	if err != nil {
		return nil, apierror.NewInternal("invalid AES key hex: " + err.Error())
	}
	if len(key) != 32 {
		return nil, apierror.NewInternal("AES key must be 32 bytes (64 hex characters)")
	}
	return &LLMProviderService{repo: repo, pool: pool, aesKey: key}, nil
}

// Create encrypts the API key and stores a new LLM provider config.
func (s *LLMProviderService) Create(ctx context.Context, orgID, userID string, req model.CreateLLMProviderRequest) (*model.LLMProviderResponse, error) {
	if !model.ValidLLMProviders[req.Provider] {
		return nil, apierror.NewBadRequest("invalid provider: " + string(req.Provider))
	}

	ciphertext, iv, err := crypto.Encrypt([]byte(req.APIKey), s.aesKey)
	if err != nil {
		return nil, apierror.NewInternal("failed to encrypt API key: " + err.Error())
	}

	hint := crypto.GenerateHint(req.APIKey)

	cfg := &model.LLMProviderConfig{
		OrgID:           orgID,
		Provider:        req.Provider,
		DisplayName:     req.DisplayName,
		APIKeyEncrypted: ciphertext,
		APIKeyIV:        iv,
		APIKeyHint:      hint,
		BaseURL:         req.BaseURL,
		Config:          req.Config,
		IsDefault:       req.IsDefault,
		Status:          model.ProviderStatusActive,
		CreatedBy:       &userID,
	}

	var result *model.LLMProviderConfig
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// If this is marked as default, unset any existing defaults.
		if req.IsDefault {
			if _, unsetErr := tx.Exec(ctx,
				`UPDATE llm_provider_configs SET is_default = false WHERE org_id = $1 AND is_default = true`,
				orgID,
			); unsetErr != nil {
				return unsetErr
			}
		}
		var createErr error
		result, createErr = s.repo.Create(ctx, tx, cfg)
		return createErr
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apierror.NewBadRequest("duplicate LLM provider config")
		}
		return nil, apierror.NewInternal("failed to create LLM provider config: " + err.Error())
	}
	return result.ToResponse(), nil
}

// GetByID retrieves an LLM provider config by ID (no encrypted key in response).
func (s *LLMProviderService) GetByID(ctx context.Context, orgID, configID string) (*model.LLMProviderResponse, error) {
	var cfg *model.LLMProviderConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var getErr error
		cfg, getErr = s.repo.GetByID(ctx, tx, orgID, configID)
		return getErr
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("LLM provider config not found")
		}
		return nil, apierror.NewInternal("failed to fetch LLM provider config: " + err.Error())
	}
	return cfg.ToResponse(), nil
}

// List returns all LLM provider configs for an org (no encrypted keys in response).
func (s *LLMProviderService) List(ctx context.Context, orgID string) ([]model.LLMProviderResponse, error) {
	var configs []model.LLMProviderConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var listErr error
		configs, listErr = s.repo.List(ctx, tx, orgID)
		return listErr
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list LLM provider configs: " + err.Error())
	}
	responses := lo.Map(configs, func(cfg model.LLMProviderConfig, _ int) model.LLMProviderResponse {
		return *cfg.ToResponse()
	})
	return responses, nil
}

// Update applies partial updates to an LLM provider config.
// If a new API key is provided, it is re-encrypted.
func (s *LLMProviderService) Update(ctx context.Context, orgID, configID string, req model.UpdateLLMProviderRequest) (*model.LLMProviderResponse, error) {
	if req.Status != nil && !model.ValidProviderStatuses[*req.Status] {
		return nil, apierror.NewBadRequest("invalid status: " + string(*req.Status))
	}

	var (
		encryptedKey []byte
		iv           []byte
		hintPtr      *string
	)
	if req.APIKey != nil {
		var err error
		encryptedKey, iv, err = crypto.Encrypt([]byte(*req.APIKey), s.aesKey)
		if err != nil {
			return nil, apierror.NewInternal("failed to encrypt API key: " + err.Error())
		}
		hint := crypto.GenerateHint(*req.APIKey)
		hintPtr = &hint
	}

	var cfg *model.LLMProviderConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var updateErr error
		cfg, updateErr = s.repo.Update(ctx, tx, orgID, configID, req.DisplayName, encryptedKey, iv, hintPtr, req.BaseURL, req.Config, req.Status)
		return updateErr
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("LLM provider config not found")
		}
		return nil, apierror.NewInternal("failed to update LLM provider config: " + err.Error())
	}
	return cfg.ToResponse(), nil
}

// Delete removes an LLM provider config.
func (s *LLMProviderService) Delete(ctx context.Context, orgID, configID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, orgID, configID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("LLM provider config not found")
		}
		return apierror.NewInternal("failed to delete LLM provider config: " + err.Error())
	}
	return nil
}

// SetDefault marks a provider config as the default for an org.
func (s *LLMProviderService) SetDefault(ctx context.Context, orgID, configID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.SetDefault(ctx, tx, orgID, configID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("LLM provider config not found")
		}
		return apierror.NewInternal("failed to set default provider: " + err.Error())
	}
	return nil
}

// GetDecryptedKey retrieves and decrypts the API key for a provider config.
// This is intended for internal use only (e.g. when calling LLM APIs).
func (s *LLMProviderService) GetDecryptedKey(ctx context.Context, orgID, configID string) (string, error) {
	var cfg *model.LLMProviderConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var getErr error
		cfg, getErr = s.repo.GetByID(ctx, tx, orgID, configID)
		return getErr
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return "", apierror.NewNotFound("LLM provider config not found")
		}
		return "", apierror.NewInternal("failed to fetch LLM provider config: " + err.Error())
	}

	if cfg.APIKeyEncrypted == nil || cfg.APIKeyIV == nil {
		return "", apierror.NewInternal("no encrypted key stored for this provider")
	}

	plaintext, err := crypto.Decrypt(cfg.APIKeyEncrypted, cfg.APIKeyIV, s.aesKey)
	if err != nil {
		return "", apierror.NewInternal("failed to decrypt API key: " + err.Error())
	}
	return string(plaintext), nil
}
