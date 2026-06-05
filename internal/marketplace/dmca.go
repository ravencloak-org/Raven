// Package marketplace — dmca: the DMCA inbox + two-stage counter-notice
// workflow service (issue #736, launch blocker per ADR-0006).
//
// The DMCA flow has three distinct write paths, each isolated behind
// SET LOCAL ROLE raven_admin so the admin-bypass RLS policy on
// dmca_notices (and marketplace_takedowns from #735) lets the writes
// through:
//
//  1. Submit — admin records a freshly arrived notice. Atomic: insert
//     the notice row AND flip the target KB into `kb_status='dmca_pending'`.
//     The 14-day counter-notice window clock is materialised at this
//     point (counter_notice_window_ends = now() + 14d) so the sweeper
//     can run a flat `< now()` filter without recomputing per row.
//     The claimant receipt email is sent OUTSIDE the transaction — same
//     justification as AdminModeration.Approve (the takedown decision
//     is the durable record; email is best-effort retry-able async).
//
//  2. SubmitCounterNotice — admin records a publisher counter-notice
//     (MVP simplification: admin acts on behalf of the publisher, who
//     replies via the dmca@ravencloak.org inbox; there is no public
//     counter-notice UI in MVP). Transitions the row to `counter_filed`
//     and stamps counter_notice_submitted_at. The KB stays in
//     `dmca_pending` until an admin makes the final keep-up / take-down
//     decision — we do NOT thaw to `active` on counter-notice submission
//     because the DMCA safe harbour requires the OSP to hold the
//     restoration for 10-14 business days to give the claimant time to
//     file suit (17 U.S.C. § 512(g)(2)(C)). The final decision is a
//     manual admin step in MVP; productionising adds a second timer.
//
//  3. The sweeper (jobs/marketplace_dmca_sweeper.go) drives the
//     auto-takedown for pending notices whose window has expired. The
//     sweep is a separate service method (SweepExpired) so the cron
//     handler stays thin and the integration test can call it directly.
//
// Why a dedicated file (mirrors admin_moderation.go's rationale):
//   - The dmca_notices repository CRUD does not warrant its own file
//     for a single-table, three-statement service. Combining the table
//     access with the multi-table orchestration keeps the call sites
//     in one place.
//   - The KB-status flip cuts across knowledge_bases; rather than
//     duplicate UPDATE statements in a hypothetical dmca_repo.go, we
//     keep the orchestration here next to its sibling Approve flow.
//
// HTTP gating: every public method assumes the caller has already
// passed RequireRavenAdmin. The service does NOT re-check the gate.

package marketplace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CounterNoticeWindow is the statutory 14-day window during which a
// publisher can file a counter-notice before the auto-takedown sweeper
// finalises the DMCA action. The window length is hard-coded — ADR-0006
// pins it and the DMCA statute does not allow shortening.
//
// Exported so tests can refer to the same constant when seeding
// past-due notices without hard-coding "14*24*time.Hour" in two places.
const CounterNoticeWindow = 14 * 24 * time.Hour

// DMCAStatus is the canonical type for dmca_notices.status. The state
// machine is intentionally simple — terminal states are write-once
// from the service layer's perspective:
//
//	pending           -> counter_filed | resolved_take_down | withdrawn
//	counter_filed     -> resolved_keep_up | resolved_take_down
//	resolved_*        -> (terminal)
//	withdrawn         -> (terminal)
type DMCAStatus string

const (
	// DMCAStatusPending is the initial state — notice received, the
	// 14-day counter-notice clock is ticking, the KB is frozen.
	DMCAStatusPending DMCAStatus = "pending"

	// DMCAStatusCounterFiled means the publisher has filed a counter-
	// notice. The KB stays frozen until the admin makes the final call.
	DMCAStatusCounterFiled DMCAStatus = "counter_filed"

	// DMCAStatusResolvedTakeDown is the terminal "claimant wins"
	// outcome — KB stays private, takedown audit row exists. The
	// sweeper writes this row when the window expires with no counter-
	// notice; admin can also set it manually after reviewing a
	// counter-notice.
	DMCAStatusResolvedTakeDown DMCAStatus = "resolved_take_down"

	// DMCAStatusResolvedKeepUp is the terminal "publisher wins" or
	// "claimant withdrew" outcome — KB restored. MVP does not expose
	// the restore button to admins (out of scope for the launch
	// blocker); the status is here so the runbook can document the
	// post-MVP restoration path.
	DMCAStatusResolvedKeepUp DMCAStatus = "resolved_keep_up"

	// DMCAStatusWithdrawn means the claimant rescinded the notice
	// before resolution. Same end state as resolved_keep_up but
	// auditable as a distinct event.
	DMCAStatusWithdrawn DMCAStatus = "withdrawn"
)

