package marketplace_test

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/marketplace"
)

// TestURL pins the public Marketplace URL format documented in ADR-0005
// so the SPA, future email templates, and any audit trail row produce
// byte-identical strings. Drift here would silently break stored links
// out in the wild.
func TestURL(t *testing.T) {
	cases := []struct {
		name     string
		orgSlug  string
		kbSlug   string
		expected string
	}{
		{
			name:     "simple_slugs",
			orgSlug:  "acme",
			kbSlug:   "support-docs",
			expected: "https://raven.ravencloak.org/marketplace/acme/support-docs",
		},
		{
			name:     "numeric_slugs",
			orgSlug:  "org-42",
			kbSlug:   "kb-1",
			expected: "https://raven.ravencloak.org/marketplace/org-42/kb-1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := marketplace.URL(tc.orgSlug, tc.kbSlug); got != tc.expected {
				t.Errorf("URL(%q,%q) = %q, want %q",
					tc.orgSlug, tc.kbSlug, got, tc.expected)
			}
		})
	}
}
