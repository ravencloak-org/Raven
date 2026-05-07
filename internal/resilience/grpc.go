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
