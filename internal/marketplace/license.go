package marketplace

// SPDX license allow-list for the Marketplace publish path (ADR-0006).
//
// The seven entries below are the entire universe of licenses a publisher
// may attach to a Public KB. The list is a code constant — NOT a config
// knob — so changing it requires a PR, code review, and the CI test
// TestAllowedSPDXLicensesExact (which pins the exact set). The review gate
// is intentional: adding a license has product, legal, and UX (badge
// rendering) consequences and must not be flippable at runtime.
//
// Order matches docs/adr/0006-licence-and-moderation.md so a side-by-side
// diff against the ADR is trivial. Do NOT alphabetise or otherwise
// re-order; the order is part of the documented contract surfaced to
// reviewers and to TestAllowedSPDXLicensesExact.

// AllowedSPDXLicenses is the canonical, ordered set of SPDX identifiers a
// publisher may attach to a Public KB. Adding or removing an entry MUST
// also update docs/adr/0006-licence-and-moderation.md and the assertion
// in TestAllowedSPDXLicensesExact.
var AllowedSPDXLicenses = []string{
	"CC0-1.0",
	"CC-BY-4.0",
	"CC-BY-SA-4.0",
	"CC-BY-NC-4.0",
	"MIT",
	"Apache-2.0",
	"GPL-3.0",
}

// IsAllowedLicense reports whether id is exactly one of the canonical SPDX
// identifiers in AllowedSPDXLicenses. The match is case-sensitive — SPDX
// identifiers are themselves case-sensitive, and we don't want
// "cc-by-4.0" or "Mit" sneaking past a typo and landing in the database
// where the badge renderer would silently miss it.
//
// O(n) linear scan over 7 entries — a map would only obfuscate this.
func IsAllowedLicense(id string) bool {
	for _, allowed := range AllowedSPDXLicenses {
		if id == allowed {
			return true
		}
	}
	return false
}
