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
