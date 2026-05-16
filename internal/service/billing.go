package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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
	CancelWithRefund(ctx context.Context, tx pgx.Tx, orgID, subscriptionID, refundID string) (*model.Subscription, error)
	UpdateSeatCount(ctx context.Context, tx pgx.Tx, orgID, subscriptionID string, seatCount int) (*model.Subscription, error)
	ExtendSubscriptionPeriod(ctx context.Context, tx pgx.Tx, hyperswitchID string) (*model.Subscription, error)
	CreatePaymentIntent(ctx context.Context, tx pgx.Tx, pi *model.PaymentIntent) (*model.PaymentIntent, error)
	InsertPaymentEvent(ctx context.Context, tx pgx.Tx, orgID, eventType, paymentID, status string, rawPayload []byte) (bool, error)
	ClearTrialFields(ctx context.Context, tx pgx.Tx, orgID, subscriptionID string) (*model.Subscription, error)
}

// HyperswitchClient defines the interface for Hyperswitch API operations.
type HyperswitchClient interface {
	CreatePayment(ctx context.Context, req *hyperswitch.CreatePaymentRequest) (*hyperswitch.PaymentResponse, error)
	CancelPayment(ctx context.Context, paymentID string) error
	CreateRefund(ctx context.Context, req *hyperswitch.CreateRefundRequest) (*hyperswitch.RefundResponse, error)
}

// BillingService contains business logic for subscription and payment management
// via the Hyperswitch payment orchestration platform.
type BillingService struct {
	repo            BillingRepository
	pool            *pgxpool.Pool
	hsClient        HyperswitchClient
	webhookSecret   string
	slackWebhookURL string
	plans           map[string]model.Plan
}

