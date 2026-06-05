package marketplace

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Takedowns is the data-access repository for the marketplace_takedowns
// table — the append-only audit log for KB removals (ADR-0006).
//
// RLS: every method runs under the caller's session context. The migration
// 00053 policy is admin-only (no non-admin policy is defined), so a
// non-admin session sees zero rows from List and cannot write via Create
// (RLS would silently elide the insert). Callers MUST switch to raven_admin
// before calling these methods.
type Takedowns struct {
	pool *pgxpool.Pool
}

// NewTakedowns returns a Takedowns repository backed by the given pool.
func NewTakedowns(pool *pgxpool.Pool) *Takedowns {
	return &Takedowns{pool: pool}
}

// Create inserts a new takedown audit-log row against kbID with the given
// source and notes. Notes may be empty (stored as NULL); a non-empty notes
// string is stored verbatim.
//
// Returns ErrInvalidTakedownSource if source is not one of the three legal
// TakedownSource values; any pgx error from the round-trip otherwise.
//
// Append-only — there is no Update path. If a takedown row was created
// in error, the correction is a new row with an explanatory note.
func (t *Takedowns) Create(ctx context.Context, kbID uuid.UUID, source TakedownSource, notes string) (Takedown, error) {
	if !source.IsValid() {
		return Takedown{}, ErrInvalidTakedownSource
	}

	// Map empty notes to a SQL NULL so the column reflects "no note" rather
	// than the empty string. The Notes field on the returned struct is
	// always a Go string for ergonomic reasons — empty == NULL on read.
	var notesArg any
	if notes != "" {
		notesArg = notes
	}

	var td Takedown
	var storedNotes *string
	if err := t.pool.QueryRow(ctx,
		`INSERT INTO marketplace_takedowns
		   (target_kb_id, source, notes)
		 VALUES ($1, $2, $3)
		 RETURNING id, target_kb_id, source, notes, created_at`,
		kbID, source, notesArg,
	).Scan(&td.ID, &td.TargetKBID, &td.Source, &storedNotes, &td.CreatedAt); err != nil {
		return Takedown{}, fmt.Errorf("Takedowns.Create: insert: %w", err)
	}
	if storedNotes != nil {
		td.Notes = *storedNotes
	}
	return td, nil
}

// CreateInTx is the in-transaction sibling of Create. The admin approve
// path (#734) batches the takedown write together with the report
// transition, the KB visibility flip, and the strike increment so the
// four side-effects commit atomically. See Reports.TransitionInTx for
// the rationale.
//
// The caller is responsible for: opening the tx, switching role to
// raven_admin (the table's RLS policy is admin-only), and committing.
func (t *Takedowns) CreateInTx(ctx context.Context, tx pgx.Tx, kbID uuid.UUID, source TakedownSource, notes string) (Takedown, error) {
	if !source.IsValid() {
		return Takedown{}, ErrInvalidTakedownSource
	}

	var notesArg any
	if notes != "" {
		notesArg = notes
	}

	var td Takedown
	var storedNotes *string
	if err := tx.QueryRow(ctx,
		`INSERT INTO marketplace_takedowns
		   (target_kb_id, source, notes)
		 VALUES ($1, $2, $3)
		 RETURNING id, target_kb_id, source, notes, created_at`,
		kbID, source, notesArg,
	).Scan(&td.ID, &td.TargetKBID, &td.Source, &storedNotes, &td.CreatedAt); err != nil {
		return Takedown{}, fmt.Errorf("Takedowns.CreateInTx: %w", err)
	}
	if storedNotes != nil {
		td.Notes = *storedNotes
	}
	return td, nil
}

// ListForKB returns every takedown audit-log row for the given KB,
// ordered by created_at ASC (oldest first — the audit log reads forward
// through history). The slice is nil-on-empty; callers using `len()` get
// 0 in both cases.
//
// Admin-only (see package doc). The HTTP gate in #735 enforces this at
// the handler layer; the RLS policy is the in-database backstop.
func (t *Takedowns) ListForKB(ctx context.Context, kbID uuid.UUID) ([]Takedown, error) {
	rows, err := t.pool.Query(ctx,
		`SELECT id, target_kb_id, source, notes, created_at
		 FROM marketplace_takedowns
		 WHERE target_kb_id = $1
		 ORDER BY created_at ASC`,
		kbID,
	)
	if err != nil {
		return nil, fmt.Errorf("Takedowns.ListForKB: query: %w", err)
	}
	defer rows.Close()

	var out []Takedown
	for rows.Next() {
		var td Takedown
		var notes *string
		if err := rows.Scan(&td.ID, &td.TargetKBID, &td.Source, &notes, &td.CreatedAt); err != nil {
			return nil, fmt.Errorf("Takedowns.ListForKB: scan: %w", err)
		}
		if notes != nil {
			td.Notes = *notes
		}
		out = append(out, td)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Takedowns.ListForKB: rows: %w", err)
	}
	return out, nil
}

