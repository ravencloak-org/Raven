package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/model"
)

// BillingRepository handles database operations for subscriptions, payment events,
// and usage queries. All operations run inside a pgx.Tx with org_id set for RLS.
type BillingRepository struct {
	pool *pgxpool.Pool
}

// NewBillingRepository creates a new BillingRepository.
func NewBillingRepository(pool *pgxpool.Pool) *BillingRepository {
	return &BillingRepository{pool: pool}
}

const (
	sqlSubscriptionInsert = `
		INSERT INTO subscriptions (org_id, plan_id, status, hyperswitch_subscription_id, current_period_start, current_period_end)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, org_id, plan_id, status, hyperswitch_subscription_id,
		          current_period_start, current_period_end, created_at`

	sqlSubscriptionByID = `
		SELECT id, org_id, plan_id, status, hyperswitch_subscription_id,
		       current_period_start, current_period_end, created_at
		FROM subscriptions
		WHERE id = $1 AND org_id = $2`

	sqlSubscriptionByHyperswitchID = `
		SELECT id, org_id, plan_id, status, hyperswitch_subscription_id,
		       current_period_start, current_period_end, created_at
		FROM subscriptions
		WHERE hyperswitch_subscription_id = $1`

	sqlSubscriptionActiveByOrg = `
		SELECT id, org_id, plan_id, status, hyperswitch_subscription_id,
		       current_period_start, current_period_end, created_at
		FROM subscriptions
		WHERE org_id = $1 AND status IN ('active', 'trialing', 'past_due')
		LIMIT 1`

	sqlSubscriptionUpdateStatus = `
		UPDATE subscriptions SET status = $3
		WHERE id = $1 AND org_id = $2
		RETURNING id, org_id, plan_id, status, hyperswitch_subscription_id,
		          current_period_start, current_period_end, created_at`

	sqlSubscriptionExtendPeriod = `
		UPDATE subscriptions SET
			status = 'active',
			current_period_start = now(),
			current_period_end   = now() + interval '1 month'
		WHERE hyperswitch_subscription_id = $1
		RETURNING id, org_id, plan_id, status, hyperswitch_subscription_id,
		          current_period_start, current_period_end, created_at`

	sqlPaymentIntentInsert = `
		INSERT INTO payment_intents (org_id, amount, currency, status, hyperswitch_payment_id, client_secret)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, org_id, amount, currency, status, hyperswitch_payment_id, client_secret, created_at`

	sqlPaymentEventInsert = `
		INSERT INTO payment_events (org_id, event_type, payment_id, status, raw_payload)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (payment_id, event_type) DO NOTHING
		RETURNING id`

	sqlPaymentEventExists = `
		SELECT EXISTS(SELECT 1 FROM payment_events WHERE payment_id = $1 AND event_type = $2)`

	sqlCountKBsByOrg = `
		SELECT COUNT(*) FROM knowledge_bases
		WHERE org_id = $1 AND archived_at IS NULL`

	sqlCountMembersByOrg = `
		SELECT COUNT(DISTINCT user_id) FROM workspace_members
		WHERE workspace_id IN (SELECT id FROM workspaces WHERE org_id = $1)`

	sqlVoiceUsageForPeriod = `
		SELECT COALESCE(SUM(total_duration_seconds), 0)
		FROM voice_usage_summaries
		WHERE org_id = $1 AND period_start >= $2`
)

