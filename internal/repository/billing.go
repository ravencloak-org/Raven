package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/model"
)

// BillingRepository handles database persistence for subscriptions and payment intents.
// All methods that mutate tenant data require a pgx.Tx obtained from db.WithOrgID so
// that PostgreSQL RLS fires correctly.
type BillingRepository struct {
	pool *pgxpool.Pool
}

// NewBillingRepository creates a new BillingRepository.
func NewBillingRepository(pool *pgxpool.Pool) *BillingRepository {
	return &BillingRepository{pool: pool}
}

// --- Subscription methods ---

const subscriptionColumns = `id, org_id, plan_id, status, COALESCE(hyperswitch_subscription_id,'') AS hyperswitch_subscription_id, current_period_start, current_period_end, created_at`

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

// UpsertSubscription inserts a new subscription or updates the existing active one for an org.
// Uses ON CONFLICT on the partial unique index (org_id WHERE status IN active states).
// If an active subscription already exists the row is updated in-place rather than creating a duplicate.
func (r *BillingRepository) UpsertSubscription(ctx context.Context, tx pgx.Tx, s *model.Subscription) (*model.Subscription, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO subscriptions
		    (org_id, plan_id, status, hyperswitch_subscription_id, current_period_start, current_period_end)
		 VALUES ($1, $2, $3, NULLIF($4,''), $5, $6)
		 ON CONFLICT (org_id) WHERE status IN ('active','trialing','past_due')
		 DO UPDATE SET
		    plan_id                     = EXCLUDED.plan_id,
		    status                      = EXCLUDED.status,
		    hyperswitch_subscription_id = EXCLUDED.hyperswitch_subscription_id,
		    current_period_start        = EXCLUDED.current_period_start,
		    current_period_end          = EXCLUDED.current_period_end
		 RETURNING `+subscriptionColumns,
		s.OrgID,
		s.PlanID,
		string(s.Status),
		s.HyperswitchSubscriptionID,
		s.CurrentPeriodStart,
		s.CurrentPeriodEnd,
	)
	sub, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.UpsertSubscription: %w", err)
	}
	return sub, nil
}

// GetActiveSubscription returns the current active/trialing/past_due subscription for an org.
func (r *BillingRepository) GetActiveSubscription(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+subscriptionColumns+`
		 FROM subscriptions
		 WHERE org_id = $1
		   AND status IN ('active','trialing','past_due')
		 ORDER BY created_at DESC
		 LIMIT 1`,
		orgID,
	)
	sub, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.GetActiveSubscription: %w", err)
	}
	return sub, nil
}

// GetSubscriptionByID fetches any subscription by its internal UUID.
func (r *BillingRepository) GetSubscriptionByID(ctx context.Context, tx pgx.Tx, subscriptionID string) (*model.Subscription, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+subscriptionColumns+`
		 FROM subscriptions
		 WHERE id = $1`,
		subscriptionID,
	)
	sub, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.GetSubscriptionByID: %w", err)
	}
	return sub, nil
}

// GetSubscriptionByHyperswitchID looks up a subscription by its Hyperswitch subscription/payment ID.
// This is used by webhook handlers to locate the affected subscription without an org_id
// (webhooks arrive before we know the tenant), so it runs without RLS via a pool-level query.
func (r *BillingRepository) GetSubscriptionByHyperswitchID(ctx context.Context, hyperswitchID string) (*model.Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+subscriptionColumns+`
		 FROM subscriptions
		 WHERE hyperswitch_subscription_id = $1
		 LIMIT 1`,
		hyperswitchID,
	)
	sub, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.GetSubscriptionByHyperswitchID: %w", err)
	}
	return sub, nil
}

// UpdateSubscriptionStatus updates the status (and optionally period boundaries) of a subscription.
// Runs inside the caller-provided RLS-scoped transaction.
func (r *BillingRepository) UpdateSubscriptionStatus(ctx context.Context, tx pgx.Tx, subscriptionID string, status model.SubscriptionStatus, periodEnd *time.Time) error {
	var tag interface{ RowsAffected() int64 }
	var err error
	if periodEnd != nil {
		tag, err = tx.Exec(ctx,
			`UPDATE subscriptions
			 SET status = $2, current_period_end = $3
			 WHERE id = $1`,
			subscriptionID, string(status), *periodEnd,
		)
	} else {
		tag, err = tx.Exec(ctx,
			`UPDATE subscriptions SET status = $2 WHERE id = $1`,
			subscriptionID, string(status),
		)
	}
	if err != nil {
		return fmt.Errorf("BillingRepository.UpdateSubscriptionStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("BillingRepository.UpdateSubscriptionStatus: subscription %s not found", subscriptionID)
	}
	return nil
}

// --- PaymentIntent methods ---

const paymentIntentColumns = `id, org_id, amount, currency, status, COALESCE(hyperswitch_payment_id,'') AS hyperswitch_payment_id, COALESCE(client_secret,'') AS client_secret, created_at`

func scanPaymentIntent(row pgx.Row) (*model.PaymentIntent, error) {
	var pi model.PaymentIntent
	err := row.Scan(
		&pi.ID,
		&pi.OrgID,
		&pi.Amount,
		&pi.Currency,
		&pi.Status,
		&pi.HyperswitchPaymentID,
		&pi.ClientSecret,
		&pi.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &pi, nil
}

// CreatePaymentIntent persists a new payment intent inside an RLS-scoped transaction.
func (r *BillingRepository) CreatePaymentIntent(ctx context.Context, tx pgx.Tx, pi *model.PaymentIntent) (*model.PaymentIntent, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO payment_intents
		    (org_id, amount, currency, status, hyperswitch_payment_id, client_secret)
		 VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($6,''))
		 RETURNING `+paymentIntentColumns,
		pi.OrgID,
		pi.Amount,
		pi.Currency,
		string(pi.Status),
		pi.HyperswitchPaymentID,
		pi.ClientSecret,
	)
	created, err := scanPaymentIntent(row)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.CreatePaymentIntent: %w", err)
	}
	return created, nil
}

// GetPaymentIntentByHyperswitchID looks up a payment intent by Hyperswitch payment_id.
// Runs without RLS (pool-level) because webhook events arrive before we know the org.
func (r *BillingRepository) GetPaymentIntentByHyperswitchID(ctx context.Context, hyperswitchPaymentID string) (*model.PaymentIntent, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+paymentIntentColumns+`
		 FROM payment_intents
		 WHERE hyperswitch_payment_id = $1
		 LIMIT 1`,
		hyperswitchPaymentID,
	)
	pi, err := scanPaymentIntent(row)
	if err != nil {
		return nil, fmt.Errorf("BillingRepository.GetPaymentIntentByHyperswitchID: %w", err)
	}
	return pi, nil
}

// UpdatePaymentIntentStatus changes the status of a payment intent identified by its Hyperswitch ID.
// Runs directly on the pool (no RLS) because it is called from the webhook path.
func (r *BillingRepository) UpdatePaymentIntentStatus(ctx context.Context, hyperswitchPaymentID string, status model.PaymentIntentStatus) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE payment_intents SET status = $2, updated_at = now() WHERE hyperswitch_payment_id = $1`,
		hyperswitchPaymentID, string(status),
	)
	if err != nil {
		return fmt.Errorf("BillingRepository.UpdatePaymentIntentStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("BillingRepository.UpdatePaymentIntentStatus: payment intent %s not found", hyperswitchPaymentID)
	}
	return nil
}
