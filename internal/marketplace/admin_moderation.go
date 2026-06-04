// Package marketplace — admin_moderation: the report-driven approve/dismiss
// service. Implements the four-side-effect atomic Approve and the trivial
// Dismiss for the admin review queue (#734, ADR-0006, ADR-0008).
//
// Why a dedicated file:
//
//   - reports.go and takedowns.go are repository-shaped (one table each,
//     one statement per method). The admin approve action crosses both
//     tables PLUS knowledge_bases.visibility PLUS organizations
//     .takedown_strikes. Putting that multi-table orchestration inside
//     either repo blurs the layering.
//   - The admin path uses SET LOCAL ROLE raven_admin to satisfy the
//     admin-only RLS policies. Bundling that ceremony into the repo
//     methods would force every other caller (reporter creating a
//     report, etc.) to know about the role-switch.
//
// The handler layer (HTTP boundary) enforces who is allowed to call
// these methods; this layer trusts the caller and focuses on atomicity.
package marketplace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PublisherNotifier sends the "your KB was taken down" email per
// ADR-0006. The default no-op implementation lets the admin endpoint
// ship before the email pipeline is wired (M9 SES + summaries handles
// general transactional mail).
//
// Notify is best-effort: the admin endpoint logs failures and returns
// success because the takedown itself has already committed. Productionising
// this should swap the no-op for an Asynq enqueue with the
// marketplace_takedown_notifier handler (plan §3 / §M3) so the email
// retries with dead-letter on persistent failure.
type PublisherNotifier interface {
	NotifyTakedown(ctx context.Context, kbID uuid.UUID, reason string) error
}

// noopPublisherNotifier is the safe default. Logs a TODO once so an
// operator can see that the email side-effect isn't actually being sent.
type noopPublisherNotifier struct{}

// NotifyTakedown is a no-op that logs the would-be email at INFO. We
// log at INFO not WARN because this is an expected state until the
// email pipeline is wired — a WARN would fire on every approve and
// drown legitimate alerts.
func (noopPublisherNotifier) NotifyTakedown(ctx context.Context, kbID uuid.UUID, reason string) error {
	slog.InfoContext(ctx, "marketplace: TODO wire publisher takedown email",
		"kb_id", kbID, "reason_len", len(reason))
	return nil
}

// NewNoopPublisherNotifier returns the default no-op notifier. Exported
// so cmd/api/main.go can wire it explicitly (the explicit wire is
// preferable to a nil-default that surprises a reader scanning main.go
// for "what notifies the publisher?").
func NewNoopPublisherNotifier() PublisherNotifier {
	return noopPublisherNotifier{}
}

// AdminModeration handles the admin review-queue write paths: approve
// (escalate to takedown) and dismiss (close without action).
//
// All methods assume the caller has already been gated by the HTTP-layer
// platform-admin middleware. The service trusts that gate; double-gating
// here would couple the service to the auth model.
type AdminModeration struct {
	pool      *pgxpool.Pool
	reports   *Reports
	takedowns *Takedowns
	notifier  PublisherNotifier
}

// NewAdminModeration constructs an AdminModeration service. A nil notifier
// is treated as the no-op notifier — callers that want a real notifier
// must wire one explicitly.
func NewAdminModeration(pool *pgxpool.Pool, reports *Reports, takedowns *Takedowns, notifier PublisherNotifier) *AdminModeration {
	if notifier == nil {
		notifier = noopPublisherNotifier{}
	}
	return &AdminModeration{
		pool:      pool,
		reports:   reports,
		takedowns: takedowns,
		notifier:  notifier,
	}
}

// ApproveResult is what the admin endpoint returns on a successful
// Approve. Fields match the response shape in plan §4.
type ApproveResult struct {
	// TakedownID is the primary key of the freshly inserted
	// marketplace_takedowns row.
	TakedownID uuid.UUID `json:"takedown_id"`
	// TargetKBID is the KB that was unpublished.
	TargetKBID uuid.UUID `json:"target_kb_id"`
	// StrikesAfter is the publisher Org's takedown_strikes value after
	// the increment. The frontend uses this to decide whether to surface
	// a "third strike — suspension review required" banner per ADR-0006.
	StrikesAfter int64 `json:"strikes_after"`
}

