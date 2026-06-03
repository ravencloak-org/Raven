package marketplace

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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