// IsValid reports whether s is one of the five legal DMCAStatus values.
// The DB CHECK is the backstop; this guards inputs at the service edge.
func (s DMCAStatus) IsValid() bool {
	switch s {
	case DMCAStatusPending, DMCAStatusCounterFiled,
		DMCAStatusResolvedTakeDown, DMCAStatusResolvedKeepUp,
		DMCAStatusWithdrawn:
		return true
	}
	return false
}

// DMCANotice is the wire+storage shape for a row in dmca_notices.
// Optional fields use pointers so a NULL counter-notice does not
// surface as the empty string in JSON.
type DMCANotice struct {
	ID                       uuid.UUID  `json:"id"`
	TargetKBID               uuid.UUID  `json:"target_kb_id"`
	NoticeText               string     `json:"notice_text"`
	ClaimantEmail            string     `json:"claimant_email"`
	ClaimantName             string     `json:"claimant_name"`
	CounterNoticeText        *string    `json:"counter_notice_text,omitempty"`
	CounterNoticeSubmittedAt *time.Time `json:"counter_notice_submitted_at,omitempty"`
	CounterNoticeWindowEnds  time.Time  `json:"counter_notice_window_ends"`
	Status                   DMCAStatus `json:"status"`
	ResolvedAt               *time.Time `json:"resolved_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
}

// DMCANoticeInput is the payload accepted by Submit. NoticeText length
// is validated server-side against the same 1..8192 range the DB
// CHECK enforces — we surface the 400 from the service rather than
// letting a pgx CHECK violation bubble up as an opaque 500.
type DMCANoticeInput struct {
	TargetKBID    uuid.UUID
	NoticeText    string
	ClaimantEmail string
	ClaimantName  string
}

// Validate runs the inline payload checks. Returns nil on success.
func (in DMCANoticeInput) Validate() error {
	if in.TargetKBID == uuid.Nil {
		return ErrDMCAInvalidInput
	}
	if l := len(in.NoticeText); l < 1 || l > 8192 {
		return ErrDMCAInvalidInput
	}
	if in.ClaimantEmail == "" || in.ClaimantName == "" {
		return ErrDMCAInvalidInput
	}
	return nil
}

// Sentinel errors. The HTTP handler maps these to status codes.
var (
	// ErrDMCAInvalidInput — Submit/SubmitCounterNotice payload failed
	// the inline validation. Surfaces as 400.
	ErrDMCAInvalidInput = errors.New("marketplace: invalid DMCA input")

	// ErrDMCATargetKBNotFound — the target KB does not exist. Surfaces
	// as 404. A KB hard-deleted mid-flight surfaces here too because
	// the FOR UPDATE in Submit returns ErrNoRows.
	ErrDMCATargetKBNotFound = errors.New("marketplace: DMCA target KB not found")

	// ErrDMCAAlreadyPending — the KB already has a pending DMCA notice.
	// Surfaces as 409. The DMCA workflow is one-active-notice-per-KB
	// in MVP; a second notice would race the sweep timer and confuse
	// the audit log.
	ErrDMCAAlreadyPending = errors.New("marketplace: DMCA notice already pending for KB")

	// ErrDMCANoticeNotFound — the notice id supplied to
	// SubmitCounterNotice does not exist. Surfaces as 404.
	ErrDMCANoticeNotFound = errors.New("marketplace: DMCA notice not found")

	// ErrDMCAIllegalTransition — counter-notice was filed against a
	// notice that is not in `pending`. Surfaces as 409.
	ErrDMCAIllegalTransition = errors.New("marketplace: illegal DMCA status transition")
)

// DMCAClaimantNotifier sends the "we received your notice" receipt
// email. Default is a no-op (logs a TODO at INFO) — see PublisherNotifier
// in admin_moderation.go for the same pattern and rationale.
type DMCAClaimantNotifier interface {
	NotifyClaimantReceipt(ctx context.Context, notice DMCANotice) error
}

// noopDMCAClaimantNotifier is the safe default. Logs a TODO at INFO
// so an operator can see the would-be email.
type noopDMCAClaimantNotifier struct{}

func (noopDMCAClaimantNotifier) NotifyClaimantReceipt(ctx context.Context, n DMCANotice) error {
	slog.InfoContext(ctx, "marketplace: TODO wire DMCA claimant receipt email",
		"notice_id", n.ID, "kb_id", n.TargetKBID, "claimant_email", n.ClaimantEmail)
	return nil
}

// NewNoopDMCAClaimantNotifier returns the no-op notifier. Exported so
// cmd/api/main.go wires it explicitly.
func NewNoopDMCAClaimantNotifier() DMCAClaimantNotifier {
	return noopDMCAClaimantNotifier{}
}

// DMCAService handles the DMCA inbox + counter-notice + sweep flow.
//
// All transactional writes use SET LOCAL ROLE raven_admin (the
// dmca_notices and marketplace_takedowns tables are admin-only via
// RLS). SET LOCAL is used (not SET) so the role reverts when the
// transaction ends — no leaked admin privilege on the next caller to
// grab this pool connection.
type DMCAService struct {
	pool      *pgxpool.Pool
	takedowns *Takedowns
	notifier  DMCAClaimantNotifier

	// now is overridable for the sweeper's time-travel tests. Production
	// callers leave it nil and the service uses time.Now.
	now func() time.Time
}

// NewDMCAService constructs a DMCAService bound to pool and the takedown
// audit writer. A nil notifier is treated as the no-op notifier.
func NewDMCAService(pool *pgxpool.Pool, takedowns *Takedowns, notifier DMCAClaimantNotifier) *DMCAService {
	if notifier == nil {
		notifier = noopDMCAClaimantNotifier{}
	}
	return &DMCAService{
		pool:      pool,
		takedowns: takedowns,
		notifier:  notifier,
	}
}

// WithClock overrides the service's time source. Test-only — production
// callers should not use this.
func (s *DMCAService) WithClock(now func() time.Time) *DMCAService {
	s.now = now
	return s
}

// nowFn returns the service's time source, defaulting to time.Now.
func (s *DMCAService) nowFn() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// Submit records a fresh DMCA notice. Atomic two-table write:
//
//  1. INSERT the dmca_notices row with counter_notice_window_ends =
//     now() + 14d and status='pending'.
//  2. UPDATE the target KB to status='dmca_pending' (the kb_status
//     gate downstream blocks chat/ingestion while the hold is active).
//
// Both side-effects commit together; if either fails the whole thing
// rolls back. The claimant receipt email is sent OUTSIDE the
// transaction (best-effort, logged on failure).
//
// Errors:
//   - ErrDMCAInvalidInput: payload validation failed (400).
//   - ErrDMCATargetKBNotFound: target_kb_id does not resolve (404).
//   - ErrDMCAAlreadyPending: KB already has a pending notice (409).
//   - Any pgx error from the underlying round-trips (500).
func (s *DMCAService) Submit(ctx context.Context, in DMCANoticeInput) (DMCANotice, error) {
	if err := in.Validate(); err != nil {
		return DMCANotice{}, err
	}

	var notice DMCANotice
	if err := s.runAdminTx(ctx, func(tx pgx.Tx) error {
		// 1. Guard: reject if a pending notice already exists for this
		//    KB. The unique-active invariant is enforced at the service
		//    layer rather than via a partial unique index because we
		//    want a structured 409 here, not a constraint-violation
		//    bubble-up at INSERT time. A partial unique index would also
		//    race against the sweep auto-resolve in a tight window.
		var existingCount int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM dmca_notices
			  WHERE target_kb_id = $1 AND status = 'pending'`,
			in.TargetKBID,
		).Scan(&existingCount); err != nil {
			return fmt.Errorf("DMCAService.Submit: dup check: %w", err)
		}
		if existingCount > 0 {
			return ErrDMCAAlreadyPending
		}

		// 2. Verify the target KB exists. UPDATE … RETURNING gives us
		//    both the existence check and the status flip in one round-
		//    trip. The kb_status enum value 'dmca_pending' was added in
		//    migration 00049.
		var kbID uuid.UUID
		if err := tx.QueryRow(ctx,
			`UPDATE knowledge_bases
			    SET status = 'dmca_pending'
			  WHERE id = $1
			  RETURNING id`,
			in.TargetKBID,
		).Scan(&kbID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrDMCATargetKBNotFound
			}
			return fmt.Errorf("DMCAService.Submit: KB status flip: %w", err)
		}

		// 3. Insert the notice row. Materialise the 14-day window now
		//    so the sweep query is a flat `< now()` filter.
		windowEnds := s.nowFn().Add(CounterNoticeWindow)
		if err := tx.QueryRow(ctx,
			`INSERT INTO dmca_notices
			   (target_kb_id, notice_text, claimant_email, claimant_name,
			    counter_notice_window_ends, status)
			 VALUES ($1, $2, $3, $4, $5, 'pending')
			 RETURNING id, target_kb_id, notice_text, claimant_email,
			           claimant_name, counter_notice_text,
			           counter_notice_submitted_at,
			           counter_notice_window_ends, status,
			           resolved_at, created_at`,
			in.TargetKBID, in.NoticeText, in.ClaimantEmail, in.ClaimantName,
			windowEnds,
		).Scan(
			&notice.ID, &notice.TargetKBID, &notice.NoticeText,
			&notice.ClaimantEmail, &notice.ClaimantName,
			&notice.CounterNoticeText, &notice.CounterNoticeSubmittedAt,
			&notice.CounterNoticeWindowEnds, &notice.Status,
			&notice.ResolvedAt, &notice.CreatedAt,
		); err != nil {
			return fmt.Errorf("DMCAService.Submit: notice insert: %w", err)
		}
		return nil
	}); err != nil {
		return DMCANotice{}, err
	}

	// Email outside the transaction. A logged failure is acceptable —
	// the notice + KB freeze have committed.
	if err := s.notifier.NotifyClaimantReceipt(ctx, notice); err != nil {
		slog.WarnContext(ctx, "marketplace: DMCA claimant receipt failed",
			"notice_id", notice.ID, "claimant_email", notice.ClaimantEmail,
			"error", err)
	}
	return notice, nil
}

