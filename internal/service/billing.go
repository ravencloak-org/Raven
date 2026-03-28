package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// BillingService contains business logic for subscription and payment management
// via the Hyperswitch payment orchestration platform.
type BillingService struct {
	httpClient     *http.Client
	baseURL        string
	apiKey         string
	webhookSecret  string
	plans          map[string]model.Plan
}

// NewBillingService creates a new BillingService.
// baseURL is the Hyperswitch API base URL (e.g. "http://localhost:8090").
// apiKey is the Hyperswitch merchant API key.
// webhookSecret is used to verify Hyperswitch webhook signatures.
func NewBillingService(baseURL, apiKey, webhookSecret string) *BillingService {
	plans := make(map[string]model.Plan)
	for _, p := range model.DefaultPlans() {
		plans[p.ID] = p
	}
	return &BillingService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:       baseURL,
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		plans:         plans,
	}
}

// GetPlans returns the available billing plans.
func (s *BillingService) GetPlans() []model.Plan {
	return model.DefaultPlans()
}

// GetPlanByID returns a plan by ID, or an error if not found.
func (s *BillingService) GetPlanByID(planID string) (*model.Plan, error) {
	p, ok := s.plans[planID]
	if !ok {
		return nil, apierror.NewNotFound("plan not found: " + planID)
	}
	return &p, nil
}

// CreateSubscription creates a new subscription for the given organisation.
// It calls Hyperswitch to set up a recurring payment and returns the subscription.
func (s *BillingService) CreateSubscription(ctx context.Context, orgID string, req model.CreateSubscriptionRequest) (*model.Subscription, error) {
	plan, err := s.GetPlanByID(req.PlanID)
	if err != nil {
		return nil, err
	}

	// For the free plan, no payment orchestration is needed.
	if plan.Tier == model.PlanTierFree {
		now := time.Now().UTC()
		sub := &model.Subscription{
			ID:                 generateID("sub"),
			OrgID:              orgID,
			PlanID:             plan.ID,
			Status:             model.SubscriptionStatusActive,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now.AddDate(0, 1, 0),
			CreatedAt:          now,
		}
		// TODO: Persist subscription to database via repository layer.
		return sub, nil
	}

	// TODO: Call Hyperswitch API to create a subscription/recurring payment.
	// The Hyperswitch API endpoint for creating payments:
	//   POST {baseURL}/payments
	//
	// For now, return a mock subscription with a placeholder Hyperswitch ID.
	hyperswitchSubID, err := s.createHyperswitchSubscription(ctx, orgID, plan)
	if err != nil {
		return nil, apierror.NewInternal("failed to create Hyperswitch subscription: " + err.Error())
	}

	now := time.Now().UTC()
	sub := &model.Subscription{
		ID:                        generateID("sub"),
		OrgID:                     orgID,
		PlanID:                    plan.ID,
		Status:                    model.SubscriptionStatusActive,
		HyperswitchSubscriptionID: hyperswitchSubID,
		CurrentPeriodStart:        now,
		CurrentPeriodEnd:          now.AddDate(0, 1, 0),
		CreatedAt:                 now,
	}

	// TODO: Persist subscription to database via repository layer.

	return sub, nil
}

// CancelSubscription cancels the subscription for the given organisation.
func (s *BillingService) CancelSubscription(ctx context.Context, orgID string, subscriptionID string) error {
	// TODO: Look up the subscription from the database.
	// TODO: Verify the subscription belongs to the given org.

	// TODO: Call Hyperswitch API to cancel the subscription/recurring payment.
	// For now, this is a placeholder.
	if err := s.cancelHyperswitchSubscription(ctx, subscriptionID); err != nil {
		return apierror.NewInternal("failed to cancel Hyperswitch subscription: " + err.Error())
	}

	// TODO: Update subscription status to "canceled" in the database.

	return nil
}

// CreatePaymentIntent creates a one-off payment intent via Hyperswitch.
func (s *BillingService) CreatePaymentIntent(ctx context.Context, orgID string, req model.CreatePaymentIntentRequest) (*model.PaymentIntent, error) {
	// TODO: Call Hyperswitch API to create a payment intent.
	//   POST {baseURL}/payments
	//   Body: { amount, currency, ... }
	//
	// For now, return a mock payment intent.
	hsPaymentID, clientSecret, err := s.createHyperswitchPayment(ctx, orgID, req.Amount, req.Currency)
	if err != nil {
		return nil, apierror.NewInternal("failed to create Hyperswitch payment: " + err.Error())
	}

	now := time.Now().UTC()
	pi := &model.PaymentIntent{
		ID:                   generateID("pi"),
		OrgID:                orgID,
		Amount:               req.Amount,
		Currency:             req.Currency,
		Status:               model.PaymentIntentStatusRequiresPayment,
		HyperswitchPaymentID: hsPaymentID,
		ClientSecret:         clientSecret,
		CreatedAt:            now,
	}

	// TODO: Persist payment intent to database via repository layer.

	return pi, nil
}

