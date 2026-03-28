package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

const (
	// keyBytes is the number of random bytes used to generate an API key.
	// 32 bytes = 64 hex characters.
	keyBytes = 32

	// defaultRateLimit is the per-key requests-per-minute when none is specified.
	defaultRateLimit = 60
)

// ApiKeyService contains business logic for API key management.
type ApiKeyService struct {
	repo *repository.ApiKeyRepository
	pool *pgxpool.Pool
}

// NewApiKeyService creates a new ApiKeyService.
func NewApiKeyService(repo *repository.ApiKeyRepository, pool *pgxpool.Pool) *ApiKeyService {
	return &ApiKeyService{repo: repo, pool: pool}
}

// generateKey creates a cryptographically random API key and returns the raw
// hex-encoded key, its SHA-256 hash, and the 8-character prefix.
func generateKey() (raw, hash, prefix string, err error) {
	b := make([]byte, keyBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate api key: %w", err)
	}
	raw = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	prefix = raw[:8]
	return raw, hash, prefix, nil
}

// Create generates a new API key, stores its SHA-256 hash, and returns
// the full key exactly once.
func (s *ApiKeyService) Create(ctx context.Context, orgID, wsID, kbID, userID string, req model.CreateApiKeyRequest) (*model.CreateApiKeyResponse, error) {
	rawKey, keyHash, keyPrefix, err := generateKey()
	if err != nil {
		return nil, apierror.NewInternal("failed to generate api key: " + err.Error())
	}

	rateLimit := defaultRateLimit
	if req.RateLimit != nil && *req.RateLimit > 0 {
		rateLimit = *req.RateLimit
	}

	var ak *model.ApiKey
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		ak, e = s.repo.Create(ctx, tx, orgID, wsID, kbID, req.Name, keyHash, keyPrefix, userID, req.AllowedDomains, rateLimit)
		return e
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apierror.NewBadRequest("an api key with this name already exists")
		}
		return nil, apierror.NewInternal("failed to create api key: " + err.Error())
	}

	return &model.CreateApiKeyResponse{
		ApiKey: *ak,
		RawKey: rawKey,
	}, nil
}

// List returns all API keys for a knowledge base.
func (s *ApiKeyService) List(ctx context.Context, orgID, kbID string) ([]model.ApiKey, error) {
	var keys []model.ApiKey
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		keys, e = s.repo.ListByKB(ctx, tx, orgID, kbID)
		return e
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list api keys: " + err.Error())
	}
	return keys, nil
}

// Revoke revokes an active API key.
func (s *ApiKeyService) Revoke(ctx context.Context, orgID, id string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Revoke(ctx, tx, orgID, id)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "already revoked") {
			return apierror.NewNotFound("api key not found or already revoked")
		}
		return apierror.NewInternal("failed to revoke api key: " + err.Error())
	}
	return nil
}
