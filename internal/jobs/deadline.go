package jobs

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/ravencloak-org/Raven/internal/queue"
)

// taskBudgets maps a task type to its per-execution deadline.
// Keys come from the TypeXxx constants in tasks.go (or per-file constants),
// plus the on-demand task types defined in internal/queue. Anything not in
// the map falls back to defaultBudget.
var taskBudgets = map[string]time.Duration{
	queue.TypeDocumentProcess: 5 * time.Minute,
	queue.TypeAirbyteSync:     5 * time.Minute,
	TypeEmailSummary:          2 * time.Minute,
	TypeSendEmail:             30 * time.Second,
	TypeVoiceUsageAggregation: 30 * time.Second,
	TypeUsageAggregation:      30 * time.Second,
	TypeWebhookDelivery:       30 * time.Second,
	TypeRecrawlSources:        2 * time.Minute,
	TypeCleanupSessions:       2 * time.Minute,
}

// defaultBudget is applied when a task type has no entry in taskBudgets.
const defaultBudget = 1 * time.Minute

// DeadlineMiddleware wraps every Asynq handler with a per-task-type
// context.WithTimeout. Apply via mux.Use(DeadlineMiddleware) so it covers
// handlers regardless of whether they are method receivers or HandlerFunc
// factories. Tasks not present in taskBudgets fall back to defaultBudget.
func DeadlineMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		budget, ok := taskBudgets[t.Type()]
		if !ok {
			budget = defaultBudget
		}
		ctx, cancel := context.WithTimeout(ctx, budget)
		defer cancel()
		return next.ProcessTask(ctx, t)
	})
}
