package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// quotaCheckFailures tracks the total number of quota check errors that resulted
// in fail-open decisions. A steadily growing counter indicates persistent cache
// or DB issues that effectively disable billing enforcement.
var quotaCheckFailures atomic.Int64

const (
	orgSubCachePrefix = "raven:org_sub:"
	orgSubCacheTTL    = 5 * time.Minute
	valkeyCmdTimeout  = 200 * time.Millisecond
)

// QuotaRepository defines the persistence methods the quota checker needs.
type QuotaRepository interface {
	GetActiveSubscription(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error)
	CountKBsByOrg(ctx context.Context, tx pgx.Tx, orgID string) (int, error)
	CountMembersByOrg(ctx context.Context, tx pgx.Tx, orgID string) (int, error)
	GetVoiceUsageForPeriod(ctx context.Context, tx pgx.Tx, orgID string, periodStart time.Time) (int, error)
}

// SubscriptionCache provides cached subscription lookups.
type SubscriptionCache interface {
	Get(ctx context.Context, orgID string) (*model.OrgSubscription, error)
	Set(ctx context.Context, orgID string, sub *model.OrgSubscription) error
}

// QuotaCheckerI is the interface for quota enforcement (used by services that need limit checks).
type QuotaCheckerI interface {
	CheckKBQuota(ctx context.Context, orgID string) error
	CheckSeatQuota(ctx context.Context, orgID string) error
	CheckVoiceMinuteQuota(ctx context.Context, orgID string) error
	GetConcurrentVoiceLimit(ctx context.Context, orgID string) int
	GetUsage(ctx context.Context, orgID string) (*model.UsageResponse, error)
	GetOrgSubscription(ctx context.Context, orgID string) (*model.OrgSubscription, error)
}

// QuotaChecker enforces billing subscription limits for organisations.
type QuotaChecker struct {
	repo  QuotaRepository
	cache SubscriptionCache
	pool  *pgxpool.Pool
	plans map[string]model.Plan
}

// NewQuotaChecker creates a new QuotaChecker.
func NewQuotaChecker(repo QuotaRepository, cache SubscriptionCache, pool *pgxpool.Pool) *QuotaChecker {
	plans := make(map[string]model.Plan)
	for _, p := range model.DefaultPlans() {
		plans[p.ID] = p
	}
	return &QuotaChecker{
		repo:  repo,
		cache: cache,
		pool:  pool,
		plans: plans,
	}
}

// GetOrgSubscription returns the resolved subscription + plan for an org.
// It checks cache first, falls back to DB, and caches the result.
// Defaults to the free plan if no subscription is found.
func (q *QuotaChecker) GetOrgSubscription(ctx context.Context, orgID string) (*model.OrgSubscription, error) {
	// Try cache first.
	if q.cache != nil {
		cached, err := q.cache.Get(ctx, orgID)
		if err != nil {
			slog.WarnContext(ctx, "quota: cache get failed", "org_id", orgID, "error", err)
		} else if cached != nil {
			return cached, nil
		}
	}

	// DB fallback.
	var sub *model.Subscription
	if q.pool != nil {
		err := db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var dbErr error
			sub, dbErr = q.repo.GetActiveSubscription(ctx, tx, orgID)
			return dbErr
		})
		if err != nil {
			slog.WarnContext(ctx, "quota: db lookup failed, defaulting to free tier", "org_id", orgID, "error", err)
			result := model.DefaultFreeSubscription()
			return &result, nil
		}
	} else {
		// Unit test path: no pool, call repo directly with nil tx.
		var err error
		sub, err = q.repo.GetActiveSubscription(ctx, nil, orgID)
		if err != nil {
			slog.WarnContext(ctx, "quota: repo lookup failed, defaulting to free tier", "org_id", orgID, "error", err)
			result := model.DefaultFreeSubscription()
			return &result, nil
		}
	}

	// Resolve plan from subscription, or default to free.
	var orgSub model.OrgSubscription
	if sub == nil {
		orgSub = model.DefaultFreeSubscription()
	} else {
		plan, ok := q.plans[sub.PlanID]
		if !ok {
			slog.WarnContext(ctx, "quota: unknown plan_id, defaulting to free tier", "org_id", orgID, "plan_id", sub.PlanID)
			orgSub = model.DefaultFreeSubscription()
			orgSub.Subscription = sub
		} else {
			orgSub = model.OrgSubscription{
				Subscription: sub,
				Plan:         plan,
			}
		}
	}

	// Cache the result.
	if q.cache != nil {
		if err := q.cache.Set(ctx, orgID, &orgSub); err != nil {
			slog.WarnContext(ctx, "quota: cache set failed", "org_id", orgID, "error", err)
		}
	}

	return &orgSub, nil
}

