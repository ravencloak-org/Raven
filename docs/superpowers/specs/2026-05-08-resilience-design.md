# Resilience Layer for Raven Go API — Design

**Date:** 2026-05-08
**Status:** Approved (awaiting implementation plan)
**Owner:** Jobin Lawrance
**Target branch:** `feat/resilience-layer` (worktree, from latest `origin/main`)

## Context

The Raven Go API today has scattered `context.WithTimeout` usage in `internal/middleware/ratelimit.go`, `internal/service/security.go`, `internal/service/billing.go`, and `internal/repository/semantic_cache.go`, but no consistent discipline at the boundaries that matter most:

- **AI worker gRPC client** (`internal/grpc/client.go`) — has fault-injection tests in `internal/integration/grpc_fault_test.go`, but no circuit breaker. A stalled Python worker today will pile up goroutines on the API process until OOM.
- **`http.Server` in `cmd/api/main.go`** — uses `gin.Default()` with no `ReadTimeout`/`WriteTimeout`/`IdleTimeout`/`ReadHeaderTimeout` set on the underlying `http.Server`. Go's defaults are zero, which means a slow client can hold a connection forever (slowloris class).
- **Per-route deadlines** — Gin handlers do not enforce per-route context budgets. A handler that forgets `ctx.WithTimeout` can run indefinitely.
- **Asynq handlers** — task handlers do not consistently apply per-task deadlines, so a stuck handler holds a worker slot indefinitely.

No `gobreaker` / `failsafe` / `circuitbreaker` imports exist anywhere in production code today.

## Goals

1. Bound every external call by an explicit deadline.
2. Trip a circuit breaker on the AI-worker gRPC client to prevent cascading failure.
3. Harden the `http.Server` against slowloris-class hangs.
4. Enforce a per-route Gin context deadline so no handler runs unbounded.
5. Apply consistent per-task deadlines in Asynq handlers.
6. Add a CI compliance gate that fails when new code introduces external calls without context deadlines.

## Non-Goals (deferred to follow-up issues)

- Bulkhead semaphores (per-dependency goroutine caps).
- Retries with exponential backoff on the gRPC client (Asynq already retries handler-level).
- A wholesale audit of every outbound HTTP client construction site (LiveKit, SeaweedFS) — only the callsites we touch as part of this milestone get refactored to the new factory. Remaining audit moves to a follow-up.
- OpenObserve dashboards for the new metrics.
- Changes to the Python AI worker.

## Architecture

A new package `internal/resilience/` is the single home for resilience primitives. A sibling middleware lives under `internal/middleware/`.

### Package layout

```
internal/resilience/
  policy.go        # Policy struct + functional options + constructor (returns error)
  policy_test.go   # Table-driven tests for timeout + breaker behaviour
  breaker.go       # Adapter over sony/gobreaker; typed ErrCircuitOpen
  grpc.go          # UnaryClientInterceptor(*Policy)
  grpc_test.go     # Interceptor unit tests
  http.go          # RoundTripper decorator + HTTPClient(*Policy) factory

internal/middleware/
  deadline.go      # Deadline(d time.Duration) gin.HandlerFunc
  deadline_test.go # Per-route deadline application
```

### `Policy` shape

```go
type Policy struct {
    Name              string
    Timeout           time.Duration
    BreakerThreshold  uint32        // consecutive failures before Open
    BreakerCooldown   time.Duration // time in Open before Half-Open probe
    BreakerHalfOpenMax uint32       // requests allowed in Half-Open
}

type Option func(*Policy)

func WithTimeout(d time.Duration) Option { ... }
func WithBreakerThreshold(n uint32) Option { ... }
func WithBreakerCooldown(d time.Duration) Option { ... }

func NewPolicy(name string, opts ...Option) (*Policy, error) {
    // sensible defaults, validate on construction
}
```

Defaults:

