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

	for i := 0; i < 2; i++ {
		_ = icpt(context.Background(), "/svc/Method", nil, nil, nil, inv.invoke)
	}
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

	for i := 0; i < 5; i++ {
		_ = icpt(context.Background(), "/svc/Method", nil, nil, nil, inv.invoke)
	}

	preCalls := inv.calls
	_ = icpt(context.Background(), "/svc/Method", nil, nil, nil, inv.invoke)
	if inv.calls == preCalls {
		t.Errorf("breaker tripped on caller errors")
	}
}

// Regression: caller errors must propagate to the caller verbatim.
// Pre-fix the interceptor returned nil for caller-classified errors, which
// caused handlers to deref nil reply and panic.
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