// CheckKBQuota checks whether the org has reached its knowledge base limit.
// Returns nil if under limit, or a 402 QuotaError if at/over limit.
func (q *QuotaChecker) CheckKBQuota(ctx context.Context, orgID string) error {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		total := quotaCheckFailures.Add(1)
		slog.WarnContext(ctx, "quota check failed, allowing request", "check", "kb_subscription", "org_id", orgID, "error", err, "total_failures", total)
		return nil
	}

	if orgSub.IsUnlimited() {
		return nil
	}

	limit := orgSub.Plan.MaxKBs
	if limit < 0 {
		return nil
	}

	var count int
	if q.pool != nil {
		err = db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var dbErr error
			count, dbErr = q.repo.CountKBsByOrg(ctx, tx, orgID)
			return dbErr
		})
	} else {
		count, err = q.repo.CountKBsByOrg(ctx, nil, orgID)
	}
	if err != nil {
		total := quotaCheckFailures.Add(1)
		slog.WarnContext(ctx, "quota check failed, allowing request", "check", "kb_count", "org_id", orgID, "error", err, "total_failures", total)
		return nil
	}

	if count >= limit {
		return apierror.NewPaymentRequired(
			fmt.Sprintf("knowledge base limit reached (%d/%d)", count, limit),
			limit,
		)
	}
	return nil
}

// CheckSeatQuota checks whether the org has reached its member (seat) limit.
// Returns nil if under limit, or a 402 QuotaError if at/over limit.
func (q *QuotaChecker) CheckSeatQuota(ctx context.Context, orgID string) error {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		total := quotaCheckFailures.Add(1)
		slog.WarnContext(ctx, "quota check failed, allowing request", "check", "seat_subscription", "org_id", orgID, "error", err, "total_failures", total)
		return nil
	}

	if orgSub.IsUnlimited() {
		return nil
	}

	limit := orgSub.Plan.MaxUsers
	if limit < 0 {
		return nil
	}

	var count int
	if q.pool != nil {
		err = db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var dbErr error
			count, dbErr = q.repo.CountMembersByOrg(ctx, tx, orgID)
			return dbErr
		})
	} else {
		count, err = q.repo.CountMembersByOrg(ctx, nil, orgID)
	}
	if err != nil {
		total := quotaCheckFailures.Add(1)
		slog.WarnContext(ctx, "quota check failed, allowing request", "check", "seat_count", "org_id", orgID, "error", err, "total_failures", total)
		return nil
	}

	if count >= limit {
		return apierror.NewPaymentRequired(
			fmt.Sprintf("seat limit reached (%d/%d)", count, limit),
			limit,
		)
	}
	return nil
}

// CheckVoiceMinuteQuota checks whether the org has reached its voice minute limit
// for the current billing period.
// Returns nil if under limit, or a 402 QuotaError if at/over limit.
func (q *QuotaChecker) CheckVoiceMinuteQuota(ctx context.Context, orgID string) error {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		total := quotaCheckFailures.Add(1)
		slog.WarnContext(ctx, "quota check failed, allowing request", "check", "voice_subscription", "org_id", orgID, "error", err, "total_failures", total)
		return nil
	}

	if orgSub.IsUnlimited() {
		return nil
	}

	limit := orgSub.Plan.MaxVoiceMinutesMonthly
	if limit < 0 {
		return nil
	}

	// Determine the billing period start.
	var periodStart time.Time
	if orgSub.Subscription != nil {
		periodStart = orgSub.Subscription.CurrentPeriodStart
	} else {
		periodStart = time.Now().UTC().AddDate(0, -1, 0)
	}

	var usageSeconds int
	if q.pool != nil {
		err = db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var dbErr error
			usageSeconds, dbErr = q.repo.GetVoiceUsageForPeriod(ctx, tx, orgID, periodStart)
			return dbErr
		})
	} else {
		usageSeconds, err = q.repo.GetVoiceUsageForPeriod(ctx, nil, orgID, periodStart)
	}
	if err != nil {
		total := quotaCheckFailures.Add(1)
		slog.WarnContext(ctx, "quota check failed, allowing request", "check", "voice_usage", "org_id", orgID, "error", err, "total_failures", total)
		return nil
	}

	usageMinutes := usageSeconds / 60
	if usageMinutes >= limit {
		return apierror.NewPaymentRequired(
			fmt.Sprintf("voice minute limit reached (%d/%d minutes)", usageMinutes, limit),
			limit,
		)
	}
	return nil
}

