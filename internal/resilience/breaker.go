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