func scanSubscription(row pgx.Row) (*model.Subscription, error) {
	var s model.Subscription
	err := row.Scan(
		&s.ID,
		&s.OrgID,
		&s.PlanID,
		&s.Status,
		&s.HyperswitchSubscriptionID,
		&s.CurrentPeriodStart,
		&s.CurrentPeriodEnd,
		&s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateSubscription inserts a new subscription and returns it with DB-assigned fields.
func (r *BillingRepository) CreateSubscription(ctx context.Context, tx pgx.Tx, sub *model.Subscription) (*model.Subscription, error) {
	row := tx.QueryRow(ctx, sqlSubscriptionInsert,
		sub.OrgID,
		sub.PlanID,
		sub.Status,
		sub.HyperswitchSubscriptionID,
		sub.CurrentPeriodStart,
		sub.CurrentPeriodEnd,
	)
	result, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.CreateSubscription: %w", err)
	}
	return result, nil
}

// CreatePaymentIntent inserts a new payment intent and returns it with DB-assigned fields.
func (r *BillingRepository) CreatePaymentIntent(ctx context.Context, tx pgx.Tx, pi *model.PaymentIntent) (*model.PaymentIntent, error) {
	row := tx.QueryRow(ctx, sqlPaymentIntentInsert,
		pi.OrgID,
		pi.Amount,
		pi.Currency,
		pi.Status,
		pi.HyperswitchPaymentID,
		pi.ClientSecret,
	)
	var result model.PaymentIntent
	err := row.Scan(
		&result.ID,
		&result.OrgID,
		&result.Amount,
		&result.Currency,
		&result.Status,
		&result.HyperswitchPaymentID,
		&result.ClientSecret,
		&result.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.CreatePaymentIntent: %w", err)
	}
	return &result, nil
}

// GetSubscriptionByID retrieves a subscription by primary key within an org.
func (r *BillingRepository) GetSubscriptionByID(ctx context.Context, tx pgx.Tx, orgID, subscriptionID string) (*model.Subscription, error) {
	row := tx.QueryRow(ctx, sqlSubscriptionByID, subscriptionID, orgID)
	s, err := scanSubscription(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("BillingRepository.GetSubscriptionByID: %w", err)
	}
	return s, nil
}

// GetSubscriptionByHyperswitchID retrieves a subscription by Hyperswitch payment ID.
// This query bypasses RLS org filter because webhooks don't have org context.
func (r *BillingRepository) GetSubscriptionByHyperswitchID(ctx context.Context, tx pgx.Tx, hsID string) (*model.Subscription, error) {
	row := tx.QueryRow(ctx, sqlSubscriptionByHyperswitchID, hsID)
	s, err := scanSubscription(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("BillingRepository.GetSubscriptionByHyperswitchID: %w", err)
	}
	return s, nil
}

// GetActiveSubscription returns the current active subscription for an org, or nil.
func (r *BillingRepository) GetActiveSubscription(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error) {
	row := tx.QueryRow(ctx, sqlSubscriptionActiveByOrg, orgID)
	s, err := scanSubscription(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("BillingRepository.GetActiveSubscription: %w", err)
	}
	return s, nil
}

// UpdateSubscriptionStatus changes the status of a subscription.
func (r *BillingRepository) UpdateSubscriptionStatus(ctx context.Context, tx pgx.Tx, orgID, subscriptionID string, status model.SubscriptionStatus) (*model.Subscription, error) {
	row := tx.QueryRow(ctx, sqlSubscriptionUpdateStatus, subscriptionID, orgID, status)
	s, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.UpdateSubscriptionStatus: %w", err)
	}
	return s, nil
}

// ExtendSubscriptionPeriod resets the billing period for a subscription identified
// by its Hyperswitch ID. Used when a recurring payment succeeds.
func (r *BillingRepository) ExtendSubscriptionPeriod(ctx context.Context, tx pgx.Tx, hyperswitchID string) (*model.Subscription, error) {
	row := tx.QueryRow(ctx, sqlSubscriptionExtendPeriod, hyperswitchID)
	s, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.ExtendSubscriptionPeriod: %w", err)
	}
	return s, nil
}

// InsertPaymentEvent records a webhook event for idempotency. Returns true if
// the row was inserted (i.e. the event is new), false if it already existed.
func (r *BillingRepository) InsertPaymentEvent(ctx context.Context, tx pgx.Tx, orgID, eventType, paymentID, status string, rawPayload []byte) (bool, error) {
	var id *string
	err := tx.QueryRow(ctx, sqlPaymentEventInsert, orgID, eventType, paymentID, status, rawPayload).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING — duplicate event, already processed.
			return false, nil
		}
		return false, fmt.Errorf("BillingRepository.InsertPaymentEvent: %w", err)
	}
	return true, nil
}

// PaymentEventExists checks whether a payment event has already been processed.
func (r *BillingRepository) PaymentEventExists(ctx context.Context, tx pgx.Tx, paymentID, eventType string) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, sqlPaymentEventExists, paymentID, eventType).Scan(&exists); err != nil {
		return false, fmt.Errorf("BillingRepository.PaymentEventExists: %w", err)
	}
	return exists, nil
}

// CountKBsByOrg returns the number of active (non-archived) knowledge bases for an org.
func (r *BillingRepository) CountKBsByOrg(ctx context.Context, tx pgx.Tx, orgID string) (int, error) {
	var count int
	err := tx.QueryRow(ctx, sqlCountKBsByOrg, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("BillingRepository.CountKBsByOrg: %w", err)
	}
	return count, nil
}

// CountMembersByOrg returns the number of distinct users across all workspaces in an org.
func (r *BillingRepository) CountMembersByOrg(ctx context.Context, tx pgx.Tx, orgID string) (int, error) {
	var count int
	err := tx.QueryRow(ctx, sqlCountMembersByOrg, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("BillingRepository.CountMembersByOrg: %w", err)
	}
	return count, nil
}

// GetVoiceUsageForPeriod returns total voice duration (seconds) for an org since periodStart.
func (r *BillingRepository) GetVoiceUsageForPeriod(ctx context.Context, tx pgx.Tx, orgID string, periodStart time.Time) (int, error) {
	var totalSeconds int
	err := tx.QueryRow(ctx, sqlVoiceUsageForPeriod, orgID, periodStart).Scan(&totalSeconds)
	if err != nil {
		return 0, fmt.Errorf("BillingRepository.GetVoiceUsageForPeriod: %w", err)
	}
	return totalSeconds, nil
}
