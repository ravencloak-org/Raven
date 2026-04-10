package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"

	"github.com/jackc/pgx/v5"
)

// --- Mock implementations ---

// mockQuotaRepo implements QuotaRepository for unit tests.
type mockQuotaRepo struct {
	subscription *model.Subscription
	subErr       error
	kbCount      int
	kbErr        error
	memberCount  int
	memberErr    error
	voiceUsage   int
	voiceErr     error
}

func (m *mockQuotaRepo) GetActiveSubscription(_ context.Context, _ pgx.Tx, _ string) (*model.Subscription, error) {
	return m.subscription, m.subErr
}

func (m *mockQuotaRepo) CountKBsByOrg(_ context.Context, _ pgx.Tx, _ string) (int, error) {
	return m.kbCount, m.kbErr
}

func (m *mockQuotaRepo) CountMembersByOrg(_ context.Context, _ pgx.Tx, _ string) (int, error) {
	return m.memberCount, m.memberErr
}

func (m *mockQuotaRepo) GetVoiceUsageForPeriod(_ context.Context, _ pgx.Tx, _ string, _ time.Time) (int, error) {
	return m.voiceUsage, m.voiceErr
}

// mockSubCache implements SubscriptionCache for unit tests.
type mockSubCache struct {
	data map[string]*model.OrgSubscription
}

func newMockSubCache() *mockSubCache {
	return &mockSubCache{data: make(map[string]*model.OrgSubscription)}
}

func (m *mockSubCache) Get(_ context.Context, orgID string) (*model.OrgSubscription, error) {
	sub, ok := m.data[orgID]
	if !ok {
		return nil, nil
	}
	return sub, nil
}

func (m *mockSubCache) Set(_ context.Context, orgID string, sub *model.OrgSubscription) error {
	m.data[orgID] = sub
	return nil
}

// --- Tests ---

func TestCheckKBQuota_FreeAtLimit_Returns402(t *testing.T) {
	repo := &mockQuotaRepo{
		subscription: nil, // no subscription → free tier
		kbCount:      3,   // free tier limit is 3
	}
	checker := NewQuotaChecker(repo, newMockSubCache(), nil)

	err := checker.CheckKBQuota(context.Background(), "org-1")
	if err == nil {
		t.Fatal("expected 402 error, got nil")
	}

	qErr, ok := err.(*apierror.QuotaError)
	if !ok {
		t.Fatalf("expected *apierror.QuotaError, got %T", err)
	}
	if qErr.Code != http.StatusPaymentRequired {
		t.Errorf("expected status 402, got %d", qErr.Code)
	}
	if qErr.Limit != 3 {
		t.Errorf("expected limit 3, got %d", qErr.Limit)
	}
}

func TestCheckKBQuota_EnterpriseBypass(t *testing.T) {
	repo := &mockQuotaRepo{
		subscription: &model.Subscription{
			PlanID: "plan_enterprise",
			Status: model.SubscriptionStatusActive,
		},
		kbCount: 999,
	}
	checker := NewQuotaChecker(repo, newMockSubCache(), nil)

	err := checker.CheckKBQuota(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected nil error for enterprise, got %v", err)
	}
}

func TestCheckKBQuota_ProUnderLimit_Passes(t *testing.T) {
	repo := &mockQuotaRepo{
		subscription: &model.Subscription{
			PlanID: "plan_pro",
			Status: model.SubscriptionStatusActive,
		},
		kbCount: 10, // pro limit is 50
	}
	checker := NewQuotaChecker(repo, newMockSubCache(), nil)

	err := checker.CheckKBQuota(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected nil error for pro under limit, got %v", err)
	}
}

func TestCheckSeatQuota_FreeAtLimit_Returns402(t *testing.T) {
	repo := &mockQuotaRepo{
		subscription: nil, // no subscription → free tier
		memberCount:  5,   // free tier limit is 5
	}
	checker := NewQuotaChecker(repo, newMockSubCache(), nil)

	err := checker.CheckSeatQuota(context.Background(), "org-1")
	if err == nil {
		t.Fatal("expected 402 error, got nil")
	}

	qErr, ok := err.(*apierror.QuotaError)
	if !ok {
		t.Fatalf("expected *apierror.QuotaError, got %T", err)
	}
	if qErr.Code != http.StatusPaymentRequired {
		t.Errorf("expected status 402, got %d", qErr.Code)
	}
	if qErr.Limit != 5 {
		t.Errorf("expected limit 5, got %d", qErr.Limit)
	}
}

func TestCheckVoiceMinuteQuota_FreeAtLimit_Returns402(t *testing.T) {
	repo := &mockQuotaRepo{
		subscription: nil,  // no subscription → free tier
		voiceUsage:   3600, // 60 minutes in seconds; free tier limit is 60
	}
	checker := NewQuotaChecker(repo, newMockSubCache(), nil)

	err := checker.CheckVoiceMinuteQuota(context.Background(), "org-1")
	if err == nil {
		t.Fatal("expected 402 error, got nil")
	}

	qErr, ok := err.(*apierror.QuotaError)
	if !ok {
		t.Fatalf("expected *apierror.QuotaError, got %T", err)
	}
	if qErr.Code != http.StatusPaymentRequired {
		t.Errorf("expected status 402, got %d", qErr.Code)
	}
	if qErr.Limit != 60 {
		t.Errorf("expected limit 60, got %d", qErr.Limit)
	}
}

func TestCheckVoiceMinuteQuota_UnderLimit_Passes(t *testing.T) {
	repo := &mockQuotaRepo{
		subscription: nil,  // no subscription → free tier
		voiceUsage:   1800, // 30 minutes in seconds; free tier limit is 60
	}
	checker := NewQuotaChecker(repo, newMockSubCache(), nil)

	err := checker.CheckVoiceMinuteQuota(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected nil error for under limit, got %v", err)
	}
}

func TestGetConcurrentVoiceLimit_EnterpriseBypass(t *testing.T) {
	repo := &mockQuotaRepo{
		subscription: &model.Subscription{
			PlanID: "plan_enterprise",
			Status: model.SubscriptionStatusActive,
		},
	}
	checker := NewQuotaChecker(repo, newMockSubCache(), nil)

	limit := checker.GetConcurrentVoiceLimit(context.Background(), "org-1")
	if limit != -1 {
		t.Errorf("expected -1 (unlimited) for enterprise, got %d", limit)
	}
}
