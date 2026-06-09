package marketplace

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/ravencloak-org/Raven/internal/model"
)

// TestSettingsAllowListExhaustive asserts the CI gate ADR-0002 §"Trade-offs
// accepted" promises: every settings JSONB key this codebase knows about
// must be in EITHER the allow-list OR the explicit deny-list. The list of
// "known" keys is enumerated below — a fixture, not a runtime sweep.
//
// IMPORTANT: when a new settings key is added to the codebase (e.g. a new
// chunker knob), append it to knownSettingsKeys and decide its destination
// (settingsAllowListConst or settingsDenyListConst). The test pointing the
// next developer at the missing decision is the entire reason this gate
// exists.
//
// Why a hand-maintained list rather than reflection over a struct? The
// settings field is a free-form `map[string]any` on model.KnowledgeBase
// (see internal/model/kb.go) — there is no Go struct schema to walk. A
// settings schema constant is a follow-up; for now, the enumeration below
// IS the schema-of-record, and this test is its enforcement seam.
func TestSettingsAllowListExhaustive(t *testing.T) {
	// All keys the codebase currently writes to or reads from
	// knowledge_bases.settings. Update this list when a new key is
	// introduced; the test fails the build until you decide.
	knownSettingsKeys := []string{
		// retrieval-side knobs — boundary-safe
		"chunker_config",
		"embedding_model_id",
		// publisher-private tenant config — boundary-deny
		"api_key",
		"webhook_secret",
		"webhook_url",
		"routing_overrides",
		"response_cache_config",
	}

	var undecided []string
	for _, k := range knownSettingsKeys {
		_, allowed := settingsAllowList[k]
		_, denied := settingsDenyList[k]
		switch {
		case allowed && denied:
			t.Errorf("settings key %q is in BOTH allow-list and deny-list — pick one", k)
		case !allowed && !denied:
			undecided = append(undecided, k)
		}
	}
	if len(undecided) > 0 {
		sort.Strings(undecided)
		t.Errorf(
			"settings keys present in the codebase but NOT in allow-list or deny-list: %v\n"+
				"Decide each one: add to settingsAllowListConst (boundary-safe) or "+
				"settingsDenyListConst (publisher-private) in projection_settings_allowlist.go.",
			undecided,
		)
	}
}

// TestIsAllowedSettingsKey pins the prefix rule. ADR-0002 names the
// `public:` prefix as the publisher's explicit "I marked this key safe"
// signal; the test guards against the prefix being accidentally narrowed
// to an exact-match.
func TestIsAllowedSettingsKey(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		allow bool
	}{
		{"allow_listed_exact", "chunker_config", true},
		{"public_prefix", "public:author_name", true},
		{"public_prefix_with_subpath", "public:branding.logo_url", true},
		{"empty_public_suffix", "public:", false}, // the prefix needs at least one char after the colon
		{"unknown_key", "frobnicate_factor", false},
		{"deny_listed_key", "api_key", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAllowedSettingsKey(tc.key); got != tc.allow {
				t.Errorf("isAllowedSettingsKey(%q) = %v, want %v", tc.key, got, tc.allow)
			}
		})
	}
}

// TestScrubSettings_DefaultDeny verifies the policy direction: an unknown
// key cannot leak through. The fixture deliberately includes an
// `api_key`-shaped secret so the assertion has a clear "this is what NOT
// publishing means" surface.
func TestScrubSettings_DefaultDeny(t *testing.T) {
	in := map[string]any{
		"chunker_config":       map[string]any{"size": 512},
		"embedding_model_id":   "nomic-embed-text-v1.5",
		"api_key":              "secret-do-not-leak",
		"webhook_secret":       "another-do-not-leak",
		"frobnicate_factor":    99,
		"public:author_name":   "Acme Docs Team",
	}
	out := scrubSettings(in)

	// What MUST survive.
	for _, k := range []string{"chunker_config", "embedding_model_id", "public:author_name"} {
		if _, ok := out[k]; !ok {
			t.Errorf("expected key %q to survive scrub, but it was dropped", k)
		}
	}
	// What MUST NOT survive.
	for _, k := range []string{"api_key", "webhook_secret", "frobnicate_factor"} {
		if _, ok := out[k]; ok {
			t.Errorf("key %q leaked through scrub", k)
		}
	}
}

// TestProjectPublished_NeverProjectedFieldsAreUnreachable asserts the
// structural hard-deny ADR-0002 §"Consequences" requires. The check is
// at the type level: we enumerate the field names the projection struct
// MUST NOT have, and use reflection to confirm they are absent.
//
// A future contributor who adds e.g. an `APIKeys []ProjectedAPIKey` field
// (intentionally or otherwise) gets a failing test pointing them at the
// boundary they are about to weaken.
func TestProjectPublished_NeverProjectedFieldsAreUnreachable(t *testing.T) {
	bannedTopLevelFields := []string{
		"APIKeys",
		"ChatSessions",
		"RoutingRules",
		"WebhookConfigs",
		"ResponseCache",
		"AirbyteConnectors",
	}
	projType := reflect.TypeOf(PublishedKBProjection{})
	for _, name := range bannedTopLevelFields {
		if _, ok := projType.FieldByName(name); ok {
			t.Errorf("PublishedKBProjection has banned field %q — never-projected per ADR-0002", name)
		}
	}

	bannedDocumentFields := []string{"StoragePath", "FileBlob"}
	docType := reflect.TypeOf(ProjectedDocument{})
	for _, name := range bannedDocumentFields {
		if _, ok := docType.FieldByName(name); ok {
			t.Errorf("ProjectedDocument has banned field %q — SeaweedFS blob refs are publisher-private per ADR-0002", name)
		}
	}
}