// NewBillingService creates a new BillingService.
func NewBillingService(repo BillingRepository, pool *pgxpool.Pool, hsClient HyperswitchClient, webhookSecret string, slackWebhookURL string) *BillingService {
	plans := make(map[string]model.Plan)
	for _, p := range model.DefaultPlans() {
		plans[p.ID] = p
	}
	return &BillingService{
		repo:            repo,
		pool:            pool,
		hsClient:        hsClient,
		webhookSecret:   webhookSecret,
		slackWebhookURL: slackWebhookURL,
		plans:           plans,
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

// CreateEnterpriseSubscription provisions an Enterprise subscription via the admin-only
// sales-led flow. No Hyperswitch payment is created -- invoicing is handled externally.
func (s *BillingService) CreateEnterpriseSubscription(ctx context.Context, req model.CreateEnterpriseSubscriptionRequest) (*model.Subscription, error) {
	const minSeats = 20
	if req.SeatCount < minSeats {
		return nil, apierror.NewBadRequest(fmt.Sprintf("enterprise plan requires at least %d seats, got %d", minSeats, req.SeatCount))
	}

	plan, err := s.GetPlanByID("plan_enterprise")
	if err != nil {
		return nil, err
	}

	// Allow caller to override the per-seat price (e.g. for negotiated deals).
	if req.PricePerSeatMonthlyPaise != nil {
		plan.PricePerSeatMonthly = *req.PricePerSeatMonthlyPaise
	}

	sub := &model.Subscription{
		OrgID:              req.OrgID,
		PlanID:             plan.ID,
		Status:             model.SubscriptionStatusActive,
		CurrentPeriodStart: req.ContractStart,
		CurrentPeriodEnd:   req.ContractEnd,
	}

	if s.pool == nil {
		return nil, apierror.NewInternal("database pool is not configured")
	}

	var result *model.Subscription
	err = db.WithOrgID(ctx, s.pool, req.OrgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.CreateSubscription(ctx, tx, sub)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to persist enterprise subscription", "error", err)
		return nil, apierror.NewInternal("failed to persist enterprise subscription")
	}

	// Best-effort Slack notification -- failure does not abort the response.
	if s.slackWebhookURL != "" {
		go s.sendSlackNotification(context.Background(), req, result.ID) //nolint:contextcheck // best-effort fire-and-forget; context must outlive request
	}

	return result, nil
}

// sendSlackNotification posts an enterprise deal notification to the configured Slack webhook.
func (s *BillingService) sendSlackNotification(ctx context.Context, req model.CreateEnterpriseSubscriptionRequest, subscriptionID string) {
	msg := fmt.Sprintf(
		":tada: New Enterprise subscription created!\nOrg: %s | Seats: %d | Sub ID: %s | Period: %s to %s",
		req.OrgID, req.SeatCount, subscriptionID,
		req.ContractStart.Format(time.DateOnly),
		req.ContractEnd.Format(time.DateOnly),
	)
	payload, err := json.Marshal(map[string]any{
		"text": msg,
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to marshal Slack notification", "error", err)
		return
	}

	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodPost, s.slackWebhookURL, strings.NewReader(string(payload)))
	if err != nil {
		slog.WarnContext(ctx, "failed to build Slack HTTP request", "error", err)
		return
	}
	reqHTTP.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(reqHTTP)
	if err != nil {
		slog.WarnContext(ctx, "Slack notification request failed", "error", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		slog.WarnContext(ctx, "Slack notification returned non-OK status", "status", resp.StatusCode)
	}
}

// GetActiveSubscription returns the current active subscription for an org, or nil.
func (s *BillingService) GetActiveSubscription(ctx context.Context, orgID string) (*model.Subscription, error) {
	var sub *model.Subscription
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		sub, e = s.repo.GetActiveSubscription(ctx, tx, orgID)
		return e
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to retrieve subscription")
	}
	return sub, nil
}

// CreateSubscription creates a new subscription for the given organisation.
// For paid plans, it calls Hyperswitch to set up a recurring payment via Razorpay.
func (s *BillingService) CreateSubscription(ctx context.Context, orgID string, req model.CreateSubscriptionRequest) (*model.Subscription, error) {
	plan, err := s.GetPlanByID(req.PlanID)
	if err != nil {
		return nil, err
	}

	if plan.MinSeats > 0 && req.SeatCount < plan.MinSeats {
		return nil, apierror.NewBadRequest(fmt.Sprintf("plan %q requires a minimum of %d seats", plan.ID, plan.MinSeats))
	}

	// Default billing cycle to "monthly" when not specified.
	if req.BillingCycle == "" {
		req.BillingCycle = "monthly"
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
		slog.ErrorContext(ctx, "failed to check existing subscription", "error", err)
		return nil, apierror.NewInternal("failed to check existing subscription")
	}
	if existing != nil {
		return nil, apierror.NewConflict("organisation already has an active subscription: " + existing.ID)
	}

	// Compute billing period end and payment amount based on billing cycle.
	// Annual billing: 10 months' price (2 months free ≈ 20% discount).
	// Monthly billing: 1 month's price.
	var periodEnd time.Time
	var amount int64
	if req.BillingCycle == "annual" {
		periodEnd = now.AddDate(1, 0, 0)
		amount = plan.PricePerSeatMonthly * int64(req.SeatCount) * 10
	} else {
		periodEnd = now.AddDate(0, 1, 0)
		amount = plan.PricePerSeatMonthly * int64(req.SeatCount)
	}

	// For the free plan, no payment orchestration is needed.
	if plan.Tier == model.PlanTierFree {
		sub := &model.Subscription{
			OrgID:              orgID,
			PlanID:             plan.ID,
			Status:             model.SubscriptionStatusActive,
			SeatCount:          req.SeatCount,
			BillingCycle:       req.BillingCycle,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   periodEnd,
		}

		var result *model.Subscription
		err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
			var e error
			result, e = s.repo.CreateSubscription(ctx, tx, sub)
			return e
		})
		if err != nil {
			slog.ErrorContext(ctx, "failed to persist free subscription", "error", err)
			return nil, apierror.NewInternal("failed to persist free subscription")
		}
		return result, nil
	}

	// Paid plan: charge per seat in INR via Hyperswitch with Razorpay as the connector.
	hsResp, err := s.hsClient.CreatePayment(ctx, &hyperswitch.CreatePaymentRequest{
		Amount:           amount,
		Currency:         "INR",
		CustomerID:       orgID,
		SetupFutureUsage: "off_session",
		Metadata: map[string]string{
			"plan_id":       plan.ID,
			"org_id":        orgID,
			"seat_count":    fmt.Sprintf("%d", req.SeatCount),
			"billing_cycle": req.BillingCycle,
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to create Hyperswitch payment", "error", err)
		return nil, apierror.NewInternal("failed to create Hyperswitch payment")
	}

	trialEndsAt := now.Add(14 * 24 * time.Hour)
	gracePeriodEndsAt := now.Add(21 * 24 * time.Hour)
	sub := &model.Subscription{
		OrgID:                     orgID,
		PlanID:                    plan.ID,
		Status:                    model.SubscriptionStatusTrialing,
		SeatCount:                 req.SeatCount,
		BillingCycle:              req.BillingCycle,
		HyperswitchSubscriptionID: hsResp.PaymentID,
		CurrentPeriodStart:        now,
		CurrentPeriodEnd:          periodEnd,
		TrialEndsAt:               &trialEndsAt,
		GracePeriodEndsAt:         &gracePeriodEndsAt,
	}

	var result *model.Subscription
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.CreateSubscription(ctx, tx, sub)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to persist subscription", "error", err)
		return nil, apierror.NewInternal("failed to persist subscription")
	}
	return result, nil
}

// CancelSubscription cancels the subscription for the given organisation.
//
// Monthly subscriptions are cancelled immediately with no refund (low-commitment).
// Annual subscriptions receive a prorated refund for unused complete months:
//
//	refund_amount = PricePerSeatMonthly × SeatCount × floor((period_end - now) / 30 days)
//
// Access continues until current_period_end regardless of billing cycle.
func (s *BillingService) CancelSubscription(ctx context.Context, orgID string, subscriptionID string) error {
	var sub *model.Subscription
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		sub, e = s.repo.GetSubscriptionByID(ctx, tx, orgID, subscriptionID)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to look up subscription", "error", err)
		return apierror.NewInternal("failed to look up subscription")
	}
	if sub == nil {
		return apierror.NewNotFound("subscription not found")
	}

	if sub.BillingCycle == "annual" && sub.HyperswitchSubscriptionID != "" {
		return s.cancelAnnualSubscription(ctx, orgID, sub)
	}

	// Monthly or free subscription: cancel immediately with no refund.
	return db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		_, e := s.repo.UpdateSubscriptionStatus(ctx, tx, orgID, subscriptionID, model.SubscriptionStatusCanceled)
		return e
	})
}

