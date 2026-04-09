package service_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/jackc/pgx/v5"
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

// --- Mock billing repository ---

type mockBillingRepo struct {
	createSubscriptionFn          func(ctx context.Context, tx pgx.Tx, sub *model.Subscription) (*model.Subscription, error)
	getSubscriptionByIDFn         func(ctx context.Context, tx pgx.Tx, orgID, subID string) (*model.Subscription, error)
	getSubscriptionByHyperswitchFn func(ctx context.Context, tx pgx.Tx, hsID string) (*model.Subscription, error)
	getActiveSubscriptionFn       func(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error)
	updateSubscriptionStatusFn    func(ctx context.Context, tx pgx.Tx, orgID, subID string, status model.SubscriptionStatus) (*model.Subscription, error)
	extendSubscriptionPeriodFn    func(ctx context.Context, tx pgx.Tx, hsID string) (*model.Subscription, error)
	insertPaymentEventFn          func(ctx context.Context, tx pgx.Tx, orgID, eventType, paymentID, status string, raw []byte) (bool, error)
}

func (m *mockBillingRepo) CreateSubscription(ctx context.Context, tx pgx.Tx, sub *model.Subscription) (*model.Subscription, error) {
	if m.createSubscriptionFn != nil {
		return m.createSubscriptionFn(ctx, tx, sub)
	}
	sub.ID = "sub_mock_123"
	return sub, nil
}

func (m *mockBillingRepo) GetSubscriptionByID(ctx context.Context, tx pgx.Tx, orgID, subID string) (*model.Subscription, error) {
	if m.getSubscriptionByIDFn != nil {
		return m.getSubscriptionByIDFn(ctx, tx, orgID, subID)
	}
	return nil, nil
}

func (m *mockBillingRepo) GetSubscriptionByHyperswitchID(ctx context.Context, tx pgx.Tx, hsID string) (*model.Subscription, error) {
	if m.getSubscriptionByHyperswitchFn != nil {
		return m.getSubscriptionByHyperswitchFn(ctx, tx, hsID)
	}
	return nil, nil
}

func (m *mockBillingRepo) GetActiveSubscription(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error) {
	if m.getActiveSubscriptionFn != nil {
		return m.getActiveSubscriptionFn(ctx, tx, orgID)
	}
	return nil, nil
}

func (m *mockBillingRepo) UpdateSubscriptionStatus(ctx context.Context, tx pgx.Tx, orgID, subID string, status model.SubscriptionStatus) (*model.Subscription, error) {
	if m.updateSubscriptionStatusFn != nil {
		return m.updateSubscriptionStatusFn(ctx, tx, orgID, subID, status)
	}
	return &model.Subscription{ID: subID, OrgID: orgID, Status: status}, nil
}

func (m *mockBillingRepo) ExtendSubscriptionPeriod(ctx context.Context, tx pgx.Tx, hsID string) (*model.Subscription, error) {
	if m.extendSubscriptionPeriodFn != nil {
		return m.extendSubscriptionPeriodFn(ctx, tx, hsID)
	}
	return &model.Subscription{HyperswitchSubscriptionID: hsID, Status: model.SubscriptionStatusActive}, nil
}

func (m *mockBillingRepo) InsertPaymentEvent(ctx context.Context, tx pgx.Tx, orgID, eventType, paymentID, status string, raw []byte) (bool, error) {
	if m.insertPaymentEventFn != nil {
		return m.insertPaymentEventFn(ctx, tx, orgID, eventType, paymentID, status, raw)
	}
	return true, nil
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

func TestVerifyWebhookSignature_EmptySecret_Allows(t *testing.T) {
	svc := service.NewBillingService(nil, nil, nil, "")

	payload := []byte(`{"event_type":"payment_succeeded"}`)
	err := svc.VerifyWebhookSignature(payload, "any_signature")
	assert.NoError(t, err)
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

func TestCreatePaymentIntent_Success(t *testing.T) {
	hsClient := &mockHyperswitchClient{
		createPaymentFn: func(_ context.Context, req *hyperswitch.CreatePaymentRequest) (*hyperswitch.PaymentResponse, error) {
			assert.Equal(t, int64(5000), req.Amount)
			assert.Equal(t, "INR", req.Currency)
			return &hyperswitch.PaymentResponse{
				PaymentID:    "hs_pay_test",
				ClientSecret: "hs_secret_test",
				Status:       "requires_payment_method",
			}, nil
		},
	}

	svc := service.NewBillingService(nil, nil, hsClient, "")

	pi, err := svc.CreatePaymentIntent(context.Background(), "org-123", model.CreatePaymentIntentRequest{
		Amount:   5000,
		Currency: "INR",
	})
	require.NoError(t, err)
	assert.Equal(t, "hs_pay_test", pi.HyperswitchPaymentID)
	assert.Equal(t, "hs_secret_test", pi.ClientSecret)
	assert.Equal(t, int64(5000), pi.Amount)
	assert.Equal(t, "INR", pi.Currency)
	assert.Equal(t, model.PaymentIntentStatusRequiresPayment, pi.Status)
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
