# Resilience Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add bounded-deadline + circuit-breaker resilience to the Raven Go API across the AI-worker gRPC client, `http.Server`, per-route Gin handlers, and Asynq job handlers — with unit + integration tests and a CI compliance gate.

**Architecture:** New `internal/resilience/` package owns reusable primitives (`Policy`, breaker adapter, gRPC interceptor, HTTP RoundTripper). New `internal/middleware/deadline.go` enforces per-route context deadlines. `cmd/api/main.go` wires explicit `http.Server` timeouts and the Deadline middleware. `.golangci.yml` adds `noctx` + `contextcheck` as blocking lint gates.

**Tech Stack:** Go 1.22+, Gin, `google.golang.org/grpc`, `github.com/sony/gobreaker`, `github.com/hibiken/asynq`, `golangci-lint v2`.

**Spec:** [`docs/superpowers/specs/2026-05-08-resilience-design.md`](../specs/2026-05-08-resilience-design.md)

**Branch:** `feat/resilience-layer` (worktree from `origin/main`).

## Parallel-execution map

Tasks group into phases. Tasks within a phase can run in parallel (disjoint file ownership). Tasks across phases are sequential.

| Phase | Tasks | Parallelizable | Notes |
|-------|-------|----------------|-------|
| 0 — Bootstrap | 1 | no | Worktree + dep + survey |
| 1 — Primitives | 2, 3, 6, 7, 8 | **yes (5 agents)** | All write disjoint files |
| 2 — Composite primitives | 4, 5 | **yes (2 agents)** | Depend on 2+3; mutually disjoint |
| 3 — Wire | 9, 10 | **yes (2 agents)** | Depend on 2+3+4 |
| 4 — Main wiring | 11 | no | Touches cmd/api/main.go (single owner) — depends on 6, 9 |
| 5 — Asynq + integration | 12, 13 | **yes (2 agents)** | 12 owns jobs/, 13 owns integration/ |
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
  echo "## apierror package"
  grep -rn 'package apierror\|func ErrorHandler' --include='*.go' internal/ cmd/ | head
  echo
  echo "## Asynq handler files (ProcessTask receivers)"
  grep -rn 'func .* ProcessTask(ctx context.Context' --include='*.go' internal/ | head -30
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
} > docs/superpowers/plans/2026-05-08-resilience-survey.md
```

Read the resulting file. Subsequent tasks reference these paths; if any path differs from what tasks below assume (e.g., `apierror` lives in a different package), update the affected task before executing it.

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

## Task 3: `resilience.Breaker` adapter (parallel-safe)

**Files:**
- Create: `internal/resilience/breaker.go`
- Test: `internal/resilience/breaker_test.go`

Owns only files in `internal/resilience/breaker*.go` — disjoint from Tasks 2, 6, 7, 8.

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

// NewBreaker constructs a Breaker from a validated Policy.
func NewBreaker(p *Policy) *Breaker {
	settings := gobreaker.Settings{
		Name:        p.Name,
		MaxRequests: p.BreakerHalfOpenMax,
		Interval:    0, // 0 = never reset counts in Closed state
		Timeout:     p.BreakerCooldown,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= p.BreakerThreshold
		},
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

func TestUnaryClientInterceptor_AppliesTimeout(t *testing.T) {
	p, _ := NewPolicy("svc", WithTimeout(20*time.Millisecond))
	icpt := UnaryClientInterceptor(p, NewBreaker(p))

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
	br := NewBreaker(p)
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
	br := NewBreaker(p)
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

// UnaryClientInterceptor returns a gRPC unary client interceptor that:
//
//   - Applies policy.Timeout to each call (only if no shorter deadline is set).
//   - Routes the call through the breaker.
//   - Counts only server-side failures (Unavailable, DeadlineExceeded,
//     Internal, ResourceExhausted) toward the breaker's failure tally;
//     caller errors (InvalidArgument, NotFound, PermissionDenied,
//     Unauthenticated) are not counted.
func UnaryClientInterceptor(p *Policy, br *Breaker) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// Apply policy timeout unless a shorter deadline already exists.
		callCtx, cancel := withTimeoutIfShorter(ctx, p.Timeout)
		defer cancel()

		_, err := br.Execute(callCtx, func(c context.Context) (any, error) {
			invErr := invoker(c, method, req, reply, cc, opts...)
			if isCallerError(invErr) {
				// Tell gobreaker the call succeeded so caller errors don't trip it.
				return nil, nil
			}
			return nil, invErr
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

func isCallerError(err error) bool {
	if err == nil {
		return false
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
```

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

