package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
)

// TrialLifecycleHandler inspects subscriptions in the trialing/expired states
// and advances them through the trial→grace→archive→delete pipeline.
//
// Day-boundary rules (all relative to trial_ends_at = T):
//   T-2  → enqueue email: trial_expiring_soon
//   T+1  → set status = expired
//   T+7  → enqueue email: data_deletion_warning  (grace_period_ends_at)
//   T+8  → enqueue task:  archive_org_data
//   T+38 → enqueue task:  delete_org_data
type TrialLifecycleHandler struct {
	pool        *pgxpool.Pool
	queueClient *queue.Client
	logger      *slog.Logger
}

// NewTrialLifecycleHandler creates a TrialLifecycleHandler.
func NewTrialLifecycleHandler(pool *pgxpool.Pool, queueClient *queue.Client, logger *slog.Logger) *TrialLifecycleHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &TrialLifecycleHandler{
		pool:        pool,
		queueClient: queueClient,
		logger:      logger,
	}
}

// ProcessTask implements asynq.Handler for the trial lifecycle daily job.
func (h *TrialLifecycleHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload TrialLifecyclePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal TrialLifecyclePayload: %w", err)
	}
	return h.run(ctx, payload)
}

func (h *TrialLifecycleHandler) run(ctx context.Context, payload TrialLifecyclePayload) error {
	now := time.Now().UTC()
	if !payload.AsOf.IsZero() {
		now = payload.AsOf
	}

	rows, err := h.querySubscriptions(ctx, payload.OrgID)
	if err != nil {
		return fmt.Errorf("query trial subscriptions: %w", err)
	}

	var errs []error
	for _, sub := range rows {
		if e := h.processOne(ctx, sub, now); e != nil {
			h.logger.ErrorContext(ctx, "trial lifecycle step failed",
				"org_id", sub.OrgID,
				"sub_id", sub.ID,
				"error", e,
			)
			errs = append(errs, e)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("trial lifecycle: %d subscription(s) failed (first: %w)", len(errs), errs[0])
	}
	return nil
}

// trialRow is a minimal row scanned from the subscriptions query.
type trialRow struct {
	ID                string
	OrgID             string
	Status            model.SubscriptionStatus
	TrialEndsAt       *time.Time
	GracePeriodEndsAt *time.Time
}

func (h *TrialLifecycleHandler) querySubscriptions(ctx context.Context, orgID string) ([]trialRow, error) {
	q := `
		SELECT id, org_id, status, trial_ends_at, grace_period_ends_at
		FROM subscriptions
		WHERE status IN ('trialing', 'expired')
		  AND trial_ends_at IS NOT NULL`
	args := []any{}
	if orgID != "" {
		q += ` AND org_id = $1`
		args = append(args, orgID)
	}

	rows, err := h.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query subscriptions: %w", err)
	}
	defer rows.Close()

	var result []trialRow
	for rows.Next() {
		var r trialRow
		if err := rows.Scan(&r.ID, &r.OrgID, &r.Status, &r.TrialEndsAt, &r.GracePeriodEndsAt); err != nil {
			return nil, fmt.Errorf("scan subscription row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (h *TrialLifecycleHandler) processOne(ctx context.Context, sub trialRow, now time.Time) error {
	if sub.TrialEndsAt == nil {
		return nil
	}
	t := *sub.TrialEndsAt

	// Day 12 (T-2): warn that trial is expiring soon.
	warnAt := t.Add(-2 * 24 * time.Hour)
	// We use a 24-hour window to avoid double-firing if the job runs slightly late.
	if isToday(now, warnAt) && sub.Status == model.SubscriptionStatusTrialing {
		if err := h.queueClient.EnqueueTrialEmail(ctx, sub.OrgID, sub.ID, "trial_expiring_soon"); err != nil {
			return fmt.Errorf("enqueue trial_expiring_soon: %w", err)
		}
		h.logger.InfoContext(ctx, "enqueued trial_expiring_soon", "org_id", sub.OrgID, "sub_id", sub.ID)
	}

	// Day 15 (T+1): expire.
	expireAt := t.Add(1 * 24 * time.Hour)
	if now.After(expireAt) && sub.Status == model.SubscriptionStatusTrialing {
		if _, err := h.pool.Exec(ctx,
			`UPDATE subscriptions SET status = 'expired' WHERE id = $1 AND org_id = $2`,
			sub.ID, sub.OrgID,
		); err != nil {
			return fmt.Errorf("mark subscription expired: %w", err)
		}
		sub.Status = model.SubscriptionStatusExpired
		h.logger.InfoContext(ctx, "subscription expired", "org_id", sub.OrgID, "sub_id", sub.ID)
	}

	// From here on we act on expired subscriptions only.
	if sub.Status != model.SubscriptionStatusExpired {
		return nil
	}

	// Day 21 (grace_period_ends_at): warn about data deletion.
	if sub.GracePeriodEndsAt != nil {
		g := *sub.GracePeriodEndsAt
		if isToday(now, g) {
			if err := h.queueClient.EnqueueTrialEmail(ctx, sub.OrgID, sub.ID, "data_deletion_warning"); err != nil {
				return fmt.Errorf("enqueue data_deletion_warning: %w", err)
			}
			h.logger.InfoContext(ctx, "enqueued data_deletion_warning", "org_id", sub.OrgID, "sub_id", sub.ID)
		}

		// Day 22 (grace_period_ends_at + 1d): archive.
		archiveAt := g.Add(1 * 24 * time.Hour)
		if isToday(now, archiveAt) {
			if err := h.queueClient.EnqueueArchiveOrgData(ctx, sub.OrgID); err != nil {
				return fmt.Errorf("enqueue archive_org_data: %w", err)
			}
			h.logger.InfoContext(ctx, "enqueued archive_org_data", "org_id", sub.OrgID, "sub_id", sub.ID)
		}

		// Day 52 (day22 + 30d): hard delete.
		deleteAt := archiveAt.Add(30 * 24 * time.Hour)
		if isToday(now, deleteAt) {
			if err := h.queueClient.EnqueueDeleteOrgData(ctx, sub.OrgID); err != nil {
				return fmt.Errorf("enqueue delete_org_data: %w", err)
			}
			h.logger.InfoContext(ctx, "enqueued delete_org_data", "org_id", sub.OrgID, "sub_id", sub.ID)
		}
	}

	return nil
}

// isToday returns true when target falls within the same UTC calendar day as now,
// or is past (up to 25 h back) to handle jobs that run slightly late.
func isToday(now, target time.Time) bool {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(25 * time.Hour) // generous window for late-running cron
	return !target.Before(start) && target.Before(end)
}
