package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

func TestDeadlineMiddleware_AppliesPerTypeBudget(t *testing.T) {
	var observedErr error
	slow := asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		select {
		case <-time.After(2 * time.Second):
			return nil
		case <-ctx.Done():
			observedErr = ctx.Err()
			return ctx.Err()
		}
	})

	budgets := map[string]time.Duration{
		"test:fast": 50 * time.Millisecond,
	}
	wrapped := DeadlineMiddleware(budgets, 1*time.Minute)(slow)

	task := asynq.NewTask("test:fast", nil)
	start := time.Now()
	err := wrapped.ProcessTask(context.Background(), task)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
	if !errors.Is(observedErr, context.DeadlineExceeded) {
		t.Errorf("handler did not observe DeadlineExceeded; got %v", observedErr)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed = %v; deadline did not fire", elapsed)
	}
}

func TestDeadlineMiddleware_UnknownTypeUsesDefault(t *testing.T) {
	const fallback = 90 * time.Second
	called := false
	h := asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		called = true
		dl, ok := ctx.Deadline()
		if !ok {
			t.Errorf("handler ctx had no deadline")
		}
		if remaining := time.Until(dl); remaining > fallback+time.Second {
			t.Errorf("deadline too far away: %v", remaining)
		}
		return nil
	})

	wrapped := DeadlineMiddleware(map[string]time.Duration{}, fallback)(h)
	if err := wrapped.ProcessTask(context.Background(), asynq.NewTask("test:unknown", nil)); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