## Task 5: `resilience` HTTP factory (depends on Tasks 2, 3)

**Files:**
- Create: `internal/resilience/http.go`
- Test: `internal/resilience/http_test.go`

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

- [ ] **Step 1: Read current `.golangci.yml`**

Current state (from survey):

```yaml
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - ineffassign
    - revive
```

- [ ] **Step 2: Add the two new linters**

Edit `.golangci.yml` so the `linters.enable` list reads:

```yaml
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - ineffassign
    - revive
    - noctx
    - contextcheck
```

(Order does not matter; alphabetised is fine.)

- [ ] **Step 3: Run golangci-lint locally to baseline existing violations**

```bash
golangci-lint run ./internal/grpc/... ./internal/handler/... ./internal/service/...
```

If pre-existing code triggers violations, do **not** suppress them globally. Instead:

1. Capture the count and locations.
2. Decide per finding whether to fix inline (preferred) or add a narrowly-scoped `//nolint:noctx // <reason>` directive on the offending line.
3. Document the rationale for any nolint directives in the commit message.

The CI gate must be green at error severity by the end of Task 15.

- [ ] **Step 4: Verify lint passes on the resilience + middleware packages**

```bash
golangci-lint run ./internal/resilience/... ./internal/middleware/...
```

Expected: PASS (these are new and clean).

- [ ] **Step 5: Commit**

```bash
git add .golangci.yml
git commit -m "ci(lint): enable noctx and contextcheck for resilience compliance gate"
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

## Task 10: Map `ErrCircuitOpen` → HTTP 503 in apierror

**Depends on Task 3.** **Path was surveyed in Task 1 step 5** — read `docs/superpowers/plans/2026-05-08-resilience-survey.md` to confirm the apierror file path before editing.

**Files:**
- Modify: the file in the apierror package that defines `ErrorHandler` (likely `internal/apierror/handler.go` or `internal/apierror/middleware.go`).

- [ ] **Step 1: Locate `ErrorHandler`**

```bash
grep -rn 'func ErrorHandler' internal/ --include='*.go' | grep -v _test
```

Note the file path. Subsequent steps refer to it as `<errfile>`.

- [ ] **Step 2: Read the existing handler to understand its dispatch shape**

Read `<errfile>` and identify how it maps Go errors to HTTP status codes (likely `errors.Is`/`errors.As` switch).

- [ ] **Step 3: Add the ErrCircuitOpen → 503 mapping**

Add an import:

```go
import "github.com/ravencloak-org/Raven/internal/resilience"
```

Add a branch in the error-mapping switch (place before the generic fallback):

```go
case errors.Is(err, resilience.ErrCircuitOpen):
    c.Header("Retry-After", "30")
    c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
        "error": "service temporarily unavailable",
        "code":  "circuit_open",
    })
    return
```

If the handler uses a custom error response shape, match the existing shape rather than `gin.H` literal.

- [ ] **Step 4: Add a unit test**

Add to the apierror package's `*_test.go` (create one if missing):

```go
func TestErrorHandler_CircuitOpenReturns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ErrorHandler())
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

- [ ] **Step 5: Run the test**

```bash
go test ./internal/apierror/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/apierror/
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
aiBreaker := resilience.NewBreaker(aiPolicy)

grpcClient, err := rpcClient.NewClient(cfg.GRPC.WorkerAddr, aiPolicy, aiBreaker)
```

