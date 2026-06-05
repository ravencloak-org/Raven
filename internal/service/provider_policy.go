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

// LLMProviderLister is the minimal LLMProviderRepository surface the
// ProviderSelectionPolicy needs. Defined as an interface so the policy is
// unit-testable with a fake repo instead of standing up Postgres for every
// table-driven case.
type LLMProviderLister interface {
	List(ctx context.Context, tx pgx.Tx, orgID string) ([]model.LLMProviderConfig, error)
	GetDefault(ctx context.Context, tx pgx.Tx, orgID string) (*model.LLMProviderConfig, error)
}

// Compile-time assertion that the real repo satisfies the interface.
var _ LLMProviderLister = (*repository.LLMProviderRepository)(nil)

// ProviderSelectionPolicy centralises the provider-routing decisions
// previously duplicated in chat.go (`resolveDefaultProvider`) and
// embed_provider_resolver.go (`ResolveEmbeddingProvider`). Both methods
// share one transactional fetch of the org's provider set + one priority
// table, so adding a third purpose (re-ranking, classifier, etc.) becomes
// a new method on this type rather than another copy-pasted file.
//
// ADR-0009's single-default invariant is preserved — the policy never
// changes which provider is the default, it only fans the default out to
// the right sibling when the chosen purpose can't be served by the
// default itself.
type ProviderSelectionPolicy struct {
	pool *pgxpool.Pool
	repo LLMProviderLister
}

// NewProviderSelectionPolicy wires the policy to the LLM-provider repo and
// the connection pool. Both are required — a nil repo or pool would force
// every method to error on first call, so we surface that at wiring time
// instead of at request time.
func NewProviderSelectionPolicy(pool *pgxpool.Pool, repo LLMProviderLister) *ProviderSelectionPolicy {
	return &ProviderSelectionPolicy{pool: pool, repo: repo}
}

// ResolveForChat returns the org's configured default LLM provider. Returns
// (nil, nil) when no default is configured — the legitimate "org hasn't set
// one yet" case that callers fall through to a literal default for. Real
// infra/repo failures surface as a non-nil error so chat handlers can 500
// explicitly instead of silently masking them (CodeRabbit PR #689).
func (p *ProviderSelectionPolicy) ResolveForChat(ctx context.Context, orgID string) (*model.LLMProvider, error) {
	if p == nil || p.repo == nil {
		return nil, nil
	}
	var provider *model.LLMProvider
	err := db.WithOrgID(ctx, p.pool, orgID, func(tx pgx.Tx) error {
		cfg, err := p.repo.GetDefault(ctx, tx, orgID)
		prov, rErr := resolveChatFrom(cfg, err)
		if rErr != nil {
			return rErr
		}
		provider = prov
		return nil
	})
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// resolveChatFrom is the pure decision function for chat-provider lookup.
// Mirrors resolveEmbedFrom — no DB, no transaction — so the "no default
// configured" vs "real repo failure" distinction is unit-testable without
// a Postgres fixture. The legitimate no-rows case maps to (nil, nil); real
// errors surface unchanged so callers can 500 explicitly.
func resolveChatFrom(cfg *model.LLMProviderConfig, getErr error) (*model.LLMProvider, error) {
	if getErr != nil {
		// no-rows is the common "no default configured" path —
		// don't surface it as an error.
		if strings.Contains(getErr.Error(), "no rows") {
			return nil, nil
		}
		return nil, getErr
	}
	if cfg == nil {
		return nil, nil
	}
	prov := cfg.Provider
	return &prov, nil
}

// ResolveForEmbed picks the provider that should serve the EMBEDDING sub-call
// of RAG / ingestion for the given org.
//
// Priority:
//  1. The org's configured default, if it's active and supports embeddings.
//  2. The first ACTIVE non-default provider that supports embeddings, walked
//     in model.EmbeddingProviderPriority order (Ollama first because that's
//     the edge-friendly choice, then OpenAI, then Cohere).
//
// Returns a clear error naming the missing capability when the org has
// nothing embedding-capable configured — surfaces the same remediation step
// (configure Ollama, OpenAI, or Cohere) the ai-worker's Anthropic embedding
// stub raises, so SREs see one consistent message regardless of where the
// call failed.
func (p *ProviderSelectionPolicy) ResolveForEmbed(ctx context.Context, orgID string) (*model.LLMProvider, error) {
	if p == nil || p.repo == nil {
		return nil, fmt.Errorf("ResolveForEmbed: nil provider policy or repo")
	}

	var (
		defaultCfg *model.LLMProviderConfig
		all        []model.LLMProviderConfig
	)
	err := db.WithOrgID(ctx, p.pool, orgID, func(tx pgx.Tx) error {
		def, dErr := p.repo.GetDefault(ctx, tx, orgID)
		if dErr != nil && !strings.Contains(dErr.Error(), "no rows") {
			return fmt.Errorf("get default provider: %w", dErr)
		}
		defaultCfg = def

		list, lErr := p.repo.List(ctx, tx, orgID)
		if lErr != nil {
			return fmt.Errorf("list providers: %w", lErr)
		}
		all = list
		return nil
	})
	if err != nil {
		return nil, err
	}

	return resolveEmbedFrom(defaultCfg, all, orgID)
}

// resolveEmbedFrom is the pure decision function — no DB, no transaction.
// Split out so the priority logic is trivially unit-testable.
func resolveEmbedFrom(
	defaultCfg *model.LLMProviderConfig,
	all []model.LLMProviderConfig,
	orgID string,
) (*model.LLMProvider, error) {
	// 1. Default is embedding-capable AND active → use it. Mirrors the
	//    pre-bug behaviour for orgs whose default is OpenAI / Ollama / Cohere.
	if defaultCfg != nil &&
		defaultCfg.Status == model.ProviderStatusActive &&
		supportsEmbeddings(defaultCfg.Provider) {
		prov := defaultCfg.Provider
		return &prov, nil
	}

	// 2. Default doesn't support embeddings (or is missing / revoked).
	//    Walk the priority list and return the first active hit.
	if prov := findFirstByType(all, model.EmbeddingProviderPriority); prov != nil {
		return prov, nil
	}

	return nil, fmt.Errorf(
		"no embedding-capable LLM provider configured for org %s — "+
			"add one of: Ollama, OpenAI, Cohere",
		orgID,
	)
}

// findFirstByType walks `priority` and returns the first active provider in
// `all` whose type matches. Kept private — it's an implementation detail of
// ResolveForEmbed (and any future capability-fallback resolver).
func findFirstByType(all []model.LLMProviderConfig, priority []model.LLMProvider) *model.LLMProvider {
	for _, want := range priority {
		for i := range all {
			cfg := &all[i]
			if cfg.Provider == want && cfg.Status == model.ProviderStatusActive {
				prov := cfg.Provider
				return &prov
			}
		}
	}
	return nil
}

// supportsEmbeddings is a tiny wrapper over model.SupportsEmbeddings so the
// policy reads against its own private name rather than reaching into model.
// Keeps the capability check swappable if we ever load it from config.
func supportsEmbeddings(p model.LLMProvider) bool {
	return model.SupportsEmbeddings(p)
}
