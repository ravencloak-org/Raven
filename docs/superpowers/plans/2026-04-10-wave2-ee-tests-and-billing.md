# Wave 2: EE Tests + Billing Enforcement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace vacuous webhook retry/dead-letter tests with real integration tests backed by testcontainers (Valkey + Asynq), and enforce subscription plan limits across KB, workspace, and voice services with a new `QuotaChecker` and `/billing/usage` endpoint.

**Architecture:** Track A creates a `NewTestValkey` testcontainer helper and rewrites the skipped webhook tests as integration tests that enqueue real Asynq tasks against a Valkey container, verifying retry behaviour and dead-letter semantics end-to-end. Track B adds a `QuotaChecker` service with Valkey-cached subscription lookups, injects limit-check calls into existing `Create`/`AddMember`/`CreateSession` service methods, exposes a `GET /billing/usage` endpoint, and returns `402 Payment Required` when limits are exceeded. Both tracks are independent and can run in parallel.

**Tech Stack:** Go 1.24, Gin, pgx/v5, go-redis/v9 (Valkey), hibiken/asynq, alicebob/miniredis, testcontainers-go, testify, httptest

---

## Track B: Billing Subscription Enforcement (#193)

Track B follows the existing plan at `docs/superpowers/plans/2026-04-10-billing-subscription-enforcement.md`.

**Validation against codebase (2026-04-10):** The existing plan is accurate. All assumptions hold:
- `NewKBService(kbRepo, pool)` at `cmd/api/main.go:267` matches the plan's modification target
- `NewWorkspaceService(wsRepo, pool)` at `cmd/api/main.go:265` matches
- `NewVoiceService(voiceRepo, pool, livekitClient, cfg.LiveKit.WSURL, 1)` at `cmd/api/main.go:313` matches (hardcoded `maxSessions=1` to be replaced with `QuotaChecker`)
- `NewBillingService(billingRepo, pool, hsClient, cfg.Hyperswitch.WebhookSecret)` at `cmd/api/main.go:295` matches (needs `valkeyClient` parameter added)
- `ByOrgTier` is already wired at `cmd/api/main.go:434` (general), `:692` (widget), `:696` (completion) -- no additional rate-limit wiring needed
- `ValkeyTierResolver` reads `raven:org_tier:{orgID}` keys -- matches the plan's cache write on subscription create
- `pkg/apierror` does not yet have `NewPaymentRequired` or `QuotaError` -- plan's Task 1 creates them
- `voice_usage_summaries` table exists (migration `00030_voice_usage.sql`) -- matches the plan's SQL query
- No code divergence detected. Execute the existing plan as-is.

---

## Track A: Webhook Retry/Dead-Letter Tests (#236)

### File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/testutil/valkey_test.go` | Smoke test for `NewTestValkey` (written first — TDD) |
| Create | `internal/testutil/valkey.go` | `NewTestValkey` — testcontainers helper for Valkey + Asynq |
| Modify | `internal/jobs/webhook_delivery.go` | Add `NewWebhookDeliveryHandlerWithClient` for test-injectable HTTP client |
| Create | `internal/jobs/webhook_retry_integration_test.go` | Integration test: failure_count incremented on HTTP 500 |
| Create | `internal/jobs/webhook_deadletter_integration_test.go` | Integration test: webhook marked "failed" after max_retries exhausted |
| Create | `internal/jobs/webhook_success_reset_integration_test.go` | Integration test: failure_count reset to 0 on successful delivery |
| Modify | `internal/ee/webhooks/webhooks_test.go` | Remove vacuous retry/dead-letter concept tests |

---

### Task 1: Create Valkey testcontainer helper

**Files:**
- Create: `internal/testutil/valkey_test.go` (first — TDD)
- Create: `internal/testutil/valkey.go` (after test is confirmed failing)

- [ ] **Step 1: Write the smoke test first (TDD — test before implementation)**

Create `internal/testutil/valkey_test.go`:

```go
package testutil_test

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/testutil"
)

func TestNewTestValkey_Ping(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vc := testutil.NewTestValkey(t)

	client := redis.NewClient(&redis.Options{Addr: vc.Addr})
	defer client.Close()

	err := client.Ping(context.Background()).Err()
	require.NoError(t, err, "Valkey container must be reachable via Ping")
}
```

- [ ] **Step 2: Run the test — confirm it fails to compile (NewTestValkey undefined)**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/testutil/ -run TestNewTestValkey_Ping -v -timeout 60s`
Expected: COMPILE ERROR — `testutil.NewTestValkey` undefined

- [ ] **Step 3: Create the helper implementation**

Create `internal/testutil/valkey.go`:

```go
package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ValkeyContainer holds the running Valkey container address for tests.
type ValkeyContainer struct {
	Addr string // host:port for redis/asynq clients
}

// NewTestValkey spins up a real Valkey (Redis-compatible) container using
// testcontainers and returns its address. The container is terminated when t ends.
func NewTestValkey(t *testing.T) *ValkeyContainer {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "valkey/valkey:8-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor: wait.ForLog("Ready to accept connections").
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start valkey container")
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	addr := fmt.Sprintf("%s:%s", host, port.Port())

	return &ValkeyContainer{Addr: addr}
}
```

- [ ] **Step 4: Run the smoke test — confirm it passes**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/testutil/ -run TestNewTestValkey_Ping -v -timeout 60s`
Expected: PASS (container starts, ping succeeds, container terminates on cleanup)

- [ ] **Step 5: Commit**

```bash
git add internal/testutil/valkey.go internal/testutil/valkey_test.go
git commit -m "test(infra): add Valkey testcontainer helper for integration tests"
```

---

### Task 2: Rewrite webhook retry test as integration test

**Files:**
- Modify: `internal/jobs/webhook_delivery.go` (add `NewWebhookDeliveryHandlerWithClient`)
- Create: `internal/jobs/webhook_retry_integration_test.go`

The current test at lines 83-91 is vacuous -- it simply asserts that a hardcoded variable equals 0. The real retry logic lives in `internal/jobs/webhook_delivery.go` where the `ProcessTask` handler:
1. Increments `failure_count` on failed delivery
2. Compares against `max_retries` on the `WebhookConfig`
3. Calls `asynq.SkipRetry` when threshold is reached
4. Resets `failure_count` on success

We need a real integration test that exercises this flow through an actual Asynq server + Valkey.

**SSRF guard bypass:** `httptest.NewServer` binds to `127.0.0.1` which is blocked by the `safeDialContext` in `NewWebhookDeliveryHandler`. Integration tests must use a separate constructor that accepts a caller-supplied `*http.Client`, bypassing the SSRF transport.

- [ ] **Step 1: Add `NewWebhookDeliveryHandlerWithClient` to `internal/jobs/webhook_delivery.go`**

Add the following constructor immediately after `NewWebhookDeliveryHandler` in `internal/jobs/webhook_delivery.go`:

```go
// NewWebhookDeliveryHandlerWithClient creates a WebhookDeliveryHandler with a
// caller-supplied HTTP client. Intended for testing only — the caller is
// responsible for ensuring the client's transport is appropriately restricted.
func NewWebhookDeliveryHandlerWithClient(pool *pgxpool.Pool, repo *repository.WebhookRepository, client *http.Client, logger *slog.Logger) *WebhookDeliveryHandler {
	return &WebhookDeliveryHandler{
		pool:       pool,
		repo:       repo,
		httpClient: client,
		logger:     logger,
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/jobinlawrance/Project/raven && go build ./internal/jobs/`
Expected: BUILD SUCCESS

- [ ] **Step 3: Create the integration test file**

Create `internal/jobs/webhook_retry_integration_test.go`:

