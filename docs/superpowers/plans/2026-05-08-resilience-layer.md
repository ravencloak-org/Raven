# Resilience Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add bounded-deadline + circuit-breaker resilience to the Raven Go API across the AI-worker gRPC client, `http.Server`, per-route Gin handlers, and Asynq job handlers — with unit + integration tests and a CI compliance gate.

**Architecture:** New `internal/resilience/` package owns reusable primitives (`Policy`, breaker adapter, gRPC interceptor, HTTP RoundTripper). New `internal/middleware/deadline.go` enforces per-route context deadlines. `cmd/api/main.go` wires explicit `http.Server` timeouts and the Deadline middleware. `.golangci.yml` adds `noctx` + `contextcheck` as blocking lint gates.

**Tech Stack:** Go 1.22+, Gin, `google.golang.org/grpc`, `github.com/sony/gobreaker`, `github.com/hibiken/asynq`, `golangci-lint v2`.

**Spec:** [`docs/superpowers/specs/2026-05-08-resilience-design.md`](../specs/2026-05-08-resilience-design.md)

**Branch:** `feat/resilience-layer` (worktree from `origin/main`).

**Revised:** 2026-05-11 after `/grill-with-docs` session. Eight design branches resolved against the actual codebase; see the **Revision change log** at the end of this file. The spec was authored against several assumptions that don't match the repo (apierror lives at `pkg/apierror/`, not `internal/apierror/`; `/readyz`, `/api/v1/upload/*`, `/api/v1/voice/*` route groups don't exist; Asynq handlers come in two shapes, not one). Tasks below incorporate the corrections. Task 5 (HTTP factory) is dropped entirely; remaining task numbers are unchanged for traceability.

## Parallel-execution map

Tasks group into phases. Tasks within a phase can run in parallel (disjoint file ownership). Tasks across phases are sequential.

| Phase | Tasks | Parallelizable | Notes |
|-------|-------|----------------|-------|
| 0 — Bootstrap | 1 | no | Worktree + dep + survey |
| 1 — Primitives | 2, 3, 6, 7, 8 | **yes (5 agents)** | All write disjoint files |
| 2 — Composite primitives | 4 | no | Task 5 dropped — see revision log |
| 3 — Wire | 9, 10 | **yes (2 agents)** | Depend on 2+3+4 |
| 4 — Main wiring | 11 | no | Touches cmd/api/main.go (single owner) — depends on 6, 9 |
| 5 — Asynq + integration | 12, 13 | **yes (2 agents)** | 12 owns jobs/ + scheduler.go middleware, 13 owns integration/ |
| 6 — Finish | 14, 15 | no | Sequential close-out |

---

## Task 1: Bootstrap — worktree, dependency, survey

**Files:**
- Create: `feat/resilience-layer` worktree branch, copy spec into it
- Modify: `go.mod`, `go.sum`
- Create: `docs/superpowers/plans/2026-05-08-resilience-survey.md` (temp survey doc, deleted before PR)

- [ ] **Step 1: Create worktree from latest origin/main**

Run from the repo root:

```bash
cd /Users/jobinlawrance/Project/raven
git fetch origin main
WORKTREE_DIR=".worktrees/feat-resilience-layer"
git worktree add -b feat/resilience-layer "$WORKTREE_DIR" origin/main
cd "$WORKTREE_DIR"
```

Expected: worktree created at `.worktrees/feat-resilience-layer`, branch `feat/resilience-layer` checked out at the tip of `origin/main`.

All subsequent steps run inside this worktree. Use `cd $WORKTREE_DIR` or set the worktree as the working directory for spawned agents.

- [ ] **Step 2: Copy the spec file into the worktree**

The spec was authored on a different branch and is not yet in `origin/main`. Copy it now so it lands with this PR.

```bash
mkdir -p docs/superpowers/specs
cp /Users/jobinlawrance/Project/raven/docs/superpowers/specs/2026-05-08-resilience-design.md docs/superpowers/specs/2026-05-08-resilience-design.md
mkdir -p docs/superpowers/plans
cp /Users/jobinlawrance/Project/raven/docs/superpowers/plans/2026-05-08-resilience-layer.md docs/superpowers/plans/2026-05-08-resilience-layer.md
```

- [ ] **Step 3: Add sony/gobreaker dependency**

```bash
go get github.com/sony/gobreaker/v2@latest
go mod tidy
```

Expected: `go.mod` and `go.sum` updated with `github.com/sony/gobreaker/v2`.

- [ ] **Step 4: Verify the dep resolves and the project still builds**

```bash
go build ./...
```

Expected: PASS, no errors.

- [ ] **Step 5: Survey unknown paths and write survey doc**

Run these and capture output into `docs/superpowers/plans/2026-05-08-resilience-survey.md`:

```bash
{
  echo "# Resilience plan — codebase survey ($(date -u +%FT%TZ))"
  echo
  echo "## apierror package (NOTE: lives at pkg/apierror/, NOT internal/apierror/)"
  grep -rn 'package apierror\|func ErrorHandler' --include='*.go' pkg/ internal/ cmd/ | head
  echo
  echo "## Asynq handlers — ProcessTask methods (some use _ context.Context)"
  grep -rEn 'func .* ProcessTask\((ctx|_) context.Context' --include='*.go' internal/jobs/
  echo
  echo "## Asynq handlers — HandlerFunc factories (return asynq.HandlerFunc)"
  grep -rEn 'asynq\.HandlerFunc' --include='*.go' internal/jobs/ | grep -v _test
  echo
  echo "## Asynq mux registration site (where mux.Use can be called)"
  grep -rEn 'asynq\.NewServeMux|mux\.Handle\b' --include='*.go' internal/jobs/ cmd/api/
  echo
  echo "## Config: ServerConfig fields"
  awk '/type ServerConfig struct/,/^}/' internal/config/config.go
  echo
  echo "## Config: GRPCConfig fields"
  awk '/type GRPCConfig struct/,/^}/' internal/config/config.go
  echo
  echo "## main.go http.Server construction (line ~855)"
  sed -n '850,890p' cmd/api/main.go
  echo
  echo "## main.go gRPC client construction (line ~313)"
  sed -n '305,330p' cmd/api/main.go
  echo
  echo "## OTel meter + tracer construction (needed by Task 3 observability)"
  grep -rEn 'otel\.Meter\(|otel\.Tracer\(|InstrumentationName|Meter\(\)|Tracer\(\)' --include='*.go' cmd/api/ internal/ | head -20
  echo
  echo "## Route groups in cmd/api/main.go (referenced by Task 11 deadline table)"
  grep -nE 'router\.Group|\.Group\(|router\.GET|router\.POST' cmd/api/main.go
} > docs/superpowers/plans/2026-05-08-resilience-survey.md
```

Read the resulting file. Confirmed paths (verified in grilling 2026-05-11):

- `pkg/apierror/apierror.go` already defines `ErrorHandler()` middleware that switches on `*AppError` / `*QuotaError` type assertions.
- `internal/jobs/` has 6 `ProcessTask` methods (one with `_ context.Context` — `airbyte_sync.go:35`) and 3 `asynq.HandlerFunc` factories (`document_process.go`, `email_summary.go`, `send_email.go`). Task 12 covers ALL of them via mux middleware.
- Real route structure differs from the spec — see Task 11 step 2 for the corrected table.

- [ ] **Step 6: Commit the bootstrap**

```bash
git add docs/superpowers/specs/2026-05-08-resilience-design.md docs/superpowers/plans/2026-05-08-resilience-layer.md docs/superpowers/plans/2026-05-08-resilience-survey.md go.mod go.sum
git commit -m "chore(resilience): bootstrap branch with spec, plan, and gobreaker dep"
```

---

## Task 2: `resilience.Policy` (primitive — parallel-safe)

**Files:**
- Create: `internal/resilience/policy.go`
- Test: `internal/resilience/policy_test.go`

Owns no shared files with Tasks 3, 6, 7, 8.

- [ ] **Step 1: Write the failing test**

Create `internal/resilience/policy_test.go`:

```go
package resilience

import (
	"errors"
	"testing"
	"time"
)

func TestNewPolicy_Defaults(t *testing.T) {
	p, err := NewPolicy("ai-worker")
	if err != nil {
		t.Fatalf("NewPolicy returned error: %v", err)
	}
	if p.Name != "ai-worker" {
		t.Errorf("Name = %q, want %q", p.Name, "ai-worker")
	}
	if p.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", p.Timeout)
	}
	if p.BreakerThreshold != 5 {
		t.Errorf("BreakerThreshold = %d, want 5", p.BreakerThreshold)
	}
	if p.BreakerCooldown != 30*time.Second {
		t.Errorf("BreakerCooldown = %v, want 30s", p.BreakerCooldown)
	}
	if p.BreakerHalfOpenMax != 1 {
		t.Errorf("BreakerHalfOpenMax = %d, want 1", p.BreakerHalfOpenMax)
	}
}

func TestNewPolicy_Options(t *testing.T) {
	p, err := NewPolicy("svc",
		WithTimeout(2*time.Second),
		WithBreakerThreshold(10),
		WithBreakerCooldown(15*time.Second),
		WithBreakerHalfOpenMax(3),
	)
	if err != nil {
		t.Fatalf("NewPolicy returned error: %v", err)
	}
	if p.Timeout != 2*time.Second {
		t.Errorf("Timeout = %v, want 2s", p.Timeout)
	}
	if p.BreakerThreshold != 10 {
		t.Errorf("BreakerThreshold = %d, want 10", p.BreakerThreshold)
	}
	if p.BreakerCooldown != 15*time.Second {
		t.Errorf("BreakerCooldown = %v, want 15s", p.BreakerCooldown)
	}
	if p.BreakerHalfOpenMax != 3 {
		t.Errorf("BreakerHalfOpenMax = %d, want 3", p.BreakerHalfOpenMax)
	}
}

func TestNewPolicy_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
	}{
		{"zero timeout", []Option{WithTimeout(0)}},
		{"negative timeout", []Option{WithTimeout(-1 * time.Second)}},
		{"zero threshold", []Option{WithBreakerThreshold(0)}},
		{"zero cooldown", []Option{WithBreakerCooldown(0)}},
		{"zero halfopen max", []Option{WithBreakerHalfOpenMax(0)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPolicy("svc", tc.opts...)
			if !errors.Is(err, ErrInvalidPolicy) {
				t.Errorf("NewPolicy err = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}

func TestNewPolicy_EmptyName(t *testing.T) {
	_, err := NewPolicy("")
	if !errors.Is(err, ErrInvalidPolicy) {
		t.Errorf("NewPolicy(\"\") err = %v, want ErrInvalidPolicy", err)
	}
}
```

- [ ] **Step 2: Run the test — expect compile failure**

```bash
go test ./internal/resilience/...
```

Expected: build error (`undefined: NewPolicy`, `undefined: ErrInvalidPolicy`, etc.).

- [ ] **Step 3: Implement `policy.go`**

Create `internal/resilience/policy.go`:

```go
// Package resilience provides timeout + circuit-breaker primitives for
// bounding external calls (gRPC, HTTP) made by the Raven API.
package resilience

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidPolicy is returned by NewPolicy when configuration validation fails.
var ErrInvalidPolicy = errors.New("resilience: invalid policy")

// Policy bundles the timeout and circuit-breaker configuration that gets
// applied to a single external dependency (e.g. the AI worker gRPC client).
type Policy struct {
	Name               string
	Timeout            time.Duration
	BreakerThreshold   uint32
	BreakerCooldown    time.Duration
	BreakerHalfOpenMax uint32
}

// Option mutates a Policy during construction.
type Option func(*Policy)

// WithTimeout sets the per-call deadline.
func WithTimeout(d time.Duration) Option {
	return func(p *Policy) { p.Timeout = d }
}

// WithBreakerThreshold sets the consecutive-failure count that flips the
// breaker from Closed to Open.
func WithBreakerThreshold(n uint32) Option {
	return func(p *Policy) { p.BreakerThreshold = n }
}

// WithBreakerCooldown sets how long the breaker stays Open before the
// next probe transitions it to Half-Open.
func WithBreakerCooldown(d time.Duration) Option {
	return func(p *Policy) { p.BreakerCooldown = d }
}

// WithBreakerHalfOpenMax caps in-flight probes during Half-Open.
func WithBreakerHalfOpenMax(n uint32) Option {
	return func(p *Policy) { p.BreakerHalfOpenMax = n }
}

// NewPolicy returns a validated Policy. Defaults: 5s timeout,
// breaker opens after 5 consecutive failures, 30s cooldown, 1 half-open probe.
func NewPolicy(name string, opts ...Option) (*Policy, error) {
	p := &Policy{
		Name:               name,
		Timeout:            5 * time.Second,
		BreakerThreshold:   5,
		BreakerCooldown:    30 * time.Second,
		BreakerHalfOpenMax: 1,
	}
	for _, opt := range opts {
		opt(p)
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Policy) validate() error {
	if p.Name == "" {
		return fmt.Errorf("%w: name must not be empty", ErrInvalidPolicy)
	}
	if p.Timeout <= 0 {
		return fmt.Errorf("%w: timeout must be > 0", ErrInvalidPolicy)
	}
	if p.BreakerThreshold == 0 {
		return fmt.Errorf("%w: breaker threshold must be > 0", ErrInvalidPolicy)
	}
	if p.BreakerCooldown <= 0 {
		return fmt.Errorf("%w: breaker cooldown must be > 0", ErrInvalidPolicy)
	}
	if p.BreakerHalfOpenMax == 0 {
		return fmt.Errorf("%w: breaker half-open max must be > 0", ErrInvalidPolicy)
	}
	return nil
}
```

- [ ] **Step 4: Run the test — expect pass**

```bash
go test ./internal/resilience/... -run TestNewPolicy -v
```

Expected: PASS for all four subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/resilience/policy.go internal/resilience/policy_test.go
git commit -m "feat(resilience): add Policy with functional options + validation"
```

---

## Task 3: `resilience.Breaker` adapter + OTel observability (parallel-safe)

**Files:**
- Create: `internal/resilience/breaker.go`
- Test: `internal/resilience/breaker_test.go`

Owns only files in `internal/resilience/breaker*.go` — disjoint from Tasks 2, 6, 7, 8.

**Two adjustments from the original spec, both surfaced in grilling:**

1. The breaker accepts an `IsSuccessful` predicate via `WithIsSuccessful(...)`. This is the hook gobreaker already provides for "this error doesn't count toward the breaker." Task 4's gRPC interceptor passes a predicate that classifies caller-side gRPC codes (`InvalidArgument`, `NotFound`, etc.) as success-equivalent, so those errors propagate to the caller AND don't trip the breaker. Without this hook, the original Task 4 design swallowed caller errors entirely (correctness bug).
2. The breaker wires `gobreaker.OnStateChange` to OTel — gauge `resilience.breaker.state` (0=Closed, 1=HalfOpen, 2=Open) and span event `resilience.breaker.transition`. The spec listed these under "Error handling" but no original task implemented them. Folded in here because OnStateChange is set inside `gobreaker.Settings`, which is built inside `NewBreaker` — there's no clean separate file to put it in.

- [ ] **Step 1: Write the failing test**

Create `internal/resilience/breaker_test.go`:

```go
package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBreaker_OpensAfterThreshold(t *testing.T) {
	p, _ := NewPolicy("svc",
		WithBreakerThreshold(3),
		WithBreakerCooldown(50*time.Millisecond),
	)
	br := NewBreaker(p)

	failing := func(context.Context) (any, error) { return nil, errors.New("boom") }

	// Three failures should open the breaker.
	for i := 0; i < 3; i++ {
		_, _ = br.Execute(context.Background(), failing)
	}

	// Fourth call should short-circuit with ErrCircuitOpen.
	_, err := br.Execute(context.Background(), failing)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("err = %v, want ErrCircuitOpen", err)
	}
}

func TestBreaker_HalfOpenRecovers(t *testing.T) {
	p, _ := NewPolicy("svc",
		WithBreakerThreshold(2),
		WithBreakerCooldown(20*time.Millisecond),
		WithBreakerHalfOpenMax(1),
	)
	br := NewBreaker(p)

	failing := func(context.Context) (any, error) { return nil, errors.New("boom") }
	succ := func(context.Context) (any, error) { return "ok", nil }

	// Open the breaker.
	for i := 0; i < 2; i++ {
		_, _ = br.Execute(context.Background(), failing)
	}
	if _, err := br.Execute(context.Background(), succ); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen while open, got %v", err)
	}

	// Wait for cooldown.
	time.Sleep(30 * time.Millisecond)

	// Half-open probe succeeds → breaker closes.
	if _, err := br.Execute(context.Background(), succ); err != nil {
		t.Fatalf("half-open probe err = %v, want nil", err)
	}

	// Subsequent call should pass.
	if _, err := br.Execute(context.Background(), succ); err != nil {
		t.Fatalf("post-recovery err = %v, want nil", err)
	}
}

