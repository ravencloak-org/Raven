package model

import "time"

// SubscriptionStatus represents the lifecycle state of a subscription.
type SubscriptionStatus string

// SubscriptionStatusActive and related constants define the valid lifecycle states for a subscription.
const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusCanceled  SubscriptionStatus = "canceled"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
	SubscriptionStatusTrialing  SubscriptionStatus = "trialing"
	SubscriptionStatusPaused    SubscriptionStatus = "paused"
	SubscriptionStatusExpired   SubscriptionStatus = "expired"
)

// PlanTier identifies a billing plan level.
type PlanTier string

// PlanTierFree and related constants define the available billing plan tiers.
const (
	PlanTierFree       PlanTier = "free"
	PlanTierPro        PlanTier = "pro"
	PlanTierEnterprise PlanTier = "enterprise"
)

// Plan describes a billing plan and its feature limits.
type Plan struct {
	ID                          string   `json:"id"`
	Tier                        PlanTier `json:"tier"`
	Name                        string   `json:"name"`
	PriceMonthly                int64    `json:"price_monthly"`  // amount in cents
	MaxUsers                    int      `json:"max_users"`
	MaxWorkspaces               int      `json:"max_workspaces"`
	MaxKBs                      int      `json:"max_kbs"`
	MaxStorageMB                int64    `json:"max_storage_mb"`
	MaxConcurrentVoiceSessions  int      `json:"max_concurrent_voice_sessions"` // -1 = unlimited
}

// DefaultPlans returns the pre-defined plans with their feature limits.
func DefaultPlans() []Plan {
	return []Plan{
		{
			ID:                         "plan_free",
			Tier:                       PlanTierFree,
			Name:                       "Free",
			PriceMonthly:               0,
			MaxUsers:                   5,
			MaxWorkspaces:              2,
			MaxKBs:                     3,
			MaxStorageMB:               500,
			MaxConcurrentVoiceSessions: 1,
		},
		{
			ID:                         "plan_pro",
			Tier:                       PlanTierPro,
			Name:                       "Pro",
			PriceMonthly:               2900, // $29.00
			MaxUsers:                   25,
			MaxWorkspaces:              10,
			MaxKBs:                     50,
			MaxStorageMB:               10240,
			MaxConcurrentVoiceSessions: 5,
		},
		{
			ID:                         "plan_enterprise",
			Tier:                       PlanTierEnterprise,
			Name:                       "Enterprise",
			PriceMonthly:               9900, // $99.00
			MaxUsers:                   -1,   // unlimited
			MaxWorkspaces:              -1,
			MaxKBs:                     -1,
			MaxStorageMB:               -1,
			MaxConcurrentVoiceSessions: -1, // unlimited
		},
	}
}

// Subscription represents an organisation's billing subscription.
type Subscription struct {
	ID                       string             `json:"id"`
	OrgID                    string             `json:"org_id"`
	PlanID                   string             `json:"plan_id"`
	Status                   SubscriptionStatus `json:"status"`
	HyperswitchSubscriptionID string            `json:"hyperswitch_subscription_id,omitempty"`
	CurrentPeriodStart       time.Time          `json:"current_period_start"`
	CurrentPeriodEnd         time.Time          `json:"current_period_end"`
	CreatedAt                time.Time          `json:"created_at"`
}

// PaymentIntentStatus represents the state of a payment intent.
type PaymentIntentStatus string

// PaymentIntentStatusRequiresPayment and related constants define the valid states for a payment intent.
const (
	PaymentIntentStatusRequiresPayment PaymentIntentStatus = "requires_payment_method"
	PaymentIntentStatusProcessing      PaymentIntentStatus = "processing"
	PaymentIntentStatusSucceeded       PaymentIntentStatus = "succeeded"
	PaymentIntentStatusFailed          PaymentIntentStatus = "failed"
	PaymentIntentStatusCanceled        PaymentIntentStatus = "canceled"
)

// PaymentIntent represents a single payment attempt orchestrated via Hyperswitch.
type PaymentIntent struct {
	ID                     string              `json:"id"`
	OrgID                  string              `json:"org_id"`
	Amount                 int64               `json:"amount"`   // amount in cents
	Currency               string              `json:"currency"` // ISO 4217, e.g. "USD"
	Status                 PaymentIntentStatus `json:"status"`
	HyperswitchPaymentID   string              `json:"hyperswitch_payment_id,omitempty"`
	ClientSecret           string              `json:"client_secret,omitempty"`
	CreatedAt              time.Time           `json:"created_at"`
}

// CreateSubscriptionRequest is the payload for POST /api/v1/billing/subscribe.
type CreateSubscriptionRequest struct {
	PlanID string `json:"plan_id" binding:"required"`
}

// CreatePaymentIntentRequest is the payload for creating a payment intent.
type CreatePaymentIntentRequest struct {
	Amount   int64  `json:"amount" binding:"required,gt=0"`
	Currency string `json:"currency" binding:"required,len=3"`
}

// HyperswitchWebhookPayload represents the incoming webhook from Hyperswitch.
type HyperswitchWebhookPayload struct {
	EventType string                 `json:"event_type"`
	Content   map[string]any         `json:"content"`
}
