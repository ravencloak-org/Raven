package service

import (
	"strings"
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
)

func cfg(provider model.LLMProvider, isDefault bool, status model.ProviderStatus) model.LLMProviderConfig {
	return model.LLMProviderConfig{
		ID:        string(provider) + "-id",
		OrgID:     "org-1",
		Provider:  provider,
		IsDefault: isDefault,
		Status:    status,
	}
}

func TestResolveEmbeddingProviderFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		defaultCfg *model.LLMProviderConfig
		all        []model.LLMProviderConfig
		want       string
		wantErrSub string
	}{
		{
			name:       "default is embedding-capable (openai) — returns default",
			defaultCfg: ptr(cfg(model.LLMProviderOpenAI, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderOpenAI, true, model.ProviderStatusActive),
			},
			want: "openai",
		},
		{
			name:       "default is embedding-capable (ollama) — returns default",
			defaultCfg: ptr(cfg(model.LLMProviderOllama, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderOllama, true, model.ProviderStatusActive),
			},
			want: "ollama",
		},
		{
			name:       "default is anthropic, ollama sibling exists — returns ollama",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: "ollama",
		},
		{
			name:       "default is anthropic, only openai exists — returns openai",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderOpenAI, false, model.ProviderStatusActive),
			},
			want: "openai",
		},
		{
			name:       "default is anthropic, only cohere exists — returns cohere",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderCohere, false, model.ProviderStatusActive),
			},
			want: "cohere",
		},
		{
			name:       "default anthropic, ollama+openai+cohere all present — ollama wins (priority)",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderCohere, false, model.ProviderStatusActive),
				cfg(model.LLMProviderOpenAI, false, model.ProviderStatusActive),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: "ollama",
		},
		{
			name:       "default anthropic, openai+cohere present (no ollama) — openai wins",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderCohere, false, model.ProviderStatusActive),
				cfg(model.LLMProviderOpenAI, false, model.ProviderStatusActive),
			},
			want: "openai",
		},
		{
			name:       "org has only anthropic — returns clear error",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
			},
			wantErrSub: "no embedding-capable",
		},
		{
			name:       "org has nothing — returns clear error",
			defaultCfg: nil,
			all:        []model.LLMProviderConfig{},
			wantErrSub: "no embedding-capable",
		},
		{
			name:       "default is revoked-anthropic, sibling ollama active — returns ollama",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusRevoked)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusRevoked),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: "ollama",
		},
		{
			name:       "default is revoked-openai (embedding-capable but inactive) — falls through to sibling",
			defaultCfg: ptr(cfg(model.LLMProviderOpenAI, true, model.ProviderStatusRevoked)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderOpenAI, true, model.ProviderStatusRevoked),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: "ollama",
		},
		{
			name:       "default is active-openai but sibling ollama exists — default still wins",
			defaultCfg: ptr(cfg(model.LLMProviderOpenAI, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderOpenAI, true, model.ProviderStatusActive),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			// ADR-0009 keeps the default — we only swap when the default
			// can't embed. Picking ollama here would silently move
			// embeddings off the user's chosen provider.
			want: "openai",
		},
		{
			name:       "default is google (no embed via ai-worker today) — sibling ollama wins",
			defaultCfg: ptr(cfg(model.LLMProviderGoogle, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderGoogle, true, model.ProviderStatusActive),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: "ollama",
		},
		{
			name:       "only revoked siblings — error, not silent OK",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusRevoked),
			},
			wantErrSub: "no embedding-capable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveEmbeddingProviderFrom(tc.defaultCfg, tc.all, "org-1")
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (got=%q)", tc.wantErrSub, got)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrSub, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("provider mismatch: want=%q got=%q", tc.want, got)
			}
		})
	}
}

func TestSupportsEmbeddings(t *testing.T) {
	t.Parallel()

	cases := map[model.LLMProvider]bool{
		model.LLMProviderOpenAI:     true,
		model.LLMProviderOllama:     true,
		model.LLMProviderCohere:     true,
		model.LLMProviderAnthropic:  false,
		model.LLMProviderGoogle:     false,
		model.LLMProviderAzureOpenAI: false,
		model.LLMProviderCustom:     false,
	}
	for p, want := range cases {
		if got := model.SupportsEmbeddings(p); got != want {
			t.Errorf("SupportsEmbeddings(%q) = %v, want %v", p, got, want)
		}
	}
}

func ptr[T any](v T) *T { return &v }
