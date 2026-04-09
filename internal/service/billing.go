package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// BillingRepository is the persistence interface required by BillingService.
type BillingRepository interface {
	UpsertSubscription(ctx context.Context, tx pgx.Tx, s *model.Subscription) (*model.Subscription, error)
	GetActiveSubscription(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error)
	GetSubscriptionByID(ctx context.Context, tx pgx.Tx, subscriptionID string) (*model.Subscription, error)
	GetSubscriptionByHyperswitchID(ctx context.Context, hyperswitchID string) (*model.Subscription, error)
	UpdateSubscriptionStatus(ctx context.Context, tx pgx.Tx, subscriptionID string, status model.SubscriptionStatus, periodEnd *time.Time) error

	CreatePaymentIntent(ctx context.Context, tx pgx.Tx, pi *model.PaymentIntent) (*model.PaymentIntent, error)
	GetPaymentIntentByHyperswitchID(ctx context.Context, hyperswitchPaymentID string) (*model.PaymentIntent, error)
	UpdatePaymentIntentStatus(ctx context.Context, hyperswitchPaymentID string, status model.PaymentIntentStatus) error
}

// BillingService contains business logic for subscription and payment management
// via the Hyperswitch payment orchestration platform with Razorpay as the gateway.
type BillingService struct {
	repo          BillingRepository
	pool          *pgxpool.Pool
	httpClient    *http.Client
	baseURL       string
	apiKey        string
	webhookSecret string
	razorpayKeyID string
	plans         map[string]model.Plan
}

// NewBillingService creates a new BillingService.
// baseURL is the Hyperswitch API base URL (e.g. "https://sandbox.hyperswitch.io").
// apiKey is the Hyperswitch merchant API key.
// webhookSecret is used to verify Hyperswitch webhook HMAC-SHA256 signatures.
// razorpayKeyID is the Razorpay public key ID surfaced to the frontend for UPI/card collection.
func NewBillingService(repo BillingRepository, pool *pgxpool.Pool, baseURL, apiKey, webhookSecret, razorpayKeyID string) *BillingService {
	plans := make(map[string]model.Plan)
	for _, p := range model.DefaultPlans() {
		plans[p.ID] = p
	}
	return &BillingService{
		repo: repo,
		pool: pool,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:       baseURL,
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		razorpayKeyID: razorpayKeyID,
		plans:         plans,
	}
}

// GetPlans returns the available billing plans.
func (s *BillingService) GetPlans() []model.Plan {
	return model.DefaultPlans()
}

// getPlanByID returns a plan by ID, or a not-found error.
func (s *BillingService) getPlanByID(planID string) (*model.Plan, error) {
	p, ok := s.plans[planID]
	if !ok {
		return nil, apierror.NewNotFound("plan not found: " + planID)
	}
	return &p, nil
}

