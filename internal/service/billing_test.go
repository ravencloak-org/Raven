package service_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ---  mock repository ---

type mockBillingRepo struct {
	upsertSubscriptionFn             func(ctx context.Context, tx pgx.Tx, s *model.Subscription) (*model.Subscription, error)
	getActiveSubscriptionFn          func(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error)
	getSubscriptionByIDFn            func(ctx context.Context, tx pgx.Tx, subscriptionID string) (*model.Subscription, error)
	getSubscriptionByHyperswitchIDFn func(ctx context.Context, hyperswitchID string) (*model.Subscription, error)
	updateSubscriptionStatusFn       func(ctx context.Context, tx pgx.Tx, subscriptionID string, status model.SubscriptionStatus, periodEnd *time.Time) error

	createPaymentIntentFn              func(ctx context.Context, tx pgx.Tx, pi *model.PaymentIntent) (*model.PaymentIntent, error)
	getPaymentIntentByHyperswitchIDFn  func(ctx context.Context, hyperswitchPaymentID string) (*model.PaymentIntent, error)
	updatePaymentIntentStatusFn        func(ctx context.Context, hyperswitchPaymentID string, status model.PaymentIntentStatus) error
}

func (m *mockBillingRepo) UpsertSubscription(ctx context.Context, tx pgx.Tx, s *model.Subscription) (*model.Subscription, error) {
	if m.upsertSubscriptionFn != nil {
		return m.upsertSubscriptionFn(ctx, tx, s)
	}
	s.ID = "sub_test"
	return s, nil
}

func (m *mockBillingRepo) GetActiveSubscription(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error) {
	if m.getActiveSubscriptionFn != nil {
		return m.getActiveSubscriptionFn(ctx, tx, orgID)
	}
	return nil, pgx.ErrNoRows
}

func (m *mockBillingRepo) GetSubscriptionByID(ctx context.Context, tx pgx.Tx, id string) (*model.Subscription, error) {
	if m.getSubscriptionByIDFn != nil {
		return m.getSubscriptionByIDFn(ctx, tx, id)
	}
	return nil, pgx.ErrNoRows
}

func (m *mockBillingRepo) GetSubscriptionByHyperswitchID(ctx context.Context, hsID string) (*model.Subscription, error) {
	if m.getSubscriptionByHyperswitchIDFn != nil {
		return m.getSubscriptionByHyperswitchIDFn(ctx, hsID)
	}
	return nil, pgx.ErrNoRows
}

func (m *mockBillingRepo) UpdateSubscriptionStatus(ctx context.Context, tx pgx.Tx, id string, status model.SubscriptionStatus, periodEnd *time.Time) error {
	if m.updateSubscriptionStatusFn != nil {
		return m.updateSubscriptionStatusFn(ctx, tx, id, status, periodEnd)
	}
	return nil
}

func (m *mockBillingRepo) CreatePaymentIntent(ctx context.Context, tx pgx.Tx, pi *model.PaymentIntent) (*model.PaymentIntent, error) {
	if m.createPaymentIntentFn != nil {
		return m.createPaymentIntentFn(ctx, tx, pi)
	}
	pi.ID = "pi_test"
	return pi, nil
}

func (m *mockBillingRepo) GetPaymentIntentByHyperswitchID(ctx context.Context, hsPaymentID string) (*model.PaymentIntent, error) {
	if m.getPaymentIntentByHyperswitchIDFn != nil {
		return m.getPaymentIntentByHyperswitchIDFn(ctx, hsPaymentID)
	}
	return nil, pgx.ErrNoRows
}

func (m *mockBillingRepo) UpdatePaymentIntentStatus(ctx context.Context, hsPaymentID string, status model.PaymentIntentStatus) error {
	if m.updatePaymentIntentStatusFn != nil {
		return m.updatePaymentIntentStatusFn(ctx, hsPaymentID, status)
	}
	return nil
}