func TestBreaker_RespectsContextCancellation(t *testing.T) {
	p, _ := NewPolicy("svc")
	br := NewBreaker(p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	_, err := br.Execute(ctx, func(context.Context) (any, error) {
		called = true
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if called {
		t.Errorf("function called despite cancelled context")
	}
}
```

- [ ] **Step 2: Run the test — expect compile failure**

```bash
go test ./internal/resilience/... -run TestBreaker
```

Expected: build error (`undefined: NewBreaker`, `undefined: ErrCircuitOpen`).

- [ ] **Step 3: Implement `breaker.go`**

Create `internal/resilience/breaker.go`:

```go
package resilience

import (
	"context"
	"errors"

	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ErrCircuitOpen is returned by a Breaker when the underlying state machine
// is Open (or in Half-Open with the probe quota exhausted). Callers should
// surface this as HTTP 503 with Retry-After.
var ErrCircuitOpen = errors.New("resilience: circuit breaker open")

// Breaker is a thin adapter over sony/gobreaker that maps its sentinel
// errors to ErrCircuitOpen and respects context cancellation up front.
type Breaker struct {
	cb *gobreaker.CircuitBreaker[any]
}

// BreakerOption configures NewBreaker. All options are optional; defaults
// match the upstream gobreaker behaviour with our error classification.
type BreakerOption func(*breakerOpts)

type breakerOpts struct {
	isSuccessful func(error) bool
	meter        metric.Meter
	tracer       trace.Tracer
}

// WithIsSuccessful classifies errors that should NOT count as breaker
// failures (e.g. gRPC InvalidArgument is a caller bug, not a server fault).
// The error still propagates to the caller; the breaker just ignores it.
func WithIsSuccessful(fn func(error) bool) BreakerOption {
	return func(o *breakerOpts) { o.isSuccessful = fn }
}

// WithObservability wires gobreaker's OnStateChange callback to OTel.
// Emits a Gauge `resilience.breaker.state` (0=Closed, 1=HalfOpen, 2=Open)
// labeled by policy name, plus a span event `resilience.breaker.transition`
// on every transition. Both meter and tracer must be non-nil; if either
// is nil this option is a no-op.
func WithObservability(meter metric.Meter, tracer trace.Tracer) BreakerOption {
	return func(o *breakerOpts) {
		if meter != nil && tracer != nil {
			o.meter = meter
			o.tracer = tracer
		}
	}
}

// NewBreaker constructs a Breaker from a validated Policy.
func NewBreaker(p *Policy, opts ...BreakerOption) *Breaker {
	o := &breakerOpts{}
	for _, opt := range opts {
		opt(o)
	}

	settings := gobreaker.Settings{
		Name:        p.Name,
		MaxRequests: p.BreakerHalfOpenMax,
		Interval:    0, // 0 = never reset counts in Closed state
		Timeout:     p.BreakerCooldown,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= p.BreakerThreshold
		},
	}
	if o.isSuccessful != nil {
		settings.IsSuccessful = o.isSuccessful
	}
	if o.meter != nil {
		// Build a current-state holder closed over by both the gauge
		// observer and the OnStateChange callback.
		var currentState gobreaker.State
		gauge, err := o.meter.Int64ObservableGauge("resilience.breaker.state",
			metric.WithDescription("Circuit breaker state: 0=Closed, 1=HalfOpen, 2=Open"))
		if err == nil {
			_, _ = o.meter.RegisterCallback(func(_ context.Context, obs metric.Observer) error {
				obs.ObserveInt64(gauge, int64(currentState),
					metric.WithAttributes(attribute.String("policy_name", p.Name)))
				return nil
			}, gauge)
		}
		settings.OnStateChange = func(name string, from, to gobreaker.State) {
			currentState = to
			_, span := o.tracer.Start(context.Background(), "resilience.breaker.transition")
			span.SetAttributes(
				attribute.String("policy_name", name),
				attribute.String("from", from.String()),
				attribute.String("to", to.String()),
			)
			span.End()
		}
	}

	return &Breaker{cb: gobreaker.NewCircuitBreaker[any](settings)}
}

// Execute runs fn through the breaker. It checks ctx cancellation first
// to avoid charging the breaker for caller-side cancellations.
func (b *Breaker) Execute(ctx context.Context, fn func(context.Context) (any, error)) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out, err := b.cb.Execute(func() (any, error) { return fn(ctx) })
	switch {
	case errors.Is(err, gobreaker.ErrOpenState),
		errors.Is(err, gobreaker.ErrTooManyRequests):
		return nil, ErrCircuitOpen
	}
	return out, err
}
```

Add at least one new test to `breaker_test.go` covering the `WithIsSuccessful` predicate:

```go
func TestBreaker_IsSuccessfulPredicate(t *testing.T) {
	p, _ := NewPolicy("svc", WithBreakerThreshold(2))
	br := NewBreaker(p, WithIsSuccessful(func(err error) bool {
		return errors.Is(err, errCallerFault)
	}))

	callerFail := func(context.Context) (any, error) { return nil, errCallerFault }

	// Five caller-classified failures must NOT trip the breaker.
	for i := 0; i < 5; i++ {
		_, err := br.Execute(context.Background(), callerFail)
		if !errors.Is(err, errCallerFault) {
			t.Fatalf("err = %v, want errCallerFault to propagate", err)
		}
	}

	// Verify the breaker is still closed: a real failure flips it.
	realFail := func(context.Context) (any, error) { return nil, errors.New("server boom") }
	for i := 0; i < 2; i++ {
		_, _ = br.Execute(context.Background(), realFail)
	}
	if _, err := br.Execute(context.Background(), realFail); !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("breaker did not open after 2 server failures; err = %v", err)
	}
}

var errCallerFault = errors.New("caller fault")
```

(OTel observability is exercised end-to-end in Task 13's integration tests via the metric reader; a unit test would mostly assert plumbing.)

- [ ] **Step 4: Run the test — expect pass**

```bash
go test ./internal/resilience/... -run TestBreaker -v
```

Expected: PASS for all three subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/resilience/breaker.go internal/resilience/breaker_test.go
git commit -m "feat(resilience): add Breaker adapter over sony/gobreaker"
```

---

## Task 4: `resilience` gRPC interceptor (depends on Tasks 2, 3)

**Files:**
- Create: `internal/resilience/grpc.go`
- Test: `internal/resilience/grpc_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/resilience/grpc_test.go`:

```go
package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeInvoker struct {
	calls int
	err   error
	delay time.Duration
}

func (f *fakeInvoker) invoke(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
	f.calls++
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.err
}

func newGRPCBreaker(p *Policy) *Breaker {
	return NewBreaker(p, WithIsSuccessful(IsGRPCCallerError))
}

func TestUnaryClientInterceptor_AppliesTimeout(t *testing.T) {
	p, _ := NewPolicy("svc", WithTimeout(20*time.Millisecond))
	icpt := UnaryClientInterceptor(p, newGRPCBreaker(p))

	inv := &fakeInvoker{delay: 100 * time.Millisecond}
	err := icpt(context.Background(), "/svc/Method", nil, nil, nil, inv.invoke)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestUnaryClientInterceptor_OpensBreakerOnUnavailable(t *testing.T) {
	p, _ := NewPolicy("svc",
		WithTimeout(1*time.Second),
		WithBreakerThreshold(2),
		WithBreakerCooldown(100*time.Millisecond),
	)
	br := newGRPCBreaker(p)
	icpt := UnaryClientInterceptor(p, br)

	inv := &fakeInvoker{err: status.Error(codes.Unavailable, "down")}

	// Two UNAVAILABLE failures should open the breaker.
	for i := 0; i < 2; i++ {
		_ = icpt(context.Background(), "/svc/Method", nil, nil, nil, inv.invoke)
	}
	// Third call short-circuits without invoking.
	preCalls := inv.calls
	err := icpt(context.Background(), "/svc/Method", nil, nil, nil, inv.invoke)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("err = %v, want ErrCircuitOpen", err)
	}
	if inv.calls != preCalls {
		t.Errorf("invoker called %d times after open; want 0", inv.calls-preCalls)
	}
}

func TestUnaryClientInterceptor_CallerErrorsDoNotTrip(t *testing.T) {
	p, _ := NewPolicy("svc", WithBreakerThreshold(2))
	br := newGRPCBreaker(p)
	icpt := UnaryClientInterceptor(p, br)

	inv := &fakeInvoker{err: status.Error(codes.InvalidArgument, "bad")}

	// Five caller errors must NOT open the breaker.
	for i := 0; i < 5; i++ {
		_ = icpt(context.Background(), "/svc/Method", nil, nil, nil, inv.invoke)
	}

	// A subsequent call should still be invoked (not short-circuited).
	preCalls := inv.calls
	_ = icpt(context.Background(), "/svc/Method", nil, nil, nil, inv.invoke)
	if inv.calls == preCalls {
		t.Errorf("breaker tripped on caller errors")
	}
}

// Regression: caller errors must propagate to the caller verbatim.
// The original (pre-grilling) interceptor returned nil to the caller for
// caller-classified errors, which would cause handlers to dereference a
// nil reply and panic.
func TestUnaryClientInterceptor_CallerErrorsPropagate(t *testing.T) {
	p, _ := NewPolicy("svc")
	br := newGRPCBreaker(p)
	icpt := UnaryClientInterceptor(p, br)

	want := status.Error(codes.InvalidArgument, "bad input")
	inv := &fakeInvoker{err: want}

	got := icpt(context.Background(), "/svc/Method", nil, nil, nil, inv.invoke)
	if got == nil {
		t.Fatal("interceptor returned nil; want the caller error to propagate")
	}
	if status.Code(got) != codes.InvalidArgument {
		t.Errorf("propagated code = %v, want InvalidArgument", status.Code(got))
	}
}
```

- [ ] **Step 2: Run the test — expect compile failure**

```bash
go test ./internal/resilience/... -run TestUnaryClientInterceptor
```

Expected: `undefined: UnaryClientInterceptor`.

- [ ] **Step 3: Implement `grpc.go`**

Create `internal/resilience/grpc.go`:

```go
package resilience

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsGRPCCallerError reports whether err is a gRPC status that represents
// a caller-side fault (bad input, missing resource, missing auth) rather
// than a server-side failure. Pass this to NewBreaker via WithIsSuccessful
// so caller errors propagate to the caller AND don't trip the breaker.
//
// Codes treated as caller errors:
//   InvalidArgument, NotFound, AlreadyExists, PermissionDenied,
//   Unauthenticated, FailedPrecondition, OutOfRange.
//
// Codes counted as breaker failures (caller WILL see them too):
//   Unavailable, DeadlineExceeded, Internal, ResourceExhausted, Unknown,
//   Aborted, Canceled, DataLoss.
func IsGRPCCallerError(err error) bool {
	if err == nil {
		return true // nil counts as success for IsSuccessful
	}
	switch status.Code(err) {
	case codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.OutOfRange:
		return true
	}
	return false
}

// UnaryClientInterceptor returns a gRPC unary client interceptor that:
//
//   - Applies policy.Timeout to each call (only if no shorter deadline is set).
//   - Routes the call through the breaker.
//   - Returns the invoker's error verbatim — the breaker decides via its
//     IsSuccessful predicate (set in NewBreaker via WithIsSuccessful) whether
//     the error counts toward the failure tally. Caller errors propagate
//     to handlers AND don't trip the breaker.
func UnaryClientInterceptor(p *Policy, br *Breaker) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		callCtx, cancel := withTimeoutIfShorter(ctx, p.Timeout)
		defer cancel()

		_, err := br.Execute(callCtx, func(c context.Context) (any, error) {
			return nil, invoker(c, method, req, reply, cc, opts...)
		})
		return err
	}
}

func withTimeoutIfShorter(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if dl, ok := ctx.Deadline(); ok && time.Until(dl) <= d {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}
```

The breaker constructed with `NewBreaker(policy, WithIsSuccessful(IsGRPCCallerError))` calls `IsGRPCCallerError(err)` for every result; if it returns true, the call is counted as success for breaker accounting and the error still flows back through `br.Execute` to the interceptor and the caller.

- [ ] **Step 4: Run the test — expect pass**

```bash
go test ./internal/resilience/... -run TestUnaryClientInterceptor -v
```

Expected: PASS for all three subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/resilience/grpc.go internal/resilience/grpc_test.go
git commit -m "feat(resilience): add UnaryClientInterceptor with caller-error filtering"
```

---

## Task 5: ~~`resilience` HTTP factory~~ — DROPPED

**Status: removed in 2026-05-11 grilling revision.**

The original Task 5 shipped `internal/resilience/http.go` (`HTTPClient`, `HTTPClientWithBreaker`, `breakerTransport`) with no callers — the spec explicitly defers the outbound-HTTP audit to a follow-up, and no task in this plan modifies any HTTP-calling site. That meant ~150 LOC of dead code on merge, plus a non-trivial bug-prone branch in `breakerTransport.RoundTrip` (the success-with-5xx-response case where the wrapped fn returns `(resp, syntheticErr)` and the code type-asserts back out) verified against zero real callers.

The HTTP factory will be designed and shipped with the follow-up audit, where a real caller (LiveKit, SeaweedFS, or Razorpay) shapes the contract.

Skip to **Task 6** below. Task numbers 6–15 unchanged; only Task 5 is removed.

<details>
<summary>Original Task 5 content (for archival reference; do not implement)</summary>

- [ ] **Step 1: Write the failing test**

Create `internal/resilience/http_test.go`:

```go
package resilience

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClient_AppliesTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p, _ := NewPolicy("svc", WithTimeout(20*time.Millisecond))
	c := HTTPClient(p)

	resp, err := c.Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHTTPClient_BreakerOpensOn5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _ := NewPolicy("svc",
		WithTimeout(500*time.Millisecond),
		WithBreakerThreshold(2),
		WithBreakerCooldown(50*time.Millisecond),
	)
	c := HTTPClientWithBreaker(p, NewBreaker(p))

	for i := 0; i < 2; i++ {
		resp, _ := c.Get(srv.URL)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	_, err := c.Get(srv.URL)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("err = %v, want ErrCircuitOpen", err)
	}
}
```

- [ ] **Step 2: Run the test — expect compile failure**

```bash
go test ./internal/resilience/... -run TestHTTPClient
```

Expected: `undefined: HTTPClient`, `undefined: HTTPClientWithBreaker`.

- [ ] **Step 3: Implement `http.go`**

Create `internal/resilience/http.go`:

```go
package resilience

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HTTPClient returns an *http.Client configured with the policy timeout
// and a transport with sensible per-stage timeouts. Use this for any
// outbound HTTP call (LiveKit, SeaweedFS, third-party APIs).
func HTTPClient(p *Policy) *http.Client {
	return &http.Client{
		Timeout:   p.Timeout,
		Transport: defaultTransport(),
	}
}

