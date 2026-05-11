package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errCallerFault = errors.New("caller fault")

func TestBreaker_OpensAfterThreshold(t *testing.T) {
	p, _ := NewPolicy("svc",
		WithBreakerThreshold(3),
		WithBreakerCooldown(50*time.Millisecond),
	)
	br := NewBreaker(p)

	failing := func(context.Context) (any, error) { return nil, errors.New("boom") }

	for i := 0; i < 3; i++ {
		_, _ = br.Execute(context.Background(), failing)
	}

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

	for i := 0; i < 2; i++ {
		_, _ = br.Execute(context.Background(), failing)
	}
	if _, err := br.Execute(context.Background(), succ); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen while open, got %v", err)
	}

	time.Sleep(40 * time.Millisecond)

	if _, err := br.Execute(context.Background(), succ); err != nil {
		t.Fatalf("half-open probe err = %v, want nil", err)
	}

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

// IsSuccessful predicate: caller-classified errors propagate AND don't trip the breaker.
func TestBreaker_IsSuccessfulPredicate(t *testing.T) {
	p, _ := NewPolicy("svc", WithBreakerThreshold(2))
	br := NewBreaker(p, WithIsSuccessful(func(err error) bool {
		return err == nil || errors.Is(err, errCallerFault)
	}))

	callerFail := func(context.Context) (any, error) { return nil, errCallerFault }

	for i := 0; i < 5; i++ {
		_, err := br.Execute(context.Background(), callerFail)
		if !errors.Is(err, errCallerFault) {
			t.Fatalf("err = %v, want errCallerFault to propagate", err)
		}
	}

	realFail := func(context.Context) (any, error) { return nil, errors.New("server boom") }
	for i := 0; i < 2; i++ {
		_, _ = br.Execute(context.Background(), realFail)
	}
	if _, err := br.Execute(context.Background(), realFail); !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("breaker did not open after 2 server failures; err = %v", err)
	}
}
