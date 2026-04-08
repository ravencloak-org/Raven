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

// TypeVoiceUsageAggregation is the task type for voice usage aggregation.
const TypeVoiceUsageAggregation = "scheduled:voice_usage_aggregation"

// CronVoiceUsageAggregation runs voice usage aggregation every hour at minute 10.
const CronVoiceUsageAggregation = "10 * * * *"

// VoiceUsagePayload is the payload for the voice usage aggregation task.
type VoiceUsagePayload struct {
	// OrgID optionally restricts aggregation to a single organisation.
	OrgID string `json:"org_id,omitempty"`

	// WindowMinutes is the look-back window in minutes for aggregation.
	// Defaults to 60 (one hour) if zero.
	WindowMinutes int `json:"window_minutes,omitempty"`
}

// NewVoiceUsageTask creates a new Asynq task for voice usage aggregation.
func NewVoiceUsageTask(p VoiceUsagePayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal VoiceUsagePayload: %w", err)
	}
	return asynq.NewTask(TypeVoiceUsageAggregation, data), nil
}

// VoiceUsageHandler aggregates ended voice sessions per organisation into
// the voice_usage_summaries table for billing and analytics.
type VoiceUsageHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewVoiceUsageHandler creates a VoiceUsageHandler.
func NewVoiceUsageHandler(pool *pgxpool.Pool, logger *slog.Logger) *VoiceUsageHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &VoiceUsageHandler{
		pool:   pool,
		logger: logger,
	}
}

// ProcessTask implements asynq.Handler for the voice usage aggregation job.
func (h *VoiceUsageHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload VoiceUsagePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal VoiceUsagePayload: %w", err)
	}

	windowMinutes := payload.WindowMinutes
	if windowMinutes <= 0 {
		windowMinutes = defaultWindowMinutes
	}

	h.logger.Info("starting voice usage aggregation",
		"org_id", payload.OrgID,
		"window_minutes", windowMinutes,
	)

	inserted, err := h.aggregateVoiceUsage(ctx, payload.OrgID, windowMinutes)
	if err != nil {
		return fmt.Errorf("aggregate voice usage: %w", err)
	}

	h.logger.Info("voice usage aggregation complete",
		"rows_upserted", inserted,
		"org_id", payload.OrgID,
		"window_minutes", windowMinutes,
	)

	return nil
}

// aggregateVoiceUsage rolls up ended voice sessions from the voice_sessions
// table into the voice_usage_summaries table, grouped by org_id and hour.
//
// Uses INSERT ... ON CONFLICT to upsert so re-runs are idempotent.
func (h *VoiceUsageHandler) aggregateVoiceUsage(ctx context.Context, orgID string, windowMinutes int) (int64, error) {
	q := `
		INSERT INTO voice_usage_summaries (org_id, period_start, total_sessions, total_duration_seconds)
		SELECT
			org_id,
			date_trunc('hour', ended_at) AS period_start,
			COUNT(*)                     AS total_sessions,
			COALESCE(SUM(call_duration_seconds), 0) AS total_duration_seconds
		FROM voice_sessions
		WHERE state = 'ended'
		  AND ended_at IS NOT NULL
		  AND ended_at >= NOW() - make_interval(mins => $1)`

	args := []any{windowMinutes}

	if orgID != "" {
		q += ` AND org_id = $2`
		args = append(args, orgID)
	}

	q += `
		GROUP BY org_id, date_trunc('hour', ended_at)
		ON CONFLICT (org_id, period_start)
		DO UPDATE SET
			total_sessions         = EXCLUDED.total_sessions,
			total_duration_seconds = EXCLUDED.total_duration_seconds,
			updated_at             = NOW()`

	tag, err := h.pool.Exec(ctx, q, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			// Tables not yet created by migrations — treat as no-op.
			h.logger.Warn("voice_usage_summaries table does not exist yet, skipping", "error", err)
			return 0, nil
		}
		return 0, fmt.Errorf("voice usage aggregation query: %w", err)
	}
	return tag.RowsAffected(), nil
}