// CreateSubscription creates a new subscription for the given organisation.
//
// For the free plan no Hyperswitch call is made. For paid plans a Hyperswitch
// payment is created so the frontend can collect the first payment; the
// subscription is recorded as active once the payment_succeeded webhook fires.
// Calling this again while an active subscription already exists replaces it
// (upsert semantics) so callers can upgrade/downgrade plans idempotently.
func (s *BillingService) CreateSubscription(ctx context.Context, orgID string, req model.CreateSubscriptionRequest) (*model.Subscription, error) {
	plan, err := s.getPlanByID(req.PlanID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	if plan.Tier == model.PlanTierFree {
		// Free plan — no payment required; activate immediately.
		sub := &model.Subscription{
			OrgID:              orgID,
			PlanID:             plan.ID,
			Status:             model.SubscriptionStatusActive,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now.AddDate(0, 1, 0),
			CreatedAt:          now,
		}
		var persisted *model.Subscription
		if err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
			var e error
			persisted, e = s.repo.UpsertSubscription(ctx, tx, sub)
			return e
		}); err != nil {
			slog.ErrorContext(ctx, "BillingService.CreateSubscription db error", "error", err, "org_id", orgID)
			return nil, apierror.NewInternal("failed to persist free subscription")
		}
		slog.InfoContext(ctx, "free subscription activated", "org_id", orgID, "subscription_id", persisted.ID)
		return persisted, nil
	}

	// Paid plan — create a Hyperswitch payment intent for the first billing cycle.
	// The subscription is recorded as trialing; it transitions to active on payment_succeeded.
	hsPaymentID, clientSecret, err := s.createHyperswitchPayment(ctx, orgID, plan.PriceMonthly, "INR")
	if err != nil {
		return nil, apierror.NewInternal("failed to create Hyperswitch payment: " + err.Error())
	}

	sub := &model.Subscription{
		OrgID:                     orgID,
		PlanID:                    plan.ID,
		Status:                    model.SubscriptionStatusTrialing,
		HyperswitchSubscriptionID: hsPaymentID,
		CurrentPeriodStart:        now,
		CurrentPeriodEnd:          now.AddDate(0, 1, 0),
		CreatedAt:                 now,
	}

	// Also record the payment intent for idempotent webhook processing.
	pi := &model.PaymentIntent{
		OrgID:                orgID,
		Amount:               plan.PriceMonthly,
		Currency:             "INR",
		Status:               model.PaymentIntentStatusRequiresPayment,
		HyperswitchPaymentID: hsPaymentID,
		ClientSecret:         clientSecret,
		CreatedAt:            now,
	}

	var persisted *model.Subscription
	if err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		persisted, e = s.repo.UpsertSubscription(ctx, tx, sub)
		if e != nil {
			return e
		}
		_, e = s.repo.CreatePaymentIntent(ctx, tx, pi)
		return e
	}); err != nil {
		slog.ErrorContext(ctx, "BillingService.CreateSubscription db error", "error", err, "org_id", orgID)
		return nil, apierror.NewInternal("failed to persist subscription")
	}

	slog.InfoContext(ctx, "paid subscription pending payment",
		"org_id", orgID,
		"subscription_id", persisted.ID,
		"hs_payment_id", hsPaymentID,
		"plan_id", plan.ID,
	)
	// Return the subscription enriched with the client_secret so the frontend
	// can open the Hyperswitch SDK / Razorpay checkout.
	persisted.ClientSecret = clientSecret
	return persisted, nil
}

// CancelSubscription cancels the subscription for the given organisation.
func (s *BillingService) CancelSubscription(ctx context.Context, orgID string, subscriptionID string) error {
	var sub *model.Subscription
	if err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		sub, e = s.repo.GetSubscriptionByID(ctx, tx, subscriptionID)
		return e
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apierror.NewNotFound("subscription not found: " + subscriptionID)
		}
		return apierror.NewInternal("failed to fetch subscription")
	}

	if sub.OrgID != orgID {
		return apierror.NewNotFound("subscription not found: " + subscriptionID)
	}

	// Attempt to cancel in Hyperswitch when an external payment ID is present.
	if sub.HyperswitchSubscriptionID != "" {
		if err := s.cancelHyperswitchPayment(ctx, sub.HyperswitchSubscriptionID); err != nil {
			// Log but do not block local cancellation — the payment may already be finalized.
			slog.WarnContext(ctx, "Hyperswitch cancel call failed; proceeding with local cancellation",
				"error", err,
				"hs_payment_id", sub.HyperswitchSubscriptionID,
			)
		}
	}

	if err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.UpdateSubscriptionStatus(ctx, tx, subscriptionID, model.SubscriptionStatusCanceled, nil)
	}); err != nil {
		return apierror.NewInternal("failed to update subscription status")
	}

	slog.InfoContext(ctx, "subscription cancelled", "org_id", orgID, "subscription_id", subscriptionID)
	return nil
}

// CreatePaymentIntent creates a standalone one-off payment intent via Hyperswitch
// and persists it to the database for idempotent webhook processing.
func (s *BillingService) CreatePaymentIntent(ctx context.Context, orgID string, req model.CreatePaymentIntentRequest) (*model.PaymentIntent, error) {
	hsPaymentID, clientSecret, err := s.createHyperswitchPayment(ctx, orgID, req.Amount, req.Currency)
	if err != nil {
		return nil, apierror.NewInternal("failed to create Hyperswitch payment: " + err.Error())
	}

	now := time.Now().UTC()
	pi := &model.PaymentIntent{
		OrgID:                orgID,
		Amount:               req.Amount,
		Currency:             req.Currency,
		Status:               model.PaymentIntentStatusRequiresPayment,
		HyperswitchPaymentID: hsPaymentID,
		ClientSecret:         clientSecret,
		CreatedAt:            now,
	}

	var persisted *model.PaymentIntent
	if err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		persisted, e = s.repo.CreatePaymentIntent(ctx, tx, pi)
		return e
	}); err != nil {
		slog.ErrorContext(ctx, "BillingService.CreatePaymentIntent db error", "error", err, "org_id", orgID)
		return nil, apierror.NewInternal("failed to persist payment intent")
	}

	slog.InfoContext(ctx, "payment intent created",
		"org_id", orgID,
		"payment_intent_id", persisted.ID,
		"hs_payment_id", hsPaymentID,
	)
	return persisted, nil
}