// cancelAnnualSubscription handles cancellation of an annual subscription with prorated refund.
// It calculates unused complete months and triggers a Hyperswitch refund if applicable.
func (s *BillingService) cancelAnnualSubscription(ctx context.Context, orgID string, sub *model.Subscription) error {
	now := time.Now().UTC()

	// Calculate unused complete months = floor((period_end - now) / 30 days).
	remaining := sub.CurrentPeriodEnd.Sub(now)
	unusedMonths := int(remaining.Hours() / (24 * 30))

	plan, err := s.GetPlanByID(sub.PlanID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to look up plan for refund calculation", "plan_id", sub.PlanID, "error", err)
		return apierror.NewInternal("failed to look up plan for refund calculation")
	}

	refundID := ""
	if unusedMonths > 0 {
		refundAmount := plan.PricePerSeatMonthly * int64(sub.SeatCount) * int64(unusedMonths)
		slog.InfoContext(ctx, "annual cancellation: issuing prorated refund",
			"subscription_id", sub.ID,
			"unused_months", unusedMonths,
			"refund_amount_paise", refundAmount,
		)

		refundResp, refundErr := s.hsClient.CreateRefund(ctx, &hyperswitch.CreateRefundRequest{
			PaymentID: sub.HyperswitchSubscriptionID,
			Amount:    refundAmount,
			Reason:    "annual_cancellation",
		})
		if refundErr != nil {
			slog.ErrorContext(ctx, "failed to create Hyperswitch refund", "error", refundErr)
			return apierror.NewInternal("failed to create refund")
		}
		refundID = refundResp.RefundID
	} else {
		slog.InfoContext(ctx, "annual cancellation: zero unused months, no refund issued",
			"subscription_id", sub.ID,
			"period_end", sub.CurrentPeriodEnd,
		)
	}

	// Persist cancellation + refund ID atomically.
	return db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		if refundID != "" {
			_, e := s.repo.CancelWithRefund(ctx, tx, orgID, sub.ID, refundID)
			return e
		}
		_, e := s.repo.UpdateSubscriptionStatus(ctx, tx, orgID, sub.ID, model.SubscriptionStatusCanceled)
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
		slog.ErrorContext(ctx, "failed to create Hyperswitch payment", "error", err)
		return nil, apierror.NewInternal("failed to create Hyperswitch payment")
	}

	pi := &model.PaymentIntent{
		OrgID:                orgID,
		Amount:               req.Amount,
		Currency:             req.Currency,
		Status:               model.PaymentIntentStatusRequiresPayment,
		HyperswitchPaymentID: hsResp.PaymentID,
		ClientSecret:         hsResp.ClientSecret,
	}

	var result *model.PaymentIntent
	if err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.CreatePaymentIntent(ctx, tx, pi)
		return e
	}); err != nil {
		slog.ErrorContext(ctx, "failed to persist payment intent", "error", err)
		// Best-effort cancel the orphaned Hyperswitch payment with a fresh
		// context -- the original ctx may already be canceled/timed out.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if cancelErr := s.hsClient.CancelPayment(cleanupCtx, hsResp.PaymentID); cancelErr != nil { //nolint:contextcheck
			slog.ErrorContext(ctx, "failed to cancel orphaned Hyperswitch payment", "payment_id", hsResp.PaymentID, "error", cancelErr)
		}
		return nil, apierror.NewInternal("failed to persist payment intent")
	}

	return result, nil
}