// SubmitCounterNotice records a publisher counter-notice on the named
// notice. The DB row pivots to `counter_filed`; the KB stays in
// `dmca_pending` (see file header for the safe-harbour rationale).
//
// Errors:
//   - ErrDMCAInvalidInput: counterText empty.
//   - ErrDMCANoticeNotFound: id does not resolve (404).
//   - ErrDMCAIllegalTransition: notice is not in `pending` (409).
//   - Any pgx error (500).
func (s *DMCAService) SubmitCounterNotice(ctx context.Context, noticeID uuid.UUID, counterText string) error {
	if noticeID == uuid.Nil || counterText == "" {
		return ErrDMCAInvalidInput
	}
	if len(counterText) > 8192 {
		return ErrDMCAInvalidInput
	}

	return s.runAdminTx(ctx, func(tx pgx.Tx) error {
		// FOR UPDATE locks the row so a concurrent sweep cannot auto-
		// resolve it underneath us mid-counter-notice.
		var current DMCAStatus
		if err := tx.QueryRow(ctx,
			`SELECT status FROM dmca_notices WHERE id = $1 FOR UPDATE`,
			noticeID,
		).Scan(&current); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrDMCANoticeNotFound
			}
			return fmt.Errorf("DMCAService.SubmitCounterNotice: load: %w", err)
		}
		if current != DMCAStatusPending {
			return ErrDMCAIllegalTransition
		}

		now := s.nowFn()
		if _, err := tx.Exec(ctx,
			`UPDATE dmca_notices
			    SET counter_notice_text         = $2,
			        counter_notice_submitted_at = $3,
			        status                      = 'counter_filed'
			  WHERE id = $1`,
			noticeID, counterText, now,
		); err != nil {
			return fmt.Errorf("DMCAService.SubmitCounterNotice: update: %w", err)
		}
		return nil
	})
}

