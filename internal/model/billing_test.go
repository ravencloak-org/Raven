package model_test

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
)

func TestOrgSubscription_IsUnlimited(t *testing.T) {
	enterprise := model.OrgSubscription{
		Plan: model.DefaultPlans()[2], // Enterprise
	}
	if !enterprise.IsUnlimited() {
		t.Error("enterprise plan should be unlimited")
	}

	free := model.OrgSubscription{
		Plan: model.DefaultPlans()[0], // Free
	}
	if free.IsUnlimited() {
		t.Error("free plan should not be unlimited")
	}
}

func TestDefaultFreeSubscription(t *testing.T) {
	sub := model.DefaultFreeSubscription()
	if sub.Plan.Tier != model.PlanTierFree {
		t.Errorf("expected free tier, got %s", sub.Plan.Tier)
	}
	if sub.Plan.MaxKBs != 3 {
		t.Errorf("expected MaxKBs 3, got %d", sub.Plan.MaxKBs)
	}
}
