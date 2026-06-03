package marketplace_test

import (
	"reflect"
	"testing"

	"github.com/ravencloak-org/Raven/internal/marketplace"
)

// TestAllowedSPDXLicensesExact is the CI guard required by ADR-0002 / ADR-0006.
// It pins the exact ordered set of SPDX identifiers the Marketplace publish
// path will accept. Adding or removing a license is a deliberate decision
// with product, legal, and UX (badge rendering) implications — this test
// is the trip-wire that fails a PR which changes the list without also
// updating the ADRs.
//
// If you are here because this test failed:
//  1. Confirm docs/adr/0006-licence-and-moderation.md has been updated to
//     reflect the new allow-list.
//  2. Update `want` below to match the new canonical list, preserving the
//     ADR's documented order.
//  3. Re-run the test.
func TestAllowedSPDXLicensesExact(t *testing.T) {
	want := []string{
		"CC0-1.0",
		"CC-BY-4.0",
		"CC-BY-SA-4.0",
		"CC-BY-NC-4.0",
		"MIT",
		"Apache-2.0",
		"GPL-3.0",
	}

	if got := marketplace.AllowedSPDXLicenses; !reflect.DeepEqual(got, want) {
		t.Errorf("AllowedSPDXLicenses drift detected.\n got:  %v\n want: %v\n"+
			"If this change is intentional, update docs/adr/0006-licence-and-moderation.md and this test in the same PR.",
			got, want)
	}
}

// TestIsAllowedLicense covers the helper's three interesting branches:
// exact match, empty string, and a near-miss that exercises the
// case-sensitive comparison guard.
func TestIsAllowedLicense(t *testing.T) {
	cases := []struct {
		name string
		id   string
		want bool
	}{
		{name: "exact_match_first", id: "CC0-1.0", want: true},
		{name: "exact_match_last", id: "GPL-3.0", want: true},
		{name: "exact_match_middle", id: "MIT", want: true},
		{name: "empty_string", id: "", want: false},
		{name: "unknown_identifier", id: "BSD-3-Clause", want: false},
		{name: "case_sensitive_lower", id: "mit", want: false},
		{name: "case_sensitive_mixed", id: "Cc-By-4.0", want: false},
		// SPDX identifiers do not have leading / trailing whitespace; the
		// helper must reject these rather than silently trim.
		{name: "leading_space", id: " MIT", want: false},
		{name: "trailing_space", id: "MIT ", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := marketplace.IsAllowedLicense(tc.id); got != tc.want {
				t.Errorf("IsAllowedLicense(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
