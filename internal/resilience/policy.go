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
