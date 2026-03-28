// Package license provides license key validation for Raven Enterprise features.
// License keys are signed JWTs containing org_id, tier, expiry, and feature list.
//
// This package is part of Raven Enterprise Edition (Go backend).
package license

import "time"

// Tier represents the enterprise license tier.
type Tier string

const (
	// TierBusiness enables Business-tier enterprise features.
	TierBusiness Tier = "business"
	// TierEnterprise enables all enterprise features.
	TierEnterprise Tier = "enterprise"
)

// License represents a validated Raven enterprise license.
type License struct {
	OrgID     string   `json:"org_id"`
	Tier      Tier     `json:"tier"`
	Features  []string `json:"features"`
	ExpiresAt int64    `json:"expires_at"`
}

// Valid reports whether the license is present and not expired.
func (l *License) Valid() bool {
	return l != nil && l.OrgID != "" && l.ExpiresAt > time.Now().Unix()
}

// HasFeature reports whether the license grants access to the named feature.
func (l *License) HasFeature(feature string) bool {
	if !l.Valid() {
		return false
	}
	if l.Tier == TierEnterprise {
		return true
	}
	for _, f := range l.Features {
		if f == feature {
			return true
		}
	}
	return false
}
