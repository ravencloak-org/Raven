package marketplace

// Settings JSONB allow-list for the Marketplace publish/import boundary
// (ADR-0002).
//
// The settings field on `knowledge_bases` is a free-form JSONB map. ADR-0002
// names the projection's policy as "default deny": only keys explicitly on
// the list below — or keys whose name starts with the `public:` prefix —
// cross the publish boundary. Every other key stays publisher-private.
//
// This is a CODE CONSTANT, not a config knob. ADR-0002 §"Trade-offs accepted"
// pins the maintenance burden on the publisher: every new settings key
// requires a deliberate decision (allow vs. deny) at code-review time. A
// runtime override would split the security perimeter — the publish boundary
// has to be reasoned about from a single source.
//
// CI gate: TestSettingsAllowListExhaustive enumerates every settings key
// surface this codebase knows about, and asserts each is either in the
// allow-list below or in the explicit deny-list. New settings keys cannot
// be added silently — the test fails the build with a pointer to the
// missing decision.

// settingsAllowListConst is the closed, exact, ordered list of `settings`
// JSONB top-level keys that cross the publish boundary. Order matches the
// order they appear in the design discussion in ADR-0002 §"Decision". Do
// NOT alphabetise — the ordering documents which fields are baseline
// retrieval policy (`chunker_config`, `embedding_model_id`) versus
// publisher-tagged public extras (the `public:` prefix family).
//
// Keys prefixed with `public:` are also allow-listed regardless of their
// presence here; the prefix is the publisher's explicit "I marked this
// key safe to ship" signal.
var settingsAllowListConst = []string{
	"chunker_config",
	"embedding_model_id",
}

// settingsAllowList is the same set materialised as a map for O(1) lookup
// inside scrubSettings. Built at init from settingsAllowListConst so the
// two surfaces cannot drift apart.
var settingsAllowList = func() map[string]bool {
	m := make(map[string]bool, len(settingsAllowListConst))
	for _, k := range settingsAllowListConst {
		m[k] = true
	}
	return m
}()

// settingsDenyList is the closed list of keys this codebase has explicitly
// considered and chosen NOT to publish. The list exists so the exhaustive
// CI test below has somewhere to record the deliberate denials — the test
// fails the build when a known settings key is in NEITHER the allow-list
// NOR the deny-list. "Forgot to decide" is a worse failure mode than
// either "allowed" or "denied".
//
// Keep deny-list entries alphabetised so reviewing additions is trivial.
var settingsDenyListConst = []string{
	"api_key",          // never publish — secret material
	"webhook_secret",   // never publish — secret material
	"webhook_url",      // never publish — tenant-specific endpoint
	"routing_overrides", // tenant-specific routing logic
	"response_cache_config", // ADR-0002: response_cache is never-projected
}

// settingsDenyList is the same set materialised as a map for O(1) lookup
// inside the CI test.
var settingsDenyList = func() map[string]bool {
	m := make(map[string]bool, len(settingsDenyListConst))
	for _, k := range settingsDenyListConst {
		m[k] = true
	}
	return m
}()

// isAllowedSettingsKey reports whether a `settings` JSONB top-level key
// crosses the publish boundary. Exposed (lowercase, package-private) so
// scrubSettings can call it from one site; lifted out of scrubSettings
// itself so the prefix rule lives next to the allow-list constant.
func isAllowedSettingsKey(k string) bool {
	if settingsAllowList[k] {
		return true
	}
	const publicPrefix = "public:"
	if len(k) > len(publicPrefix) && k[:len(publicPrefix)] == publicPrefix {
		return true
	}
	return false
}

// settingsAllowListSlice returns the canonical settings allow-list as a
// fresh slice in the order it was declared. Exported because the SQL
// function `marketplace_import_kb` is called with this exact list as its
// `p_settings_allowlist` argument — the Go code is the single source of
// truth for what keys may transit the boundary, and the SQL function
// receives the list at call-time rather than mirroring it in a separate
// migration.
//
// The returned slice is a copy so callers can safely pass it to pgx as a
// TEXT[] argument without risk of the underlying array being mutated by a
// future code change.
func settingsAllowListSlice() []string {
	out := make([]string, len(settingsAllowListConst))
	copy(out, settingsAllowListConst)
	return out
}