// VerifyWebhookSignature verifies the Hyperswitch webhook HMAC-SHA256 signature.
// Returns nil if the signature is valid.
func (s *BillingService) VerifyWebhookSignature(payload []byte, signature string) error {
	if s.webhookSecret == "" {
		// TODO: In production, always require a webhook secret.
		return nil
	}

	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return apierror.NewUnauthorized("invalid webhook signature")
	}
	return nil
}

// HandleWebhook processes a verified Hyperswitch webhook event.
func (s *BillingService) HandleWebhook(_ context.Context, event model.HyperswitchWebhookPayload) error {
	// TODO: Implement webhook event handling for different event types.
	// Common Hyperswitch event types:
	//   - payment_succeeded
	//   - payment_failed
	//   - payment_processing
	//   - refund_succeeded
	//   - dispute_opened
	//
	// Each event type should update the relevant subscription or payment
	// record in the database.

	switch event.EventType {
	case "payment_succeeded":
		// TODO: Mark the corresponding payment intent as succeeded.
		// TODO: If linked to a subscription, extend the billing period.
		return nil

	case "payment_failed":
		// TODO: Mark the corresponding payment intent as failed.
		// TODO: If linked to a subscription, set status to "past_due".
		return nil

	case "refund_succeeded":
		// TODO: Handle refund logic.
		return nil

	case "dispute_opened":
		// TODO: Handle dispute logic; possibly suspend the subscription.
		return nil

	default:
		// Unhandled event types are logged but not treated as errors.
		// TODO: Add structured logging here.
		return nil
	}
}

// --- Hyperswitch API helpers (mock/placeholder implementations) ---

// createHyperswitchSubscription calls the Hyperswitch API to create a
// recurring payment for the given plan.
func (s *BillingService) createHyperswitchSubscription(ctx context.Context, orgID string, plan *model.Plan) (string, error) {
	// TODO: Replace with real Hyperswitch API call.
	// POST {baseURL}/payments
	// Headers: api-key: {apiKey}
	// Body:
	//   {
	//     "amount": plan.PriceMonthly,
	//     "currency": "USD",
	//     "customer_id": orgID,
	//     "setup_future_usage": "off_session",
	//     "metadata": { "plan_id": plan.ID, "org_id": orgID }
	//   }

	body := map[string]any{
		"amount":              plan.PriceMonthly,
		"currency":            "USD",
		"customer_id":         orgID,
		"setup_future_usage":  "off_session",
		"metadata": map[string]string{
			"plan_id": plan.ID,
			"org_id":  orgID,
		},
	}

	respBody, err := s.hyperswitchRequest(ctx, http.MethodPost, "/payments", body)
	if err != nil {
		return "", err
	}

	// TODO: Parse the actual Hyperswitch response to extract payment_id.
	paymentID, _ := respBody["payment_id"].(string)
	if paymentID == "" {
		paymentID = "hs_sub_" + orgID + "_" + plan.ID
	}
	return paymentID, nil
}

// cancelHyperswitchSubscription cancels a subscription via Hyperswitch.
func (s *BillingService) cancelHyperswitchSubscription(ctx context.Context, subscriptionID string) error {
	// TODO: Replace with real Hyperswitch API call.
	// POST {baseURL}/payments/{payment_id}/cancel
	// Headers: api-key: {apiKey}

	_, err := s.hyperswitchRequest(ctx, http.MethodPost, "/payments/"+subscriptionID+"/cancel", nil)
	return err
}

// createHyperswitchPayment creates a one-off payment via Hyperswitch.
func (s *BillingService) createHyperswitchPayment(ctx context.Context, orgID string, amount int64, currency string) (string, string, error) {
	// TODO: Replace with real Hyperswitch API call.
	// POST {baseURL}/payments
	// Headers: api-key: {apiKey}
	// Body:
	//   {
	//     "amount": amount,
	//     "currency": currency,
	//     "customer_id": orgID
	//   }

	body := map[string]any{
		"amount":      amount,
		"currency":    currency,
		"customer_id": orgID,
	}

	respBody, err := s.hyperswitchRequest(ctx, http.MethodPost, "/payments", body)
	if err != nil {
		return "", "", err
	}

	// TODO: Parse the actual Hyperswitch response.
	paymentID, _ := respBody["payment_id"].(string)
	clientSecret, _ := respBody["client_secret"].(string)
	if paymentID == "" {
		paymentID = "hs_pay_" + orgID
	}
	if clientSecret == "" {
		clientSecret = "hs_secret_placeholder"
	}
	return paymentID, clientSecret, nil
}

// hyperswitchRequest sends an HTTP request to the Hyperswitch API.
// It returns the decoded JSON response body.
func (s *BillingService) hyperswitchRequest(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := s.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hyperswitch API error (status %d): %s", resp.StatusCode, string(respData))
	}

	var result map[string]any
	if len(respData) > 0 {
		if err := json.Unmarshal(respData, &result); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
	}
	return result, nil
}

// generateID creates a simple prefixed ID.
// TODO: Replace with UUID generation from the repository layer.
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