```go
package jobs_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/jobs"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// TestWebhookRetry_Integration verifies that the webhook delivery handler
// increments failure_count on HTTP failures and eventually stops retrying
// (dead-letter behaviour) when max_retries is reached.
//
// This test requires Docker and spins up real Valkey + Postgres containers.
func TestWebhookRetry_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// --- Infrastructure ---
	pool := testutil.NewTestDB(t)
	vc := testutil.NewTestValkey(t)
	ctx := context.Background()
	orgID := uuid.New().String()
	webhookID := uuid.New().String()
	deliveryID := uuid.New().String()
	logger := slog.Default()

	// --- Seed test data ---
	// Insert org, then webhook config with max_retries=2 via direct SQL
	// so we have a known webhook_id to reference.
	_, err := pool.Exec(ctx, "INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		orgID, "Retry Test Org", "retry-test-org")
	require.NoError(t, err)

	// Set RLS context for subsequent inserts.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID)
	require.NoError(t, err)

	// Create a target HTTP server that always returns 500.
	// Use httptest.NewServer with NewWebhookDeliveryHandlerWithClient (bypasses SSRF guard).
	var hitCount atomic.Int32
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "forced failure"}`))
	}))
	defer targetServer.Close()

	// Insert webhook config.
	_, err = tx.Exec(ctx, `
		INSERT INTO webhook_configs (id, org_id, name, url, secret, events, max_retries, status, failure_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		webhookID, orgID, "Retry Test Hook", targetServer.URL,
		"test-secret-key", []string{"lead.generated"}, 2, "active", 0)
	require.NoError(t, err)

	// Insert a delivery record.
	_, err = tx.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, webhook_id, org_id, event_type, payload)
		VALUES ($1, $2, $3, $4, $5)`,
		deliveryID, webhookID, orgID, "lead.generated", `{"lead_id": "l-1"}`)
	require.NoError(t, err)

	require.NoError(t, tx.Commit(ctx))

	// --- Set up Asynq server ---
	// Use NewWebhookDeliveryHandlerWithClient with a plain http.Client to bypass
	// the SSRF safeDialContext that blocks loopback addresses used by httptest.
	webhookRepo := repository.NewWebhookRepository(pool)
	handler := jobs.NewWebhookDeliveryHandlerWithClient(pool, webhookRepo, &http.Client{}, logger)

	asynqSrv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: vc.Addr},
		asynq.Config{
			Concurrency: 1,
			Queues:      map[string]int{"default": 1},
		},
	)
	mux := asynq.NewServeMux()
	mux.Handle(queue.TypeWebhookDelivery, handler)

	require.NoError(t, asynqSrv.Start(mux))
	defer asynqSrv.Shutdown()

	// --- Enqueue the task ---
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: vc.Addr})
	defer asynqClient.Close()

	payload := queue.WebhookDeliveryPayload{
		DeliveryID: deliveryID,
		WebhookID:  webhookID,
		OrgID:      orgID,
		EventType:  "lead.generated",
		Payload:    map[string]any{"lead_id": "l-1"},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(queue.TypeWebhookDelivery, data)
	_, err = asynqClient.Enqueue(task,
		asynq.MaxRetry(0), // retries managed by handler
		asynq.Queue("default"),
	)
	require.NoError(t, err)

	// --- Wait for processing (polling, not fixed sleep) ---
	require.Eventually(t, func() bool {
		return hitCount.Load() >= 1
	}, 10*time.Second, 200*time.Millisecond, "target server must have been hit at least once")

	// --- Verify: failure_count incremented ---
	// Check failure_count in DB using WithOrgID to set RLS context.
	var failureCount int
	var status string
	err = db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			"SELECT failure_count, status FROM webhook_configs WHERE id = $1 AND org_id = $2",
			webhookID, orgID).Scan(&failureCount, &status)
	})
	require.NoError(t, err)

	assert.GreaterOrEqual(t, failureCount, 1,
		"failure_count must be incremented after failed delivery")
}
```

- [ ] **Step 4: Run the integration test**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/jobs/ -run TestWebhookRetry_Integration -v -timeout 120s`
Expected: PASS -- the handler hits the 500 server, increments failure_count, delivery record is updated.

- [ ] **Step 5: Commit**

```bash
git add internal/jobs/webhook_delivery.go internal/jobs/webhook_retry_integration_test.go
git commit -m "test(webhooks): add integration test for retry behaviour with real Valkey + Asynq"
```

---

### Task 3: Rewrite dead-letter test as integration test

**Files:**
- Create: `internal/jobs/webhook_deadletter_integration_test.go`