// SweepResult summarises a sweep run. Returned by SweepExpired so the
// cron handler can log structured counters and the integration test can
// assert per-run effects.
type SweepResult struct {
	// Examined is the number of pending notices the sweep scanned.
	Examined int
	// Resolved is the number successfully auto-resolved to
	// `resolved_take_down`. Examined - Resolved = errors hit per-row
	// (the sweep continues past a single failure).
	Resolved int
}

// SweepExpired auto-resolves every `pending` notice whose 14-day
// counter-notice window has expired. For each row it:
//
//  1. Transitions status -> resolved_take_down + stamps resolved_at.
//  2. Flips the target KB to visibility='private' AND status='active'
//     (so the kb_status gate releases — the legal hold has resolved
//     to a takedown decision; the KB now sits in the same state as
//     any publisher-initiated unpublish).
//  3. Writes a marketplace_takedowns row with source='dmca' and the
//     notice id in the notes column.
//  4. Dispatches the OnTakedownCreated registry (DerivativeNotifier
//     fires the lineage-walk email outside the transaction).
//
// One transaction per notice keeps a slow downstream notifier from
// holding the sweep's overall lock. Per-row errors are logged and the
// loop continues — a transient FK or network failure on row N must not
// block rows N+1..M.
//
// Called by the cron handler in jobs/marketplace_dmca_sweeper.go and
// directly by the integration test. The sweep is idempotent: re-running
// it picks up only rows still in `pending`.
func (s *DMCAService) SweepExpired(ctx context.Context) (SweepResult, error) {
	var result SweepResult
	now := s.nowFn()

	// Collect candidate IDs first under a short SELECT-only transaction
	// so we don't hold a snapshot across the per-row writes. Reading
	// under raven_admin role to satisfy the admin-bypass RLS policy.
	ids, err := s.listExpiredPending(ctx, now)
	if err != nil {
		return result, fmt.Errorf("DMCAService.SweepExpired: list: %w", err)
	}
	result.Examined = len(ids)

	// Per-notice resolution. Each call runs its own raven_admin tx and
	// fires the takedown dispatch outside that tx.
	for _, id := range ids {
		td, err := s.resolveExpiredOne(ctx, id, now)
		if err != nil {
			slog.ErrorContext(ctx, "marketplace: DMCA sweep notice failed",
				"notice_id", id, "error", err)
			continue
		}
		result.Resolved++

		// Outside-tx side-effect: fire the derivative-owner notifier.
		// The takedown row has committed; a notifier failure is
		// best-effort and must not block the sweep.
		if err := DispatchOnTakedownCreated(ctx, td, "dmca:"+id.String()); err != nil {
			slog.WarnContext(ctx, "marketplace: DMCA takedown dispatch failed",
				"notice_id", id, "takedown_id", td.ID, "error", err)
		}
	}
	return result, nil
}