// newTestBillingService builds a BillingService with the given repo and no HTTP client.
// The nil *pgxpool.Pool is intentional: unit tests in this file exercise only the
// service-layer logic (webhook verification, plan listing, event dispatch) and never
// call db.WithOrgID, which is the only code path that dereferences the pool.
// Integration tests that need a real pool use testutil.NewTestDB instead.
func newTestBillingService(repo service.BillingRepository, webhookSecret string) *service.BillingService {
	return service.NewBillingService(repo, (*pgxpool.Pool)(nil), "http://localhost:8090", "", webhookSecret, "rzp_test_key")
}

// --- VerifyWebhookSignature tests ---

func TestVerifyWebhookSignature_ValidSignature(t *testing.T) {
	secret := "test-secret-abcdef"
	payload := []byte(`{"event_type":"payment_succeeded","content":{"payment_id":"pay_123"}}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	svc := newTestBillingService(&mockBillingRepo{}, secret)
	if err := svc.VerifyWebhookSignature(payload, sig); err != nil {
		t.Errorf("expected valid signature to pass, got: %v", err)
	}
}

func TestVerifyWebhookSignature_InvalidSignature(t *testing.T) {
	secret := "test-secret-abcdef"
	payload := []byte(`{"event_type":"payment_succeeded","content":{}}`)

	svc := newTestBillingService(&mockBillingRepo{}, secret)
	err := svc.VerifyWebhookSignature(payload, "deadbeef")
	if err == nil {
		t.Fatal("expected invalid signature to fail")
	}
	var appErr *apierror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != 401 {
		t.Errorf("expected HTTP 401, got %d", appErr.Code)
	}
}

func TestVerifyWebhookSignature_EmptySecret_FailsClosed(t *testing.T) {
	// When no webhook secret is configured, verification must fail closed (not skip).
	payload := []byte(`{"event_type":"payment_succeeded"}`)
	svc := newTestBillingService(&mockBillingRepo{}, "")
	err := svc.VerifyWebhookSignature(payload, "any-value")
	if err == nil {
		t.Fatal("expected error when webhook secret is empty, got nil")
	}
	var appErr *apierror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != 500 {
		t.Errorf("expected HTTP 500, got %d", appErr.Code)
	}
}

func TestVerifyWebhookSignature_TamperedPayload(t *testing.T) {
	secret := "secure-secret"
	original := []byte(`{"event_type":"payment_succeeded","content":{"amount":100}}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(original)
	sig := hex.EncodeToString(mac.Sum(nil))

	// Tamper with the payload.
	tampered := []byte(`{"event_type":"payment_succeeded","content":{"amount":1}}`)

	svc := newTestBillingService(&mockBillingRepo{}, secret)
	if err := svc.VerifyWebhookSignature(tampered, sig); err == nil {
		t.Error("expected tampered payload to fail signature verification")
	}
}

// --- Subscription state machine tests ---

func TestGetPlans_ReturnsAllTiers(t *testing.T) {
	svc := newTestBillingService(&mockBillingRepo{}, "")
	plans := svc.GetPlans()
	if len(plans) != 3 {
		t.Fatalf("expected 3 plans, got %d", len(plans))
	}
	tiers := make(map[model.PlanTier]bool)
	for _, p := range plans {
		tiers[p.Tier] = true
	}
	for _, tier := range []model.PlanTier{model.PlanTierFree, model.PlanTierPro, model.PlanTierEnterprise} {
		if !tiers[tier] {
			t.Errorf("missing plan tier: %s", tier)
		}
	}
}

func TestHandleWebhook_PaymentSucceeded_ActivatesSubscription(t *testing.T) {
	// HandleWebhook calls db.WithOrgID which requires a real pgxpool.Pool.
	// This test requires an integration database; skip in unit test runs.
	// Full flow is covered by the integration test suite with testcontainers.
	t.Skip("requires real pgxpool.Pool — covered by integration tests")
}

func TestHandleWebhook_PaymentSucceeded_NoLinkedSubscription_IsNoop(t *testing.T) {
	repo := &mockBillingRepo{
		updatePaymentIntentStatusFn: func(_ context.Context, _ string, _ model.PaymentIntentStatus) error {
			return nil
		},
		getSubscriptionByHyperswitchIDFn: func(_ context.Context, _ string) (*model.Subscription, error) {
			return nil, pgx.ErrNoRows
		},
	}

	svc := newTestBillingService(repo, "")
	event := model.HyperswitchWebhookPayload{
		EventType: "payment_succeeded",
		Content:   map[string]any{"payment_id": "pay_standalone"},
	}

	// Should not error — standalone payment not linked to a subscription.
	if err := svc.HandleWebhook(context.Background(), event); err != nil {
		t.Errorf("unexpected error for standalone payment: %v", err)
	}
}

func TestHandleWebhook_PaymentFailed_NoPanic(t *testing.T) {
	repo := &mockBillingRepo{
		updatePaymentIntentStatusFn: func(_ context.Context, _ string, _ model.PaymentIntentStatus) error {
			return nil
		},
		getSubscriptionByHyperswitchIDFn: func(_ context.Context, _ string) (*model.Subscription, error) {
			return nil, pgx.ErrNoRows
		},
	}

	svc := newTestBillingService(repo, "")
	event := model.HyperswitchWebhookPayload{
		EventType: "payment_failed",
		Content:   map[string]any{"payment_id": "pay_bad"},
	}

	if err := svc.HandleWebhook(context.Background(), event); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleWebhook_SubscriptionCancelled_NoLinked_IsNoop(t *testing.T) {
	repo := &mockBillingRepo{
		getSubscriptionByHyperswitchIDFn: func(_ context.Context, _ string) (*model.Subscription, error) {
			return nil, pgx.ErrNoRows
		},
	}

	svc := newTestBillingService(repo, "")
	event := model.HyperswitchWebhookPayload{
		EventType: "subscription.cancelled",
		Content:   map[string]any{"subscription_id": "hs_sub_gone"},
	}

	if err := svc.HandleWebhook(context.Background(), event); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleWebhook_UnknownEvent_IsNoop(t *testing.T) {
	svc := newTestBillingService(&mockBillingRepo{}, "")
	event := model.HyperswitchWebhookPayload{
		EventType: "dispute_opened",
		Content:   map[string]any{},
	}
	if err := svc.HandleWebhook(context.Background(), event); err != nil {
		t.Errorf("unexpected error for unhandled event: %v", err)
	}
}

// TestWebhookIdempotency verifies that processing the same payment_succeeded event
// twice does not panic or error — the UpdatePaymentIntentStatus and
// UpdateSubscriptionStatus calls are both idempotent no-ops on the second call.
func TestWebhookIdempotency_DuplicatePaymentSucceeded(t *testing.T) {
	callCount := 0
	repo := &mockBillingRepo{
		updatePaymentIntentStatusFn: func(_ context.Context, _ string, _ model.PaymentIntentStatus) error {
			callCount++
			return nil // idempotent
		},
		getSubscriptionByHyperswitchIDFn: func(_ context.Context, _ string) (*model.Subscription, error) {
			return nil, pgx.ErrNoRows // no subscription linked
		},
	}

	svc := newTestBillingService(repo, "")
	event := model.HyperswitchWebhookPayload{
		EventType: "payment_succeeded",
		Content:   map[string]any{"payment_id": "pay_dup"},
	}

	// Fire twice.
	for i := 0; i < 2; i++ {
		if err := svc.HandleWebhook(context.Background(), event); err != nil {
			t.Errorf("iteration %d: unexpected error: %v", i, err)
		}
	}

	if callCount != 2 {
		t.Errorf("expected UpdatePaymentIntentStatus to be called twice, got %d", callCount)
	}
}
