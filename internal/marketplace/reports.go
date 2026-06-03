package marketplace

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reports is the data-access repository for the marketplace_reports table.
//
// Construction takes a pgxpool.Pool; the repository does NOT manage
// transactions itself — callers wrap related operations (e.g. "create
// report and increment strike on KB" in #735's admin approval path) in
// their own pgx.Tx. The repository's job is one statement per method.
//
// RLS: every method runs under the caller's existing session context. The
// migration 00053 policies make a reporter see only their own rows; the
// admin path (List) MUST be called from a session that has switched to
// the raven_admin role — the repository does NOT change roles for you.
type Reports struct {
	pool *pgxpool.Pool
}

// NewReports returns a Reports repository backed by the given pool.
func NewReports(pool *pgxpool.Pool) *Reports {
	return &Reports{pool: pool}
}

// Create inserts a new report against kbID submitted by userID with the
// given free-text reason. Returns ErrInvalidReason if reason is empty or
// over 4000 chars, ErrRateLimit if the user already has MaxOpenReportsPerUser
// reports in the 'open' state, and any underlying pgx error otherwise.
//
// The rate-limit check and the INSERT run in a single transaction so a
// concurrent burst of submissions cannot slip past the cap (read-then-write
// without a transaction would race).
func (r *Reports) Create(ctx context.Context, kbID, userID uuid.UUID, reason string) (Report, error) {
	// Validate before opening a transaction — a malformed reason should
	// never even touch the connection pool.
	if l := len(reason); l < 1 || l > 4000 {
		return Report{}, ErrInvalidReason
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Report{}, fmt.Errorf("Reports.Create: begin: %w", err)
	}
	// Rollback is a no-op if Commit succeeded.
	defer func() { _ = tx.Rollback(ctx) }()

	// Per-user rate limit. Count the user's OPEN reports — once one moves
	// to reviewing/resolved/dismissed it stops counting against the cap,
	// which is the right shape: the limit exists so an attacker cannot
	// flood the review queue, and once admins process a row that pressure
	// is gone.
	var openCount int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM marketplace_reports
		 WHERE reporter_user_id = $1 AND status = $2`,
		userID, ReportStatusOpen,
	).Scan(&openCount); err != nil {
		return Report{}, fmt.Errorf("Reports.Create: count open: %w", err)
	}
	if openCount >= MaxOpenReportsPerUser {
		return Report{}, ErrRateLimit
	}

	var rep Report
	var reporter uuid.NullUUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO marketplace_reports
		   (reported_kb_id, reporter_user_id, reason)
		 VALUES ($1, $2, $3)
		 RETURNING id, reported_kb_id, reporter_user_id, reason, status, created_at`,
		kbID, userID, reason,
	).Scan(&rep.ID, &rep.ReportedKBID, &reporter, &rep.Reason, &rep.Status, &rep.CreatedAt); err != nil {
		return Report{}, fmt.Errorf("Reports.Create: insert: %w", err)
	}
	if reporter.Valid {
		uid := reporter.UUID
		rep.ReporterUserID = &uid
	}

	if err := tx.Commit(ctx); err != nil {
		return Report{}, fmt.Errorf("Reports.Create: commit: %w", err)
	}
	return rep, nil
}

// List returns reports filtered by status, ordered by created_at ASC
// (oldest first — the review queue is FIFO).
//
// This is the admin path. The caller MUST be running under the raven_admin
// role so the admin_bypass RLS policy lets the SELECT see every row.
// Calling this from a non-admin session silently returns the caller's own
// rows only — that is a per-RLS guarantee, not a bug, but the API surface
// in #735 is admin-only by HTTP gating.
//
// status must be one of the four legal ReportStatus values; an empty or
// invalid status returns ErrInvalidReportStatus. limit must be > 0; offset
// must be >= 0. These bounds are validated here so the SQL never sees
// a malformed pagination request.
func (r *Reports) List(ctx context.Context, status ReportStatus, limit, offset int) ([]Report, error) {
	if !status.IsValid() {
		return nil, ErrInvalidReportStatus
	}
	if limit <= 0 {
		return nil, fmt.Errorf("Reports.List: limit must be positive, got %d", limit)
	}
	if offset < 0 {
		return nil, fmt.Errorf("Reports.List: offset must be non-negative, got %d", offset)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, reported_kb_id, reporter_user_id, reason, status, created_at
		 FROM marketplace_reports
		 WHERE status = $1
		 ORDER BY created_at ASC
		 LIMIT $2 OFFSET $3`,
		status, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("Reports.List: query: %w", err)
	}
	defer rows.Close()

	// Pre-allocate to `limit` to avoid repeated slice growth on the
	// happy path where the result fills the page.
	out := make([]Report, 0, limit)
	for rows.Next() {
		var rep Report
		var reporter uuid.NullUUID
		if err := rows.Scan(
			&rep.ID, &rep.ReportedKBID, &reporter,
			&rep.Reason, &rep.Status, &rep.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("Reports.List: scan: %w", err)
		}
		if reporter.Valid {
			uid := reporter.UUID
			rep.ReporterUserID = &uid
		}
		out = append(out, rep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Reports.List: rows: %w", err)
	}
	return out, nil
}

// Transition moves report id from its current status to newStatus, but
// only if (current -> newStatus) is a legal state-machine move. Returns:
//
//   - ErrInvalidReportStatus if newStatus is not a valid status.
//   - ErrReportNotFound if no report with the given id is visible to the
//     caller (or does not exist at all).
//   - ErrIllegalTransition if the move is not legal per reportTransitions.
//   - Any pgx error from the SELECT/UPDATE round-trip otherwise.
//
// Performed in a transaction with FOR UPDATE so a concurrent transition
// attempt against the same report cannot race (e.g. two admins clicking
// "Resolve" simultaneously).
func (r *Reports) Transition(ctx context.Context, id uuid.UUID, newStatus ReportStatus) error {
	if !newStatus.IsValid() {
		return ErrInvalidReportStatus
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("Reports.Transition: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current ReportStatus
	if err := tx.QueryRow(ctx,
		`SELECT status FROM marketplace_reports WHERE id = $1 FOR UPDATE`,
		id,
	).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReportNotFound
		}
		return fmt.Errorf("Reports.Transition: load current: %w", err)
	}

	// Defensive: a status pulled from the DB that does not match the Go
	// enum means the schema has drifted (or someone bypassed the CHECK
	// with SET session_replication_role = replica). Refuse to act.
	if !current.IsValid() {
		return fmt.Errorf("%w: stored value %q", ErrInvalidReportStatus, current)
	}

	if !CanTransition(current, newStatus) {
		return fmt.Errorf("%w: from=%s to=%s", ErrIllegalTransition, current, newStatus)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE marketplace_reports SET status = $2 WHERE id = $1`,
		id, newStatus,
	); err != nil {
		return fmt.Errorf("Reports.Transition: update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Reports.Transition: commit: %w", err)
	}
	return nil
}

// CountOpenForUser returns the number of reports in the OPEN state for
// the given user. Exposed as a helper for callers that want to surface a
// "you have N open reports" UI hint before submission — Create runs the
// same count inside its transaction so this method is purely advisory.
func (r *Reports) CountOpenForUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM marketplace_reports
		 WHERE reporter_user_id = $1 AND status = $2`,
		userID, ReportStatusOpen,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("Reports.CountOpenForUser: %w", err)
	}
	return n, nil
}
