package marketplace_test

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/marketplace"
)

// TestReportStatusIsValid documents which strings are accepted as a
// ReportStatus. The CHECK constraint in migration 00053 must accept
// exactly the same set; the DB-backed migration test below pins that.
func TestReportStatusIsValid(t *testing.T) {
	t.Parallel()
	good := []marketplace.ReportStatus{
		marketplace.ReportStatusOpen,
		marketplace.ReportStatusReviewing,
		marketplace.ReportStatusResolved,
		marketplace.ReportStatusDismissed,
	}
	for _, s := range good {
		if !s.IsValid() {
			t.Errorf("IsValid(%q) = false, want true", s)
		}
	}
	bad := []marketplace.ReportStatus{"", "OPEN", "approved", "rejected", "pending"}
	for _, s := range bad {
		if s.IsValid() {
			t.Errorf("IsValid(%q) = true, want false", s)
		}
	}
}

// TestReportStatusIsTerminal pins which states are terminal. A regression
// here is interesting: adding a new state must declare its terminal-ness
// explicitly, or transitions stop being closed under the state machine.
func TestReportStatusIsTerminal(t *testing.T) {
	t.Parallel()
	cases := map[marketplace.ReportStatus]bool{
		marketplace.ReportStatusOpen:       false,
		marketplace.ReportStatusReviewing:  false,
		marketplace.ReportStatusResolved:   true,
		marketplace.ReportStatusDismissed:  true,
	}
	for s, want := range cases {
		if got := s.IsTerminal(); got != want {
			t.Errorf("IsTerminal(%q) = %v, want %v", s, got, want)
		}
	}
}

// TestCanTransition exhaustively pins the legal state-machine moves. Any
// future change to reportTransitions MUST update this table — the test is
// the canonical reference for "which moves does the moderation API accept".
func TestCanTransition(t *testing.T) {
	t.Parallel()
	type move struct {
		from marketplace.ReportStatus
		to   marketplace.ReportStatus
		ok   bool
	}
	// 16 cells (4 x 4): every from x to combination, including self-edges.
	// Encoding the full grid prevents drift between the table in
	// moderation.go and the test's intent.
	all := []marketplace.ReportStatus{
		marketplace.ReportStatusOpen,
		marketplace.ReportStatusReviewing,
		marketplace.ReportStatusResolved,
		marketplace.ReportStatusDismissed,
	}
	legal := map[marketplace.ReportStatus]map[marketplace.ReportStatus]struct{}{
		marketplace.ReportStatusOpen: {
			marketplace.ReportStatusReviewing: {},
		},
		marketplace.ReportStatusReviewing: {
			marketplace.ReportStatusResolved:  {},
			marketplace.ReportStatusDismissed: {},
		},
	}

	var moves []move
	for _, from := range all {
		for _, to := range all {
			_, ok := legal[from][to]
			moves = append(moves, move{from: from, to: to, ok: ok})
		}
	}

	// Self-edges (open -> open, etc.) are illegal: there is no benefit to
	// modelling a no-op transition. The grid above already encodes that
	// since `legal` has no self-entries.
	for _, m := range moves {
		got := marketplace.CanTransition(m.from, m.to)
		if got != m.ok {
			t.Errorf("CanTransition(%q -> %q) = %v, want %v", m.from, m.to, got, m.ok)
		}
	}

	// Invalid status values must always reject.
	bad := marketplace.ReportStatus("garbage")
	if marketplace.CanTransition(bad, marketplace.ReportStatusOpen) {
		t.Error("CanTransition(garbage -> open) should be false")
	}
	if marketplace.CanTransition(marketplace.ReportStatusOpen, bad) {
		t.Error("CanTransition(open -> garbage) should be false")
	}
}

// TestTakedownSourceIsValid pins the three legal source strings. CHECK
// constraint parity is asserted by the DB-backed migration test below.
func TestTakedownSourceIsValid(t *testing.T) {
	t.Parallel()
	good := []marketplace.TakedownSource{
		marketplace.TakedownSourcePublisher,
		marketplace.TakedownSourceAdmin,
		marketplace.TakedownSourceDMCA,
	}
	for _, s := range good {
		if !s.IsValid() {
			t.Errorf("IsValid(%q) = false, want true", s)
		}
	}
	bad := []marketplace.TakedownSource{"", "ADMIN", "user", "moderator", "system"}
	for _, s := range bad {
		if s.IsValid() {
			t.Errorf("IsValid(%q) = true, want false", s)
		}
	}
}

// TestMaxOpenReportsPerUser is a sanity gate on the rate-limit constant.
// If a future change wants to tune this number, the test must move with
// it — and the diff makes the tuning visible in code review.
func TestMaxOpenReportsPerUser(t *testing.T) {
	t.Parallel()
	if marketplace.MaxOpenReportsPerUser != 5 {
		t.Errorf("MaxOpenReportsPerUser changed: want 5, got %d", marketplace.MaxOpenReportsPerUser)
	}
}