// VerifyWebhookSignature verifies the Hyperswitch webhook HMAC-SHA256 signature.
func (s *BillingService) VerifyWebhookSignature(payload []byte, signature string) error {
	if s.webhookSecret == "" {
		slog.Warn("webhook signature verification skipped: RAVEN_HYPERSWITCH_WEBHOOK_SECRET is not configured")
		return apierror.NewUnauthorized("webhook signature verification is not configured")
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

	// Check whether this payment carries a pending seat count update (proration intent).
	// If so, update seat_count on the subscription rather than extending the period.
	pendingSeatCount := extractPendingSeatCount(event)
	subscriptionID := extractSubscriptionIDFromWebhook(event)

	if pendingSeatCount > 0 && subscriptionID != "" {
		// Seat proration payment: update seat_count on the referenced subscription.
		// The subscription_id is stored in metadata; use bypass-RLS since we already set it above.
		if _, err := tx.Exec(ctx,
			"UPDATE subscriptions SET seat_count = $1 WHERE id = $2",
			pendingSeatCount, subscriptionID,
		); err != nil {
			return fmt.Errorf("update seat count via webhook: %w", err)
		}
	} else {
		// Standard recurring payment: find subscription by Hyperswitch ID and extend billing period.
		sub, err := s.repo.GetSubscriptionByHyperswitchID(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("get subscription by hyperswitch id: %w", err)
		}
		if sub != nil {
			if _, err := s.repo.ExtendSubscriptionPeriod(ctx, tx, paymentID); err != nil {
				return fmt.Errorf("extend subscription period: %w", err)
			}
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

	// Mark subscription as past_due -- triggers free tier downgrade.
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

// UpdateSeatCount initiates a mid-cycle seat increase for an active subscription.
// It calculates the prorated charge for the remaining billing period, creates a
// Hyperswitch payment intent, and returns it to the caller. The seat_count is updated
// on the subscription only after the payment succeeds via the webhook handler.
func (s *BillingService) UpdateSeatCount(ctx context.Context, orgID, subscriptionID string, req model.UpdateSeatCountRequest) (*model.PaymentIntent, error) {
	// Fetch subscription under RLS.
	var sub *model.Subscription
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		sub, e = s.repo.GetSubscriptionByID(ctx, tx, orgID, subscriptionID)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to look up subscription", "error", err)
		return nil, apierror.NewInternal("failed to look up subscription")
	}
	if sub == nil {
		return nil, apierror.NewNotFound("subscription not found")
	}
	if sub.Status != model.SubscriptionStatusActive && sub.Status != model.SubscriptionStatusTrialing {
		return nil, apierror.NewBadRequest("seat count can only be increased on an active or trialing subscription")
	}

	plan, err := s.GetPlanByID(sub.PlanID)
	if err != nil {
		return nil, err
	}

	// Validate requested seat count.
	if plan.MinSeats > 0 && req.SeatCount < plan.MinSeats {
		return nil, apierror.NewBadRequest(fmt.Sprintf("plan %q requires a minimum of %d seats", plan.ID, plan.MinSeats))
	}
	if req.SeatCount <= sub.SeatCount {
		return nil, apierror.NewBadRequest(fmt.Sprintf("new seat count (%d) must be greater than current seat count (%d)", req.SeatCount, sub.SeatCount))
	}

	// Calculate prorated charge for the remaining billing period.
	now := time.Now().UTC()
	totalDays := sub.CurrentPeriodEnd.Sub(sub.CurrentPeriodStart).Hours() / 24
	remainingDays := sub.CurrentPeriodEnd.Sub(now).Hours() / 24
	if remainingDays < 0 {
		remainingDays = 0
	}
	addedSeats := req.SeatCount - sub.SeatCount

	var proratedAmount int64
	if totalDays > 0 {
		proratedAmount = int64(float64(plan.PricePerSeatMonthly) * float64(addedSeats) * (remainingDays / totalDays))
	}
	// Minimum charge of 1 paise to satisfy Hyperswitch's amount > 0 constraint.
	if proratedAmount < 1 {
		proratedAmount = 1
	}

	// Create Hyperswitch payment intent for the prorated amount.
	hsResp, err := s.hsClient.CreatePayment(ctx, &hyperswitch.CreatePaymentRequest{
		Amount:     proratedAmount,
		Currency:   "INR",
		CustomerID: orgID,
		Metadata: map[string]string{
			"org_id":              orgID,
			"subscription_id":     subscriptionID,
			"pending_seat_count":  strconv.Itoa(req.SeatCount),
			"intent_type":         "seat_proration",
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to create Hyperswitch payment for seat proration", "error", err)
		return nil, apierror.NewInternal("failed to create payment intent for seat proration")
	}

	pi := &model.PaymentIntent{
		OrgID:                orgID,
		Amount:               proratedAmount,
		Currency:             "INR",
		Status:               model.PaymentIntentStatusRequiresPayment,
		HyperswitchPaymentID: hsResp.PaymentID,
		ClientSecret:         hsResp.ClientSecret,
	}

	var result *model.PaymentIntent
	if err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		result, e = s.repo.CreatePaymentIntent(ctx, tx, pi)
		return e
	}); err != nil {
		slog.ErrorContext(ctx, "failed to persist proration payment intent", "error", err)
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if cancelErr := s.hsClient.CancelPayment(cleanupCtx, hsResp.PaymentID); cancelErr != nil { //nolint:contextcheck
			slog.ErrorContext(ctx, "failed to cancel orphaned Hyperswitch payment", "payment_id", hsResp.PaymentID, "error", cancelErr)
		}
		return nil, apierror.NewInternal("failed to persist payment intent")
	}

	return result, nil
}

// ReactivateSubscription sets a subscription back to active and clears trial timestamps.
// This is called when an org successfully completes a payment during the trial/grace period.
func (s *BillingService) ReactivateSubscription(ctx context.Context, orgID, subscriptionID string) error {
	return db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		sub, e := s.repo.GetSubscriptionByID(ctx, tx, orgID, subscriptionID)
		if e != nil {
			return fmt.Errorf("get subscription: %w", e)
		}
		if sub == nil {
			return apierror.NewNotFound("subscription not found")
		}
		if _, e = s.repo.UpdateSubscriptionStatus(ctx, tx, orgID, subscriptionID, model.SubscriptionStatusActive); e != nil {
			return fmt.Errorf("update status to active: %w", e)
		}
		if _, e = s.repo.ClearTrialFields(ctx, tx, orgID, subscriptionID); e != nil {
			return fmt.Errorf("clear trial fields: %w", e)
		}
		return nil
	})
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

// extractPendingSeatCount returns the pending_seat_count from a webhook's metadata,
// or 0 if the field is absent or unparseable.
func extractPendingSeatCount(event model.HyperswitchWebhookPayload) int {
	metadata, ok := event.Content["metadata"].(map[string]any)
	if !ok {
		return 0
	}
	raw, _ := metadata["pending_seat_count"].(string)
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

// extractSubscriptionIDFromWebhook returns the subscription_id from a webhook's metadata.
func extractSubscriptionIDFromWebhook(event model.HyperswitchWebhookPayload) string {
	metadata, ok := event.Content["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	id, _ := metadata["subscription_id"].(string)
	return id
}