// AuditRow is the join shape returned by ListAudit. It collects the
// columns the AdminTakedownsView in #735's frontend needs in a single
// round-trip: the takedown event itself, the target KB's display name,
// the publisher Org's slug + display name, and that Org's running
// strike total at read time.
//
// The strike count is the live `organizations.takedown_strikes` value,
// not a snapshot stored on the takedown row. Snapshotting would require
// schema migration #735 didn't budget for, and the live value matches
// the "current state of the publisher" semantic the audit log is for:
// an admin looking up a takedown wants to know "where does this Org
// sit on the strike scale right now?".
type AuditRow struct {
	TakedownID            uuid.UUID
	TargetKBID            uuid.UUID
	TargetKBName          string
	TargetOrgSlug         string
	TargetOrgDisplayName  string
	Source                TakedownSource
	Notes                 string
	CreatedAt             time.Time
	StrikesAfterOrgTotal  int64
}

// auditListCap is the hard ceiling on pagination page size. Audit reads
// are admin-only and infrequent; a deeper page than this is almost
// certainly a misconfigured caller and would expand the response payload
// without payoff.
const auditListCap = 100

// auditListDefault is the soft default when no limit is supplied. Picked
// to match the report queue page size (#734) so the two admin surfaces
// behave consistently.
const auditListDefault = 25

// ListAudit returns a page of takedown audit-log rows, newest first,
// joined with KB + publisher Org metadata. Pagination uses a
// (created_at, id) keyset cursor — opaque to the caller, parsed by
// EncodeAuditCursor / parseAuditCursor.
//
// Admin-only (see package doc): the SELECT crosses every tenant via the
// raven_admin RLS bypass on marketplace_takedowns, knowledge_bases, and
// organizations. The caller MUST be on a session that has switched to
// raven_admin before invoking this. The handler in
// internal/handler/admin_takedowns.go does so inside a short-lived tx.
func (t *Takedowns) ListAudit(ctx context.Context, limit int, cursor string) ([]AuditRow, string, error) {
	if limit <= 0 {
		limit = auditListDefault
	}
	if limit > auditListCap {
		limit = auditListCap
	}

	var (
		cursorCreatedAt time.Time
		cursorID        uuid.UUID
		hasCursor       bool
	)
	if cursor != "" {
		ca, id, err := parseAuditCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("Takedowns.ListAudit: cursor: %w", err)
		}
		cursorCreatedAt, cursorID, hasCursor = ca, id, true
	}

	// Fetch limit+1 rows so we can detect "more available" without a
	// COUNT(*). The extra row, if present, becomes the next cursor.
	//
	// Switch to raven_admin inside a transaction so the audit cross-tenant
	// SELECT can see every row. SET LOCAL ROLE reverts on COMMIT/ROLLBACK,
	// leaving the pool connection in its original role for the next caller.
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("Takedowns.ListAudit: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE raven_admin"); err != nil {
		return nil, "", fmt.Errorf("Takedowns.ListAudit: set role: %w", err)
	}

	// Keyset pagination predicate: rows with (created_at, id) strictly
	// less than the cursor. The tie-break on id keeps the order
	// deterministic when two takedowns landed in the same microsecond.
	//
	// $2 / $3 are NULL on the first page; the WHERE clause's NULL guard
	// then short-circuits the comparison so the same prepared statement
	// serves both the first-page and follow-page cases.
	const sql = `
		SELECT
		    td.id, td.target_kb_id, kb.name, o.slug, o.name,
		    td.source, td.notes, td.created_at,
		    o.takedown_strikes
		FROM marketplace_takedowns AS td
		JOIN knowledge_bases  AS kb ON kb.id = td.target_kb_id
		JOIN organizations    AS o  ON o.id  = kb.org_id
		WHERE ($2::timestamptz IS NULL)
		   OR (td.created_at, td.id) < ($2::timestamptz, $3::uuid)
		ORDER BY td.created_at DESC, td.id DESC
		LIMIT $1
	`

	var (
		cursorCreatedArg any
		cursorIDArg      any
	)
	if hasCursor {
		cursorCreatedArg = cursorCreatedAt
		cursorIDArg = cursorID
	}

	rows, err := tx.Query(ctx, sql, limit+1, cursorCreatedArg, cursorIDArg)
	if err != nil {
		return nil, "", fmt.Errorf("Takedowns.ListAudit: query: %w", err)
	}
	defer rows.Close()

	out := make([]AuditRow, 0, limit)
	for rows.Next() {
		var r AuditRow
		var notes *string
		if err := rows.Scan(
			&r.TakedownID, &r.TargetKBID, &r.TargetKBName,
			&r.TargetOrgSlug, &r.TargetOrgDisplayName,
			&r.Source, &notes, &r.CreatedAt,
			&r.StrikesAfterOrgTotal,
		); err != nil {
			return nil, "", fmt.Errorf("Takedowns.ListAudit: scan: %w", err)
		}
		if notes != nil {
			r.Notes = *notes
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("Takedowns.ListAudit: rows: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("Takedowns.ListAudit: commit: %w", err)
	}

	// Trim the look-ahead row and emit a cursor pointing at the last
	// returned row so the caller can page forward.
	var nextCursor string
	if len(out) > limit {
		out = out[:limit]
	}
	if len(out) == limit {
		last := out[len(out)-1]
		nextCursor = EncodeAuditCursor(last.CreatedAt, last.TakedownID)
	}
	return out, nextCursor, nil
}