The current test at lines 95-115 in `webhooks_test.go` uses inline structs to simulate dead-letter logic. We need a real test that verifies: after `max_retries` failed deliveries, the webhook status is set to `"failed"` (dead-lettered).

- [ ] **Step 1: Create the dead-letter integration test**

Create `internal/jobs/webhook_deadletter_integration_test.go`:

```go
package jobs_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/jobs"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// TestWebhookDeadLetter_Integration verifies that after max_retries failed
// deliveries, the webhook config status is set to "failed" (dead-lettered)
// and asynq.SkipRetry prevents further processing.
//
// This test requires Docker and spins up real Valkey + Postgres containers.
func TestWebhookDeadLetter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	vc := testutil.NewTestValkey(t)
	ctx := context.Background()
	orgID := uuid.New().String()
	webhookID := uuid.New().String()
	deliveryID := uuid.New().String()
	logger := slog.Default()

	// --- Seed data ---
	_, err := pool.Exec(ctx, "INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		orgID, "Dead Letter Org", "deadletter-org")
	require.NoError(t, err)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID)
	require.NoError(t, err)

	// Server that always fails.
	// Use httptest.NewServer with NewWebhookDeliveryHandlerWithClient (bypasses SSRF guard).
	var hitCount atomic.Int32
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error": "always fails"}`))
	}))
	defer targetServer.Close()

	// Webhook with max_retries=1 and failure_count already at 0.
	// After one failed delivery, failure_count reaches 1 == max_retries,
	// so the webhook should be marked "failed".
	_, err = tx.Exec(ctx, `
		INSERT INTO webhook_configs (id, org_id, name, url, secret, events, max_retries, status, failure_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		webhookID, orgID, "Dead Letter Hook", targetServer.URL,
		"dl-secret-key", []string{"lead.generated"}, 1, "active", 0)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, webhook_id, org_id, event_type, payload)
		VALUES ($1, $2, $3, $4, $5)`,
		deliveryID, webhookID, orgID, "lead.generated", `{"lead_id": "dl-1"}`)
	require.NoError(t, err)

	require.NoError(t, tx.Commit(ctx))

	// --- Asynq server ---
	// Use NewWebhookDeliveryHandlerWithClient with a plain http.Client to bypass
	// the SSRF safeDialContext that blocks loopback addresses used by httptest.
	webhookRepo := repository.NewWebhookRepository(pool)
	handler := jobs.NewWebhookDeliveryHandlerWithClient(pool, webhookRepo, &http.Client{}, logger)

	asynqSrv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: vc.Addr},
		asynq.Config{
			Concurrency: 1,
			Queues:      map[string]int{"default": 1},
		},
	)
	mux := asynq.NewServeMux()
	mux.Handle(queue.TypeWebhookDelivery, handler)

	require.NoError(t, asynqSrv.Start(mux))
	defer asynqSrv.Shutdown()

	// --- Enqueue ---
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: vc.Addr})
	defer asynqClient.Close()

	payload := queue.WebhookDeliveryPayload{
		DeliveryID: deliveryID,
		WebhookID:  webhookID,
		OrgID:      orgID,
		EventType:  "lead.generated",
		Payload:    map[string]any{"lead_id": "dl-1"},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(queue.TypeWebhookDelivery, data)
	_, err = asynqClient.Enqueue(task,
		asynq.MaxRetry(0),
		asynq.Queue("default"),
	)
	require.NoError(t, err)

	// --- Wait for processing (polling, not fixed sleep) ---
	require.Eventually(t, func() bool {
		return hitCount.Load() >= 1
	}, 10*time.Second, 200*time.Millisecond, "target server must have been hit")

	// --- Verify: webhook marked as "failed" (dead-lettered) ---
	var failureCount int
	var status string
	require.Eventually(t, func() bool {
		err = db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				"SELECT failure_count, status FROM webhook_configs WHERE id = $1 AND org_id = $2",
				webhookID, orgID).Scan(&failureCount, &status)
		})
		return err == nil && status == "failed"
	}, 10*time.Second, 200*time.Millisecond, "webhook status must become 'failed' after max retries exhausted")

	assert.GreaterOrEqual(t, failureCount, 1,
		"failure_count must reach max_retries")
	assert.Equal(t, "failed", status,
		"webhook status must be 'failed' after max retries exhausted (dead-lettered)")
}
```

- [ ] **Step 2: Run the dead-letter integration test**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/jobs/ -run TestWebhookDeadLetter_Integration -v -timeout 120s`
Expected: PASS -- after one failed delivery with max_retries=1, webhook status becomes "failed".

