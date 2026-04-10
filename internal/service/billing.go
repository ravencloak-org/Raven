package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/hyperswitch"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// BillingRepository defines the persistence interface for billing operations.
type BillingRepository interface {
	CreateSubscription(ctx context.Context, tx pgx.Tx, sub *model.Subscription) (*model.Subscription, error)
	GetSubscriptionByID(ctx context.Context, tx pgx.Tx, orgID, subscriptionID string) (*model.Subscription, error)
	GetSubscriptionByHyperswitchID(ctx context.Context, tx pgx.Tx, hsID string) (*model.Subscription, error)
	GetActiveSubscription(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error)
	UpdateSubscriptionStatus(ctx context.Context, tx pgx.Tx, orgID, subscriptionID string, status model.SubscriptionStatus) (*model.Subscription, error)
	ExtendSubscriptionPeriod(ctx context.Context, tx pgx.Tx, hyperswitchID string) (*model.Subscription, error)
	InsertPaymentEvent(ctx context.Context, tx pgx.Tx, orgID, eventType, paymentID, status string, rawPayload []byte) (bool, error)
}

// HyperswitchClient defines the interface for Hyperswitch API operations.
type HyperswitchClient interface {
	CreatePayment(ctx context.Context, req *hyperswitch.CreatePaymentRequest) (*hyperswitch.PaymentResponse, error)
	CancelPayment(ctx context.Context, paymentID string) error
}

// BillingService contains business logic for subscription and payment management
// via the Hyperswitch payment orchestration platform.
type BillingService struct {
	repo          BillingRepository
	pool          *pgxpool.Pool
	hsClient      HyperswitchClient
	webhookSecret string
	plans         map[string]model.Plan
}

// NewBillingService creates a new BillingService.
func NewBillingService(repo BillingRepository, pool *pgxpool.Pool, hsClient HyperswitchClient, webhookSecret string) *BillingService {
	plans := make(map[string]model.Plan)
	for _, p := range model.DefaultPlans() {
		plans[p.ID] = p
	}
	return &BillingService{
		repo:          repo,
		pool:          pool,
		hsClient:      hsClient,
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
// For paid plans, it calls Hyperswitch to set up a recurring payment via Razorpay.
func (s *BillingService) CreateSubscription(ctx context.Context, orgID string, req model.CreateSubscriptionRequest) (*model.Subscription, error) {
	plan, err := s.GetPlanByID(req.PlanID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	// Check if org already has an active subscription.
	var existing *model.Subscription
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		existing, e = s.repo.GetActiveSubscription(ctx, tx, orgID)
		return e
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to check existing subscription: " + err.Error())
	}
	if existing != nil {
		return nil, apierror.NewConflict("organisation already has an active subscription: " + existing.ID)
	}

	// For the free plan, no payment orchestration is needed.
	if plan.Tier == model.PlanTierFree {
		sub := &model.Subscription{
			OrgID:              orgID,
			PlanID:             plan.ID,
			Status:             model.SubscriptionStatusActive,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		}

		var result *model.Subscription
		err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
			var e error
			result, e = s.repo.CreateSubscription(ctx, tx, sub)
			return e
		})
		if err != nil {
			return nil, apierror.NewInternal("failed to persist free subscription: " + err.Error())
		}
		return result, nil
	}

	// Paid plan: create a payment via Hyperswitch with Razorpay as the connector.
	hsResp, err := s.hsClient.CreatePayment(ctx, &hyperswitch.CreatePaymentRequest{
		Amount:           plan.PriceMonthly,
		Currency:         "USD",
		CustomerID:       orgID,
		SetupFutureUsage: "off_session",
		Metadata: map[string]string{
			"plan_id": plan.ID,
			"org_id":  orgID,
		},
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to create Hyperswitch payment: " + err.Error())
	}

	sub := &model.Subscription{
		OrgID:                     orgID,
		PlanID:                    plan.ID,
		Status:                    model.SubscriptionStatusActive,
		HyperswitchSubscriptionID: hsResp.PaymentID,
		CurrentPeriodStart:        now,
		CurrentPeriodEnd:          now.AddDate(0, 1, 0),
	}

	var result *model.Subscription
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.CreateSubscription(ctx, tx, sub)
		return e
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to persist subscription: " + err.Error())
	}
	return result, nil
}

// CancelSubscription cancels the subscription for the given organisation.
func (s *BillingService) CancelSubscription(ctx context.Context, orgID string, subscriptionID string) error {
	var sub *model.Subscription
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		sub, e = s.repo.GetSubscriptionByID(ctx, tx, orgID, subscriptionID)
		return e
	})
	if err != nil {
		return apierror.NewInternal("failed to look up subscription: " + err.Error())
	}
	if sub == nil {
		return apierror.NewNotFound("subscription not found")
	}

	// Cancel on Hyperswitch if there is a linked payment.
	if sub.HyperswitchSubscriptionID != "" {
		if err := s.hsClient.CancelPayment(ctx, sub.HyperswitchSubscriptionID); err != nil {
			return apierror.NewInternal("failed to cancel Hyperswitch payment: " + err.Error())
		}
	}

	// Update status in DB.
	return db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		_, e := s.repo.UpdateSubscriptionStatus(ctx, tx, orgID, subscriptionID, model.SubscriptionStatusCanceled)
		return e
	})
}

