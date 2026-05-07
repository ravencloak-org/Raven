package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Default retention periods used when the payload does not specify values.
const (
	defaultSessionMaxAgeDays  = 30
	defaultEventRetentionDays = 90
)

// CleanupHandler handles the scheduled cleanup of expired sessions and stale
// processing events. It runs as a daily cron job (2 AM UTC by default).
type CleanupHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewCleanupHandler creates a CleanupHandler.
func NewCleanupHandler(pool *pgxpool.Pool, logger *slog.Logger) *CleanupHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CleanupHandler{
		pool:   pool,
		logger: logger,
	}
}

// ProcessTask implements asynq.Handler for the cleanup scheduled job.
func (h *CleanupHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	var payload CleanupPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal CleanupPayload: %w", err)
	}

	sessionDays := payload.SessionMaxAgeDays
	if sessionDays <= 0 {
		sessionDays = defaultSessionMaxAgeDays
	}
	eventDays := payload.EventRetentionDays
	if eventDays <= 0 {
		eventDays = defaultEventRetentionDays
	}

	h.logger.Info("starting scheduled cleanup",
		"session_max_age_days", sessionDays,
		"event_retention_days", eventDays,
	)

	sessionsDeleted, err := h.cleanupExpiredSessions(ctx, sessionDays)
	if err != nil {
		return fmt.Errorf("cleanup expired sessions: %w", err)
	}
	h.logger.Info("expired sessions cleaned up", "deleted", sessionsDeleted)

	eventsDeleted, err := h.cleanupStaleProcessingEvents(ctx, eventDays)
	if err != nil {
		return fmt.Errorf("cleanup stale processing events: %w", err)
	}
	h.logger.Info("stale processing events cleaned up", "deleted", eventsDeleted)

	return nil
}

// cleanupExpiredSessions removes user sessions that have been idle longer than
// the specified number of days. Sessions are stored in the sessions table with
// an expires_at column; rows past that timestamp are deleted.
func (h *CleanupHandler) cleanupExpiredSessions(ctx context.Context, maxAgeDays int) (int64, error) {
	// Delete sessions where the last activity (or creation) is older than maxAgeDays.
	// The sessions table uses expires_at for absolute expiry. If it does not exist yet
	// (table created in a later migration), this is a no-op.
	tag, err := h.pool.Exec(ctx,
		`DELETE FROM sessions WHERE expires_at < NOW() - make_interval(days => $1)`,
		maxAgeDays,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			// Table not yet created by migrations — treat as no-op.
			h.logger.Warn("sessions table does not exist yet, skipping cleanup", "error", err)
			return 0, nil
		}
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// cleanupStaleProcessingEvents removes processing_events older than the
// specified retention period.
func (h *CleanupHandler) cleanupStaleProcessingEvents(ctx context.Context, retentionDays int) (int64, error) {
	tag, err := h.pool.Exec(ctx,
		`DELETE FROM processing_events WHERE created_at < NOW() - make_interval(days => $1)`,
		retentionDays,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			h.logger.Warn("processing_events table does not exist yet, skipping cleanup", "error", err)
			return 0, nil
		}
		return 0, fmt.Errorf("delete stale processing events: %w", err)
	}
	return tag.RowsAffected(), nil
}