- [ ] **Step 3: Commit**

```bash
git add internal/jobs/webhook_deadletter_integration_test.go
git commit -m "test(webhooks): add integration test for dead-letter behaviour after max retries"
```

---

### Task 4: Add success-resets-failure-count integration test

**Files:**
- Create: `internal/jobs/webhook_success_reset_integration_test.go`

The handler also resets `failure_count` to 0 on a successful delivery. This verifies the "recovery from failure" path.

- [ ] **Step 1: Create the success reset integration test**

Create `internal/jobs/webhook_success_reset_integration_test.go`:

```go
package jobs_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/jobs"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// TestWebhookSuccessReset_Integration verifies that a successful delivery
// resets the failure_count to 0 on the webhook config.
func TestWebhookSuccessReset_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	vc := testutil.NewTestValkey(t)
	ctx := context.Background()
	orgID := uuid.New().String()
	webhookID := uuid.New().String()
	deliveryID := uuid.New().String()
	logger := slog.Default()

	// --- Seed data ---
	_, err := pool.Exec(ctx, "INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		orgID, "Success Reset Org", "success-reset-org")
	require.NoError(t, err)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID)
	require.NoError(t, err)

	// Server that returns 200 OK.
	// Use httptest.NewServer with NewWebhookDeliveryHandlerWithClient (bypasses SSRF guard).
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer targetServer.Close()

	// Webhook with prior failures (failure_count=2, max_retries=3).
	_, err = tx.Exec(ctx, `
		INSERT INTO webhook_configs (id, org_id, name, url, secret, events, max_retries, status, failure_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		webhookID, orgID, "Success Reset Hook", targetServer.URL,
		"success-secret", []string{"lead.generated"}, 3, "active", 2)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, webhook_id, org_id, event_type, payload)
		VALUES ($1, $2, $3, $4, $5)`,
		deliveryID, webhookID, orgID, "lead.generated", `{"lead_id": "s-1"}`)
	require.NoError(t, err)

	require.NoError(t, tx.Commit(ctx))

	// --- Asynq server ---
	// Use NewWebhookDeliveryHandlerWithClient with a plain http.Client to bypass
	// the SSRF safeDialContext that blocks loopback addresses used by httptest.
	webhookRepo := repository.NewWebhookRepository(pool)
	handler := jobs.NewWebhookDeliveryHandlerWithClient(pool, webhookRepo, &http.Client{}, logger)

	asynqSrv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: vc.Addr},
		asynq.Config{
			Concurrency: 1,
			Queues:      map[string]int{"default": 1},
		},
	)
	mux := asynq.NewServeMux()
	mux.Handle(queue.TypeWebhookDelivery, handler)

	require.NoError(t, asynqSrv.Start(mux))
	defer asynqSrv.Shutdown()

	// --- Enqueue ---
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: vc.Addr})
	defer asynqClient.Close()

	payload := queue.WebhookDeliveryPayload{
		DeliveryID: deliveryID,
		WebhookID:  webhookID,
		OrgID:      orgID,
		EventType:  "lead.generated",
		Payload:    map[string]any{"lead_id": "s-1"},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(queue.TypeWebhookDelivery, data)
	_, err = asynqClient.Enqueue(task,
		asynq.MaxRetry(0),
		asynq.Queue("default"),
	)
	require.NoError(t, err)

	// --- Wait for processing (polling, not fixed sleep) ---
	// Poll until failure_count is reset to 0 indicating successful delivery.
	var failureCount int
	var status string
	require.Eventually(t, func() bool {
		err = db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				"SELECT failure_count, status FROM webhook_configs WHERE id = $1 AND org_id = $2",
				webhookID, orgID).Scan(&failureCount, &status)
		})
		return err == nil && failureCount == 0
	}, 10*time.Second, 200*time.Millisecond, "failure_count must be reset to 0 after successful delivery")

	// --- Verify: failure_count reset to 0, status still active ---
	assert.Equal(t, 0, failureCount,
		"failure_count must be reset to 0 after successful delivery")
	assert.Equal(t, "active", status,
		"webhook status must remain active after successful delivery")

	// Verify delivery record was updated as successful.
	// The column is `status` (VARCHAR), not `success` (bool).
	var deliveryStatus string
	var responseStatus int
	err = db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			"SELECT status, response_status FROM webhook_deliveries WHERE id = $1 AND org_id = $2",
			deliveryID, orgID).Scan(&deliveryStatus, &responseStatus)
	})
	require.NoError(t, err)

	assert.Equal(t, "delivered", deliveryStatus, "delivery must be marked as 'delivered'")
	assert.Equal(t, http.StatusOK, responseStatus, "response status must be 200")
}
```