// CreatePaymentIntent creates a one-off payment intent via Hyperswitch.
func (s *BillingService) CreatePaymentIntent(ctx context.Context, orgID string, req model.CreatePaymentIntentRequest) (*model.PaymentIntent, error) {
	hsResp, err := s.hsClient.CreatePayment(ctx, &hyperswitch.CreatePaymentRequest{
		Amount:     req.Amount,
		Currency:   req.Currency,
		CustomerID: orgID,
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to create Hyperswitch payment: " + err.Error())
	}

	now := time.Now().UTC()
	pi := &model.PaymentIntent{
		ID:                   fmt.Sprintf("pi_%d", now.UnixNano()),
		OrgID:                orgID,
		Amount:               req.Amount,
		Currency:             req.Currency,
		Status:               model.PaymentIntentStatusRequiresPayment,
		HyperswitchPaymentID: hsResp.PaymentID,
		ClientSecret:         hsResp.ClientSecret,
		CreatedAt:            now,
	}
	return pi, nil
}

// VerifyWebhookSignature verifies the Hyperswitch webhook HMAC-SHA256 signature.
func (s *BillingService) VerifyWebhookSignature(payload []byte, signature string) error {
	if s.webhookSecret == "" {
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
// Events are idempotent: replaying the same event is a no-op.
func (s *BillingService) HandleWebhook(ctx context.Context, event model.HyperswitchWebhookPayload) error {
	paymentID, _ := event.Content["payment_id"].(string)
	if paymentID == "" {
		slog.Warn("webhook event missing payment_id", "event_type", event.EventType)
		return nil
	}

	// Extract org_id from metadata if available.
	orgID := extractOrgIDFromWebhook(event)

	switch event.EventType {
	case "payment_succeeded":
		return s.handlePaymentSucceeded(ctx, paymentID, orgID, event)
	case "payment_failed":
		return s.handlePaymentFailed(ctx, paymentID, orgID, event)
	case "subscription_cancelled":
		return s.handleSubscriptionCancelled(ctx, paymentID, orgID, event)
	default:
		slog.Info("unhandled webhook event type", "event_type", event.EventType)
		return nil
	}
}

func (s *BillingService) handlePaymentSucceeded(ctx context.Context, paymentID, orgID string, event model.HyperswitchWebhookPayload) error {
	rawPayload, _ := json.Marshal(event)

	// Use a bypass-RLS transaction for webhook processing since we may not
	// have org context from the webhook caller.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Enable RLS bypass for webhook processing.
	if _, err := tx.Exec(ctx, "SELECT set_config('app.bypass_rls', 'true', true)"); err != nil {
		return fmt.Errorf("set bypass_rls: %w", err)
	}

	// Idempotency check.
	isNew, err := s.repo.InsertPaymentEvent(ctx, tx, orgID, event.EventType, paymentID, "succeeded", rawPayload)
	if err != nil {
		return fmt.Errorf("insert payment event: %w", err)
	}
	if !isNew {
		slog.Info("duplicate webhook event, skipping", "payment_id", paymentID, "event_type", event.EventType)
		return tx.Commit(ctx)
	}

	// Find subscription by Hyperswitch ID and extend billing period.
	sub, err := s.repo.GetSubscriptionByHyperswitchID(ctx, tx, paymentID)
	if err != nil {
		return fmt.Errorf("get subscription by hyperswitch id: %w", err)
	}
	if sub != nil {
		if _, err := s.repo.ExtendSubscriptionPeriod(ctx, tx, paymentID); err != nil {
			return fmt.Errorf("extend subscription period: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (s *BillingService) handlePaymentFailed(ctx context.Context, paymentID, orgID string, event model.HyperswitchWebhookPayload) error {
	rawPayload, _ := json.Marshal(event)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SELECT set_config('app.bypass_rls', 'true', true)"); err != nil {
		return fmt.Errorf("set bypass_rls: %w", err)
	}

	isNew, err := s.repo.InsertPaymentEvent(ctx, tx, orgID, event.EventType, paymentID, "failed", rawPayload)
	if err != nil {
		return fmt.Errorf("insert payment event: %w", err)
	}
	if !isNew {
		return tx.Commit(ctx)
	}

	// Mark subscription as past_due — triggers free tier downgrade.
	sub, err := s.repo.GetSubscriptionByHyperswitchID(ctx, tx, paymentID)
	if err != nil {
		return fmt.Errorf("get subscription by hyperswitch id: %w", err)
	}
	if sub != nil {
		if _, err := s.repo.UpdateSubscriptionStatus(ctx, tx, sub.OrgID, sub.ID, model.SubscriptionStatusPastDue); err != nil {
			return fmt.Errorf("update subscription status: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (s *BillingService) handleSubscriptionCancelled(ctx context.Context, paymentID, orgID string, event model.HyperswitchWebhookPayload) error {
	rawPayload, _ := json.Marshal(event)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SELECT set_config('app.bypass_rls', 'true', true)"); err != nil {
		return fmt.Errorf("set bypass_rls: %w", err)
	}

	isNew, err := s.repo.InsertPaymentEvent(ctx, tx, orgID, event.EventType, paymentID, "cancelled", rawPayload)
	if err != nil {
		return fmt.Errorf("insert payment event: %w", err)
	}
	if !isNew {
		return tx.Commit(ctx)
	}

	sub, err := s.repo.GetSubscriptionByHyperswitchID(ctx, tx, paymentID)
	if err != nil {
		return fmt.Errorf("get subscription by hyperswitch id: %w", err)
	}
	if sub != nil {
		if _, err := s.repo.UpdateSubscriptionStatus(ctx, tx, sub.OrgID, sub.ID, model.SubscriptionStatusCanceled); err != nil {
			return fmt.Errorf("update subscription status: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// extractOrgIDFromWebhook attempts to extract org_id from the webhook payload metadata.
func extractOrgIDFromWebhook(event model.HyperswitchWebhookPayload) string {
	metadata, ok := event.Content["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	orgID, _ := metadata["org_id"].(string)
	return orgID
}
