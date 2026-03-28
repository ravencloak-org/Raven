package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultWindowMinutes is the default look-back window for usage aggregation.
const defaultWindowMinutes = 60

// UsageAggregationHandler aggregates API usage metrics per organisation and
// workspace into a summary table for billing. It runs as an hourly cron job.
type UsageAggregationHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewUsageAggregationHandler creates a UsageAggregationHandler.
func NewUsageAggregationHandler(pool *pgxpool.Pool, logger *slog.Logger) *UsageAggregationHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &UsageAggregationHandler{
		pool:   pool,
		logger: logger,
	}
}

// ProcessTask implements asynq.Handler for the usage aggregation scheduled job.
func (h *UsageAggregationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload UsageAggregationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal UsageAggregationPayload: %w", err)
	}

	windowMinutes := payload.WindowMinutes
	if windowMinutes <= 0 {
		windowMinutes = defaultWindowMinutes
	}

	h.logger.Info("starting usage aggregation",
		"org_id", payload.OrgID,
		"window_minutes", windowMinutes,
	)

	inserted, err := h.aggregateUsage(ctx, payload.OrgID, windowMinutes)
	if err != nil {
		return fmt.Errorf("aggregate usage: %w", err)
	}

	h.logger.Info("usage aggregation complete",
		"rows_inserted", inserted,
		"org_id", payload.OrgID,
		"window_minutes", windowMinutes,
	)

	return nil
}

// aggregateUsage rolls up raw API usage events from the api_usage_events table
// into the usage_aggregations table, grouped by org_id, workspace_id, and hour.
//
// The query uses INSERT ... ON CONFLICT to upsert so that re-runs within the
// same window are idempotent.
func (h *UsageAggregationHandler) aggregateUsage(ctx context.Context, orgID string, windowMinutes int) (int64, error) {
	q := `
		INSERT INTO usage_aggregations (org_id, workspace_id, period_start, request_count, token_count)
		SELECT
			org_id,
			workspace_id,
			date_trunc('hour', created_at) AS period_start,
			COUNT(*)                       AS request_count,
			COALESCE(SUM(tokens_used), 0)  AS token_count
		FROM api_usage_events
		WHERE created_at >= NOW() - make_interval(mins => $1)`

	args := []any{windowMinutes}

	if orgID != "" {
		q += ` AND org_id = $2`
		args = append(args, orgID)
	}

	q += `
		GROUP BY org_id, workspace_id, date_trunc('hour', created_at)
		ON CONFLICT (org_id, workspace_id, period_start)
		DO UPDATE SET
			request_count = EXCLUDED.request_count,
			token_count   = EXCLUDED.token_count,
			updated_at    = NOW()`

	tag, err := h.pool.Exec(ctx, q, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			// Tables not yet created by migrations — treat as no-op.
			h.logger.Warn("usage aggregation tables do not exist yet, skipping", "error", err)
			return 0, nil
		}
		return 0, fmt.Errorf("usage aggregation query: %w", err)
	}
	return tag.RowsAffected(), nil
}
