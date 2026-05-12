package jobs

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/queue"
)

// BudgetsFromConfig builds the per-task-type budget map from AsynqConfig,
// keeping scheduler.go tidy and making the mapping unit-testable.
func BudgetsFromConfig(c config.AsynqConfig) map[string]time.Duration {
	return map[string]time.Duration{
		queue.TypeDocumentProcess: c.DocumentProcessBudget,
		queue.TypeAirbyteSync:     c.AirbyteSyncBudget,
		TypeEmailSummary:          c.EmailSummaryBudget,
		TypeSendEmail:             c.SendEmailBudget,
		TypeVoiceUsageAggregation: c.VoiceUsageAggBudget,
		TypeUsageAggregation:      c.UsageAggBudget,
		TypeWebhookDelivery:       c.WebhookDeliveryBudget,
		TypeRecrawlSources:        c.RecrawlSourcesBudget,
		TypeCleanupSessions:       c.CleanupSessionsBudget,
	}
}

// DeadlineMiddleware returns an Asynq middleware factory that wraps every
// handler with a per-task-type context.WithTimeout drawn from budgets.
// Tasks not present in the map fall back to defaultBudget.
//
// Usage:
//
//	mux.Use(jobs.DeadlineMiddleware(jobs.BudgetsFromConfig(cfg.Asynq), cfg.Asynq.DefaultBudget))
func DeadlineMiddleware(budgets map[string]time.Duration, defaultBudget time.Duration) func(asynq.Handler) asynq.Handler {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			budget, ok := budgets[t.Type()]
			if !ok || budget == 0 {
				budget = defaultBudget
			}
			ctx, cancel := context.WithTimeout(ctx, budget)
			defer cancel()
			return next.ProcessTask(ctx, t)
		})
	}
}
