// Package jobs — marketplace_dmca_sweeper: daily cron handler that
// auto-resolves DMCA notices whose 14-day counter-notice window has
// expired (issue #736, ADR-0006 launch blocker).
//
// The actual orchestration (re-checking each row under FOR UPDATE,
// flipping the KB, writing the takedown audit row, dispatching the
// OnTakedownCreated registry) lives in marketplace.DMCAService —
// keeping it in the service layer means the integration test can call
// it directly without spinning up an Asynq harness.
//
// This handler is the thin cron seam: unmarshal the payload, invoke
// the service, log the structured counter, return any orchestration
// error so Asynq retries per the configured MaxRetry.

package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/ravencloak-org/Raven/internal/marketplace"
)

// DMCASweeper is the Asynq handler for TypeMarketplaceDMCASweep.
type DMCASweeper struct {
	svc    *marketplace.DMCAService
	logger *slog.Logger
}

// NewDMCASweeper returns a handler bound to svc. A nil logger falls
// back to slog.Default so the handler never panics for a missing
// dependency.
func NewDMCASweeper(svc *marketplace.DMCAService, logger *slog.Logger) *DMCASweeper {
	if logger == nil {
		logger = slog.Default()
	}
	return &DMCASweeper{svc: svc, logger: logger}
}

// ProcessTask implements asynq.Handler. Decodes the payload (currently
// empty — the sweep always scans every expired pending notice),
// invokes SweepExpired, and emits a structured log line.
//
// Errors are propagated so Asynq retries per MaxRetry. The DMCAService
// already swallows per-notice failures and continues the loop — a
// non-nil return here indicates a setup-level failure (DB pool down,
// payload parse error) where retry is the right answer.
func (h *DMCASweeper) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload MarketplaceDMCASweepPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal MarketplaceDMCASweepPayload: %w", err)
	}
	if h.svc == nil {
		// Defensive: the cron is registered before the service in
		// cmd/api/main.go only if a wiring bug lands. Emit a structured
		// warn and no-op so the queue doesn't accumulate retry pressure
		// against a known-broken handler.
		h.logger.WarnContext(ctx, "DMCA sweeper invoked without service wired; skipping")
		return nil
	}

	h.logger.InfoContext(ctx, "starting DMCA counter-notice sweep")
	result, err := h.svc.SweepExpired(ctx)
	if err != nil {
		return fmt.Errorf("DMCA sweep: %w", err)
	}
	h.logger.InfoContext(ctx, "DMCA counter-notice sweep complete",
		"examined", result.Examined,
		"resolved", result.Resolved,
	)
	return nil
}