// HTTPClientWithBreaker wraps HTTPClient's transport with a breaker-aware
// RoundTripper. 5xx responses count toward breaker failures.
func HTTPClientWithBreaker(p *Policy, br *Breaker) *http.Client {
	return &http.Client{
		Timeout: p.Timeout,
		Transport: &breakerTransport{
			next:    defaultTransport(),
			breaker: br,
		},
	}
}

func defaultTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
	}
}

type breakerTransport struct {
	next    http.RoundTripper
	breaker *Breaker
}

func (t *breakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	out, err := t.breaker.Execute(req.Context(), func(_ context.Context) (any, error) {
		resp, rerr := t.next.RoundTrip(req)
		if rerr != nil {
			return nil, rerr
		}
		if resp.StatusCode >= 500 {
			return resp, fmt.Errorf("upstream %d", resp.StatusCode)
		}
		return resp, nil
	})
	if errors.Is(err, ErrCircuitOpen) {
		return nil, err
	}
	if err != nil {
		// breakerTransport.Execute may return a non-nil response alongside
		// the synthetic 5xx error; surface that response.
		if r, ok := out.(*http.Response); ok && r != nil {
			return r, nil
		}
		return nil, err
	}
	resp, _ := out.(*http.Response)
	return resp, nil
}
```

Add `"context"` to the import block alongside `errors`, `fmt`, `net`, `net/http`, `time`.

- [ ] **Step 4: Run the test — expect pass**

```bash
go test ./internal/resilience/... -run TestHTTPClient -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/resilience/http.go internal/resilience/http_test.go
git commit -m "feat(resilience): add HTTPClient + breaker-aware RoundTripper"
```

</details>

---

## Task 6: `middleware.Deadline` Gin middleware (parallel-safe)

**Files:**
- Create: `internal/middleware/deadline.go`
- Test: `internal/middleware/deadline_test.go`

Disjoint from all other Phase 1 tasks.

- [ ] **Step 1: Write the failing test**

Create `internal/middleware/deadline_test.go`:

```go
package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestDeadline_AppliesTimeoutToRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Deadline(50 * time.Millisecond))
	r.GET("/", func(c *gin.Context) {
		dl, ok := c.Request.Context().Deadline()
		if !ok {
			t.Errorf("request ctx has no deadline")
		}
		if remaining := time.Until(dl); remaining > 60*time.Millisecond {
			t.Errorf("deadline too far away: %v", remaining)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

func TestDeadline_PropagatesCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Deadline(20 * time.Millisecond))

	var observed error
	r.GET("/", func(c *gin.Context) {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-c.Request.Context().Done():
			observed = c.Request.Context().Err()
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !errors.Is(observed, context.DeadlineExceeded) {
		t.Errorf("ctx err = %v, want DeadlineExceeded", observed)
	}
}
```

- [ ] **Step 2: Run the test — expect compile failure**

```bash
go test ./internal/middleware/... -run TestDeadline
```

Expected: `undefined: Deadline`.

- [ ] **Step 3: Implement `deadline.go`**

Create `internal/middleware/deadline.go`:

```go
package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Deadline returns a Gin middleware that wraps the request context
// in context.WithTimeout(d). Apply at the route group level so each
// group can have its own budget.
func Deadline(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
```

- [ ] **Step 4: Run the test — expect pass**

```bash
go test ./internal/middleware/... -run TestDeadline -v
```

Expected: PASS for both subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/middleware/deadline.go internal/middleware/deadline_test.go
git commit -m "feat(middleware): add Deadline middleware for per-route ctx budgets"
```

---

## Task 7: Config — add timeout knobs to `ServerConfig` (parallel-safe)

**Files:**
- Modify: `internal/config/config.go` (extend `ServerConfig` struct + viper bindings)

Owns only the diff to `ServerConfig` — disjoint from all other Phase 1 tasks.

- [ ] **Step 1: Read current ServerConfig**

Open `internal/config/config.go`, locate `type ServerConfig struct` (around line 232 per survey).

- [ ] **Step 2: Add timeout fields**

Append the following fields to `ServerConfig` (after the existing fields, before the closing brace):

```go
	// HTTP server timeouts. Zero means "use the http.Server zero value",
	// which disables the timeout — set explicit values in production.
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`

	// AI worker resilience knobs.
	AIWorkerTimeout          time.Duration `mapstructure:"ai_worker_timeout"`
	AIWorkerBreakerThreshold uint32        `mapstructure:"ai_worker_breaker_threshold"`
	AIWorkerBreakerCooldown  time.Duration `mapstructure:"ai_worker_breaker_cooldown"`
```

- [ ] **Step 3: Add viper defaults + env bindings**

Locate the function that sets viper defaults / `BindEnv` (search for an existing line like `v.BindEnv("server.port"`). Add:

```go
	v.SetDefault("server.read_header_timeout", "5s")
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "60s")
	v.SetDefault("server.idle_timeout", "120s")
	v.SetDefault("server.ai_worker_timeout", "5s")
	v.SetDefault("server.ai_worker_breaker_threshold", 5)
	v.SetDefault("server.ai_worker_breaker_cooldown", "30s")

	_ = v.BindEnv("server.read_header_timeout", "RAVEN_HTTP_READ_HEADER_TIMEOUT")
	_ = v.BindEnv("server.read_timeout", "RAVEN_HTTP_READ_TIMEOUT")
	_ = v.BindEnv("server.write_timeout", "RAVEN_HTTP_WRITE_TIMEOUT")
	_ = v.BindEnv("server.idle_timeout", "RAVEN_HTTP_IDLE_TIMEOUT")
	_ = v.BindEnv("server.ai_worker_timeout", "RAVEN_AI_WORKER_TIMEOUT")
	_ = v.BindEnv("server.ai_worker_breaker_threshold", "RAVEN_AI_WORKER_BREAKER_THRESHOLD")
	_ = v.BindEnv("server.ai_worker_breaker_cooldown", "RAVEN_AI_WORKER_BREAKER_COOLDOWN")
```

If `time` is not already imported in `config.go`, add it.

- [ ] **Step 4: Verify the project still builds**

```bash
go build ./...
go test ./internal/config/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add HTTP server + AI worker resilience knobs to ServerConfig"
```

---

## Task 8: `.golangci.yml` — enable `noctx` + `contextcheck` (parallel-safe)

**Files:**
- Modify: `.golangci.yml`

Disjoint from all Go source files.

**Scope decision (2026-05-11 grilling):** apply globally to ALL production Go, not the spec's narrower `internal/grpc + internal/handler + internal/service` carve-out. Verified blast radius locally:

```
golangci-lint run --no-config --default=none -E noctx -E contextcheck \
  ./internal/handler/... ./internal/service/... ./internal/grpc/... ./internal/jobs/...
# EXIT=0 for every package
```

The codebase is already clean. The wider scope catches future regressions in `cmd/api/`, `internal/middleware/`, `internal/repository/`, and `pkg/` (where `apierror` is about to start importing `internal/resilience`).

- [ ] **Step 1: Read current `.golangci.yml`**

Current state from the actual file (NOT the simplified survey listing):

```yaml
version: "2"

run:
  timeout: 5m
  modules-download-mode: readonly

linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - ineffassign
    - revive
  settings:
    revive:
      rules:
        - name: exported
        - name: blank-imports
        # ... etc.

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

- [ ] **Step 2: Add the two new linters globally**

Append to the `linters.enable` list:

```yaml
    - noctx
    - contextcheck
```

No `issues.exclude-rules` needed — tests are excluded by golangci-lint's defaults; production Go is uniformly clean today.

- [ ] **Step 3: Verify lint passes across the whole repo**

```bash
golangci-lint run ./...
```

Expected: zero new findings. If anything trips, fix the call site rather than adding a `//nolint` — the audit confirmed there's nothing to suppress.

- [ ] **Step 4: Commit**

```bash
git add .golangci.yml
git commit -m "ci(lint): enable noctx and contextcheck repo-wide for resilience compliance gate"
```

---

## Task 9: Refactor `internal/grpc/client.go` to accept `*resilience.Policy`

**Depends on Tasks 2, 3, 4.**

**Files:**
- Modify: `internal/grpc/client.go`
- Modify: `internal/grpc/client_test.go`

- [ ] **Step 1: Write/extend the failing test**

Open `internal/grpc/client_test.go` and add (or replace existing constructor test with) the following:

```go
func TestNewClient_AppliesResilienceInterceptor(t *testing.T) {
	p, _ := resilience.NewPolicy("ai-worker", resilience.WithTimeout(50*time.Millisecond))
	br := resilience.NewBreaker(p)

	// We can't easily dial a real server in a unit test; just assert
	// NewClient accepts the policy and returns no error for a syntactically
	// valid address. Connection establishment is lazy in grpc.NewClient.
	c, err := NewClient("passthrough:///localhost:1", p, br)
	if err != nil {
		t.Fatalf("NewClient err = %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if c.Worker() == nil {
		t.Errorf("Worker() returned nil")
	}
}
```

Add the necessary import:

```go
import (
	"testing"
	"time"

	"github.com/ravencloak-org/Raven/internal/resilience"
)
```

(Adjust the module path to match `go.mod`.)

- [ ] **Step 2: Run the test — expect compile failure**

```bash
go test ./internal/grpc/...
```

Expected: build failure (`NewClient takes 1 arg, got 3`).

- [ ] **Step 3: Update `client.go` to accept Policy + Breaker**

Replace `internal/grpc/client.go` with:

```go
// Package grpc provides a gRPC client for communicating with the Python AI worker.
package grpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
	"github.com/ravencloak-org/Raven/internal/resilience"
)

// Client wraps a gRPC connection and exposes the AIWorker service stub.
type Client struct {
	conn   *grpc.ClientConn
	worker pb.AIWorkerClient
}

// NewClient dials the AI worker at addr and returns a ready-to-use Client.
// The unary interceptor wires policy.Timeout and the breaker around every call.
func NewClient(addr string, policy *resilience.Policy, breaker *resilience.Breaker) (*Client, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(resilience.UnaryClientInterceptor(policy, breaker)),
	)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, worker: pb.NewAIWorkerClient(conn)}, nil
}

// Worker returns the AIWorkerClient stub for making RPC calls.
func (c *Client) Worker() pb.AIWorkerClient { return c.worker }

// Close releases the underlying gRPC connection.
func (c *Client) Close() error { return c.conn.Close() }
```

(Confirm the module path `github.com/ravencloak-org/Raven` matches `go.mod` — adjust if different.)

- [ ] **Step 4: Run the test — expect pass**

```bash
go test ./internal/grpc/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/grpc/client.go internal/grpc/client_test.go
git commit -m "feat(grpc): wire resilience.Policy + Breaker into AI worker client"
```

---

## Task 10: Map `ErrCircuitOpen` → HTTP 503 in `pkg/apierror`

**Depends on Task 3.**

**Files:**
- Modify: `pkg/apierror/apierror.go` — extend the existing `ErrorHandler()` middleware.
- Modify: `pkg/apierror/apierror_test.go` — add the new case.

**Note from grilling (2026-05-11):** the spec said `internal/apierror/`. The package actually lives at `pkg/apierror/` and an `ErrorHandler()` already exists (line 100). Its dispatch shape is **type assertion**, not `errors.Is`:

```go
if quotaErr, ok := err.(*QuotaError); ok { c.JSON(quotaErr.Code, quotaErr) }
else if appErr, ok := err.(*AppError); ok { c.JSON(appErr.Code, appErr) }
else { /* 500 fallback */ }
```

We add an `errors.Is` branch BEFORE the type assertions so `ErrCircuitOpen` is routed to a 503 response with `Retry-After` set as a real HTTP header (not a JSON field). `pkg/apierror` will import `internal/resilience` — Go's `internal` rule allows this because `pkg/` is a sibling of `internal/` under the module root.

- [ ] **Step 1: Update `pkg/apierror/apierror.go`**

Add to the import block:

```go
import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/resilience"
)
```

(Adjust the module path to match `go.mod` if different.)

Replace the body of `ErrorHandler` so the resilience branch fires first:

```go
// ErrorHandler is a Gin middleware that catches errors set via c.Error()
// and returns a JSON error response.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err

		// Resilience errors map to HTTP transport semantics, not the
		// AppError type assertions below.
		if errors.Is(err, resilience.ErrCircuitOpen) {
			// Retry-After in seconds. The breaker cooldown is a Policy
			// field, but pkg/apierror doesn't see the policy here, so
			// we use a sane default that matches the AI worker policy
			// default (30s). If a more precise value is needed later,
			// thread it via a typed error that carries the cooldown.
			c.Header("Retry-After", strconv.Itoa(30))
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, &AppError{
				Code:    http.StatusServiceUnavailable,
				Message: "Service Unavailable",
				Detail:  "upstream temporarily unavailable; circuit breaker open",
			})
			return
		}

		if quotaErr, ok := err.(*QuotaError); ok {
			c.JSON(quotaErr.Code, quotaErr)
		} else if appErr, ok := err.(*AppError); ok {
			c.JSON(appErr.Code, appErr)
		} else {
			c.JSON(http.StatusInternalServerError, &AppError{
				Code:    http.StatusInternalServerError,
				Message: "Internal Server Error",
				Detail:  err.Error(),
			})
		}
	}
}
```

- [ ] **Step 2: Add a unit test**

Add to `pkg/apierror/apierror_test.go`:

```go
func TestErrorHandler_CircuitOpenReturns503WithRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.GET("/", func(c *gin.Context) {
		_ = c.Error(resilience.ErrCircuitOpen)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Errorf("missing Retry-After header")
	}
}
```

Plus the matching imports.

- [ ] **Step 3: Run the test**

```bash
go test ./pkg/apierror/... -v
```

Expected: PASS, including the existing tests.

- [ ] **Step 4: Commit**

```bash
git add pkg/apierror/
git commit -m "feat(apierror): map resilience.ErrCircuitOpen to 503 with Retry-After"
```

---

## Task 11: Wire everything in `cmd/api/main.go`

**Depends on Tasks 6, 7, 9, 10.** **Single owner — must run sequentially.**

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Update gRPC client construction (around line 313)**

Replace:

```go
grpcClient, err := rpcClient.NewClient(cfg.GRPC.WorkerAddr)
```

With:

```go
aiPolicy, err := resilience.NewPolicy("ai-worker",
    resilience.WithTimeout(cfg.Server.AIWorkerTimeout),
    resilience.WithBreakerThreshold(cfg.Server.AIWorkerBreakerThreshold),
    resilience.WithBreakerCooldown(cfg.Server.AIWorkerBreakerCooldown),
)
if err != nil {
    log.Fatalf("invalid AI worker resilience policy: %v", err)
}
aiBreaker := resilience.NewBreaker(aiPolicy,
    resilience.WithIsSuccessful(resilience.IsGRPCCallerError),
    resilience.WithObservability(meter, tracer),
)

grpcClient, err := rpcClient.NewClient(cfg.GRPC.WorkerAddr, aiPolicy, aiBreaker)
```

Where `meter` and `tracer` are the OTel `metric.Meter` and `trace.Tracer` already constructed elsewhere in `main.go` (see Task 1 step 5 survey output for their names — search for `otel.Meter(` / `otel.Tracer(`). If they're not yet in scope at line 313, hoist their construction up.

Add the import: `"github.com/ravencloak-org/Raven/internal/resilience"` (adjust module path to match `go.mod`).

- [ ] **Step 2: Apply Deadline middleware per route group**

The original spec table referenced routes that don't exist (`/readyz`, `/api/v1/upload/*`, `/api/v1/voice/*`). The corrected mapping, anchored to actual line numbers in `cmd/api/main.go`:

| Real route / group (line)                                                    | Budget | Notes                                                      |
|------------------------------------------------------------------------------|--------|------------------------------------------------------------|
| `router.GET("/healthz", ...)` (line 496)                                     | 1s     | Standalone — apply via per-route middleware list           |
| `chatAPI := router.Group("/api/v1/chat")` (line 794)                         | 30s    | Streaming chat                                             |
| `voice := api.Group("/orgs/:org_id/voice-sessions")` (line 717)              | 30s    | Voice session endpoints — applied at this nested group     |
| `doc := kb.Group("/:kb_id/documents")` (line 586)                            | 60s    | Document upload-shaped endpoints                           |
| `src := kb.Group("/:kb_id/sources")` (line 576)                              | 60s    | Source ingestion (large payloads)                          |
| `api := router.Group("/api/v1")` (line 517)                                  | 10s    | Default for everything not overridden above                |
| `admin := router.Group("/api/v1/admin")` (line 825)                          | 10s    | Admin CRUD                                                 |
| `router.POST("/api/v1/billing/webhook", ...)` (line 817)                     | 10s    | Stripe/Razorpay webhook receiver                           |
| `router.GET("/webhooks/meta", ...)` / `POST` (lines 821–822)                 | 10s    | Meta WhatsApp webhook                                      |
| `router.GET("/api/v1/auth/callback", ...)` (line 841)                        | 10s    | OAuth callback                                             |
| `router.GET/POST("/api/v1/notifications/unsubscribe", ...)` (lines 501–502)  | 10s    | Unsubscribe link handler                                   |

Apply order: outer `api` group gets the 10s default first (line 517 area), then the inner `voice`, `doc`, `src` groups override with their longer budgets at their respective definitions. Since `context.WithTimeout` only shortens, an inner `Deadline(60s)` on a route already wrapped in `Deadline(10s)` would actually clamp at 10s. To work around this:

- Define the inner groups (`doc`, `src`, `voice`) BEFORE applying `api.Use(middleware.Deadline(10*time.Second))`, OR
- Skip the outer-`api` Deadline and apply per-leaf-group budgets explicitly.

Recommended approach: skip the outer-`api` Deadline; apply Deadline per-leaf-group:

```go
// Inside api := router.Group("/api/v1"):
//   - Each leaf endpoint group gets its own Deadline.
//   - No outer api.Use(Deadline(...)) — would clamp inner overrides.

// Standalone routes get the middleware in their per-route handler chain:
router.GET("/healthz", middleware.Deadline(1*time.Second), handler.HealthCheck)
router.POST("/api/v1/billing/webhook", middleware.Deadline(10*time.Second), billingHandler.Webhook)
router.GET("/webhooks/meta", middleware.Deadline(10*time.Second), metaWebhookHandler.VerifyWebhook)
router.POST("/webhooks/meta", middleware.Deadline(10*time.Second), metaWebhookHandler.HandleEvent)
router.GET("/api/v1/auth/callback", middleware.Deadline(10*time.Second), authCallback)
router.GET("/api/v1/notifications/unsubscribe", middleware.Deadline(10*time.Second), notifPrefsHandler.Unsubscribe)
router.POST("/api/v1/notifications/unsubscribe", middleware.Deadline(10*time.Second), notifPrefsHandler.UnsubscribePost)

// Inner leaf groups get their tailored budgets:
chatAPI := router.Group("/api/v1/chat")
chatAPI.Use(middleware.Deadline(30 * time.Second))

voice := api.Group("/orgs/:org_id/voice-sessions")
voice.Use(middleware.Deadline(30 * time.Second))

doc := kb.Group("/:kb_id/documents")
doc.Use(middleware.Deadline(60 * time.Second))

src := kb.Group("/:kb_id/sources")
src.Use(middleware.Deadline(60 * time.Second))
```

For all OTHER endpoints under `api` (CRUD-style: `webhooks`, `leads`, `org`, `billing`, `connectors`, `llm-providers`, etc.), accept that they have NO per-route Gin deadline and rely instead on the `http.Server.WriteTimeout` (60s in step 3) as the upper bound. This is a conscious trade-off — the spec's "default 10s on all of `/api/v1/*`" would have required restructuring how nested groups attach Deadline, and the inheritance vs. shortening semantics make a single outer Deadline brittle. The 60s `WriteTimeout` is the safety net.

- [ ] **Step 3: Set explicit `http.Server` timeouts (around line 855)**

Replace the existing struct literal:

```go
srv := &http.Server{
    // ... existing fields ...
}
```

With:

```go
srv := &http.Server{
    Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
    Handler:           router,
    ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
    ReadTimeout:       cfg.Server.ReadTimeout,
    WriteTimeout:      cfg.Server.WriteTimeout,
    IdleTimeout:       cfg.Server.IdleTimeout,
}
```

(Keep any existing fields like `ErrorLog`, `BaseContext`, etc. — only add the timeout fields.)

- [ ] **Step 4: Build and run the existing test suite**

```bash
go build ./...
go test ./cmd/... ./internal/...
```

Expected: PASS. If any test fails because of changed Gin route deadlines (e.g., tests that intentionally hold a request longer than the default 10s), update the test or scope the Deadline middleware to skip the test route — do not remove the middleware.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat(api): wire resilience policy + http.Server timeouts + per-route deadlines"
```

---

## Task 12: Asynq per-task deadlines via `mux.Use` middleware (parallel-safe with Task 13)

**Depends on Task 1.** Owns `internal/jobs/scheduler.go` (where the mux is built), a new `internal/jobs/deadline.go` (the middleware + budget table), and `internal/jobs/airbyte_sync.go` (one independent bug fix).

**Why this shape (decided in 2026-05-11 grilling):** the original per-handler `ProcessTask` wrapping pattern would have missed 4 of the 9 handlers in `internal/jobs/`:

| File | Shape | Caught by original grep? |
|---|---|---|
| `voice_usage.go`, `usage.go`, `cleanup.go`, `recrawl.go`, `webhook_delivery.go` | `func (h *X) ProcessTask(ctx context.Context, ...)` | ✅ |
| `airbyte_sync.go` | `func (h *X) ProcessTask(_ context.Context, ...)` (drops ctx) | ❌ — also has independent ctx-drop bug |
| `document_process.go`, `email_summary.go`, `send_email.go` | `func HandleX(deps) asynq.HandlerFunc` (returns a closure) | ❌ |

A single `asynq.MiddlewareFunc` registered via `mux.Use(...)` wraps every handler regardless of shape, plus catches future handlers automatically. A small per-task-type budget table preserves the spec's per-handler granularity without 9 file edits.

- [ ] **Step 1: Add `internal/jobs/deadline.go`**

Create the file:

```go
package jobs

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
)

// taskBudgets maps a task type to its per-execution deadline.
// Keys come from the TypeXxx constants in tasks.go. Anything not in
// the map gets defaultBudget. Tune empirically.
var taskBudgets = map[string]time.Duration{
	TypeDocumentProcess:       5 * time.Minute,
	TypeEmailSummary:          2 * time.Minute,
	TypeSendEmail:             30 * time.Second,
	TypeVoiceUsageAggregation: 30 * time.Second,
	TypeUsageAggregation:      30 * time.Second,
	TypeWebhookDelivery:       30 * time.Second,
	TypeRecrawlSources:        2 * time.Minute,
	TypeCleanupSessions:       2 * time.Minute,
	TypeAirbyteSync:           5 * time.Minute,
}

const defaultBudget = 1 * time.Minute

// DeadlineMiddleware wraps every Asynq handler with a per-task-type
// context.WithTimeout. Apply via mux.Use(DeadlineMiddleware) so it
// covers handlers regardless of whether they're method receivers or
// HandlerFunc factories.
func DeadlineMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		budget, ok := taskBudgets[t.Type()]
		if !ok {
			budget = defaultBudget
		}
		ctx, cancel := context.WithTimeout(ctx, budget)
		defer cancel()
		return next.ProcessTask(ctx, t)
	})
}
```

If any of the `TypeXxx` constants in the budget map don't exist yet (check `internal/jobs/tasks.go` and the per-handler files), use the literal string instead and leave a `// TODO: hoist to a TypeXxx constant` comment.