// TestProjectPublished_HappyPath exercises the projection on a fixture and
// asserts (a) every allowed surface lands, (b) the deny-listed settings
// key is gone, (c) the embedding model filter applied. Single source of
// truth for "this is what `projectPublished` does to one fixture".
func TestProjectPublished_HappyPath(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	license := "CC-BY-4.0"

	src := SourceKB{
		KB: &model.KnowledgeBase{
			ID:             "kb-src",
			Name:           "Public Docs",
			Slug:           "public-docs",
			Description:    "the docs",
			LastModifiedAt: now,
			LicenseSPDXID:  &license,
			Settings: map[string]any{
				"chunker_config":     map[string]any{"size": 512},
				"embedding_model_id": "nomic-embed-text-v1.5",
				"api_key":            "secret",
				"public:author":      "Acme",
				"unknown_key":        42,
			},
		},
		Sources: []model.Source{
			{ID: "src-1", URL: "https://example.com/docs", SourceType: model.SourceTypeWebSite, CrawlDepth: 3, CrawlFrequency: model.CrawlFrequencyWeekly, Title: "Docs", PagesCrawled: 42, Metadata: map[string]any{"foo": "bar"}},
		},
		Documents: []model.Document{
			{ID: "doc-1", FileName: "intro.md", FileType: "md", FileHash: "abc", Title: "Intro", Metadata: map[string]any{}},
		},
		Chunks: []model.Chunk{
			{ID: "chk-1", DocumentID: ptr("doc-1"), Content: "hello world", ChunkIndex: 0, ChunkType: model.ChunkTypeText, Metadata: map[string]any{}},
			{ID: "chk-2", SourceID: ptr("src-1"), Content: "from the website", ChunkIndex: 1, ChunkType: model.ChunkTypeText, Metadata: map[string]any{}},
		},
		Embeddings: []model.Embedding{
			{ID: "emb-1", ChunkID: "chk-1", ModelName: "nomic-embed-text-v1.5", Dimensions: 768, Embedding: pgvector.NewVector([]float32{0.1, 0.2})},
			{ID: "emb-stale", ChunkID: "chk-1", ModelName: "some-other-model", Dimensions: 768, Embedding: pgvector.NewVector([]float32{0.9, 0.9})},
		},
	}

	proj := projectPublished(src, "nomic-embed-text-v1.5")

	if proj.KBMetadata.Name != "Public Docs" || proj.KBMetadata.Slug != "public-docs" {
		t.Errorf("KB metadata not projected correctly: %+v", proj.KBMetadata)
	}
	if proj.SourceLicenseSPDXID != "CC-BY-4.0" {
		t.Errorf("license: got %q want CC-BY-4.0", proj.SourceLicenseSPDXID)
	}
	if !proj.ImportedFromRevisionAt.Equal(now) {
		t.Errorf("ImportedFromRevisionAt: got %v want %v", proj.ImportedFromRevisionAt, now)
	}

	// Settings: deny-listed key dropped, allow-listed + public: prefix kept.
	if _, ok := proj.KBMetadata.Settings["api_key"]; ok {
		t.Error("api_key leaked into projection settings")
	}
	if _, ok := proj.KBMetadata.Settings["unknown_key"]; ok {
		t.Error("unknown_key leaked into projection settings")
	}
	if _, ok := proj.KBMetadata.Settings["chunker_config"]; !ok {
		t.Error("chunker_config missing from projection settings")
	}
	if _, ok := proj.KBMetadata.Settings["public:author"]; !ok {
		t.Error("public:author missing from projection settings")
	}

	// Sources, documents, chunks fully projected.
	if len(proj.Sources) != 1 || len(proj.Documents) != 1 || len(proj.Chunks) != 2 {
		t.Errorf("counts mismatch: sources=%d documents=%d chunks=%d",
			len(proj.Sources), len(proj.Documents), len(proj.Chunks))
	}

	// Embeddings filtered to the destination's model only.
	if len(proj.Embeddings) != 1 {
		t.Fatalf("expected exactly one embedding to survive the model filter, got %d", len(proj.Embeddings))
	}
	if proj.Embeddings[0].ModelName != "nomic-embed-text-v1.5" {
		t.Errorf("wrong embedding model survived: %s", proj.Embeddings[0].ModelName)
	}
}

// TestProjectPublished_EmptyEmbeddingModel_KeepsAll exercises the
// "destination has no default model" path: every embedding crosses (the
// orchestrator decides what to do next). Pins the contract: projectPublished
// is silent on mismatch policy; Importer.Import owns the loud failure.
func TestProjectPublished_EmptyEmbeddingModel_KeepsAll(t *testing.T) {
	src := SourceKB{
		KB: &model.KnowledgeBase{ID: "kb", Name: "n", Slug: "s", LastModifiedAt: time.Now()},
		Chunks: []model.Chunk{
			{ID: "chk-1", Content: "x", ChunkIndex: 0, ChunkType: model.ChunkTypeText},
		},
		Embeddings: []model.Embedding{
			{ID: "emb-a", ChunkID: "chk-1", ModelName: "model-a", Dimensions: 768, Embedding: pgvector.NewVector([]float32{0.1})},
			{ID: "emb-b", ChunkID: "chk-1", ModelName: "model-b", Dimensions: 768, Embedding: pgvector.NewVector([]float32{0.2})},
		},
	}
	proj := projectPublished(src, "")
	if len(proj.Embeddings) != 2 {
		t.Errorf("expected both embeddings to pass an empty filter, got %d", len(proj.Embeddings))
	}
}

func ptr[T any](v T) *T { return &v }
