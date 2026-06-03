// Package marketplace — moderation: shared types, errors, and the canonical
// ReportStatus state machine used by both the database CHECK constraint and
// the Go transition helper.
//
// The split across files in this package:
//
//   - moderation.go (this file): shared types, sentinel errors, the
//     ReportStatus state-machine table. No SQL, no I/O.
//   - reports.go: Reports repository — Create, List, Transition, plus the
//     per-user open-report rate limiter.
//   - takedowns.go: Takedowns repository — Create + List.
//
// Two tables, two repositories. Combining them into a single "moderation"
// store would couple two concerns whose RLS shapes are deliberately
// different (reports: reporter-scoped; takedowns: admin-only audit log)
// and whose access patterns share nothing beyond the foreign key to
// knowledge_bases. The separation matches the schema (00053) and the API
// surface (one POST /reports endpoint, distinct admin endpoints per table).
package marketplace

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ReportStatus is the canonical type for the marketplace_reports.status
// column. Use this anywhere the status string crosses a function boundary —
// it prevents stringly-typed "open"/"reviewing" comparisons scattered
// across the codebase and lets the state-machine helpers stay in one place.
type ReportStatus string

const (
	// ReportStatusOpen is the initial state of a newly created report.
	// The admin review queue lists reports in this state.
	ReportStatusOpen ReportStatus = "open"

	// ReportStatusReviewing means an admin has acknowledged the report and
	// is investigating. It is the only state from which a terminal state
	// (resolved or dismissed) may be reached.
	ReportStatusReviewing ReportStatus = "reviewing"

	// ReportStatusResolved is a terminal state: the report led to action
	// (typically a takedown + strike increment per ADR-0006).
	ReportStatusResolved ReportStatus = "resolved"

	// ReportStatusDismissed is a terminal state: the report was reviewed
	// and rejected as without merit. The reporter is not notified by
	// default (ADR-0008 §Why — report-confirms-takedown loop is a known
	// harassment vector).
	ReportStatusDismissed ReportStatus = "dismissed"
)

// allReportStatuses lists every legal ReportStatus value. Mirrors the
// CHECK constraint in migration 00053. Used by IsValid below and by tests
// asserting parity between the Go enum and the DB CHECK.
var allReportStatuses = []ReportStatus{
	ReportStatusOpen,
	ReportStatusReviewing,
	ReportStatusResolved,
	ReportStatusDismissed,
}

