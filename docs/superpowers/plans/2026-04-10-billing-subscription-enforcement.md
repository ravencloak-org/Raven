# Billing Subscription Enforcement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce plan-tier limits (KB count, workspace seats, voice-minute quota, API rate limits) so Free/Pro orgs cannot exceed their plan allowances, while Enterprise bypasses all soft limits.

**Architecture:** A new `QuotaChecker` service provides `GetOrgSubscription(ctx, orgID)` and limit-check methods. Enforcement is wired into existing service-layer `Create`/`AddMember`/`CreateSession` methods via dependency injection. Subscription plan data is cached in Valkey (TTL 5 min) to avoid per-request DB hits. A new `GET /billing/usage` endpoint exposes current-period usage vs limits. Returns `402 Payment Required` with `{"upgrade_required": true, "limit": N}` when limits are exceeded.

**Tech Stack:** Go 1.24, Gin, pgx/v5, go-redis/v9 (Valkey), testify-free handler tests (httptest + mock services)

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/service/quota.go` | `QuotaChecker` — subscription lookup, Valkey caching, limit-check methods, usage aggregation |
| Create | `internal/service/quota_test.go` | Unit tests for QuotaChecker (mock repo + mock Valkey) |
| Create | `internal/handler/usage.go` | `UsageHandler` — `GET /billing/usage` endpoint |
| Create | `internal/handler/usage_test.go` | Handler-level tests for usage endpoint |
| Modify | `internal/model/billing.go` | Add `OrgSubscription`, `UsageResponse`, `QuotaExceededError` types |
| Modify | `internal/repository/billing.go` | Add `CountKBsByOrg`, `CountMembersByOrg`, `GetVoiceUsageForPeriod` queries |
| Modify | `internal/service/kb.go` | Inject `QuotaChecker`, call limit check in `Create` |
| Modify | `internal/service/kb_test.go` *(create if absent)* | Test KB creation with quota enforcement |
| Modify | `internal/service/workspace.go` | Inject `QuotaChecker`, call limit check in `AddMember` |
| Modify | `internal/service/workspace_test.go` *(create if absent)* | Test AddMember with quota enforcement |
| Modify | `internal/service/voice.go` | Replace hardcoded `maxConcurrentSessions` with `QuotaChecker` lookup + monthly minute check |
| Modify | `internal/service/voice_test.go` *(create if absent)* | Test voice session creation with quota enforcement |
| Modify | `pkg/apierror/apierror.go` | Add `NewPaymentRequired` constructor (402) |
| Modify | `cmd/api/main.go` | Wire `QuotaChecker` into services, register `/billing/usage` route, populate Valkey tier cache on subscription create, verify `ByOrgTier` middleware is wired |

---

### Task 1: Add 402 Payment Required error constructor

**Files:**
- Modify: `pkg/apierror/apierror.go`
- Modify: existing test file or create `pkg/apierror/apierror_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/apierror/apierror_test.go` (if it doesn't exist) with:

```go
package apierror_test

import (
	"net/http"
	"testing"

	"github.com/ravencloak-org/Raven/pkg/apierror"
)

