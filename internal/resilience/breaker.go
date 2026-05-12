package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// CircuitOpenError is returned by a Breaker when the underlying state machine
// is Open (or in Half-Open with the probe quota exhausted). It carries the
// PolicyName and the RetryAfter duration so callers can emit a precise
// Retry-After HTTP header without relying on a hardcoded constant.
type CircuitOpenError struct {
	PolicyName string
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *CircuitOpenError) Error() string {
	return fmt.Sprintf("resilience: circuit breaker %q open", e.PolicyName)
}

// Is reports whether target is a *CircuitOpenError, enabling errors.Is
// comparisons against the ErrCircuitOpen sentinel for back-compat.
func (e *CircuitOpenError) Is(target error) bool {
	_, ok := target.(*CircuitOpenError)
	return ok
}

// ErrCircuitOpen is a zero-value sentinel kept for errors.Is back-compat.
// New code should use errors.As to obtain RetryAfter from *CircuitOpenError.
var ErrCircuitOpen = &CircuitOpenError{}

// Breaker is a thin adapter over sony/gobreaker that maps its sentinel
// errors to *CircuitOpenError and respects context cancellation up front.
type Breaker struct {
	cb         *gobreaker.CircuitBreaker[any]
	policyName string
	cooldown   time.Duration
}

// BreakerOption configures NewBreaker.
type BreakerOption func(*breakerOpts)

type breakerOpts struct {
	isSuccessful func(error) bool
	meter        metric.Meter
	tracer       trace.Tracer
}

// WithIsSuccessful classifies errors that should NOT count as breaker
// failures (e.g. gRPC InvalidArgument is a caller bug, not a server fault).
// The error still propagates to the caller; the breaker just ignores it.
// The predicate must return true for nil.
func WithIsSuccessful(fn func(error) bool) BreakerOption {
	return func(o *breakerOpts) { o.isSuccessful = fn }
}

// WithObservability wires gobreaker's OnStateChange callback to OTel.
// Emits a Gauge `resilience.breaker.state` (0=Closed, 1=HalfOpen, 2=Open)
// labeled by policy name, plus a span event `resilience.breaker.transition`
// on every transition. If meter or tracer is nil this option is a no-op.
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
	policyName := p.Name
	cooldown := p.BreakerCooldown
	if o.isSuccessful != nil {
		settings.IsSuccessful = o.isSuccessful
	}
	if o.meter != nil {
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

	return &Breaker{
		cb:         gobreaker.NewCircuitBreaker[any](settings),
		policyName: policyName,
		cooldown:   cooldown,
	}
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
		return nil, &CircuitOpenError{PolicyName: b.policyName, RetryAfter: b.cooldown}
	}
	return out, err
}
