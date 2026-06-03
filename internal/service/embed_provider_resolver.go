package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
)

// llmProviderLister is the minimal repo surface ResolveEmbeddingProvider needs.
// Defined as an interface so the resolver can be unit-tested with a fake repo
// instead of standing up a Postgres connection just to exercise priority logic.
type llmProviderLister interface {
	List(ctx context.Context, tx pgx.Tx, orgID string) ([]model.LLMProviderConfig, error)
	GetDefault(ctx context.Context, tx pgx.Tx, orgID string) (*model.LLMProviderConfig, error)
}

// Compile-time assertion that the real repo satisfies the interface.
var _ llmProviderLister = (*repository.LLMProviderRepository)(nil)

// ResolveEmbeddingProvider picks the provider slug that should serve the
// EMBEDDING sub-call of RAG / ingestion for the given org. ADR-0009 keeps a
// single default LLM provider per org for chat. This resolver layers on top
// of that for the mixed-vendor case the ADR explicitly called out: when an
// org's default is a chat-only provider (Anthropic — no public embeddings
// API), embed should fall back to a sibling provider that supports it,
// without disturbing the chat default.
//
// Priority:
//  1. The org's configured default, if it supports embeddings.
//  2. The first ACTIVE non-default provider that supports embeddings,
//     walked in `model.EmbeddingProviderPriority` order (Ollama first
//     because that's the edge-friendly choice, then OpenAI, then Cohere).
//
// Returns the provider slug as a plain string ("ollama", "openai", etc.) so
// it drops straight into the gRPC `provider` / `embed_provider` fields
// without further conversion at the call site. Returns a human-readable
// error when the org has nothing embedding-capable configured — surfaces
// the same fix as the Python AnthropicEmbeddingProvider.NotImplementedError
// message ("Configure Ollama, OpenAI, or Cohere") so SREs see one consistent
// remediation step regardless of where the call failed.
func ResolveEmbeddingProvider(
	ctx context.Context,
	pool *pgxpool.Pool,
	repo llmProviderLister,
	orgID string,
) (string, error) {
	if repo == nil {
		return "", fmt.Errorf("ResolveEmbeddingProvider: nil llm provider repo")
	}

	var (
		defaultCfg *model.LLMProviderConfig
		all        []model.LLMProviderConfig
	)
	err := db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error {
		def, dErr := repo.GetDefault(ctx, tx, orgID)
		if dErr != nil && !strings.Contains(dErr.Error(), "no rows") {
			return fmt.Errorf("get default provider: %w", dErr)
		}
		defaultCfg = def

		list, lErr := repo.List(ctx, tx, orgID)
		if lErr != nil {
			return fmt.Errorf("list providers: %w", lErr)
		}
		all = list
		return nil
	})
	if err != nil {
		return "", err
	}

	return resolveEmbeddingProviderFrom(defaultCfg, all, orgID)
}

// resolveEmbeddingProviderFrom is the pure decision function — no DB,
// no transaction. Split out so the priority logic is trivially unit-testable.
func resolveEmbeddingProviderFrom(
	defaultCfg *model.LLMProviderConfig,
	all []model.LLMProviderConfig,
	orgID string,
) (string, error) {
	// 1. Default is embedding-capable AND active → use it. Mirrors the
	//    pre-bug behaviour for orgs whose default is OpenAI / Ollama / Cohere.
	if defaultCfg != nil &&
		defaultCfg.Status == model.ProviderStatusActive &&
		model.SupportsEmbeddings(defaultCfg.Provider) {
		return string(defaultCfg.Provider), nil
	}

	// 2. Default doesn't support embeddings (or is missing / revoked).
	//    Walk the priority list and return the first active hit.
	for _, want := range model.EmbeddingProviderPriority {
		for i := range all {
			cfg := &all[i]
			if cfg.Provider == want && cfg.Status == model.ProviderStatusActive {
				return string(cfg.Provider), nil
			}
		}
	}

	return "", fmt.Errorf(
		"no embedding-capable LLM provider configured for org %s — "+
			"add one of: Ollama, OpenAI, Cohere",
		orgID,
	)
}