- [ ] **Step 2: Wire the middleware in `scheduler.go`**

After `mux := asynq.NewServeMux()` (around line 103) and before the `mux.Handle(...)` calls:

```go
mux.Use(DeadlineMiddleware)
```

- [ ] **Step 3: Fix `airbyte_sync.go`'s ctx drop (independent bug)**

Currently:

```go
func (h *AirbyteSyncHandler) ProcessTask(_ context.Context, t *asynq.Task) error {
```

Change `_` to `ctx` and thread it into the work loop. Without this, the new middleware sets a deadline on a context the handler ignores, so the budget never fires. Pattern:

```go
func (h *AirbyteSyncHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
    // ... existing work, with any blocking calls accepting ctx ...
}
```

If the existing body has long-running calls (HTTP, DB) that don't take a context, refactor them to accept one. If that's beyond the resilience milestone scope, at minimum check `ctx.Err()` between iterations of any loop.

- [ ] **Step 4: Add a middleware unit test**

Create `internal/jobs/deadline_test.go`:

```go
package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

func TestDeadlineMiddleware_AppliesPerTypeBudget(t *testing.T) {
	// Stub a slow handler that observes its ctx.
	var observedErr error
	slow := asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		select {
		case <-time.After(2 * time.Second):
			return nil
		case <-ctx.Done():
			observedErr = ctx.Err()
			return ctx.Err()
		}
	})

	wrapped := DeadlineMiddleware(slow)

	// Use a task type with a short budget by injecting a temporary entry.
	const testType = "test:fast"
	taskBudgets[testType] = 50 * time.Millisecond
	t.Cleanup(func() { delete(taskBudgets, testType) })

	task := asynq.NewTask(testType, nil)
	start := time.Now()
	err := wrapped.ProcessTask(context.Background(), task)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
	if !errors.Is(observedErr, context.DeadlineExceeded) {
		t.Errorf("handler did not observe DeadlineExceeded; got %v", observedErr)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed = %v; deadline did not fire", elapsed)
	}
}

func TestDeadlineMiddleware_UnknownTypeUsesDefault(t *testing.T) {
	called := false
	h := asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		called = true
		dl, ok := ctx.Deadline()
		if !ok {
			t.Errorf("handler ctx had no deadline")
		}
		if remaining := time.Until(dl); remaining > defaultBudget+time.Second {
			t.Errorf("deadline too far away: %v", remaining)
		}
		return nil
	})

	wrapped := DeadlineMiddleware(h)
	if err := wrapped.ProcessTask(context.Background(), asynq.NewTask("test:unknown", nil)); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
```

