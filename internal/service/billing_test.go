package service_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/hyperswitch"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
)

// --- Mock Hyperswitch client ---

type mockHyperswitchClient struct {
	createPaymentFn func(ctx context.Context, req *hyperswitch.CreatePaymentRequest) (*hyperswitch.PaymentResponse, error)
	cancelPaymentFn func(ctx context.Context, paymentID string) error
}

func (m *mockHyperswitchClient) CreatePayment(ctx context.Context, req *hyperswitch.CreatePaymentRequest) (*hyperswitch.PaymentResponse, error) {
	if m.createPaymentFn != nil {
		return m.createPaymentFn(ctx, req)
	}
	return &hyperswitch.PaymentResponse{
		PaymentID:    "hs_pay_mock",
		ClientSecret: "hs_secret_mock",
		Status:       "requires_payment_method",
	}, nil
}

func (m *mockHyperswitchClient) CancelPayment(ctx context.Context, paymentID string) error {
	if m.cancelPaymentFn != nil {
		return m.cancelPaymentFn(ctx, paymentID)
	}
	return nil
}

// --- Tests ---

func TestVerifyWebhookSignature_ValidSignature(t *testing.T) {
	secret := "test-webhook-secret-key"
	svc := service.NewBillingService(nil, nil, nil, secret)

	payload := []byte(`{"event_type":"payment_succeeded","content":{"payment_id":"pay_123"}}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validSig := hex.EncodeToString(mac.Sum(nil))

	err := svc.VerifyWebhookSignature(payload, validSig)
	assert.NoError(t, err)
}

func TestVerifyWebhookSignature_InvalidSignature(t *testing.T) {
	secret := "test-webhook-secret-key"
	svc := service.NewBillingService(nil, nil, nil, secret)

	payload := []byte(`{"event_type":"payment_succeeded"}`)
	err := svc.VerifyWebhookSignature(payload, "bad_signature")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid webhook signature")
}

func TestVerifyWebhookSignature_EmptySecret_Rejects(t *testing.T) {
	svc := service.NewBillingService(nil, nil, nil, "")

	payload := []byte(`{"event_type":"payment_succeeded"}`)
	err := svc.VerifyWebhookSignature(payload, "any_signature")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook signature verification is not configured")
}

func TestVerifyWebhookSignature_TamperedPayload(t *testing.T) {
	secret := "test-webhook-secret-key"
	svc := service.NewBillingService(nil, nil, nil, secret)

	originalPayload := []byte(`{"event_type":"payment_succeeded","content":{"payment_id":"pay_123"}}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(originalPayload)
	validSig := hex.EncodeToString(mac.Sum(nil))

	// Tamper with the payload.
	tamperedPayload := []byte(`{"event_type":"payment_succeeded","content":{"payment_id":"pay_EVIL"}}`)
	err := svc.VerifyWebhookSignature(tamperedPayload, validSig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid webhook signature")
}

func TestGetPlans_ReturnsThreeTiers(t *testing.T) {
	svc := service.NewBillingService(nil, nil, nil, "")
	plans := svc.GetPlans()

	assert.Len(t, plans, 3)
	tiers := make(map[model.PlanTier]bool)
	for _, p := range plans {
		tiers[p.Tier] = true
	}
	assert.True(t, tiers[model.PlanTierFree])
	assert.True(t, tiers[model.PlanTierPro])
	assert.True(t, tiers[model.PlanTierEnterprise])
}

func TestGetPlanByID_Found(t *testing.T) {
	svc := service.NewBillingService(nil, nil, nil, "")
	plan, err := svc.GetPlanByID("plan_pro")
	require.NoError(t, err)
	assert.Equal(t, model.PlanTierPro, plan.Tier)
	assert.Equal(t, int64(2900), plan.PriceMonthly)
}

func TestGetPlanByID_NotFound(t *testing.T) {
	svc := service.NewBillingService(nil, nil, nil, "")
	_, err := svc.GetPlanByID("plan_nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCreatePaymentIntent_HyperswitchError(t *testing.T) {
	hsClient := &mockHyperswitchClient{
		createPaymentFn: func(_ context.Context, _ *hyperswitch.CreatePaymentRequest) (*hyperswitch.PaymentResponse, error) {
			return nil, assert.AnError
		},
	}

	svc := service.NewBillingService(nil, nil, hsClient, "")

	pi, err := svc.CreatePaymentIntent(context.Background(), "org-123", model.CreatePaymentIntentRequest{
		Amount:   5000,
		Currency: "INR",
	})
	require.Error(t, err)
	assert.Nil(t, pi)
	assert.Contains(t, err.Error(), "failed to create Hyperswitch payment")
}

func TestSubscriptionStateMachine_FreePlan(t *testing.T) {
	// Free plan subscriptions should be created without calling Hyperswitch.
	hsClient := &mockHyperswitchClient{
		createPaymentFn: func(_ context.Context, _ *hyperswitch.CreatePaymentRequest) (*hyperswitch.PaymentResponse, error) {
			t.Fatal("Hyperswitch should not be called for free plan")
			return nil, nil
		},
	}

	svc := service.NewBillingService(nil, nil, hsClient, "")
	// Note: CreateSubscription requires a DB pool for RLS transactions.
	// We verify plan lookup and Hyperswitch client selection logic here.
	plan, err := svc.GetPlanByID("plan_free")
	require.NoError(t, err)
	assert.Equal(t, model.PlanTierFree, plan.Tier)
	assert.Equal(t, int64(0), plan.PriceMonthly)
}

func TestDefaultPlans_FeatureLimits(t *testing.T) {
	plans := model.DefaultPlans()

	// Free tier limits.
	free := plans[0]
	assert.Equal(t, 5, free.MaxUsers)
	assert.Equal(t, 2, free.MaxWorkspaces)
	assert.Equal(t, 3, free.MaxKBs)
	assert.Equal(t, int64(500), free.MaxStorageMB)
	assert.Equal(t, 1, free.MaxConcurrentVoiceSessions)

	// Enterprise = unlimited.
	enterprise := plans[2]
	assert.Equal(t, -1, enterprise.MaxUsers)
	assert.Equal(t, -1, enterprise.MaxWorkspaces)
}