func TestNewPaymentRequired(t *testing.T) {
	qErr := apierror.NewPaymentRequired("KB limit reached", 3)
	if qErr.Code != http.StatusPaymentRequired {
		t.Errorf("expected code 402, got %d", qErr.Code)
	}
	if qErr.Message != "Payment Required" {
		t.Errorf("expected message 'Payment Required', got %q", qErr.Message)
	}
	if !qErr.UpgradeRequired {
		t.Error("expected upgrade_required to be true")
	}
	if qErr.Limit != 3 {
		t.Errorf("expected limit 3, got %d", qErr.Limit)
	}

	// Verify it satisfies the error interface.
	var e error = qErr
	if e.Error() != "Payment Required: KB limit reached" {
		t.Errorf("unexpected Error() output: %q", e.Error())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./pkg/apierror/ -run TestNewPaymentRequired -v`
Expected: FAIL — `NewPaymentRequired` and `QuotaError` don't exist yet.

- [ ] **Step 3: Add QuotaError type and NewPaymentRequired constructor**

Add to `pkg/apierror/apierror.go`:

```go
// QuotaError extends AppError with billing-specific fields for 402 responses.
type QuotaError struct {
	AppError
	UpgradeRequired bool `json:"upgrade_required"`
	Limit           int  `json:"limit"`
}

// NewPaymentRequired creates a 402 Payment Required error with upgrade context.
func NewPaymentRequired(detail string, limit int) *QuotaError {
	return &QuotaError{
		AppError: AppError{
			Code:    http.StatusPaymentRequired,
			Message: "Payment Required",
			Detail:  detail,
		},
		UpgradeRequired: true,
		Limit:           limit,
	}
}
```

Also update `ErrorHandler` to handle `*QuotaError`:

```go
// In ErrorHandler, before the AppError check:
if quotaErr, ok := err.(*QuotaError); ok {
	c.JSON(quotaErr.Code, quotaErr)
} else if appErr, ok := err.(*AppError); ok {
	c.JSON(appErr.Code, appErr)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./pkg/apierror/ -run TestNewPaymentRequired -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/apierror/apierror.go pkg/apierror/apierror_test.go
git commit -m "feat(billing): add 402 Payment Required error with QuotaError type"
```

---

### Task 2: Add billing model types (OrgSubscription, UsageResponse)

**Files:**
- Modify: `internal/model/billing.go`

- [ ] **Step 1: Write a compile-check test**

Create `internal/model/billing_test.go`:

```go
package model_test

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
)

func TestOrgSubscription_IsUnlimited(t *testing.T) {
	enterprise := model.OrgSubscription{
		Plan: model.DefaultPlans()[2], // Enterprise
	}
	if !enterprise.IsUnlimited() {
		t.Error("enterprise plan should be unlimited")
	}

	free := model.OrgSubscription{
		Plan: model.DefaultPlans()[0], // Free
	}
	if free.IsUnlimited() {
		t.Error("free plan should not be unlimited")
	}
}

func TestDefaultFreeSubscription(t *testing.T) {
	sub := model.DefaultFreeSubscription()
	if sub.Plan.Tier != model.PlanTierFree {
		t.Errorf("expected free tier, got %s", sub.Plan.Tier)
	}
	if sub.Plan.MaxKBs != 3 {
		t.Errorf("expected MaxKBs 3, got %d", sub.Plan.MaxKBs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/model/ -run TestOrgSubscription -v`
Expected: FAIL — types don't exist yet.

- [ ] **Step 3: Add types to billing.go**

Append to `internal/model/billing.go`:

First, add `MaxVoiceMinutesMonthly` to the `Plan` struct (insert after `MaxConcurrentVoiceSessions`):

```go
MaxVoiceMinutesMonthly int `json:"max_voice_minutes_monthly"` // -1 = unlimited
```

Update `DefaultPlans()` to include this field:
- Free: `MaxVoiceMinutesMonthly: 60` (1 hour/month)
- Pro: `MaxVoiceMinutesMonthly: 1200` (20 hours/month)
- Enterprise: `MaxVoiceMinutesMonthly: -1` (unlimited)

Then append these new types:

```go
// OrgSubscription holds the resolved subscription + plan for an org,
// used by the quota checker to enforce limits.
type OrgSubscription struct {
	Subscription *Subscription `json:"subscription,omitempty"`
	Plan         Plan          `json:"plan"`
}

// IsUnlimited returns true if the org is on the Enterprise tier (all limits are -1).
func (o *OrgSubscription) IsUnlimited() bool {
	return o.Plan.Tier == PlanTierEnterprise
}

// DefaultFreeSubscription returns the implicit subscription for orgs without
// an explicit subscription record (defaults to Free tier).
func DefaultFreeSubscription() OrgSubscription {
	plans := DefaultPlans()
	return OrgSubscription{Plan: plans[0]}
}

// UsageResponse is the response body for GET /billing/usage.
type UsageResponse struct {
	Plan                Plan  `json:"plan"`
	KBsUsed             int   `json:"kbs_used"`
	KBsLimit            int   `json:"kbs_limit"`
	SeatsUsed           int   `json:"seats_used"`
	SeatsLimit          int   `json:"seats_limit"`
	VoiceMinutesUsed     int  `json:"voice_minutes_used"`
	VoiceMinutesLimit    int  `json:"voice_minutes_limit"`    // from Plan.MaxVoiceMinutesMonthly
	ConcurrentVoiceUsed  int  `json:"concurrent_voice_used"`
	ConcurrentVoiceLimit int  `json:"concurrent_voice_limit"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/model/ -run TestOrgSubscription -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/model/billing.go internal/model/billing_test.go
git commit -m "feat(billing): add OrgSubscription and UsageResponse model types"
```

---

### Task 3: Add count queries to billing repository

**Files:**
- Modify: `internal/repository/billing.go`

- [ ] **Step 1: Add SQL constants and repository methods**

Append to `internal/repository/billing.go`:

```go
const (
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
```

Note: Add `"time"` to the imports in `internal/repository/billing.go`.

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/jobinlawrance/Project/raven && go build ./internal/repository/`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/repository/billing.go
git commit -m "feat(billing): add count queries for KBs, members, and voice usage"
```

---

### Task 4: Create QuotaChecker service

**Files:**
- Create: `internal/service/quota.go`
- Create: `internal/service/quota_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/service/quota_test.go`:

```go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockQuotaRepo implements service.QuotaRepository for testing.
type mockQuotaRepo struct {
	activeSubscription *model.Subscription
	activeSubErr       error
	kbCount            int
	kbCountErr         error
	memberCount        int
	memberCountErr     error
	voiceUsage         int
	voiceUsageErr      error
}

func (m *mockQuotaRepo) GetActiveSubscription(ctx context.Context, tx pgx.Tx, orgID string) (*model.Subscription, error) {
	return m.activeSubscription, m.activeSubErr
}

func (m *mockQuotaRepo) CountKBsByOrg(ctx context.Context, tx pgx.Tx, orgID string) (int, error) {
	return m.kbCount, m.kbCountErr
}

func (m *mockQuotaRepo) CountMembersByOrg(ctx context.Context, tx pgx.Tx, orgID string) (int, error) {
	return m.memberCount, m.memberCountErr
}

func (m *mockQuotaRepo) GetVoiceUsageForPeriod(ctx context.Context, tx pgx.Tx, orgID string, periodStart time.Time) (int, error) {
	return m.voiceUsage, m.voiceUsageErr
}

// mockValkeyCache implements service.SubscriptionCache for testing.
type mockValkeyCache struct {
	cached *model.OrgSubscription
}

func (m *mockValkeyCache) Get(ctx context.Context, orgID string) (*model.OrgSubscription, error) {
	if m.cached != nil {
		return m.cached, nil
	}
	return nil, nil
}

func (m *mockValkeyCache) Set(ctx context.Context, orgID string, sub *model.OrgSubscription) error {
	m.cached = sub
	return nil
}

func TestCheckKBQuota_FreeAtLimit_Returns402(t *testing.T) {
	repo := &mockQuotaRepo{
		activeSubscription: nil, // no subscription = free tier
		kbCount:            3,   // at the free limit
	}
	qc := service.NewQuotaChecker(repo, &mockValkeyCache{}, nil)

	err := qc.CheckKBQuota(context.Background(), "org-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	quotaErr, ok := err.(*apierror.QuotaError)
	if !ok {
		t.Fatalf("expected *QuotaError, got %T: %v", err, err)
	}
	if quotaErr.Code != 402 {
		t.Errorf("expected 402, got %d", quotaErr.Code)
	}
	if quotaErr.Limit != 3 {
		t.Errorf("expected limit 3, got %d", quotaErr.Limit)
	}
}

func TestCheckKBQuota_EnterpriseBypass(t *testing.T) {
	now := time.Now().UTC()
	repo := &mockQuotaRepo{
		activeSubscription: &model.Subscription{
			ID:                 "sub-1",
			OrgID:              "org-1",
			PlanID:             "plan_enterprise",
			Status:             model.SubscriptionStatusActive,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		},
		kbCount: 999, // way over free/pro limits
	}
	qc := service.NewQuotaChecker(repo, &mockValkeyCache{}, nil)

	err := qc.CheckKBQuota(context.Background(), "org-1")
	if err != nil {
		t.Errorf("enterprise should bypass, got error: %v", err)
	}
}

func TestCheckKBQuota_ProUnderLimit_Passes(t *testing.T) {
	now := time.Now().UTC()
	repo := &mockQuotaRepo{
		activeSubscription: &model.Subscription{
			ID:                 "sub-1",
			OrgID:              "org-1",
			PlanID:             "plan_pro",
			Status:             model.SubscriptionStatusActive,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		},
		kbCount: 10, // under pro limit of 50
	}
	qc := service.NewQuotaChecker(repo, &mockValkeyCache{}, nil)

	err := qc.CheckKBQuota(context.Background(), "org-1")
	if err != nil {
		t.Errorf("expected pass, got error: %v", err)
	}
}

func TestCheckSeatQuota_FreeAtLimit_Returns402(t *testing.T) {
	repo := &mockQuotaRepo{
		activeSubscription: nil,
		memberCount:        5, // free limit
	}
	qc := service.NewQuotaChecker(repo, &mockValkeyCache{}, nil)

	err := qc.CheckSeatQuota(context.Background(), "org-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	quotaErr, ok := err.(*apierror.QuotaError)
	if !ok {
		t.Fatalf("expected *QuotaError, got %T", err)
	}
	if quotaErr.Code != 402 {
		t.Errorf("expected 402, got %d", quotaErr.Code)
	}
}

func TestCheckVoiceMinuteQuota_FreeAtLimit_Returns402(t *testing.T) {
	repo := &mockQuotaRepo{
		activeSubscription: nil,  // free tier (60 min/month)
		voiceUsage:         3600, // 60 minutes in seconds — at limit
	}
	qc := service.NewQuotaChecker(repo, &mockValkeyCache{}, nil)

	err := qc.CheckVoiceMinuteQuota(context.Background(), "org-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	quotaErr, ok := err.(*apierror.QuotaError)
	if !ok {
		t.Fatalf("expected *QuotaError, got %T: %v", err, err)
	}
	if quotaErr.Code != 402 {
		t.Errorf("expected 402, got %d", quotaErr.Code)
	}
	if quotaErr.Limit != 60 {
		t.Errorf("expected limit 60, got %d", quotaErr.Limit)
	}
}

func TestCheckVoiceMinuteQuota_UnderLimit_Passes(t *testing.T) {
	repo := &mockQuotaRepo{
		activeSubscription: nil,  // free tier (60 min/month)
		voiceUsage:         1800, // 30 minutes — under limit
	}
	qc := service.NewQuotaChecker(repo, &mockValkeyCache{}, nil)

	err := qc.CheckVoiceMinuteQuota(context.Background(), "org-1")
	if err != nil {
		t.Errorf("expected pass, got error: %v", err)
	}
}

func TestCheckConcurrentVoiceQuota_EnterpriseBypass(t *testing.T) {
	now := time.Now().UTC()
	repo := &mockQuotaRepo{
		activeSubscription: &model.Subscription{
			PlanID:             "plan_enterprise",
			Status:             model.SubscriptionStatusActive,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		},
	}
	qc := service.NewQuotaChecker(repo, &mockValkeyCache{}, nil)

	// Should not error even though we don't check count — enterprise bypasses
	limit := qc.GetConcurrentVoiceLimit(context.Background(), "org-1")
	if limit != -1 {
		t.Errorf("expected -1 (unlimited), got %d", limit)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/service/ -run TestCheck -v`
Expected: FAIL — `QuotaChecker`, `QuotaRepository`, `SubscriptionCache` don't exist.

- [ ] **Step 3: Implement QuotaChecker**

Create `internal/service/quota.go`:

```go
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
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

// QuotaChecker enforces plan-tier resource limits for organisations.
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
	return &QuotaChecker{repo: repo, cache: cache, pool: pool, plans: plans}
}

// GetOrgSubscription resolves the current subscription and plan for an org.
// Checks Valkey cache first, falls back to DB, caches result.
// If no subscription exists, returns the default free plan.
func (q *QuotaChecker) GetOrgSubscription(ctx context.Context, orgID string) (*model.OrgSubscription, error) {
	// Check cache first.
	if q.cache != nil {
		cached, err := q.cache.Get(ctx, orgID)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// Fall back to DB.
	var sub *model.Subscription
	var lookupErr error

	if q.pool != nil {
		lookupErr = db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var err error
			sub, err = q.repo.GetActiveSubscription(ctx, tx, orgID)
			return err
		})
	} else {
		// No pool (unit test path) — call repo directly with nil tx.
		sub, lookupErr = q.repo.GetActiveSubscription(ctx, nil, orgID)
	}

	if lookupErr != nil {
		slog.ErrorContext(ctx, "QuotaChecker: failed to look up subscription", "org_id", orgID, "error", lookupErr)
		// Fail-open: default to free tier rather than blocking the request.
		result := model.DefaultFreeSubscription()
		return &result, nil
	}

	var result model.OrgSubscription
	if sub == nil {
		result = model.DefaultFreeSubscription()
	} else {
		plan, ok := q.plans[sub.PlanID]
		if !ok {
			slog.WarnContext(ctx, "QuotaChecker: unknown plan_id, defaulting to free", "plan_id", sub.PlanID)
			result = model.DefaultFreeSubscription()
			result.Subscription = sub
		} else {
			result = model.OrgSubscription{Subscription: sub, Plan: plan}
		}
	}

	// Cache the result.
	if q.cache != nil {
		if err := q.cache.Set(ctx, orgID, &result); err != nil {
			slog.WarnContext(ctx, "QuotaChecker: failed to cache subscription", "org_id", orgID, "error", err)
		}
	}

	return &result, nil
}

// CheckKBQuota verifies the org has not reached its knowledge base limit.
// Returns nil if allowed, or a 402 QuotaError if at/over the limit.
func (q *QuotaChecker) CheckKBQuota(ctx context.Context, orgID string) error {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		return err
	}
	if orgSub.IsUnlimited() {
		return nil
	}

	var count int
	if q.pool != nil {
		err = db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var e error
			count, e = q.repo.CountKBsByOrg(ctx, tx, orgID)
			return e
		})
	} else {
		count, err = q.repo.CountKBsByOrg(ctx, nil, orgID)
	}
	if err != nil {
		slog.ErrorContext(ctx, "QuotaChecker: failed to count KBs", "org_id", orgID, "error", err)
		return nil // fail-open
	}

	if count >= orgSub.Plan.MaxKBs {
		return apierror.NewPaymentRequired(
			fmt.Sprintf("knowledge base limit reached (%d/%d)", count, orgSub.Plan.MaxKBs),
			orgSub.Plan.MaxKBs,
		)
	}
	return nil
}

// CheckSeatQuota verifies the org has not reached its user seat limit.
func (q *QuotaChecker) CheckSeatQuota(ctx context.Context, orgID string) error {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		return err
	}
	if orgSub.IsUnlimited() {
		return nil
	}

	var count int
	if q.pool != nil {
		err = db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var e error
			count, e = q.repo.CountMembersByOrg(ctx, tx, orgID)
			return e
		})
	} else {
		count, err = q.repo.CountMembersByOrg(ctx, nil, orgID)
	}
	if err != nil {
		slog.ErrorContext(ctx, "QuotaChecker: failed to count members", "org_id", orgID, "error", err)
		return nil // fail-open
	}

	if count >= orgSub.Plan.MaxUsers {
		return apierror.NewPaymentRequired(
			fmt.Sprintf("seat limit reached (%d/%d)", count, orgSub.Plan.MaxUsers),
			orgSub.Plan.MaxUsers,
		)
	}
	return nil
}

// GetConcurrentVoiceLimit returns the concurrent voice session limit for an org.
// Returns -1 for unlimited (Enterprise).
func (q *QuotaChecker) GetConcurrentVoiceLimit(ctx context.Context, orgID string) int {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		return 1 // default to free tier
	}
	return orgSub.Plan.MaxConcurrentVoiceSessions
}

// CheckVoiceMinuteQuota verifies the org has not exceeded its monthly voice-minute quota.
// Returns nil if allowed, or a 402 QuotaError if at/over the limit.
func (q *QuotaChecker) CheckVoiceMinuteQuota(ctx context.Context, orgID string) error {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		return err
	}
	if orgSub.IsUnlimited() || orgSub.Plan.MaxVoiceMinutesMonthly < 0 {
		return nil
	}

	// Determine billing period start.
	periodStart := time.Now().UTC().AddDate(0, -1, 0)
	if orgSub.Subscription != nil {
		periodStart = orgSub.Subscription.CurrentPeriodStart
	}

	var totalSeconds int
	if q.pool != nil {
		err = db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var e error
			totalSeconds, e = q.repo.GetVoiceUsageForPeriod(ctx, tx, orgID, periodStart)
			return e
		})
	} else {
		totalSeconds, err = q.repo.GetVoiceUsageForPeriod(ctx, nil, orgID, periodStart)
	}
	if err != nil {
		slog.ErrorContext(ctx, "QuotaChecker: failed to check voice usage", "org_id", orgID, "error", err)
		return nil // fail-open
	}

	usedMinutes := totalSeconds / 60
	if usedMinutes >= orgSub.Plan.MaxVoiceMinutesMonthly {
		return apierror.NewPaymentRequired(
			fmt.Sprintf("monthly voice minute quota exceeded (%d/%d minutes)", usedMinutes, orgSub.Plan.MaxVoiceMinutesMonthly),
			orgSub.Plan.MaxVoiceMinutesMonthly,
		)
	}
	return nil
}

// GetUsage returns the full usage breakdown for an org for the billing/usage endpoint.
func (q *QuotaChecker) GetUsage(ctx context.Context, orgID string) (*model.UsageResponse, error) {
	orgSub, err := q.GetOrgSubscription(ctx, orgID)
	if err != nil {
		return nil, apierror.NewInternal("failed to resolve subscription")
	}

	var kbCount, memberCount, voiceSeconds, activeSessions int

	if q.pool != nil {
		err = db.WithOrgID(ctx, q.pool, orgID, func(tx pgx.Tx) error {
			var e error
			kbCount, e = q.repo.CountKBsByOrg(ctx, tx, orgID)
			if e != nil {
				return e
			}
			memberCount, e = q.repo.CountMembersByOrg(ctx, tx, orgID)
			if e != nil {
				return e
			}
			// Voice usage since the start of the current billing period.
			periodStart := time.Now().UTC().AddDate(0, -1, 0)
			if orgSub.Subscription != nil {
				periodStart = orgSub.Subscription.CurrentPeriodStart
			}
			voiceSeconds, e = q.repo.GetVoiceUsageForPeriod(ctx, tx, orgID, periodStart)
			return e
		})
	}
	if err != nil {
		slog.ErrorContext(ctx, "QuotaChecker.GetUsage: failed to aggregate usage", "org_id", orgID, "error", err)
		return nil, apierror.NewInternal("failed to aggregate usage")
	}

	return &model.UsageResponse{
		Plan:                 orgSub.Plan,
		KBsUsed:              kbCount,
		KBsLimit:             orgSub.Plan.MaxKBs,
		SeatsUsed:            memberCount,
		SeatsLimit:           orgSub.Plan.MaxUsers,
		VoiceMinutesUsed:     voiceSeconds / 60,
		VoiceMinutesLimit:    orgSub.Plan.MaxVoiceMinutesMonthly,
		ConcurrentVoiceUsed:  0, // real-time count not tracked here; see voice service
		ConcurrentVoiceLimit: orgSub.Plan.MaxConcurrentVoiceSessions,
	}, nil
}

// ValkeySubscriptionCache implements SubscriptionCache using Valkey with a 5-minute TTL.
type ValkeySubscriptionCache struct {
	client redis.Cmdable
	ttl    time.Duration
}

// NewValkeySubscriptionCache creates a cache backed by Valkey.
func NewValkeySubscriptionCache(client redis.Cmdable) *ValkeySubscriptionCache {
	return &ValkeySubscriptionCache{client: client, ttl: 5 * time.Minute}
}

const valkeyCacheKeyPrefix = "raven:org_sub:"

// Get reads the cached OrgSubscription. Returns (nil, nil) on miss.
func (c *ValkeySubscriptionCache) Get(ctx context.Context, orgID string) (*model.OrgSubscription, error) {
	callCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	data, err := c.client.Get(callCtx, valkeyCacheKeyPrefix+orgID).Bytes()
	if err != nil {
		return nil, nil // cache miss
	}

	var sub model.OrgSubscription
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, nil
	}
	return &sub, nil
}

// Set caches the OrgSubscription with a 5-minute TTL.
func (c *ValkeySubscriptionCache) Set(ctx context.Context, orgID string, sub *model.OrgSubscription) error {
	callCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	data, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	return c.client.Set(callCtx, valkeyCacheKeyPrefix+orgID, data, c.ttl).Err()
}
```

Note: Add `"encoding/json"` to imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/service/ -run TestCheck -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/quota.go internal/service/quota_test.go
git commit -m "feat(billing): add QuotaChecker service with Valkey-cached subscription lookups"
```

---

### Task 5: Wire QuotaChecker into KBService

**Files:**
- Modify: `internal/service/kb.go`
- Create: `internal/service/kb_test.go`

- [ ] **Step 1: Modify KBService to accept and use QuotaChecker**

In `internal/service/kb.go`, update the struct and constructor:

```go
// QuotaCheckerI is the interface for quota enforcement.
type QuotaCheckerI interface {
	CheckKBQuota(ctx context.Context, orgID string) error
	CheckSeatQuota(ctx context.Context, orgID string) error
	CheckVoiceMinuteQuota(ctx context.Context, orgID string) error
	GetConcurrentVoiceLimit(ctx context.Context, orgID string) int
	GetUsage(ctx context.Context, orgID string) (*model.UsageResponse, error)
	GetOrgSubscription(ctx context.Context, orgID string) (*model.OrgSubscription, error)
}
```

Update `KBService`:

```go
type KBService struct {
	repo  *repository.KBRepository
	pool  *pgxpool.Pool
	quota QuotaCheckerI
}

func NewKBService(repo *repository.KBRepository, pool *pgxpool.Pool, quota QuotaCheckerI) *KBService {
	return &KBService{repo: repo, pool: pool, quota: quota}
}
```

Add quota check at the top of `Create`:

```go
func (s *KBService) Create(ctx context.Context, orgID, wsID string, req model.CreateKBRequest) (*model.KnowledgeBase, error) {
	// Enforce plan-tier KB limit.
	if s.quota != nil {
		if err := s.quota.CheckKBQuota(ctx, orgID); err != nil {
			return nil, err
		}
	}

	slug := toSlug(req.Name)
	// ... rest unchanged
}
```

- [ ] **Step 2: Write integration test verifying quota enforcement through KBService.Create**

Create `internal/service/kb_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockQuotaChecker implements service.QuotaCheckerI for testing.
type mockQuotaChecker struct {
	checkKBErr         error
	checkSeatErr       error
	checkVoiceMinErr   error
	concurrentLimit    int
	usage              *model.UsageResponse
	usageErr           error
	orgSub             *model.OrgSubscription
}

func (m *mockQuotaChecker) CheckKBQuota(_ context.Context, _ string) error           { return m.checkKBErr }
func (m *mockQuotaChecker) CheckSeatQuota(_ context.Context, _ string) error         { return m.checkSeatErr }
func (m *mockQuotaChecker) CheckVoiceMinuteQuota(_ context.Context, _ string) error  { return m.checkVoiceMinErr }
func (m *mockQuotaChecker) GetConcurrentVoiceLimit(_ context.Context, _ string) int  { return m.concurrentLimit }
func (m *mockQuotaChecker) GetUsage(_ context.Context, _ string) (*model.UsageResponse, error) { return m.usage, m.usageErr }
func (m *mockQuotaChecker) GetOrgSubscription(_ context.Context, _ string) (*model.OrgSubscription, error) { return m.orgSub, nil }

func TestKBCreate_QuotaExceeded_Returns402(t *testing.T) {
	quota := &mockQuotaChecker{
		checkKBErr: apierror.NewPaymentRequired("KB limit reached (3/3)", 3),
	}
	// Pass nil repo and pool since we expect the quota check to short-circuit
	// before any DB call.
	svc := service.NewKBService(nil, nil, quota)

	_, err := svc.Create(context.Background(), "org-1", "ws-1", model.CreateKBRequest{Name: "test"})
	if err == nil {
		t.Fatal("expected quota error, got nil")
	}
	qErr, ok := err.(*apierror.QuotaError)
	if !ok {
		t.Fatalf("expected *QuotaError, got %T: %v", err, err)
	}
	if qErr.Code != 402 {
		t.Errorf("expected 402, got %d", qErr.Code)
	}
	if !qErr.UpgradeRequired {
		t.Error("expected upgrade_required true")
	}
}

func TestKBCreate_QuotaOK_ProceedsToCreate(t *testing.T) {
	quota := &mockQuotaChecker{checkKBErr: nil}
	// With nil repo/pool, the service will panic at the DB step (after quota check).
	// We recover and verify the panic is NOT a quota error — confirming the quota
	// check passed and execution continued past it.
	svc := service.NewKBService(nil, nil, quota)

	defer func() {
		if r := recover(); r != nil {
			// Panic from nil pool is expected — means we got past the quota check.
		}
	}()

	_, err := svc.Create(context.Background(), "org-1", "ws-1", model.CreateKBRequest{Name: "test"})
	if err != nil {
		if _, ok := err.(*apierror.QuotaError); ok {
			t.Fatal("should not get quota error when under limit")
		}
		// Any other error is expected (nil repo)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/service/ -run TestKBCreate -v`
Expected: PASS — quota enforcement short-circuits before DB calls

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/jobinlawrance/Project/raven && go build ./internal/service/`
Expected: BUILD SUCCESS (may need to update callers in main.go — defer to Task 9)

- [ ] **Step 5: Commit**

```bash
git add internal/service/kb.go internal/service/kb_test.go
git commit -m "feat(billing): enforce KB quota in KBService.Create"
```

---

### Task 6: Wire QuotaChecker into WorkspaceService

**Files:**
- Modify: `internal/service/workspace.go`

- [ ] **Step 1: Update WorkspaceService struct and constructor**

```go
type WorkspaceService struct {
	repo  *repository.WorkspaceRepository
	pool  *pgxpool.Pool
	quota QuotaCheckerI
}

func NewWorkspaceService(repo *repository.WorkspaceRepository, pool *pgxpool.Pool, quota QuotaCheckerI) *WorkspaceService {
	return &WorkspaceService{repo: repo, pool: pool, quota: quota}
}
```

- [ ] **Step 2: Add seat quota check to AddMember**

At the top of `AddMember`:

```go
func (s *WorkspaceService) AddMember(ctx context.Context, orgID, wsID string, req model.AddWorkspaceMemberRequest) (*model.WorkspaceMember, error) {
	// Enforce plan-tier seat limit.
	if s.quota != nil {
		if err := s.quota.CheckSeatQuota(ctx, orgID); err != nil {
			return nil, err
		}
	}

	var member *model.WorkspaceMember
	// ... rest unchanged
}
```

- [ ] **Step 3: Write integration test for AddMember quota enforcement**

Add to `internal/service/kb_test.go` (shared test file) or create `internal/service/workspace_test.go`:

```go
func TestAddMember_QuotaExceeded_Returns402(t *testing.T) {
	quota := &mockQuotaChecker{
		checkSeatErr: apierror.NewPaymentRequired("seat limit reached (5/5)", 5),
	}
	svc := service.NewWorkspaceService(nil, nil, quota)

	_, err := svc.AddMember(context.Background(), "org-1", "ws-1", model.AddWorkspaceMemberRequest{
		UserID: "user-99",
		Role:   "member",
	})
	if err == nil {
		t.Fatal("expected quota error, got nil")
	}
	qErr, ok := err.(*apierror.QuotaError)
	if !ok {
		t.Fatalf("expected *QuotaError, got %T: %v", err, err)
	}
	if qErr.Code != 402 {
		t.Errorf("expected 402, got %d", qErr.Code)
	}
}
```

- [ ] **Step 4: Run tests and verify compilation**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/service/ -run TestAddMember_Quota -v && go build ./internal/service/`
Expected: PASS + BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add internal/service/workspace.go internal/service/workspace_test.go
git commit -m "feat(billing): enforce seat quota in WorkspaceService.AddMember"
```

---

### Task 7: Wire QuotaChecker into VoiceService

**Files:**
- Modify: `internal/service/voice.go`

- [ ] **Step 1: Update VoiceService to use QuotaChecker for concurrent session limit**

Replace the hardcoded `maxConcurrentSessions` with a dynamic lookup. Update the struct:

```go
type VoiceService struct {
	repo   VoiceRepository
	pool   *pgxpool.Pool
	lkc    LiveKitClient
	lkHost string
	quota  QuotaCheckerI
}

func NewVoiceService(repo VoiceRepository, pool *pgxpool.Pool, lkc LiveKitClient, lkHost string, quota QuotaCheckerI) *VoiceService {
	return &VoiceService{
		repo:   repo,
		pool:   pool,
		lkc:    lkc,
		lkHost: lkHost,
		quota:  quota,
	}
}
```

- [ ] **Step 2: Update CreateSession to use QuotaChecker for both concurrent limit and monthly quota**

In `CreateSession`, replace the hardcoded limit check:

```go
func (s *VoiceService) CreateSession(ctx context.Context, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error) {
	if req == nil {
		return nil, apierror.NewBadRequest("request body must not be nil")
	}

	// Enforce monthly voice-minute quota before creating session.
	if s.quota != nil {
		if err := s.quota.CheckVoiceMinuteQuota(ctx, orgID); err != nil {
			return nil, err
		}
	}

	req.LiveKitRoom = generateRoomName(orgID)

	// Resolve concurrent session limit from subscription tier.
	maxSessions := 1 // default free
	if s.quota != nil {
		maxSessions = s.quota.GetConcurrentVoiceLimit(ctx, orgID)
	}

	var session *model.VoiceSession
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		if maxSessions >= 0 {
			lockKey := int64(fnv32a(orgID))
			if _, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); e != nil {
				return e
			}
			active, e := s.repo.CountActiveSessions(ctx, tx, orgID)
			if e != nil {
				return e
			}
			if active >= maxSessions {
				return apierror.NewTooManyRequests("concurrent voice session limit reached")
			}
		}

		var e error
		session, e = s.repo.CreateSession(ctx, tx, orgID, req)
		return e
	})
	// ... rest unchanged
}
```

- [ ] **Step 3: Write integration test for voice quota enforcement**

Add to a test file (e.g. `internal/service/voice_quota_test.go`):

```go
package service_test

import (
	"context"
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

func TestVoiceCreateSession_MonthlyQuotaExceeded_Returns402(t *testing.T) {
	quota := &mockQuotaChecker{
		checkVoiceMinErr: apierror.NewPaymentRequired("monthly voice minute quota exceeded (60/60 minutes)", 60),
		concurrentLimit:  1,
	}
	svc := service.NewVoiceService(nil, nil, nil, "", quota)

	_, err := svc.CreateSession(context.Background(), "org-1", &model.CreateVoiceSessionRequest{})
	if err == nil {
		t.Fatal("expected quota error, got nil")
	}
	qErr, ok := err.(*apierror.QuotaError)
	if !ok {
		t.Fatalf("expected *QuotaError, got %T: %v", err, err)
	}
	if qErr.Code != 402 {
		t.Errorf("expected 402, got %d", qErr.Code)
	}
}
```

- [ ] **Step 4: Run tests and verify compilation**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/service/ -run TestVoiceCreateSession -v && go build ./internal/service/`
Expected: PASS + BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add internal/service/voice.go internal/service/voice_quota_test.go
git commit -m "feat(billing): enforce concurrent + monthly voice quota in VoiceService"
```

---

### Task 8: Create usage endpoint handler

**Files:**
- Create: `internal/handler/usage.go`
- Create: `internal/handler/usage_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/handler/usage_test.go`:

```go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

type mockUsageService struct {
	getUsageFn func(ctx context.Context, orgID string) (*model.UsageResponse, error)
}

func (m *mockUsageService) GetUsage(ctx context.Context, orgID string) (*model.UsageResponse, error) {
	return m.getUsageFn(ctx, orgID)
}

func newUsageRouter(svc handler.UsageServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewUsageHandler(svc)

	authed := r.Group("/api/v1/billing")
	authed.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-123")
		c.Next()
	})
	authed.GET("/usage", h.GetUsage)

	return r
}

func TestGetUsage_Success(t *testing.T) {
	svc := &mockUsageService{
		getUsageFn: func(_ context.Context, orgID string) (*model.UsageResponse, error) {
			return &model.UsageResponse{
				Plan:     model.DefaultPlans()[0],
				KBsUsed:  2,
				KBsLimit: 3,
			}, nil
		},
	}
	r := newUsageRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/billing/usage", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var usage model.UsageResponse
	if err := json.Unmarshal(w.Body.Bytes(), &usage); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if usage.KBsUsed != 2 {
		t.Errorf("expected kbs_used 2, got %d", usage.KBsUsed)
	}
}

func TestGetUsage_NoAuth_Returns401(t *testing.T) {
	svc := &mockUsageService{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewUsageHandler(svc)
	r.GET("/api/v1/billing/usage", h.GetUsage)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/billing/usage", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/handler/ -run TestGetUsage -v`
Expected: FAIL — `UsageHandler`, `UsageServicer` don't exist.

- [ ] **Step 3: Implement usage handler**

Create `internal/handler/usage.go`:

```go
package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// UsageServicer defines what the usage handler needs from the service layer.
type UsageServicer interface {
	GetUsage(ctx context.Context, orgID string) (*model.UsageResponse, error)
}

// UsageHandler handles HTTP requests for billing usage.
type UsageHandler struct {
	svc UsageServicer
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler(svc UsageServicer) *UsageHandler {
	return &UsageHandler{svc: svc}
}

// GetUsage handles GET /api/v1/billing/usage.
//
// @Summary     Get billing usage
// @Tags        billing
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} model.UsageResponse
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /billing/usage [get]
func (h *UsageHandler) GetUsage(c *gin.Context) {
	orgID, exists := c.Get(string(middleware.ContextKeyOrgID))
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized",
			Detail:  "missing organisation context",
		})
		return
	}

	usage, err := h.svc.GetUsage(c.Request.Context(), orgID.(string))
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, usage)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/handler/ -run TestGetUsage -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handler/usage.go internal/handler/usage_test.go
git commit -m "feat(billing): add GET /billing/usage endpoint"
```

---

### Task 9: Wire everything together in main.go and update Valkey tier cache

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Create QuotaChecker and wire into services**

In `cmd/api/main.go`, after the `billingSvc` initialization (~line 295), add:

```go
// --- Quota enforcement ---
subCache := service.NewValkeySubscriptionCache(valkeyClient)
quotaChecker := service.NewQuotaChecker(billingRepo, subCache, pool)
```

- [ ] **Step 2: Update service constructors to pass quotaChecker**

Update the `NewKBService`, `NewWorkspaceService`, and `NewVoiceService` calls:

```go
// Before (existing):
kbSvc := service.NewKBService(kbRepo, pool)
wsSvc := service.NewWorkspaceService(wsRepo, pool)
voiceSvc := service.NewVoiceService(voiceRepo, pool, livekitClient, cfg.LiveKit.WSURL, 1)

// After:
kbSvc := service.NewKBService(kbRepo, pool, quotaChecker)
wsSvc := service.NewWorkspaceService(wsRepo, pool, quotaChecker)
voiceSvc := service.NewVoiceService(voiceRepo, pool, livekitClient, cfg.LiveKit.WSURL, quotaChecker)
```

- [ ] **Step 3: Create UsageHandler and register route**

After billingHandler initialization:

```go
usageHandler := handler.NewUsageHandler(quotaChecker)
```

In the billing route group (~line 670):

```go
billing := api.Group("/billing")
{
	billing.GET("/plans", billingHandler.GetPlans)
	billing.POST("/subscriptions", billingHandler.Subscribe)
	billing.DELETE("/subscriptions/:id", billingHandler.Unsubscribe)
	billing.POST("/payment-intents", billingHandler.CreatePaymentIntent)
	billing.GET("/usage", usageHandler.GetUsage) // NEW
}
```

- [ ] **Step 4: Update Valkey tier cache on subscription create**

In `internal/service/billing.go`, add a cache update after creating a subscription. First, add a `tierCache` field:

```go
type BillingService struct {
	repo          BillingRepository
	pool          *pgxpool.Pool
	hsClient      HyperswitchClient
	webhookSecret string
	plans         map[string]model.Plan
	valkeyClient  redis.Cmdable // for tier cache
}
```

Update `NewBillingService` to accept `valkeyClient redis.Cmdable`.

After `s.repo.CreateSubscription` succeeds in `CreateSubscription`, cache the tier:

```go
// Update Valkey tier cache so rate limiter picks up the new tier.
if s.valkeyClient != nil {
	cacheCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	s.valkeyClient.Set(cacheCtx, "raven:org_tier:"+orgID, string(plan.Tier), 5*time.Minute)
}
```

- [ ] **Step 5: Verify `ByOrgTier` middleware is already wired on API routes**

The spec requires "Enforce API call rate limit by plan tier (wire into the existing Valkey-backed rate limiter)." Verify that `cmd/api/main.go` already has this line on the `api` group:

```go
api.Use(middleware.ByOrgTier(rl, tierResolver, tierCfg, middleware.RouteGroupGeneral))
```

This is already present at line ~434. The `ValkeyTierResolver` reads `raven:org_tier:{orgID}` from Valkey (line ~373 of `ratelimit.go`), and Step 4 above caches the tier there on subscription create. **No additional wiring needed** — just verify the line exists. If for any reason it's missing, add it.

Also verify the completion route group uses `RouteGroupCompletion`. Check if any routes use `ByOrgTier(rl, tierResolver, tierCfg, middleware.RouteGroupCompletion)`. If not, and there are AI completion routes, add that middleware to those route groups.

- [ ] **Step 6: Verify compilation and run existing tests**

Run: `cd /Users/jobinlawrance/Project/raven && go build ./cmd/api/ && go test ./internal/... -count=1`
Expected: BUILD SUCCESS, all tests PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/api/main.go internal/service/billing.go
git commit -m "feat(billing): wire QuotaChecker into services and register usage endpoint"
```

---

### Task 10: Run full test suite and lint

**Files:** None (verification only)

- [ ] **Step 1: Run all Go tests**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./... -count=1 -timeout 120s`
Expected: All PASS

- [ ] **Step 2: Run golangci-lint**

Run: `cd /Users/jobinlawrance/Project/raven && golangci-lint run ./...`
Expected: No errors (fix any issues before proceeding)

- [ ] **Step 3: Final commit if lint fixes were needed**

```bash
git add -A
git commit -m "fix(lint): address linter findings in billing enforcement"
```
