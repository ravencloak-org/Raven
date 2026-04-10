package license_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	license "github.com/ravencloak-org/Raven/internal/ee/licensing"
)

func TestLicense_Valid_NilLicense(t *testing.T) {
	var l *license.License
	assert.False(t, l.Valid(), "nil license must be invalid")
}

func TestLicense_Valid_EmptyOrgID(t *testing.T) {
	l := &license.License{
		OrgID:     "",
		Tier:      license.TierBusiness,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	assert.False(t, l.Valid(), "license with empty OrgID must be invalid")
}

func TestLicense_Valid_Expired(t *testing.T) {
	l := &license.License{
		OrgID:     "org-123",
		Tier:      license.TierBusiness,
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
	}
	assert.False(t, l.Valid(), "expired license must be invalid")
}

func TestLicense_Valid_ValidLicense(t *testing.T) {
	l := &license.License{
		OrgID:     "org-123",
		Tier:      license.TierBusiness,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	assert.True(t, l.Valid(), "non-expired license with orgID must be valid")
}

func TestLicense_HasFeature_InvalidLicense_ReturnsFalse(t *testing.T) {
	var l *license.License
	assert.False(t, l.HasFeature("sso"), "nil license must not have any feature")
}

func TestLicense_HasFeature_ExpiredLicense_ReturnsFalse(t *testing.T) {
	l := &license.License{
		OrgID:     "org-123",
		Tier:      license.TierBusiness,
		Features:  []string{"sso", "webhooks"},
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
	}
	assert.False(t, l.HasFeature("sso"), "expired license must not grant features")
}

func TestLicense_HasFeature_EnterpriseTier_AllFeaturesGranted(t *testing.T) {
	l := &license.License{
		OrgID:     "org-123",
		Tier:      license.TierEnterprise,
		Features:  []string{},
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	assert.True(t, l.HasFeature("sso"), "enterprise tier must grant all features")
	assert.True(t, l.HasFeature("webhooks"), "enterprise tier must grant all features")
	assert.True(t, l.HasFeature("audit_logs"), "enterprise tier must grant all features")
}

func TestLicense_HasFeature_BusinessTier_GrantedFeature(t *testing.T) {
	l := &license.License{
		OrgID:     "org-123",
		Tier:      license.TierBusiness,
		Features:  []string{"webhooks", "audit_logs"},
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	assert.True(t, l.HasFeature("webhooks"), "business license must grant listed feature")
	assert.True(t, l.HasFeature("audit_logs"), "business license must grant listed feature")
}

func TestLicense_HasFeature_BusinessTier_UnlistedFeatureDenied(t *testing.T) {
	l := &license.License{
		OrgID:     "org-123",
		Tier:      license.TierBusiness,
		Features:  []string{"webhooks"},
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	assert.False(t, l.HasFeature("sso"), "business license must not grant unlisted feature")
	assert.False(t, l.HasFeature("audit_logs"), "business license must not grant unlisted feature")
}

func TestLicense_HasFeature_FeatureADoesNotUnlockFeatureB(t *testing.T) {
	l := &license.License{
		OrgID:     "org-123",
		Tier:      license.TierBusiness,
		Features:  []string{"feature_a"},
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	assert.True(t, l.HasFeature("feature_a"))
	assert.False(t, l.HasFeature("feature_b"), "granting feature_a must not grant feature_b")
}