- [ ] **Step 5: Run the jobs tests**

```bash
go test ./internal/jobs/... -v
```

Expected: PASS, including the existing tests.

- [ ] **Step 6: Commit**

```bash
git add internal/jobs/deadline.go internal/jobs/deadline_test.go internal/jobs/scheduler.go internal/jobs/airbyte_sync.go
git commit -m "feat(jobs): apply per-task-type Asynq deadlines via mux middleware"
```

---

## Task 13: Extend `internal/integration/grpc_fault_test.go`

**Depends on Tasks 4, 9.** Owns only `internal/integration/grpc_fault_test.go`.

**Files:**
- Modify: `internal/integration/grpc_fault_test.go`

- [ ] **Step 1: Read the existing test file**

```bash
cat internal/integration/grpc_fault_test.go
```

Identify the test setup pattern (likely a fault-injection gRPC server with configurable response/delay).

- [ ] **Step 2: Add three new test functions**

Append:

```go
func TestResilience_SlowAIWorker_HitsClientDeadline(t *testing.T) {
	srv, addr := startFaultServer(t, faultConfig{Delay: 5 * time.Second})
	defer srv.Stop()

	policy, _ := resilience.NewPolicy("ai-worker",
		resilience.WithTimeout(200*time.Millisecond),
	)
	breaker := resilience.NewBreaker(policy, resilience.WithIsSuccessful(resilience.IsGRPCCallerError))
	client, err := rpcClient.NewClient(addr, policy, breaker)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	start := time.Now()
	_, err = client.Worker().SomeMethod(context.Background(), &pb.SomeRequest{})
	elapsed := time.Since(start)

	if status.Code(err) != codes.DeadlineExceeded {
		t.Errorf("err code = %v, want DeadlineExceeded", status.Code(err))
	}
	if elapsed > 400*time.Millisecond {
		t.Errorf("call took %v; expected ≤ ~200ms", elapsed)
	}
}

func TestResilience_RepeatedUnavailable_OpensBreaker(t *testing.T) {
	srv, addr := startFaultServer(t, faultConfig{Code: codes.Unavailable})
	defer srv.Stop()

	policy, _ := resilience.NewPolicy("ai-worker",
		resilience.WithTimeout(500*time.Millisecond),
		resilience.WithBreakerThreshold(3),
		resilience.WithBreakerCooldown(2*time.Second),
	)
	breaker := resilience.NewBreaker(policy, resilience.WithIsSuccessful(resilience.IsGRPCCallerError))
	client, err := rpcClient.NewClient(addr, policy, breaker)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	for i := 0; i < 3; i++ {
		_, _ = client.Worker().SomeMethod(context.Background(), &pb.SomeRequest{})
	}

	preCalls := srv.CallCount()
	_, err = client.Worker().SomeMethod(context.Background(), &pb.SomeRequest{})
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Errorf("err = %v, want ErrCircuitOpen", err)
	}
	if srv.CallCount() != preCalls {
		t.Errorf("server saw extra call after breaker opened")
	}
}

func TestResilience_HalfOpenProbe_ClosesBreaker(t *testing.T) {
	srv, addr := startFaultServer(t, faultConfig{Code: codes.Unavailable})
	defer srv.Stop()

	policy, _ := resilience.NewPolicy("ai-worker",
		resilience.WithTimeout(500*time.Millisecond),
		resilience.WithBreakerThreshold(2),
		resilience.WithBreakerCooldown(100*time.Millisecond),
		resilience.WithBreakerHalfOpenMax(1),
	)
	breaker := resilience.NewBreaker(policy, resilience.WithIsSuccessful(resilience.IsGRPCCallerError))
	client, err := rpcClient.NewClient(addr, policy, breaker)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Trip the breaker.
	for i := 0; i < 2; i++ {
		_, _ = client.Worker().SomeMethod(context.Background(), &pb.SomeRequest{})
	}

	// Wait for cooldown, then flip server to OK.
	time.Sleep(150 * time.Millisecond)
	srv.SetConfig(faultConfig{Code: codes.OK})

	// Probe call should succeed and close the breaker.
	if _, err := client.Worker().SomeMethod(context.Background(), &pb.SomeRequest{}); err != nil {
		t.Fatalf("probe err = %v, want nil", err)
	}

	// Subsequent call should also succeed.
	if _, err := client.Worker().SomeMethod(context.Background(), &pb.SomeRequest{}); err != nil {
		t.Errorf("post-recovery err = %v", err)
	}
}
```

