package service

import (
	"context"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/crypto"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// LLMProbeFunc probes a vendor with the supplied credentials and reports
// whether the credentials look valid. Implementations should bound their own
// timeout and never panic. Returning a non-nil error indicates the probe could
// not be carried out (network failure, unexpected vendor response); returning
// (false, nil) indicates the probe completed and the credentials were rejected.
type LLMProbeFunc func(ctx context.Context, provider model.LLMProvider, baseURL *string, apiKey string) (ok bool, detail string, err error)

// LLMProviderService contains business logic for LLM provider config management.
type LLMProviderService struct {
	repo   *repository.LLMProviderRepository
	pool   *pgxpool.Pool
	aesKey []byte
	probe  LLMProbeFunc
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
	return &LLMProviderService{repo: repo, pool: pool, aesKey: key, probe: defaultLLMProbe}, nil
}

// WithProbe overrides the vendor probe func; intended for tests.
func (s *LLMProviderService) WithProbe(probe LLMProbeFunc) *LLMProviderService {
	s.probe = probe
	return s
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

// TestConnection probes a vendor either with inline credentials or with the
// stored credentials of an existing provider row. When req.ProviderID is set,
// the row is loaded, the encrypted key decrypted, and that key used for the
// probe. Otherwise req.Provider and req.APIKey are required and used directly.
//
// The vendor probe itself is delegated to s.probe so callers (and tests) can
// inject behaviour without making real HTTP calls.
func (s *LLMProviderService) TestConnection(ctx context.Context, orgID string, req model.TestProviderRequest) (*model.TestConnectionResult, error) {
	var (
		provider model.LLMProvider
		baseURL  *string
		apiKey   string
	)

	switch {
	case req.ProviderID != nil && *req.ProviderID != "":
		var cfg *model.LLMProviderConfig
		err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
			var getErr error
			cfg, getErr = s.repo.GetByID(ctx, tx, orgID, *req.ProviderID)
			return getErr
		})
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				return nil, apierror.NewNotFound("LLM provider config not found")
			}
			return nil, apierror.NewInternal("failed to fetch LLM provider config: " + err.Error())
		}
		if cfg.APIKeyEncrypted == nil || cfg.APIKeyIV == nil {
			return nil, apierror.NewBadRequest("stored provider has no API key to test")
		}
		plaintext, err := crypto.Decrypt(cfg.APIKeyEncrypted, cfg.APIKeyIV, s.aesKey)
		if err != nil {
			return nil, apierror.NewInternal("failed to decrypt API key: " + err.Error())
		}
		provider = cfg.Provider
		baseURL = cfg.BaseURL
		apiKey = string(plaintext)

	case req.Provider != nil && req.APIKey != nil && *req.APIKey != "":
		if !model.ValidLLMProviders[*req.Provider] {
			return nil, apierror.NewBadRequest("invalid provider: " + string(*req.Provider))
		}
		provider = *req.Provider
		baseURL = req.BaseURL
		apiKey = *req.APIKey

	default:
		return nil, apierror.NewBadRequest("must supply either provider_id or {provider, api_key}")
	}

	if s.probe == nil {
		return nil, apierror.NewInternal("provider probe not configured")
	}
	ok, detail, err := s.probe(ctx, provider, baseURL, apiKey)
	if err != nil {
		return nil, apierror.NewInternal("provider probe failed: " + err.Error())
	}
	return &model.TestConnectionResult{
		OK:       ok,
		Provider: string(provider),
		Detail:   detail,
	}, nil
}