- [ ] **Step 2: Run the success reset test**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/jobs/ -run TestWebhookSuccessReset_Integration -v -timeout 120s`
Expected: PASS -- successful delivery resets failure_count to 0.

- [ ] **Step 3: Commit**

```bash
git add internal/jobs/webhook_success_reset_integration_test.go
git commit -m "test(webhooks): add integration test for failure_count reset on successful delivery"
```

---

### Task 5: Remove vacuous tests from webhooks_test.go

**Files:**
- Modify: `internal/ee/webhooks/webhooks_test.go`

Now that real integration tests exist in `internal/jobs/`, the vacuous concept tests can be removed. The HMAC tests (lines 31-78) are legitimate unit tests and should be kept. The `TestPackageCompiles` test (line 18-20) can also be kept as a compile guard.

- [ ] **Step 1: Remove the two vacuous tests**

Remove `TestWebhookDelivery_Retry_ManagedByHandler` (lines 83-91) and `TestWebhookDelivery_DeadLetter_AfterMaxRetries` (lines 95-115) from `internal/ee/webhooks/webhooks_test.go`.

The file should retain:
- `TestPackageCompiles` (line 18)
- `computeHMAC` helper (line 23)
- `TestWebhookDelivery_HMACSignature_Correct` (line 31)
- `TestWebhookDelivery_HMACSignature_WrongSecret_NotEqual` (line 53)
- `TestWebhookDelivery_HMAC_Verification` (line 64)

After removal, the file ends at line 78 (the closing brace of `TestWebhookDelivery_HMAC_Verification`).

- [ ] **Step 2: Remove unused imports if any**

After removing the two tests, verify no unused imports remain. The `assert` and `require` imports are still used by the remaining HMAC tests.

- [ ] **Step 3: Run remaining tests to confirm they still pass**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/ee/webhooks/ -v`
Expected: PASS (3 HMAC tests + 1 compile test)

- [ ] **Step 4: Run the new integration tests to confirm they still pass**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./internal/jobs/ -run 'TestWebhook(Retry|DeadLetter|SuccessReset)_Integration' -v -timeout 180s`
Expected: PASS (all 3 integration tests)

- [ ] **Step 5: Commit**

```bash
git add internal/ee/webhooks/webhooks_test.go
git commit -m "test(webhooks): remove vacuous retry/dead-letter concept tests replaced by integration tests"
```

---

### Task 6: Run full test suite and lint for Track A

**Files:** None (verification only)

- [ ] **Step 1: Run all Go tests (excluding short mode so integration tests run)**

Run: `cd /Users/jobinlawrance/Project/raven && go test ./... -count=1 -timeout 300s`
Expected: All PASS

- [ ] **Step 2: Run golangci-lint**

Run: `cd /Users/jobinlawrance/Project/raven && golangci-lint run ./...`
Expected: No errors

- [ ] **Step 3: Fix any lint or test issues and commit if needed**

```bash
git add -A
git commit -m "fix(lint): address linter findings in webhook integration tests"
```
