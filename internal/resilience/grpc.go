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
// Returns true for nil so it satisfies the IsSuccessful contract (success
// counts as success).
//
// Codes treated as caller errors (success-equivalent for breaker accounting):
//
//	InvalidArgument, NotFound, AlreadyExists, PermissionDenied,
//	Unauthenticated, FailedPrecondition, OutOfRange.
//
// Codes counted as breaker failures (caller still sees them):
//
//	Unavailable, DeadlineExceeded, Internal, ResourceExhausted, Unknown,
//	Aborted, Canceled, DataLoss.
func IsGRPCCallerError(err error) bool {
	if err == nil {
		return true
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
//   - Returns the invoker's error verbatim. Whether the error counts toward
//     the breaker's failure tally is decided by the breaker's IsSuccessful
//     predicate (set via NewBreaker(p, WithIsSuccessful(...))).
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
