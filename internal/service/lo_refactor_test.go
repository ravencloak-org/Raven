package service

import (
	"testing"

	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/model"
)

// TestNilSliceCoalescing verifies that lo.Ternary correctly replaces nil slices
// with empty slices (the same behavior as the original if-nil guard).
func TestNilSliceCoalescing(t *testing.T) {
	t.Run("nil slice becomes empty", func(t *testing.T) {
		var results []model.ChunkWithRank
		results = lo.Ternary(results == nil, []model.ChunkWithRank{}, results)
		if results == nil {
			t.Fatal("expected non-nil empty slice, got nil")
		}
		if len(results) != 0 {
			t.Fatalf("expected empty slice, got length %d", len(results))
		}
	})

	t.Run("non-nil slice preserved", func(t *testing.T) {
		results := []model.ChunkWithRank{{ID: "chunk-1"}}
		results = lo.Ternary(results == nil, []model.ChunkWithRank{}, results)
		if len(results) != 1 {
			t.Fatalf("expected 1 element, got %d", len(results))
		}
		if results[0].ID != "chunk-1" {
			t.Errorf("expected ID 'chunk-1', got %q", results[0].ID)
		}
	})

	t.Run("empty non-nil slice preserved", func(t *testing.T) {
		results := []model.ChunkWithRank{}
		results = lo.Ternary(results == nil, []model.ChunkWithRank{}, results)
		if results == nil {
			t.Fatal("expected non-nil empty slice, got nil")
		}
		if len(results) != 0 {
			t.Fatalf("expected empty slice, got length %d", len(results))
		}
	})
}

// TestLoContainsStatusTransition verifies that lo.Contains correctly identifies
// valid and invalid status transitions (replaces manual for-loop contains).
func TestLoContainsStatusTransition(t *testing.T) {
	allowed := validStatusTransitions[model.ProcessingStatusCrawling]

	t.Run("valid transition found", func(t *testing.T) {
		if !lo.Contains(allowed, model.ProcessingStatusParsing) {
			t.Error("expected crawling -> parsing to be allowed")
		}
		if !lo.Contains(allowed, model.ProcessingStatusFailed) {
			t.Error("expected crawling -> failed to be allowed")
		}
	})

	t.Run("invalid transition not found", func(t *testing.T) {
		if lo.Contains(allowed, model.ProcessingStatusQueued) {
			t.Error("expected crawling -> queued to NOT be allowed")
		}
		if lo.Contains(allowed, model.ProcessingStatusCrawling) {
			t.Error("expected crawling -> crawling to NOT be allowed")
		}
	})
}

// TestLoSliceToMap verifies that lo.SliceToMap produces the same map as the
// manual loop used in NewUploadService for allowed types.
func TestLoSliceToMap(t *testing.T) {
	allowedTypes := []string{"Application/PDF", "text/plain", "TEXT/HTML"}

	allowed := lo.SliceToMap(allowedTypes, func(t string) (string, bool) {
		return toLower(t), true
	})

	t.Run("all types present lowercase", func(t *testing.T) {
		if !allowed["application/pdf"] {
			t.Error("expected 'application/pdf' in map")
		}
		if !allowed["text/plain"] {
			t.Error("expected 'text/plain' in map")
		}
		if !allowed["text/html"] {
			t.Error("expected 'text/html' in map")
		}
	})

	t.Run("original case not in map", func(t *testing.T) {
		if allowed["Application/PDF"] {
			t.Error("expected 'Application/PDF' NOT in map (should be lowered)")
		}
		if allowed["TEXT/HTML"] {
			t.Error("expected 'TEXT/HTML' NOT in map (should be lowered)")
		}
	})

	t.Run("absent type not in map", func(t *testing.T) {
		if allowed["image/png"] {
			t.Error("expected 'image/png' NOT in map")
		}
	})
}

// TestLoMapTransform verifies that lo.Map correctly transforms LLMProviderConfig
// slices to LLMProviderResponse slices (replaces pre-alloc+append loop).
func TestLoMapTransform(t *testing.T) {
	configs := []model.LLMProviderConfig{
		{ID: "cfg-1", Provider: model.LLMProviderOpenAI, DisplayName: "OpenAI prod"},
		{ID: "cfg-2", Provider: model.LLMProviderAnthropic, DisplayName: "Anthropic"},
	}

	responses := lo.Map(configs, func(cfg model.LLMProviderConfig, _ int) model.LLMProviderResponse {
		return *cfg.ToResponse()
	})

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
	if responses[0].ID != "cfg-1" {
		t.Errorf("expected ID 'cfg-1', got %q", responses[0].ID)
	}
	if responses[0].Provider != model.LLMProviderOpenAI {
		t.Errorf("expected provider 'openai', got %q", responses[0].Provider)
	}
	if responses[1].ID != "cfg-2" {
		t.Errorf("expected ID 'cfg-2', got %q", responses[1].ID)
	}
	if responses[1].DisplayName != "Anthropic" {
		t.Errorf("expected display name 'Anthropic', got %q", responses[1].DisplayName)
	}

	t.Run("empty input produces empty output", func(t *testing.T) {
		empty := lo.Map([]model.LLMProviderConfig{}, func(cfg model.LLMProviderConfig, _ int) model.LLMProviderResponse {
			return *cfg.ToResponse()
		})
		if len(empty) != 0 {
			t.Errorf("expected 0 responses, got %d", len(empty))
		}
	})
}

// toLower is a simple strings.ToLower equivalent used in tests to avoid importing strings.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