// listExpiredPending returns notice IDs whose 14-day window has expired
// and which are still in `pending`. Runs under raven_admin so the
// admin-bypass RLS policy lets the SELECT see every row.
func (s *DMCAService) listExpiredPending(ctx context.Context, now time.Time) ([]uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("listExpiredPending: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SET LOCAL ROLE raven_admin"); err != nil {
		return nil, fmt.Errorf("listExpiredPending: set role: %w", err)
	}

	rows, err := tx.Query(ctx,
		`SELECT id FROM dmca_notices
		  WHERE status = 'pending' AND counter_notice_window_ends < $1
		  ORDER BY counter_notice_window_ends ASC`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("listExpiredPending: query: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("listExpiredPending: scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listExpiredPending: rows: %w", err)
	}
	return ids, tx.Commit(ctx)
}

// resolveExpiredOne atomically auto-resolves one expired pending
// notice. Returns the takedown audit row so the caller can dispatch
// the OnTakedownCreated registry outside the transaction.
//
// The same-row pivot: SELECT FOR UPDATE prevents a racing
// SubmitCounterNotice from re-stating the row while we resolve it.
// If the row has already transitioned (e.g. counter_filed mid-sweep),
// we skip it and return a sentinel that the caller logs at INFO.
func (s *DMCAService) resolveExpiredOne(ctx context.Context, noticeID uuid.UUID, now time.Time) (Takedown, error) {
	var td Takedown
	if err := s.runAdminTx(ctx, func(tx pgx.Tx) error {
		// Lock + re-check. Required: between listExpiredPending and
		// this transaction, a SubmitCounterNotice could have raced in.
		var current DMCAStatus
		var targetKBID uuid.UUID
		if err := tx.QueryRow(ctx,
			`SELECT status, target_kb_id FROM dmca_notices
			  WHERE id = $1 FOR UPDATE`,
			noticeID,
		).Scan(&current, &targetKBID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrDMCANoticeNotFound
			}
			return fmt.Errorf("resolveExpiredOne: re-load: %w", err)
		}
		if current != DMCAStatusPending {
			// Lost the race to a counter-notice. Treat as success-skip
			// (no error returned to the loop, caller increments
			// Resolved as 0 by virtue of this path NOT returning a
			// valid takedown row — see SweepExpired's check below).
			return errSweepSkip
		}

		// 1. Resolve the notice row.
		if _, err := tx.Exec(ctx,
			`UPDATE dmca_notices
			    SET status      = 'resolved_take_down',
			        resolved_at = $2
			  WHERE id = $1`,
			noticeID, now,
		); err != nil {
			return fmt.Errorf("resolveExpiredOne: resolve notice: %w", err)
		}

		// 2. Flip the target KB. We set visibility=private (it may
		//    already be) AND status=active (so the dmca_pending freeze
		//    lifts — the legal hold has been resolved into a takedown).
		//    The marketplace_takedowns row below is the durable audit
		//    record for the action.
		if _, err := tx.Exec(ctx,
			`UPDATE knowledge_bases
			    SET visibility = 'private',
			        status     = 'active'
			  WHERE id = $1`,
			targetKBID,
		); err != nil {
			return fmt.Errorf("resolveExpiredOne: KB flip: %w", err)
		}

		// 3. Write the takedown audit row. Notes column carries the
		//    notice id for forensic linkage; this is the same shape
		//    AdminModeration.Approve uses (notes=report.Reason).
		created, err := s.takedowns.CreateInTx(ctx, tx, targetKBID, TakedownSourceDMCA, "dmca_notice_id="+noticeID.String())
		if err != nil {
			return fmt.Errorf("resolveExpiredOne: takedown insert: %w", err)
		}
		td = created
		return nil
	}); err != nil {
		if errors.Is(err, errSweepSkip) {
			// Racing counter-notice — treat as non-fatal skip.
			slog.InfoContext(ctx, "marketplace: DMCA sweep skipped (counter-notice raced in)",
				"notice_id", noticeID)
			return Takedown{}, errSweepSkip
		}
		return Takedown{}, err
	}
	return td, nil
}