// Approve runs the four-side-effect atomic transaction described in
// ADR-0006: drive the report through open -> reviewing -> resolved,
// write a `source='admin'` takedown row, flip the target KB to
// `visibility='private'`, and increment the publisher Org's
// `takedown_strikes`. All four happen in a single transaction; if any
// step fails the whole thing rolls back.
//
// The publisher email is sent OUTSIDE the transaction. The justification:
//   - The DB transaction is the source of truth. Once committed, the
//     takedown is real regardless of whether the email goes out.
//   - SMTP latency under a held DB transaction would extend lock-hold
//     time for no benefit.
//   - Email failure must not roll back a confirmed takedown — the
//     publisher is still entitled to know, just via a retry-able async
//     path. Productionising swaps the inline call for an Asynq enqueue.
//
// Errors:
//   - ErrReportNotFound: id does not exist.
//   - ErrIllegalTransition: report has already been resolved/dismissed
//     and cannot be approved again. HTTP layer surfaces this as 409.
//   - Any pgx error from the underlying round-trips.
func (s *AdminModeration) Approve(ctx context.Context, reportID uuid.UUID) (ApproveResult, error) {
	var result ApproveResult
	var reasonForEmail string

	if err := s.runAdminTx(ctx, func(tx pgx.Tx) error {
		// 1. Load the report so we have target_kb_id + reason for the
		//    takedown row + email. Done first under FOR-UPDATE-equivalent
		//    semantics in step 3 below — here we just need the body.
		rep, err := s.reports.GetByID(ctx, tx, reportID)
		if err != nil {
			return err
		}
		reasonForEmail = rep.Reason

		// 2. Move report through the state machine. open -> reviewing,
		//    then reviewing -> resolved. Two transitions because the
		//    state-machine table (moderation.go) enforces open is not
		//    directly terminal-reachable — that invariant is shared with
		//    the DB CHECK and we honour it here rather than special-casing
		//    the admin path. FOR UPDATE inside TransitionInTx prevents a
		//    concurrent Approve race.
		if err := s.reports.TransitionInTx(ctx, tx, reportID, ReportStatusReviewing); err != nil {
			return err
		}
		if err := s.reports.TransitionInTx(ctx, tx, reportID, ReportStatusResolved); err != nil {
			return err
		}

		// 3. Flip the target KB to private. The UPDATE + RETURNING
		//    pattern verifies the KB still exists (a concurrent hard-
		//    delete during review would surface as ErrNoRows here and
		//    roll the whole tx back) and gives us the org_id we need to
		//    increment strikes against. RLS is bypassed because we are
		//    under SET LOCAL ROLE raven_admin.
		var publisherOrgID uuid.UUID
		if err := tx.QueryRow(ctx,
			`UPDATE knowledge_bases
			    SET visibility = 'private'
			  WHERE id = $1
			  RETURNING org_id`,
			rep.ReportedKBID,
		).Scan(&publisherOrgID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("Approve: target KB %s missing: %w", rep.ReportedKBID, ErrReportNotFound)
			}
			return fmt.Errorf("Approve: unpublish KB: %w", err)
		}

		// 4. Increment the publisher Org's strike counter. RETURNING the
		//    new value gives the handler something to display in the
		//    "this was their N-th strike" banner.
		if err := tx.QueryRow(ctx,
			`UPDATE organizations
			    SET takedown_strikes = takedown_strikes + 1
			  WHERE id = $1
			  RETURNING takedown_strikes`,
			publisherOrgID,
		).Scan(&result.StrikesAfter); err != nil {
			return fmt.Errorf("Approve: increment strikes: %w", err)
		}

		// 5. Write the takedown audit-log row. Last in the chain so that
		//    if any earlier step had rolled back, no orphan audit row
		//    exists.
		td, err := s.takedowns.CreateInTx(ctx, tx, rep.ReportedKBID, TakedownSourceAdmin, rep.Reason)
		if err != nil {
			return fmt.Errorf("Approve: takedown insert: %w", err)
		}
		result.TakedownID = td.ID
		result.TargetKBID = td.TargetKBID
		return nil
	}); err != nil {
		return ApproveResult{}, err
	}

	// Email is best-effort and runs OUTSIDE the transaction. A logged
	// failure here is acceptable — the takedown has committed.
	if err := s.notifier.NotifyTakedown(ctx, result.TargetKBID, reasonForEmail); err != nil {
		slog.WarnContext(ctx, "marketplace: publisher takedown notification failed",
			"kb_id", result.TargetKBID, "error", err)
	}

	return result, nil
}