// VerifyWebhookSignature verifies the Hyperswitch webhook HMAC-SHA256 signature.
// Returns nil if the signature is valid; returns an error if the secret is not
// configured (fail closed) or the signature does not match.
func (s *BillingService) VerifyWebhookSignature(payload []byte, signature string) error {
	if s.webhookSecret == "" {
		return apierror.NewInternal("webhook secret not configured")
	}

	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return apierror.NewUnauthorized("invalid webhook signature")
	}
	return nil
}

// HandleWebhook processes a verified Hyperswitch webhook event and updates
// the subscription state machine.
//
// Idempotency: duplicate events for the same payment_id are safe because the
// DB updates are idempotent (setting the same status twice is a no-op).
func (s *BillingService) HandleWebhook(ctx context.Context, event model.HyperswitchWebhookPayload) error {
	switch event.EventType {
	case "payment_succeeded", "payment.succeeded":
		return s.handlePaymentSucceeded(ctx, event.Content)

	case "payment_failed", "payment.failed":
		return s.handlePaymentFailed(ctx, event.Content)

	case "subscription_cancelled", "subscription.cancelled":
		return s.handleSubscriptionCancelled(ctx, event.Content)

	default:
		slog.InfoContext(ctx, "unhandled Hyperswitch webhook event", "event_type", event.EventType)
		return nil
	}
}

// handlePaymentSucceeded activates the linked subscription when a payment succeeds.
func (s *BillingService) handlePaymentSucceeded(ctx context.Context, content map[string]any) error {
	paymentID := extractPaymentID(content)
	if paymentID == "" {
		slog.WarnContext(ctx, "payment_succeeded webhook missing payment_id")
		return nil
	}

	// Update payment intent status.
	if err := s.repo.UpdatePaymentIntentStatus(ctx, paymentID, model.PaymentIntentStatusSucceeded); err != nil {
		// Not fatal if the intent doesn't exist (may be a sub-payment we don't track).
		slog.WarnContext(ctx, "could not update payment intent status", "hs_payment_id", paymentID, "error", err)
	}

	// Look up the associated subscription.
	sub, err := s.repo.GetSubscriptionByHyperswitchID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Payment not linked to a subscription — standalone payment intent, nothing to activate.
			slog.InfoContext(ctx, "payment_succeeded: no linked subscription found", "hs_payment_id", paymentID)
			return nil
		}
		return fmt.Errorf("handlePaymentSucceeded: lookup subscription: %w", err)
	}

	// Idempotency: if the subscription is already active, skip reprocessing.
	if sub.Status == model.SubscriptionStatusActive {
		slog.InfoContext(ctx, "payment_succeeded: subscription already active, skipping", "subscription_id", sub.ID, "hs_payment_id", paymentID)
		return nil
	}

	// Transition to active and extend the billing period by one month.
	newPeriodEnd := time.Now().UTC().AddDate(0, 1, 0)
	if err := db.WithOrgID(ctx, s.pool, sub.OrgID, func(tx pgx.Tx) error {
		return s.repo.UpdateSubscriptionStatus(ctx, tx, sub.ID, model.SubscriptionStatusActive, &newPeriodEnd)
	}); err != nil {
		return fmt.Errorf("handlePaymentSucceeded: activate subscription: %w", err)
	}

	slog.InfoContext(ctx, "subscription activated via payment_succeeded",
		"subscription_id", sub.ID,
		"org_id", sub.OrgID,
		"hs_payment_id", paymentID,
	)
	return nil
}

