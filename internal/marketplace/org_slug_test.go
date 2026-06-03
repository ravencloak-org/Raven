package marketplace_test

import (
	"strings"
	"testing"

	"github.com/ravencloak-org/Raven/internal/marketplace"
)

func TestSlugify(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"simple", "Acme Corp", "acme-corp"},
		{"already-slug", "acme-corp", "acme-corp"},
		{"mixed-case", "AcMe CoRp", "acme-corp"},
		{"runs-of-symbols", "Acme!!! @@ Corp", "acme-corp"},
		{"leading-trailing-symbols", "  --Acme--  ", "acme"},
		{"unicode-stripped", "café résumé", "caf-r-sum"},
		{"all-symbols", "!@#$%^&*()", ""},
		{"digits-allowed", "team 42", "team-42"},
		{"long-clipped", strings.Repeat("a", 80), strings.Repeat("a", 64)},
		{"trailing-dash-after-clip", strings.Repeat("a", 63) + "-bbbbb", strings.Repeat("a", 63)},
		{"single-char", "X", "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := marketplace.Slugify(tc.input)
			if got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.input, got, tc.want)
			}
			// Anything non-empty Slugify produces should pass IsValidSlug —
			// the migration's CHECK constraint must accept it.
			if got != "" && !marketplace.IsValidSlug(got) {
				t.Errorf("Slugify(%q) = %q failed IsValidSlug — would violate CHECK", tc.input, got)
			}
		})
	}
}

func TestIsValidSlug(t *testing.T) {
	t.Parallel()
	good := []string{"a", "acme", "acme-corp", "team-42", strings.Repeat("a", 64), "0", "0-1"}
	bad := []string{
		"",
		"-acme",
		"acme-",
		"Acme",                  // uppercase
		"acme corp",             // space
		"acme_corp",             // underscore
		strings.Repeat("a", 65), // too long
	}
	// Internal double-dashes are allowed by the regex (dash is in the body
	// class). Document the behaviour explicitly so the test stays honest.
	if !marketplace.IsValidSlug("acme--corp") {
		t.Error("acme--corp should be accepted by IsValidSlug; regex allows internal dashes")
	}
	for _, s := range good {
		if !marketplace.IsValidSlug(s) {
			t.Errorf("IsValidSlug(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if marketplace.IsValidSlug(s) {
			t.Errorf("IsValidSlug(%q) = true, want false", s)
		}
	}
}

func TestSlugifyWithSuffix(t *testing.T) {
	t.Parallel()
	t.Run("no_collision", func(t *testing.T) {
		t.Parallel()
		got := marketplace.SlugifyWithSuffix("Acme Corp", map[string]struct{}{})
		if got != "acme-corp" {
			t.Errorf("got %q, want acme-corp", got)
		}
	})
	t.Run("first_collision_gets_dash_2", func(t *testing.T) {
		t.Parallel()
		taken := map[string]struct{}{"acme-corp": {}}
		got := marketplace.SlugifyWithSuffix("Acme Corp", taken)
		if got != "acme-corp-2" {
			t.Errorf("got %q, want acme-corp-2", got)
		}
	})
	t.Run("deterministic_collision_chain", func(t *testing.T) {
		t.Parallel()
		taken := map[string]struct{}{
			"acme-corp":   {},
			"acme-corp-2": {},
			"acme-corp-3": {},
		}
		got := marketplace.SlugifyWithSuffix("Acme Corp", taken)
		if got != "acme-corp-4" {
			t.Errorf("got %q, want acme-corp-4", got)
		}
	})
	t.Run("clipped_base_when_suffix_would_overflow", func(t *testing.T) {
		t.Parallel()
		// Base is 64 chars already. With a "-2" suffix, base must shrink
		// to 62 chars to keep the total at MaxSlugLen.
		base := strings.Repeat("a", 64)
		taken := map[string]struct{}{base: {}}
		got := marketplace.SlugifyWithSuffix(base, taken)
		if len(got) > marketplace.MaxSlugLen {
			t.Fatalf("result %q exceeds MaxSlugLen=%d", got, marketplace.MaxSlugLen)
		}
		if !strings.HasSuffix(got, "-2") {
			t.Errorf("expected -2 suffix, got %q", got)
		}
		if !marketplace.IsValidSlug(got) {
			t.Errorf("clipped-and-suffixed slug %q is not URL-safe", got)
		}
	})
	t.Run("empty_input_returns_empty", func(t *testing.T) {
		t.Parallel()
		got := marketplace.SlugifyWithSuffix("!!!", map[string]struct{}{})
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