// Dismiss drives the report through open -> reviewing -> dismissed.
// No takedown, no strike, no email — ADR-0006 explicitly says the
// reporter is NOT notified of dismissal (harassment-vector protection).
//
// Same error vocabulary as Approve.
func (s *AdminModeration) Dismiss(ctx context.Context, reportID uuid.UUID) error {
	return s.runAdminTx(ctx, func(tx pgx.Tx) error {
		if err := s.reports.TransitionInTx(ctx, tx, reportID, ReportStatusReviewing); err != nil {
			return err
		}
		return s.reports.TransitionInTx(ctx, tx, reportID, ReportStatusDismissed)
	})
}

// runAdminTx opens a transaction, switches to the raven_admin role
// (required for the admin-bypass RLS policies on marketplace_reports
// and marketplace_takedowns), runs fn, and commits. Rolls back on any
// returned error.
//
// SET LOCAL is used (not SET) so the role reverts to the pool
// connection's default when the transaction ends — no leaked admin
// privilege on the next caller to grab this connection.
func (s *AdminModeration) runAdminTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("admin tx begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SET LOCAL ROLE raven_admin"); err != nil {
		return fmt.Errorf("admin tx set role: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ListReports is the admin-only paginated review queue read. Wraps
// Reports.List but runs the query under SET LOCAL ROLE raven_admin
// so the admin_bypass RLS policy lets the SELECT see every reporter's
// rows.
//
// The handler layer takes (status, limit, offset) from the query string
// and forwards them. status validation is delegated to Reports.List.
func (s *AdminModeration) ListReports(ctx context.Context, status ReportStatus, limit, offset int) ([]Report, error) {
	if !status.IsValid() {
		return nil, ErrInvalidReportStatus
	}
	if limit <= 0 {
		return nil, fmt.Errorf("AdminModeration.ListReports: limit must be positive, got %d", limit)
	}
	if offset < 0 {
		return nil, fmt.Errorf("AdminModeration.ListReports: offset must be non-negative, got %d", offset)
	}

	var out []Report
	if err := s.runAdminTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id, reported_kb_id, reporter_user_id, reason, status, created_at
			 FROM marketplace_reports
			 WHERE status = $1
			 ORDER BY created_at ASC
			 LIMIT $2 OFFSET $3`,
			status, limit, offset,
		)
		if err != nil {
			return fmt.Errorf("AdminModeration.ListReports: query: %w", err)
		}
		defer rows.Close()

		out = make([]Report, 0, limit)
		for rows.Next() {
			var rep Report
			var reporter uuid.NullUUID
			if err := rows.Scan(
				&rep.ID, &rep.ReportedKBID, &reporter,
				&rep.Reason, &rep.Status, &rep.CreatedAt,
			); err != nil {
				return fmt.Errorf("AdminModeration.ListReports: scan: %w", err)
			}
			if reporter.Valid {
				uid := reporter.UUID
				rep.ReporterUserID = &uid
			}
			out = append(out, rep)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}
	return out, nil
}
