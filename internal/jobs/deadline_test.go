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

	wrapped := DeadlineMiddleware(slow)

	const testType = "test:fast"
	taskBudgets[testType] = 50 * time.Millisecond
	t.Cleanup(func() { delete(taskBudgets, testType) })

	task := asynq.NewTask(testType, nil)
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
	called := false
	h := asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		called = true
		dl, ok := ctx.Deadline()
		if !ok {
			t.Errorf("handler ctx had no deadline")
		}
		if remaining := time.Until(dl); remaining > defaultBudget+time.Second {
			t.Errorf("deadline too far away: %v", remaining)
		}
		return nil
	})

	wrapped := DeadlineMiddleware(h)
	if err := wrapped.ProcessTask(context.Background(), asynq.NewTask("test:unknown", nil)); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
