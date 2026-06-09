package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ravencloak-org/Raven/internal/model"
)

// TestResolveChatFrom exercises the pure chat-provider decision function.
// Covers: happy path returns default; no-default maps to (nil, nil) so
// chat.go's literal fallback still fires; real repo errors surface so
// handlers can 500 instead of silently masking infra failures.
func TestResolveChatFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        *model.LLMProviderConfig
		getErr     error
		want       *model.LLMProvider
		wantErrSub string
	}{
		{
			name: "happy path — returns default openai",
			cfg:  ptr(cfg(model.LLMProviderOpenAI, true, model.ProviderStatusActive)),
			want: ptr(model.LLMProviderOpenAI),
		},
		{
			name: "happy path — returns default ollama",
			cfg:  ptr(cfg(model.LLMProviderOllama, true, model.ProviderStatusActive)),
			want: ptr(model.LLMProviderOllama),
		},
		{
			name: "no default configured (no-rows) — returns (nil, nil)",
			cfg:  nil,
			getErr: fmt.Errorf(
				"LLMProviderRepository.GetDefault: %w", pgx.ErrNoRows),
			want: nil,
		},
		{
			name: "GetDefault returned nil cfg with no error — returns (nil, nil)",
			cfg:  nil,
			want: nil,
		},
		{
			name:       "repo failure surfaces as error, not silent (nil, nil)",
			cfg:        nil,
			getErr:     errors.New("connection refused"),
			wantErrSub: "connection refused",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveChatFrom(tc.cfg, tc.getErr)
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (got=%v)", tc.wantErrSub, got)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrSub, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil provider, got %q", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected provider %q, got nil", *tc.want)
			}
			if *got != *tc.want {
				t.Fatalf("provider mismatch: want=%q got=%q", *tc.want, *got)
			}
		})
	}
}

// cfg builds an LLMProviderConfig for a single test row. Status defaults to
// active; flip via the last positional arg when exercising revoked siblings.
func cfg(provider model.LLMProvider, isDefault bool, status model.ProviderStatus) model.LLMProviderConfig {
	return model.LLMProviderConfig{
		ID:        string(provider) + "-id",
		OrgID:     "org-1",
		Provider:  provider,
		IsDefault: isDefault,
		Status:    status,
	}
}

// TestResolveEmbedFrom covers the pure decision function — the priority
// ladder shared by every embed path. ResolveForEmbed wraps this with a DB
// fetch; the pool-backed integration is exercised separately.
func TestResolveEmbedFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		defaultCfg *model.LLMProviderConfig
		all        []model.LLMProviderConfig
		want       model.LLMProvider
		wantErrSub string
	}{
		{
			name:       "default is embedding-capable (openai) — returns default",
			defaultCfg: ptr(cfg(model.LLMProviderOpenAI, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderOpenAI, true, model.ProviderStatusActive),
			},
			want: model.LLMProviderOpenAI,
		},
		{
			name:       "default is embedding-capable (ollama) — returns default",
			defaultCfg: ptr(cfg(model.LLMProviderOllama, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderOllama, true, model.ProviderStatusActive),
			},
			want: model.LLMProviderOllama,
		},
		{
			name:       "default anthropic + ollama sibling — returns ollama",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: model.LLMProviderOllama,
		},
		{
			name:       "default anthropic + only openai sibling — returns openai",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderOpenAI, false, model.ProviderStatusActive),
			},
			want: model.LLMProviderOpenAI,
		},
		{
			name:       "default anthropic + only cohere sibling — returns cohere",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderCohere, false, model.ProviderStatusActive),
			},
			want: model.LLMProviderCohere,
		},
		{
			name:       "default anthropic + ollama+openai+cohere — ollama wins (priority)",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderCohere, false, model.ProviderStatusActive),
				cfg(model.LLMProviderOpenAI, false, model.ProviderStatusActive),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: model.LLMProviderOllama,
		},
		{
			name:       "default anthropic + openai+cohere (no ollama) — openai wins",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
				cfg(model.LLMProviderCohere, false, model.ProviderStatusActive),
				cfg(model.LLMProviderOpenAI, false, model.ProviderStatusActive),
			},
			want: model.LLMProviderOpenAI,
		},
		{
			name:       "org has only anthropic — clear error mentions embed capability",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusActive),
			},
			wantErrSub: "no embedding-capable",
		},
		{
			name:       "org has nothing — clear error",
			defaultCfg: nil,
			all:        []model.LLMProviderConfig{},
			wantErrSub: "no embedding-capable",
		},
		{
			name:       "default is revoked-anthropic + active ollama sibling — returns ollama",
			defaultCfg: ptr(cfg(model.LLMProviderAnthropic, true, model.ProviderStatusRevoked)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderAnthropic, true, model.ProviderStatusRevoked),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: model.LLMProviderOllama,
		},
		{
			name:       "default is revoked-openai (embed-capable but inactive) — falls through to sibling",
			defaultCfg: ptr(cfg(model.LLMProviderOpenAI, true, model.ProviderStatusRevoked)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderOpenAI, true, model.ProviderStatusRevoked),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: model.LLMProviderOllama,
		},
		{
			name:       "default active-openai + sibling ollama — default still wins (ADR-0009)",
			defaultCfg: ptr(cfg(model.LLMProviderOpenAI, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderOpenAI, true, model.ProviderStatusActive),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			// We only swap when the default cannot embed. Picking ollama
			// here would silently move embeddings off the user's choice.
			want: model.LLMProviderOpenAI,
		},
		{
			name:       "default google (no embed today) + ollama sibling — ollama wins",
			defaultCfg: ptr(cfg(model.LLMProviderGoogle, true, model.ProviderStatusActive)),
			all: []model.LLMProviderConfig{
				cfg(model.LLMProviderGoogle, true, model.ProviderStatusActive),
				cfg(model.LLMProviderOllama, false, model.ProviderStatusActive),
			},
			want: model.LLMProviderOllama,
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
			got, err := resolveEmbedFrom(tc.defaultCfg, tc.all, "org-1")
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (got=%v)", tc.wantErrSub, got)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrSub, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatalf("expected provider %q, got nil", tc.want)
			}
			if *got != tc.want {
				t.Fatalf("provider mismatch: want=%q got=%q", tc.want, *got)
			}
		})
	}
}

func TestSupportsEmbeddings(t *testing.T) {
	t.Parallel()

	cases := map[model.LLMProvider]bool{
		model.LLMProviderOpenAI:      true,
		model.LLMProviderOllama:      true,
		model.LLMProviderCohere:      true,
		model.LLMProviderAnthropic:   false,
		model.LLMProviderGoogle:      false,
		model.LLMProviderAzureOpenAI: false,
		model.LLMProviderCustom:      false,
	}
	for p, want := range cases {
		if got := supportsEmbeddings(p); got != want {
			t.Errorf("supportsEmbeddings(%q) = %v, want %v", p, got, want)
		}
	}
}

func ptr[T any](v T) *T { return &v }