| Knob                | Default | Env override                          |
|---------------------|---------|---------------------------------------|
| Timeout             | 5s      | `RAVEN_AI_WORKER_TIMEOUT`             |
| BreakerThreshold    | 5       | `RAVEN_AI_WORKER_BREAKER_THRESHOLD`   |
| BreakerCooldown     | 30s     | `RAVEN_AI_WORKER_BREAKER_COOLDOWN`    |
| BreakerHalfOpenMax  | 1       | (no env override; spec'd at 1 probe)  |

### Typed errors

```go
var ErrCircuitOpen = errors.New("resilience: circuit breaker open")
```

`apierror.ErrorHandler` maps `ErrCircuitOpen` to HTTP 503 with `Retry-After`; `context.DeadlineExceeded` continues to map to 504.

## Wiring per boundary

### 1. AI worker gRPC client (`internal/grpc/client.go`)

- `NewClient` signature changes to accept `*resilience.Policy`.
- Interceptor is wired via `grpc.WithChainUnaryInterceptor(resilience.UnaryClientInterceptor(policy))`.
- The interceptor:
  1. Wraps each call with `context.WithTimeout(ctx, policy.Timeout)` if no shorter deadline already set.
  2. Submits the call through `gobreaker.Execute`.
  3. Maps `gobreaker.ErrOpenState` / `ErrTooManyRequests` to `ErrCircuitOpen`.
  4. Treats `codes.OK`, `codes.NotFound`, `codes.InvalidArgument`, `codes.PermissionDenied`, `codes.Unauthenticated` as **non-failures** for breaker accounting (these are caller errors, not server failures).
  5. Treats `codes.Unavailable`, `codes.DeadlineExceeded`, `codes.Internal`, `codes.ResourceExhausted` as failures.

`internal/grpc/client_test.go` is updated for the new signature.

### 2. `http.Server` in `cmd/api/main.go`

Replace the bare server construction with explicit timeouts:

```go
srv := &http.Server{
    Addr:              cfg.HTTP.Addr,
    Handler:           router,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      60 * time.Second,
    IdleTimeout:       120 * time.Second,
}
```

These values are sized for the longest expected legitimate request (file upload at 30s read, streaming chat response at 60s write). Knobs surface through `cfg.HTTP.*` for environment overrides.

### 3. Per-route Gin deadline middleware

`internal/middleware/deadline.go`:

```go
func Deadline(d time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx, cancel := context.WithTimeout(c.Request.Context(), d)
        defer cancel()
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

Application in `cmd/api/main.go`, applied per route group:

| Route group           | Deadline | Rationale                                      |
|-----------------------|----------|------------------------------------------------|
| `/healthz`, `/readyz` | 1s       | Should be near-instant or treated as down      |
| `/api/v1/chat/*`      | 30s      | Streaming chat may hold the request open       |
| `/api/v1/upload/*`    | 60s      | Large file uploads                             |
| `/api/v1/voice/*`     | 30s      | Voice synth/transcribe over LiveKit            |
| Default `/api/v1/*`   | 10s      | Most CRUD-style endpoints                      |

### 4. Asynq handlers

Each handler entry point wraps its work with `context.WithTimeout` keyed by task type. Where Asynq's native `Timeout` registration suffices we use that instead of inline `WithTimeout`. The exact handler-package path is surveyed during implementation planning (the package containing `asynq.HandlerFunc` registrations); the convention is documented in a short comment at the top of that file.

## Error handling

- `ErrCircuitOpen` → HTTP 503 + `Retry-After: <BreakerCooldown seconds>`.
- `context.DeadlineExceeded` → HTTP 504 (existing behaviour preserved).
- Breaker state transitions logged at WARN with structured fields: `service`, `state`, `consecutive_failures`, `policy_name`.
- OTel:
  - Gauge metric `resilience.breaker.state` (`0=closed, 1=half_open, 2=open`) labeled by `policy_name`.
  - Span event `resilience.breaker.transition` on every transition.

## Testing strategy

### Unit tests

- `internal/resilience/policy_test.go` — table-driven:
  - Timeout fires when work exceeds `Timeout`.
  - Breaker opens after `BreakerThreshold` consecutive failures.
  - Breaker stays open for `BreakerCooldown`.
  - Breaker transitions to half-open after cooldown; success closes it; failure re-opens it.
  - Failures classified correctly (caller errors do not count).
- `internal/resilience/grpc_test.go` — interceptor wraps + propagates ctx + applies CB; uses a fake `grpc.UnaryInvoker`.
- `internal/middleware/deadline_test.go` — middleware injects deadline; downstream handler observes correct deadline; cancellation propagates.

### Integration tests

Extend `internal/integration/grpc_fault_test.go` with three new cases:

1. **Slow worker** — fault server sleeps 10s; client policy `Timeout=2s`; assert call returns `context.DeadlineExceeded` within ~2s (±100ms tolerance).
2. **Breaker opens** — fault server returns `codes.Unavailable` 5×; assert 6th call returns `ErrCircuitOpen` *without* attempting RPC (verify via server-side counter).
3. **Half-open recovery** — after breaker opens, sleep `BreakerCooldown`, configure server to return OK; single probe call closes the breaker; subsequent calls go through normally.

All tests run under standard `go test ./...` — no docker-compose or external infra required.

## CI compliance gate

`.golangci.yml` adds two linters at error severity:

- `noctx` — flags `http.Get`/`http.Post` and similar without context.
- `contextcheck` — flags functions that should accept `ctx context.Context` but don't, and functions that drop a parent context.

Scoped to `internal/grpc/`, `internal/handler/`, `internal/service/` via `issues.exclude-rules`. CI fails on any violation.

**Optional follow-up (not blocking this milestone):** custom `analysis.Analyzer` plugin in `tools/analyzers/extcall/` that flags `grpc.Dial`, `grpc.NewClient`, `http.NewRequest`, and `http.NewRequestWithContext` outside `internal/resilience/`. Tracked as a follow-up issue.

## Compliance / OSPS-L2 alignment

- Documents threat: unbounded external calls → resource exhaustion → DoS pathway. Closing this gap is consistent with the OSPS-L2 trajectory already in flight (`docs/osps-l2-compliance-spec`, recent PRs #348, #391).
- Adds CI gate that prevents regression — auditable, version-controlled, enforced.
- Logged breaker transitions are auditable via OpenObserve.

## Configuration knobs (env, with defaults inline)

| Variable                              | Default | Used by                                |
|---------------------------------------|---------|----------------------------------------|
| `RAVEN_AI_WORKER_TIMEOUT`             | `5s`    | gRPC interceptor policy                |
| `RAVEN_AI_WORKER_BREAKER_THRESHOLD`   | `5`     | gRPC interceptor policy                |
| `RAVEN_AI_WORKER_BREAKER_COOLDOWN`    | `30s`   | gRPC interceptor policy                |
| `RAVEN_HTTP_READ_HEADER_TIMEOUT`      | `5s`    | `http.Server.ReadHeaderTimeout`        |
| `RAVEN_HTTP_READ_TIMEOUT`             | `30s`   | `http.Server.ReadTimeout`              |
| `RAVEN_HTTP_WRITE_TIMEOUT`            | `60s`   | `http.Server.WriteTimeout`             |
| `RAVEN_HTTP_IDLE_TIMEOUT`             | `120s`  | `http.Server.IdleTimeout`              |

`.env.example` updated with the new variables.

## Files added

- `internal/resilience/policy.go`
- `internal/resilience/policy_test.go`
- `internal/resilience/breaker.go`
- `internal/resilience/grpc.go`
- `internal/resilience/grpc_test.go`
- `internal/resilience/http.go`
- `internal/middleware/deadline.go`
- `internal/middleware/deadline_test.go`

## Files modified

- `internal/grpc/client.go` — `NewClient` accepts `*resilience.Policy`, wires interceptor.
- `internal/grpc/client_test.go` — adapt to new signature.
- `internal/integration/grpc_fault_test.go` — add three new test cases.
- `cmd/api/main.go` — `http.Server` timeouts; `Deadline` middleware applied per route group.
- `internal/apierror/` — map `ErrCircuitOpen` → 503 + `Retry-After` in the file that defines `ErrorHandler` (resolved during plan survey).
- The config package (path resolved during plan survey, e.g., `internal/config/`) — surface the new env knobs.
- `.env.example` — document new variables.
- `.golangci.yml` — enable `noctx` + `contextcheck`.
- `go.mod` / `go.sum` — add `github.com/sony/gobreaker`.

## Rollout

1. Implement on `feat/resilience-layer` worktree branched from `origin/main`.
2. Run full local test suite (unit + integration) and `golangci-lint run` before pushing.
3. Open PR; CI must be green, including the new linter checks.
4. Squash-merge per repo policy. No AI attribution in commit message.
5. Auto-merge enqueued at PR creation per `CLAUDE.md`.

## Open questions

None. All decisions made during brainstorming.
