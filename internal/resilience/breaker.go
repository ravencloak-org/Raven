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