// GetConcurrentVoiceLimit returns the maximum concurrent voice sessions for the org.
// Returns -1 for unlimited (Enterprise).
func (q *QuotaChecker) GetConcurrentVoiceLimit(ctx context.Context, orgID string) int {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		total := quotaCheckFailures.Add(1)
		slog.WarnContext(ctx, "quota check failed, defaulting to 1", "check", "concurrent_voice", "org_id", orgID, "error", err, "total_failures", total)
		return 1
	}
	return orgSub.Plan.MaxConcurrentVoiceSessions
}

// GetUsage aggregates all usage counts into a UsageResponse.
func (q *QuotaChecker) GetUsage(ctx context.Context, orgID string) (*model.UsageResponse, error) {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		slog.WarnContext(ctx, "quota: failed to get subscription for usage", "org_id", orgID, "error", err)
		result := model.DefaultFreeSubscription()
		orgSub = &result
	}

	var kbCount, memberCount, voiceSeconds int

	if q.pool != nil {
		err = db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var dbErr error
			kbCount, dbErr = q.repo.CountKBsByOrg(ctx, tx, orgID)
			if dbErr != nil {
				return dbErr
			}
			memberCount, dbErr = q.repo.CountMembersByOrg(ctx, tx, orgID)
			if dbErr != nil {
				return dbErr
			}

			var periodStart time.Time
			if orgSub.Subscription != nil {
				periodStart = orgSub.Subscription.CurrentPeriodStart
			} else {
				periodStart = time.Now().UTC().AddDate(0, -1, 0)
			}
			voiceSeconds, dbErr = q.repo.GetVoiceUsageForPeriod(ctx, tx, orgID, periodStart)
			return dbErr
		})
	} else {
		kbCount, err = q.repo.CountKBsByOrg(ctx, nil, orgID)
		if err == nil {
			memberCount, err = q.repo.CountMembersByOrg(ctx, nil, orgID)
		}
		if err == nil {
			var periodStart time.Time
			if orgSub.Subscription != nil {
				periodStart = orgSub.Subscription.CurrentPeriodStart
			} else {
				periodStart = time.Now().UTC().AddDate(0, -1, 0)
			}
			voiceSeconds, err = q.repo.GetVoiceUsageForPeriod(ctx, nil, orgID, periodStart)
		}
	}

	if err != nil {
		slog.WarnContext(ctx, "quota: failed to aggregate usage", "org_id", orgID, "error", err)
		// Fail-open: return what we have with zero counts.
	}

	return &model.UsageResponse{
		Plan:                 orgSub.Plan,
		KBsUsed:              kbCount,
		KBsLimit:             orgSub.Plan.MaxKBs,
		SeatsUsed:            memberCount,
		SeatsLimit:           orgSub.Plan.MaxUsers,
		VoiceMinutesUsed:     voiceSeconds / 60,
		VoiceMinutesLimit:    orgSub.Plan.MaxVoiceMinutesMonthly,
		ConcurrentVoiceUsed:  0, // Live count would come from a different source (e.g., LiveKit).
		ConcurrentVoiceLimit: orgSub.Plan.MaxConcurrentVoiceSessions,
	}, nil
}

// --- ValkeySubscriptionCache ---

// ValkeySubscriptionCache implements SubscriptionCache using Valkey (Redis-compatible).
type ValkeySubscriptionCache struct {
	client redis.Cmdable
	ttl    time.Duration
}

// NewValkeySubscriptionCache creates a new ValkeySubscriptionCache with a 5-minute TTL.
func NewValkeySubscriptionCache(client redis.Cmdable) *ValkeySubscriptionCache {
	return &ValkeySubscriptionCache{
		client: client,
		ttl:    orgSubCacheTTL,
	}
}

// Get retrieves a cached OrgSubscription for the given org ID.
// Returns (nil, nil) on cache miss.
func (c *ValkeySubscriptionCache) Get(ctx context.Context, orgID string) (*model.OrgSubscription, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, valkeyCmdTimeout)
	defer cancel()

	data, err := c.client.Get(cmdCtx, orgSubCachePrefix+orgID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("valkey subscription cache get: %w", err)
	}

	var sub model.OrgSubscription
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, fmt.Errorf("valkey subscription cache unmarshal: %w", err)
	}
	return &sub, nil
}

// Set stores an OrgSubscription in the cache with the configured TTL.
func (c *ValkeySubscriptionCache) Set(ctx context.Context, orgID string, sub *model.OrgSubscription) error {
	cmdCtx, cancel := context.WithTimeout(ctx, valkeyCmdTimeout)
	defer cancel()

	data, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("valkey subscription cache marshal: %w", err)
	}

	if err := c.client.Set(cmdCtx, orgSubCachePrefix+orgID, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("valkey subscription cache set: %w", err)
	}
	return nil
}
