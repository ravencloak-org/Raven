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

// MarketplaceSlugHoldSweepPayload is the payload for the daily slug-hold
// sweep job. Empty in production; the OrgID hook is reserved for tests
// that want to scope a run to a single tenant without polluting other
// fixtures. Per docs/plans/marketplace-mvp.md §3, this job deletes
// both `kb_slug_holds` and `org_slug_holds` rows whose 90-day /
// 30-day windows have elapsed (ADR-0007).
type MarketplaceSlugHoldSweepPayload struct {
	// OrgID optionally restricts the sweep to a single organisation.
	// Production runs leave this empty so the sweep is global.
	OrgID string `json:"org_id,omitempty"`
}

// NewMarketplaceSlugHoldSweepTask creates an Asynq task for the slug
// hold sweep cron job. Mirrors the constructor shape used by every
// other scheduled task in this package so wiring stays uniform.
func NewMarketplaceSlugHoldSweepTask(p MarketplaceSlugHoldSweepPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal MarketplaceSlugHoldSweepPayload: %w", err)
	}
	return asynq.NewTask(TypeMarketplaceSlugHoldSweep, data), nil
}

// MarketplaceSlugHoldSweepHandler handles the scheduled sweep of
// expired Marketplace slug holds. Runs daily; deletes rows whose
// `held_until < now()` from both `kb_slug_holds` (ADR-0007, 90-day
// window) and `org_slug_holds` (ADR-0007 Q7e, 30-day window). The
// operation is idempotent — a run that deletes nothing is a healthy
// outcome and not an error.
type MarketplaceSlugHoldSweepHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewMarketplaceSlugHoldSweepHandler creates a handler. Pool is
// required; logger falls back to slog.Default() when nil so unit
// tests can omit it.
func NewMarketplaceSlugHoldSweepHandler(pool *pgxpool.Pool, logger *slog.Logger) *MarketplaceSlugHoldSweepHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &MarketplaceSlugHoldSweepHandler{
		pool:   pool,
		logger: logger,
	}
}

// ProcessTask implements asynq.Handler. The per-task-type deadline is
// applied by DeadlineMiddleware via mux.Use in scheduler.go, so this
// handler does not wrap ctx itself.
//
// Each table is swept in its own statement so that a pre-existing
// schema (a fresh dev DB without 00051 applied yet, for example)
// doesn't take down the whole job — the 42P01 "undefined_table"
// SQLSTATE is treated as a no-op for that table only.
func (h *MarketplaceSlugHoldSweepHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload MarketplaceSlugHoldSweepPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal MarketplaceSlugHoldSweepPayload: %w", err)
	}

	h.logger.Info("starting marketplace slug-hold sweep",
		"org_id_scope", payload.OrgID,
	)

	kbDeleted, err := h.sweepKBHolds(ctx, payload.OrgID)
	if err != nil {
		return fmt.Errorf("sweep kb_slug_holds: %w", err)
	}
	h.logger.Info("expired kb slug holds deleted", "deleted", kbDeleted)

	orgDeleted, err := h.sweepOrgHolds(ctx, payload.OrgID)
	if err != nil {
		return fmt.Errorf("sweep org_slug_holds: %w", err)
	}
	h.logger.Info("expired org slug holds deleted", "deleted", orgDeleted)

	return nil
}

// sweepKBHolds deletes expired rows from `kb_slug_holds`. When orgID
// is non-empty the delete is scoped to that org — used by tests; the
// production cron always passes an empty payload.
func (h *MarketplaceSlugHoldSweepHandler) sweepKBHolds(ctx context.Context, orgID string) (int64, error) {
	var (
		tag pgconn.CommandTag
		err error
	)
	if orgID == "" {
		tag, err = h.pool.Exec(ctx,
			`DELETE FROM kb_slug_holds WHERE held_until < NOW()`,
		)
	} else {
		tag, err = h.pool.Exec(ctx,
			`DELETE FROM kb_slug_holds WHERE held_until < NOW() AND org_id = $1`,
			orgID,
		)
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			h.logger.Warn("kb_slug_holds table does not exist yet, skipping sweep", "error", err)
			return 0, nil
		}
		return 0, fmt.Errorf("delete expired kb slug holds: %w", err)
	}
	return tag.RowsAffected(), nil
}

// sweepOrgHolds deletes expired rows from `org_slug_holds`. The org
// slug-hold table predates this sweeper (migration 00048) and was
// previously expected to be cleaned out by an admin tool; the plan
// (`docs/plans/marketplace-mvp.md` §3) consolidates both windows into
// this one job so ops have a single cron to watch.
func (h *MarketplaceSlugHoldSweepHandler) sweepOrgHolds(ctx context.Context, orgID string) (int64, error) {
	var (
		tag pgconn.CommandTag
		err error
	)
	if orgID == "" {
		tag, err = h.pool.Exec(ctx,
			`DELETE FROM org_slug_holds WHERE held_until < NOW()`,
		)
	} else {
		tag, err = h.pool.Exec(ctx,
			`DELETE FROM org_slug_holds WHERE held_until < NOW() AND org_id = $1`,
			orgID,
		)
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			h.logger.Warn("org_slug_holds table does not exist yet, skipping sweep", "error", err)
			return 0, nil
		}
		return 0, fmt.Errorf("delete expired org slug holds: %w", err)
	}
	return tag.RowsAffected(), nil
}