// errSweepSkip is a sentinel returned by resolveExpiredOne when a
// concurrent counter-notice raced in. SweepExpired handles it by
// continuing without counting it as a resolution.
var errSweepSkip = errors.New("marketplace: DMCA sweep skipped (counter-notice raced)")

// ListNotices returns paginated DMCA notices for the admin UI. status
// filters the list (empty means all). limit/offset honour the same
// caps as the report queue.
//
// Runs under SET LOCAL ROLE raven_admin so the admin-bypass RLS policy
// lets the cross-tenant read through.
func (s *DMCAService) ListNotices(ctx context.Context, status DMCAStatus, limit, offset int) ([]DMCANotice, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var out []DMCANotice
	if err := s.runAdminTx(ctx, func(tx pgx.Tx) error {
		var rows pgx.Rows
		var err error
		if status == "" {
			rows, err = tx.Query(ctx,
				`SELECT id, target_kb_id, notice_text, claimant_email,
				        claimant_name, counter_notice_text,
				        counter_notice_submitted_at,
				        counter_notice_window_ends, status,
				        resolved_at, created_at
				   FROM dmca_notices
				  ORDER BY created_at DESC
				  LIMIT $1 OFFSET $2`,
				limit, offset)
		} else {
			if !status.IsValid() {
				return ErrDMCAInvalidInput
			}
			rows, err = tx.Query(ctx,
				`SELECT id, target_kb_id, notice_text, claimant_email,
				        claimant_name, counter_notice_text,
				        counter_notice_submitted_at,
				        counter_notice_window_ends, status,
				        resolved_at, created_at
				   FROM dmca_notices
				  WHERE status = $1
				  ORDER BY created_at DESC
				  LIMIT $2 OFFSET $3`,
				status, limit, offset)
		}
		if err != nil {
			return fmt.Errorf("DMCAService.ListNotices: query: %w", err)
		}
		defer rows.Close()

		out = make([]DMCANotice, 0, limit)
		for rows.Next() {
			var n DMCANotice
			if err := rows.Scan(
				&n.ID, &n.TargetKBID, &n.NoticeText,
				&n.ClaimantEmail, &n.ClaimantName,
				&n.CounterNoticeText, &n.CounterNoticeSubmittedAt,
				&n.CounterNoticeWindowEnds, &n.Status,
				&n.ResolvedAt, &n.CreatedAt,
			); err != nil {
				return fmt.Errorf("DMCAService.ListNotices: scan: %w", err)
			}
			out = append(out, n)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// runAdminTx opens a transaction, switches to the raven_admin role,
// runs fn, and commits. Rolls back on any returned error. Identical
// shape to AdminModeration.runAdminTx — kept duplicated rather than
// promoted to a shared helper because the call sites are exactly two
// and the helper would have to live in a third file.
func (s *DMCAService) runAdminTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("DMCA admin tx begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SET LOCAL ROLE raven_admin"); err != nil {
		return fmt.Errorf("DMCA admin tx set role: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