// TestDefaultConnection probes the org's currently-configured default LLM
// provider end-to-end. Used by the frontend health-check cron to surface a
// persistent banner when the chat path would fail (typical demo failure
// modes: dead Cloudflare tunnel to a self-hosted Ollama, expired API key).
//
// Unlike TestConnection this method accepts keyless providers (Ollama) by
// passing an empty api_key to the probe — for those the probe checks
// reachability of the base URL only. When no default is configured the
// result is OK=false with a "no default provider" detail so the caller can
// distinguish "everything fine" from "operator never set anything up".
func (s *LLMProviderService) TestDefaultConnection(ctx context.Context, orgID string) (*model.TestConnectionResult, error) {
	var cfg *model.LLMProviderConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var getErr error
		cfg, getErr = s.repo.GetDefault(ctx, tx, orgID)
		return getErr
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return &model.TestConnectionResult{
				OK:     false,
				Detail: "no default LLM provider configured",
			}, nil
		}
		return nil, apierror.NewInternal("failed to fetch default LLM provider: " + err.Error())
	}
	if cfg == nil {
		return &model.TestConnectionResult{
			OK:     false,
			Detail: "no default LLM provider configured",
		}, nil
	}

	// Decrypt the stored key if present. Keyless providers (Ollama) have
	// nil ciphertext and IV; in that case we pass an empty api_key to the
	// probe and rely on base-URL reachability alone.
	apiKey := ""
	if cfg.APIKeyEncrypted != nil && cfg.APIKeyIV != nil {
		plaintext, dErr := crypto.Decrypt(cfg.APIKeyEncrypted, cfg.APIKeyIV, s.aesKey)
		if dErr != nil {
			return nil, apierror.NewInternal("failed to decrypt API key: " + dErr.Error())
		}
		apiKey = string(plaintext)
	}

	if s.probe == nil {
		return nil, apierror.NewInternal("provider probe not configured")
	}
	ok, detail, probeErr := s.probe(ctx, cfg.Provider, cfg.BaseURL, apiKey)
	if probeErr != nil {
		// A probe-level error (DNS failure, connection refused, TLS) is a
		// reachability problem — surface it as OK=false rather than a 500
		// so the cron caller renders the toast instead of treating it as
		// an internal API error.
		return &model.TestConnectionResult{
			OK:       false,
			Provider: string(cfg.Provider),
			Detail:   probeErr.Error(),
		}, nil
	}
	return &model.TestConnectionResult{
		OK:       ok,
		Provider: string(cfg.Provider),
		Detail:   detail,
	}, nil
}

// defaultLLMProbe issues a lightweight authenticated GET against the vendor's
// public models endpoint to verify the API key. It treats 2xx as success and
// any other response (including 401/403) as a failed probe. Network and parse
// errors propagate up so callers can distinguish "probe could not run" from
// "credentials rejected".
func defaultLLMProbe(ctx context.Context, provider model.LLMProvider, baseURL *string, apiKey string) (bool, string, error) {
	endpoint, header := probeEndpointFor(provider, baseURL)
	if endpoint == "" {
		// Provider does not expose a stable probe endpoint; treat as success
		// when an api_key is present (caller-side validation only).
		return apiKey != "", "no remote probe configured for this provider", nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, "", err
	}
	for k, v := range header(apiKey) {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, "", nil
	}
	return false, "vendor returned status " + resp.Status, nil
}

// probeEndpointFor returns the URL and an auth-header-builder for the given
// provider. Returning an empty url tells the caller there is no remote probe
// for this provider.
func probeEndpointFor(provider model.LLMProvider, baseURL *string) (string, func(apiKey string) map[string]string) {
	withBase := func(defaultBase, path string) string {
		base := defaultBase
		if baseURL != nil && *baseURL != "" {
			base = strings.TrimRight(*baseURL, "/")
		}
		return base + path
	}
	bearer := func(apiKey string) map[string]string {
		return map[string]string{"Authorization": "Bearer " + apiKey}
	}
	switch provider {
	case model.LLMProviderOpenAI:
		return withBase("https://api.openai.com", "/v1/models"), bearer
	case model.LLMProviderAnthropic:
		return withBase("https://api.anthropic.com", "/v1/models"), func(apiKey string) map[string]string {
			return map[string]string{
				"x-api-key":         apiKey,
				"anthropic-version": "2023-06-01",
			}
		}
	case model.LLMProviderCohere:
		return withBase("https://api.cohere.com", "/v1/models"), bearer
	case model.LLMProviderGoogle:
		// Google AI Studio uses the API key as a query param; skip remote probe.
		return "", nil
	case model.LLMProviderOllama:
		// Ollama's REST API exposes /api/tags for the list of pulled models.
		// No auth header (Ollama itself is keyless); the base URL is required
		// because there is no canonical public host.
		if baseURL == nil || *baseURL == "" {
			return "", nil
		}
		noAuth := func(_ string) map[string]string { return nil }
		return withBase("", "/api/tags"), noAuth
	case model.LLMProviderCustom:
		// Customer-deployed OpenAI-compatible: probe via /v1/models with
		// bearer auth, same shape as upstream OpenAI.
		if baseURL == nil || *baseURL == "" {
			return "", nil
		}
		return withBase("", "/v1/models"), bearer
	case model.LLMProviderAzureOpenAI:
		// Azure's resource-scoped endpoints don't expose a flat /models route;
		// skip remote probe and rely on the apiKey-present heuristic.
		return "", nil
	default:
		return "", nil
	}
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