Add the import: `"github.com/ravencloak-org/Raven/internal/resilience"` (adjust module path to match `go.mod`).

- [ ] **Step 2: Apply Deadline middleware per route group**

Locate each route group definition (search for `router.Group(` and `api := ...`). Add a `Deadline` middleware call right after the group is created, e.g.:

```go
api := router.Group("/api/v1")
api.Use(middleware.Deadline(10 * time.Second)) // default budget
```

Sized per spec:

| Route group           | Budget                                |
|-----------------------|---------------------------------------|
| `/healthz`, `/readyz` | `1 * time.Second`                     |
| `/api/v1/chat/*`      | `30 * time.Second`                    |
| `/api/v1/upload/*`    | `60 * time.Second`                    |
| `/api/v1/voice/*`     | `30 * time.Second`                    |
| Default `/api/v1/*`   | `10 * time.Second`                    |

For sub-groups that need a different budget than their parent, call `chatAPI.Use(middleware.Deadline(30 * time.Second))` on the sub-group — Gin's `Use` is additive but the inner Deadline will override the outer one because `context.WithTimeout` only shortens.

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

## Task 12: Asynq handler timeouts (parallel-safe with Task 13)

**Depends on Task 1.** Owns only files under `internal/jobs/`.

**Files:**
- Modify: every file in `internal/jobs/` that defines a `ProcessTask(ctx context.Context, task *asynq.Task)` method.

- [ ] **Step 1: Survey ProcessTask implementations**

```bash
grep -rln 'ProcessTask(ctx context.Context, task \*asynq.Task)' internal/jobs/
```

For each handler file:

- [ ] **Step 2: Wrap each `ProcessTask` body with a per-handler timeout**

For a representative file like `internal/jobs/voice_usage.go` — at the top of the `ProcessTask` method, before any other logic:

```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

Sizing per handler:

| Handler                      | Timeout |
|------------------------------|---------|
| Document processing          | 5m      |
| Voice usage aggregation      | 30s     |
| Webhook delivery             | 30s     |
| Email summary generation     | 2m      |
| Default (anything else)      | 1m      |

If unsure, use 1m and leave a `// TODO(resilience): tune per workload` comment (this is the **only** acceptable TODO in this plan, because Asynq retention/timeout tuning is genuinely empirical).

- [ ] **Step 3: Add a unit test asserting a slow handler is bounded**

Pick one representative handler (e.g., `VoiceUsageHandler`) and add a test that injects a stub dependency which sleeps longer than the configured timeout, asserting the handler returns `context.DeadlineExceeded` within the budget.

- [ ] **Step 4: Run the jobs tests**

```bash
go test ./internal/jobs/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/jobs/
git commit -m "feat(jobs): apply per-handler context.WithTimeout to all Asynq processors"
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
	breaker := resilience.NewBreaker(policy)
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
	breaker := resilience.NewBreaker(policy)
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
	breaker := resilience.NewBreaker(policy)
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

- All spec sections covered: resilience package ✓ (Tasks 2–5), Deadline middleware ✓ (Task 6), config knobs ✓ (Task 7), CI gate ✓ (Task 8), gRPC wiring ✓ (Task 9), apierror mapping ✓ (Task 10), main.go wiring ✓ (Task 11), Asynq ✓ (Task 12), integration tests ✓ (Task 13), env docs ✓ (Task 14), close-out ✓ (Task 15).
- Type consistency: `*resilience.Policy` and `*resilience.Breaker` referenced consistently; `ErrCircuitOpen` and `ErrInvalidPolicy` used uniformly.
- Placeholders: only the Asynq sizing TODO is allowed (genuinely empirical); all other steps contain complete code or exact commands.
- Out-of-scope items confirmed deferred: bulkheads, retries, full outbound-HTTP audit, observability dashboards.