// handlePaymentFailed marks the subscription as past_due and the payment intent as failed.
func (s *BillingService) handlePaymentFailed(ctx context.Context, content map[string]any) error {
	paymentID := extractPaymentID(content)
	if paymentID == "" {
		slog.WarnContext(ctx, "payment_failed webhook missing payment_id")
		return nil
	}

	// Update payment intent status.
	if err := s.repo.UpdatePaymentIntentStatus(ctx, paymentID, model.PaymentIntentStatusFailed); err != nil {
		slog.WarnContext(ctx, "could not update payment intent status", "hs_payment_id", paymentID, "error", err)
	}

	// Downgrade linked subscription to past_due.
	sub, err := s.repo.GetSubscriptionByHyperswitchID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.InfoContext(ctx, "payment_failed: no linked subscription found", "hs_payment_id", paymentID)
			return nil
		}
		return fmt.Errorf("handlePaymentFailed: lookup subscription: %w", err)
	}

	if err := db.WithOrgID(ctx, s.pool, sub.OrgID, func(tx pgx.Tx) error {
		return s.repo.UpdateSubscriptionStatus(ctx, tx, sub.ID, model.SubscriptionStatusPastDue, nil)
	}); err != nil {
		return fmt.Errorf("handlePaymentFailed: set past_due: %w", err)
	}

	slog.InfoContext(ctx, "subscription set to past_due via payment_failed",
		"subscription_id", sub.ID,
		"org_id", sub.OrgID,
		"hs_payment_id", paymentID,
	)
	return nil
}

// handleSubscriptionCancelled downgrades the org to free tier on subscription cancellation.
func (s *BillingService) handleSubscriptionCancelled(ctx context.Context, content map[string]any) error {
	// Hyperswitch subscription.cancelled uses subscription_id or payment_id.
	subscriptionID := extractStringField(content, "subscription_id", "payment_id", "id")
	if subscriptionID == "" {
		slog.WarnContext(ctx, "subscription.cancelled webhook missing subscription identifier")
		return nil
	}

	sub, err := s.repo.GetSubscriptionByHyperswitchID(ctx, subscriptionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.InfoContext(ctx, "subscription.cancelled: no linked subscription found", "hs_id", subscriptionID)
			return nil
		}
		return fmt.Errorf("handleSubscriptionCancelled: lookup: %w", err)
	}

	if err := db.WithOrgID(ctx, s.pool, sub.OrgID, func(tx pgx.Tx) error {
		return s.repo.UpdateSubscriptionStatus(ctx, tx, sub.ID, model.SubscriptionStatusCanceled, nil)
	}); err != nil {
		return fmt.Errorf("handleSubscriptionCancelled: cancel: %w", err)
	}

	slog.InfoContext(ctx, "subscription cancelled via webhook",
		"subscription_id", sub.ID,
		"org_id", sub.OrgID,
	)
	return nil
}

// --- Hyperswitch HTTP helpers ---

// createHyperswitchPayment creates a payment via the Hyperswitch /payments endpoint
// with Razorpay as the connector. Returns (payment_id, client_secret, error).
func (s *BillingService) createHyperswitchPayment(ctx context.Context, orgID string, amount int64, currency string) (string, string, error) {
	body := map[string]any{
		"amount":      amount,
		"currency":    currency,
		"customer_id": orgID,
		// Route payment through Razorpay for UPI/card collection in India.
		"routing": map[string]any{
			"type": "single",
			"data": "razorpay",
		},
		"payment_method_types": []string{"upi", "card"},
		"metadata": map[string]string{
			"org_id": orgID,
		},
	}

	resp, err := s.hyperswitchRequest(ctx, http.MethodPost, "/payments", body)
	if err != nil {
		return "", "", err
	}

	paymentID, _ := resp["payment_id"].(string)
	clientSecret, _ := resp["client_secret"].(string)
	if paymentID == "" {
		return "", "", fmt.Errorf("hyperswitch response missing payment_id")
	}
	return paymentID, clientSecret, nil
}

// cancelHyperswitchPayment cancels a payment via Hyperswitch.
func (s *BillingService) cancelHyperswitchPayment(ctx context.Context, paymentID string) error {
	_, err := s.hyperswitchRequest(ctx, http.MethodPost, "/payments/"+paymentID+"/cancel", nil)
	return err
}

// hyperswitchRequest sends an authenticated JSON request to the Hyperswitch API.
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

// --- helpers ---

// extractPaymentID retrieves the payment_id from a webhook content map.
func extractPaymentID(content map[string]any) string {
	return extractStringField(content, "payment_id", "id")
}

// extractStringField returns the first non-empty string value for the given keys.
func extractStringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