// IsValid reports whether s is one of the four legal ReportStatus values.
// The DB CHECK is the backstop; this guards against bad inputs at the
// service boundary before a round-trip to Postgres.
func (s ReportStatus) IsValid() bool {
	for _, v := range allReportStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// IsTerminal reports whether s is a state from which no further transition
// is legal. Used by Transition to short-circuit illegal moves before the
// table lookup.
func (s ReportStatus) IsTerminal() bool {
	return s == ReportStatusResolved || s == ReportStatusDismissed
}

// reportTransitions encodes the legal state-machine moves:
//
//	open       -> reviewing
//	reviewing  -> resolved | dismissed
//	resolved   -> (terminal — no moves)
//	dismissed  -> (terminal — no moves)
//
// Encoding this in code rather than relying solely on the DB CHECK gives
// us a structured ErrIllegalTransition diagnostic at the service boundary
// — the alternative ("SQL violated check constraint X on column Y") is
// opaque to API callers.
var reportTransitions = map[ReportStatus]map[ReportStatus]struct{}{
	ReportStatusOpen: {
		ReportStatusReviewing: {},
	},
	ReportStatusReviewing: {
		ReportStatusResolved:  {},
		ReportStatusDismissed: {},
	},
	// Terminal states intentionally have no outgoing edges; we leave them
	// out of the map so a Transition attempt falls through to "not found"
	// and returns ErrIllegalTransition.
}

// CanTransition reports whether from -> to is a legal state-machine move.
// Both states must be valid; an unknown ReportStatus is rejected even if
// some accidental map entry would otherwise allow it.
func CanTransition(from, to ReportStatus) bool {
	if !from.IsValid() || !to.IsValid() {
		return false
	}
	outgoing, ok := reportTransitions[from]
	if !ok {
		return false
	}
	_, ok = outgoing[to]
	return ok
}

// Report is the row shape for marketplace_reports. The repository returns
// this type from Create and List.
type Report struct {
	// ID is the report's primary key.
	ID uuid.UUID
	// ReportedKBID is the Public KB being reported.
	ReportedKBID uuid.UUID
	// ReporterUserID is the user who submitted the report. Nullable —
	// becomes NULL when the user is deleted (ON DELETE SET NULL on the FK).
	ReporterUserID *uuid.UUID
	// Reason is the free-text justification (1..4000 chars, per CHECK).
	Reason string
	// Status is the current report state.
	Status ReportStatus
	// CreatedAt is the report submission timestamp.
	CreatedAt time.Time
}

// TakedownSource is the canonical type for marketplace_takedowns.source.
// Three legal values per ADR-0006: publisher self-takedown, admin
// (report-driven) takedown, or DMCA notice.
type TakedownSource string

const (
	// TakedownSourcePublisher is a publisher-initiated unpublish that is
	// being audit-logged. See POST /knowledge_bases/{id}/unpublish.
	TakedownSourcePublisher TakedownSource = "publisher"

	// TakedownSourceAdmin is an admin action driven by a confirmed report.
	// Increments organizations.takedown_strikes per ADR-0006.
	TakedownSourceAdmin TakedownSource = "admin"

	// TakedownSourceDMCA is a DMCA notice. Drives the 14-day counter-notice
	// workflow per ADR-0008; the KB transitions to kb_status='dmca_pending'
	// during that window.
	TakedownSourceDMCA TakedownSource = "dmca"
)

// allTakedownSources lists every legal TakedownSource value. Mirrors the
// CHECK constraint in migration 00053.
var allTakedownSources = []TakedownSource{
	TakedownSourcePublisher,
	TakedownSourceAdmin,
	TakedownSourceDMCA,
}

// IsValid reports whether s is one of the three legal TakedownSource values.
func (s TakedownSource) IsValid() bool {
	for _, v := range allTakedownSources {
		if s == v {
			return true
		}
	}
	return false
}

// Takedown is the row shape for marketplace_takedowns. Append-only — there
// is no Update method on the repository.
type Takedown struct {
	// ID is the takedown event's primary key.
	ID uuid.UUID
	// TargetKBID is the Public KB that was taken down.
	TargetKBID uuid.UUID
	// Source attributes the takedown (publisher / admin / dmca).
	Source TakedownSource
	// Notes is free-text context (empty string if unset — the column is
	// nullable but we present it as "" to callers for ergonomic reasons).
	Notes string
	// CreatedAt is the takedown event timestamp.
	CreatedAt time.Time
}

// ─── Sentinel errors ─────────────────────────────────────────────────────────
//
// Errors are sentinel values (errors.Is-friendly) rather than wrapped
// strings so HTTP handlers and tests can branch on them cleanly.

// ErrInvalidReason is returned by Reports.Create when the supplied reason
// is empty or exceeds 4000 characters. Matches the DB CHECK; surfaced
// before round-trip for a useful API error (vs. opaque PG SQLSTATE 23514).
var ErrInvalidReason = errors.New("marketplace: report reason must be 1..4000 chars")

// ErrInvalidReportStatus is returned by Reports.Transition when either
// the destination status or (if read from the DB) the current status is
// not one of the four legal ReportStatus values.
var ErrInvalidReportStatus = errors.New("marketplace: invalid report status")

// ErrIllegalTransition is returned by Reports.Transition when the requested
// (from -> to) move is not in the legal transition table. The from-state is
// included by the caller via fmt.Errorf("%w: from=%s to=%s", ...) where
// useful; the sentinel itself is the branch point for callers.
var ErrIllegalTransition = errors.New("marketplace: illegal report status transition")

// ErrReportNotFound is returned by Reports.Transition when the report id
// does not match any row visible under the caller's RLS context.
var ErrReportNotFound = errors.New("marketplace: report not found")

// ErrInvalidTakedownSource is returned by Takedowns.Create when the
// supplied source is not one of the three legal TakedownSource values.
var ErrInvalidTakedownSource = errors.New("marketplace: invalid takedown source")

// ErrRateLimit is returned by Reports.Create when the user has hit the
// per-user open-report cap (MaxOpenReportsPerUser). The HTTP handler
// translates this to 429 Too Many Requests per the API table in
// docs/plans/marketplace-mvp.md §4.
var ErrRateLimit = errors.New("marketplace: user has too many open reports")

// MaxOpenReportsPerUser is the per-user cap on simultaneous open reports.
// A constant rather than configuration because:
//  1. The MVP review queue runs at human latency — a single user being
//     able to file 5 open reports is already generous.
//  2. Tuning this requires a code review (catch coordinated spam attempts
//     in the diff), not a config push.
//
// Five chosen so a good-faith reporter on a typical Marketplace browsing
// session is never blocked, while a script firing reports in a loop hits
// the cap within seconds.
const MaxOpenReportsPerUser = 5