If the existing fault server lacks `SetConfig` or `CallCount` helpers, add them as small additions to the test fixture in the same file.

If the actual gRPC method names in the AI worker proto differ from `SomeMethod` / `SomeRequest`, substitute the real names from the existing tests above the new code (read the file for examples).

- [ ] **Step 3: Run the new tests**

```bash
go test ./internal/integration/... -run TestResilience -v
```

Expected: PASS for all three.

- [ ] **Step 4: Run the full integration suite to make sure nothing regressed**

```bash
go test ./internal/integration/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/integration/grpc_fault_test.go
git commit -m "test(integration): cover slow worker, breaker open, half-open recovery"
```

---

## Task 14: Update `.env.example` (sequential close-out)

**Files:**
- Modify: `.env.example`

- [ ] **Step 1: Append a new section**

Add to the end of `.env.example`:

```bash
# ─── Resilience: HTTP server + AI worker ──────────────────────────────────
# All accept Go time.Duration syntax (e.g. 5s, 500ms, 2m).
RAVEN_HTTP_READ_HEADER_TIMEOUT=5s
RAVEN_HTTP_READ_TIMEOUT=30s
RAVEN_HTTP_WRITE_TIMEOUT=60s
RAVEN_HTTP_IDLE_TIMEOUT=120s

RAVEN_AI_WORKER_TIMEOUT=5s
RAVEN_AI_WORKER_BREAKER_THRESHOLD=5
RAVEN_AI_WORKER_BREAKER_COOLDOWN=30s
```

- [ ] **Step 2: Commit**

```bash
git add .env.example
git commit -m "docs(env): document resilience timeouts and AI worker breaker knobs"
```

---

## Task 15: Final verification + PR

- [ ] **Step 1: Delete the temp survey doc**

```bash
rm docs/superpowers/plans/2026-05-08-resilience-survey.md
git add docs/superpowers/plans/2026-05-08-resilience-survey.md
git commit -m "chore: drop temp survey doc"
```

- [ ] **Step 2: Run full local test suite**

```bash
go test ./...
```

Expected: PASS. If any test fails, fix root cause — do not skip or `t.Skip()`.

- [ ] **Step 3: Run golangci-lint**

```bash
golangci-lint run ./...
```

Expected: zero errors. Fix root cause for any new violations introduced by this branch.

- [ ] **Step 4: Push branch**

```bash
git push -u origin feat/resilience-layer
```

- [ ] **Step 5: Open PR + enqueue auto-merge**

```bash
gh pr create --title "feat: resilience layer for AI worker gRPC, HTTP server, and Asynq handlers" --body "$(cat <<'EOF'
## Summary

- New `internal/resilience/` package: `Policy`, `Breaker` (gobreaker adapter), gRPC unary interceptor, HTTPClient + breaker RoundTripper
- New `internal/middleware/Deadline` for per-route Gin context budgets
- `cmd/api/main.go`: explicit `http.Server` timeouts, Deadline middleware applied per route group, AI worker gRPC client wired with `*resilience.Policy`
- `apierror.ErrorHandler` maps `resilience.ErrCircuitOpen` → 503 + `Retry-After`
- Asynq handlers: each `ProcessTask` wrapped with `context.WithTimeout`
- `.golangci.yml`: `noctx` + `contextcheck` enabled at error severity
- New env knobs documented in `.env.example`
- New integration tests cover slow worker, breaker open, half-open recovery

Spec: `docs/superpowers/specs/2026-05-08-resilience-design.md`
Plan: `docs/superpowers/plans/2026-05-08-resilience-layer.md`

## Test plan

- [ ] `go test ./...` green locally
- [ ] `golangci-lint run ./...` clean
- [ ] CI green (build + lint + integration)
EOF
)"
```

- [ ] **Step 6: Enqueue auto-merge per repo policy**

```bash
PR_NUMBER=$(gh pr view --json number -q .number)
gh pr merge "$PR_NUMBER" --auto --squash
```

Expected: auto-merge enqueued; PR will squash-merge automatically once CI passes.

---

## Self-review notes

- All spec sections covered: resilience package ✓ (Tasks 2–4; Task 5 dropped), OTel observability ✓ (folded into Task 3), Deadline middleware ✓ (Task 6), config knobs ✓ (Task 7), CI gate ✓ (Task 8), gRPC wiring ✓ (Task 9), apierror mapping ✓ (Task 10), main.go wiring ✓ (Task 11), Asynq ✓ (Task 12), integration tests ✓ (Task 13), env docs ✓ (Task 14), close-out ✓ (Task 15).
- Type consistency: `*resilience.Policy` and `*resilience.Breaker` referenced consistently; `ErrCircuitOpen`, `ErrInvalidPolicy`, `IsGRPCCallerError` used uniformly.
- Placeholders: only the Asynq sizing tunables are empirical; all other steps contain complete code or exact commands.
- Out-of-scope items confirmed deferred: bulkheads, retries, full outbound-HTTP audit, the HTTP factory itself (now Task 5 dropped), observability dashboards (the metric + span event ARE shipped; dashboards are operational follow-up).

---

## Revision change log (2026-05-11)

After a `/grill-with-docs` session, eight design branches were resolved against the actual codebase:

| # | Topic | Resolution | Tasks affected |
|---|---|---|---|
| 1 | Error mapping package | Reuse existing `pkg/apierror` (NOT spec's `internal/apierror/`). Verified `ErrorHandler()` already exists at `pkg/apierror/apierror.go:100`. | Task 1 step 5, Task 10 |
| 2 | `ErrCircuitOpen` wiring | Add `errors.Is` branch at top of existing `ErrorHandler`; `pkg/apierror` imports `internal/resilience` (Go's `internal` rule allows it; pkg is sibling of internal under module root). | Task 10 |
| 3 | Caller-error swallowing | Use `gobreaker.Settings.IsSuccessful` predicate. Original Task 4 returned `nil` to caller for caller-classified errors → handlers would deref nil reply and panic. New `IsGRPCCallerError` exported from resilience pkg; passed via new `WithIsSuccessful(...)` option on `NewBreaker`. Regression test added. | Task 3, Task 4, Task 11 step 1, Task 13 |
| 4 | Asynq coverage | `mux.Use` middleware + per-task-type budget map in new `internal/jobs/deadline.go`. Catches all 9 handlers (5 ProcessTask methods, 1 underscore-ctx variant, 3 HandlerFunc factories). Also fixes independent ctx-drop bug in `airbyte_sync.go`. | Task 12 (rewritten) |
| 5 | Lint scope | Widen `noctx` + `contextcheck` to all production Go (NOT just the spec's 3-package carve-out). Verified blast radius = 0 locally. No `exclude-rules` needed. | Task 8 (simplified) |
| 6 | Route deadline table | Spec referenced `/readyz`, `/api/v1/upload/*`, `/api/v1/voice/*` — none exist. Real groups documented with line numbers in Task 11 step 2. Inner-group budget overrides require defining inner groups before any outer `api.Use(Deadline(...))`, which the new step covers; a single outer `api` Deadline was abandoned in favor of explicit per-leaf-group + standalone-route Deadlines. | Task 11 step 2 (rewritten) |
| 7 | HTTP factory | Task 5 DROPPED. ~150 LOC of dead code (no callers in this milestone, spec defers HTTP audit). Will be designed with the follow-up audit when a real caller (LiveKit / SeaweedFS / Razorpay) shapes the contract. | Task 5 (dropped) |
| 8 | OTel observability | Spec required `resilience.breaker.state` gauge + `resilience.breaker.transition` span event but no original task implemented them. Folded into Task 3 via `gobreaker.OnStateChange` + new `WithObservability(meter, tracer)` option on `NewBreaker`. | Task 3 |

ADRs offered but **declined** by the user during the grilling session:

- ADR-A: "Resilience errors map to HTTP via `pkg/apierror.ErrorHandler` branch" (decision #2 above) — captured in this plan instead.
- ADR-B: "Asynq per-task deadlines via `mux.Use` middleware, not `asynq.Timeout` at NewTask sites" (decision #4) — captured in this plan instead.

If those decisions later prove load-bearing, the rationale for re-opening them lives in this change log.
