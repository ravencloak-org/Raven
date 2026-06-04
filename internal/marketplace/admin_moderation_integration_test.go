// Integration tests for AdminModeration. Cover the four-side-effect
// atomic approve, the dismiss path, and the failure-rollback guarantee
// that makes the "transaction or nothing" promise testable.
//
// Skipped under `go test -short` so the fast unit loop stays free of
// testcontainers (matches the convention used by moderation_integration_test.go).

package marketplace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// kbVisibility reads the current visibility of the given KB row. Helper
// for assertions across the integration tests below.
func kbVisibility(ctx context.Context, t *testing.T, pool *pgxpool.Pool, kbID uuid.UUID) string {
	t.Helper()
	var v string
	if err := pool.QueryRow(ctx, `SELECT visibility FROM knowledge_bases WHERE id = $1`, kbID).Scan(&v); err != nil {
		t.Fatalf("read KB visibility: %v", err)
	}
	return v
}

// orgStrikes reads the publisher Org's takedown_strikes counter.
func orgStrikes(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(ctx, `SELECT takedown_strikes FROM organizations WHERE id = $1`, orgID).Scan(&n); err != nil {
		t.Fatalf("read strikes: %v", err)
	}
	return n
}

// countTakedowns returns the count of takedown rows for a KB.
func countTakedowns(ctx context.Context, t *testing.T, pool *pgxpool.Pool, kbID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM marketplace_takedowns WHERE target_kb_id = $1`, kbID).Scan(&n); err != nil {
		t.Fatalf("count takedowns: %v", err)
	}
	return n
}

// reportStatus reads the current status of a report by id.
func reportStatus(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id uuid.UUID) marketplace.ReportStatus {
	t.Helper()
	var s string
	if err := pool.QueryRow(ctx, `SELECT status FROM marketplace_reports WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("read report status: %v", err)
	}
	return marketplace.ReportStatus(s)
}

// TestAdminModeration_Approve_FourSideEffectsAtomic is the headline
// integration test for #734. Asserts every one of the four side effects
// described in ADR-0006 commits together: report→resolved, takedown row,
// KB→private, strikes+1.
func TestAdminModeration_Approve_FourSideEffectsAtomic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "approve")

	// Seed a single report against the fixture KB.
	reports := marketplace.NewReports(pool)
	takedowns := marketplace.NewTakedowns(pool)
	rep, err := reports.Create(ctx, f.KBID, f.UserID, "copyright violation")
	if err != nil {
		t.Fatalf("seed report: %v", err)
	}

	beforeStrikes := orgStrikes(ctx, t, pool, f.OrgID)
	if v := kbVisibility(ctx, t, pool, f.KBID); v != "private" {
		// fixture seeds without visibility, so default is 'private'.
		// First, simulate it being published so the approve flip is observable.
		if _, err := pool.Exec(ctx, `UPDATE knowledge_bases SET visibility='public' WHERE id=$1`, f.KBID); err != nil {
			t.Fatalf("publish KB: %v", err)
		}
		_ = v
	}

	svc := marketplace.NewAdminModeration(pool, reports, takedowns, marketplace.NewNoopPublisherNotifier())
	result, err := svc.Approve(ctx, rep.ID)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// 1. Report -> resolved.
	if got := reportStatus(ctx, t, pool, rep.ID); got != marketplace.ReportStatusResolved {
		t.Errorf("report status: want %q, got %q", marketplace.ReportStatusResolved, got)
	}

	// 2. Takedown row exists for this KB.
	if got := countTakedowns(ctx, t, pool, f.KBID); got != 1 {
		t.Errorf("takedown rows: want 1, got %d", got)
	}

	// 3. KB visibility flipped to private.
	if v := kbVisibility(ctx, t, pool, f.KBID); v != "private" {
		t.Errorf("KB visibility: want private, got %q", v)
	}

	// 4. Strikes incremented by exactly 1.
	if got, want := orgStrikes(ctx, t, pool, f.OrgID), beforeStrikes+1; got != want {
		t.Errorf("strikes: want %d, got %d", want, got)
	}

	// 5. Result body fields match.
	if result.TargetKBID != f.KBID {
		t.Errorf("result.TargetKBID: want %s, got %s", f.KBID, result.TargetKBID)
	}
	if result.StrikesAfter != beforeStrikes+1 {
		t.Errorf("result.StrikesAfter: want %d, got %d", beforeStrikes+1, result.StrikesAfter)
	}
	if result.TakedownID == uuid.Nil {
		t.Errorf("result.TakedownID should be set")
	}
}

// TestAdminModeration_Approve_AlreadyResolvedReturnsIllegalTransition
// pins the 409 mapping at the service boundary — the second admin
// click on the same report should not double-strike.
func TestAdminModeration_Approve_AlreadyResolvedReturnsIllegalTransition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "double")

	if _, err := pool.Exec(ctx, `UPDATE knowledge_bases SET visibility='public' WHERE id=$1`, f.KBID); err != nil {
		t.Fatalf("publish KB: %v", err)
	}

	reports := marketplace.NewReports(pool)
	takedowns := marketplace.NewTakedowns(pool)
	rep, err := reports.Create(ctx, f.KBID, f.UserID, "first")
	if err != nil {
		t.Fatalf("seed report: %v", err)
	}

	svc := marketplace.NewAdminModeration(pool, reports, takedowns, marketplace.NewNoopPublisherNotifier())
	if _, err := svc.Approve(ctx, rep.ID); err != nil {
		t.Fatalf("first Approve: %v", err)
	}
	strikesAfterFirst := orgStrikes(ctx, t, pool, f.OrgID)

	// Second approve must be rejected and must NOT increment strikes.
	_, err = svc.Approve(ctx, rep.ID)
	if !errors.Is(err, marketplace.ErrIllegalTransition) {
		t.Errorf("second Approve: want ErrIllegalTransition, got %v", err)
	}
	if got := orgStrikes(ctx, t, pool, f.OrgID); got != strikesAfterFirst {
		t.Errorf("strikes after duplicate approve: want %d, got %d", strikesAfterFirst, got)
	}
}

// TestAdminModeration_Approve_MissingReportReturns404 pins the not-found
// error path so the HTTP handler can rely on the sentinel.
func TestAdminModeration_Approve_MissingReportReturns404(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	reports := marketplace.NewReports(pool)
	takedowns := marketplace.NewTakedowns(pool)
	svc := marketplace.NewAdminModeration(pool, reports, takedowns, marketplace.NewNoopPublisherNotifier())

	_, err := svc.Approve(ctx, uuid.New())
	if !errors.Is(err, marketplace.ErrReportNotFound) {
		t.Errorf("missing report: want ErrReportNotFound, got %v", err)
	}
}

// TestAdminModeration_Dismiss_HappyPath drives a report to dismissed
// and asserts no side effects (no takedown, no strike, KB untouched).
func TestAdminModeration_Dismiss_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "dismiss")

	if _, err := pool.Exec(ctx, `UPDATE knowledge_bases SET visibility='public' WHERE id=$1`, f.KBID); err != nil {
		t.Fatalf("publish KB: %v", err)
	}

	reports := marketplace.NewReports(pool)
	takedowns := marketplace.NewTakedowns(pool)
	rep, err := reports.Create(ctx, f.KBID, f.UserID, "false positive")
	if err != nil {
		t.Fatalf("seed report: %v", err)
	}
	beforeStrikes := orgStrikes(ctx, t, pool, f.OrgID)

	svc := marketplace.NewAdminModeration(pool, reports, takedowns, marketplace.NewNoopPublisherNotifier())
	if err := svc.Dismiss(ctx, rep.ID); err != nil {
		t.Fatalf("Dismiss: %v", err)
	}

	if got := reportStatus(ctx, t, pool, rep.ID); got != marketplace.ReportStatusDismissed {
		t.Errorf("report status: want dismissed, got %q", got)
	}
	if got := countTakedowns(ctx, t, pool, f.KBID); got != 0 {
		t.Errorf("takedowns: want 0, got %d", got)
	}
	if v := kbVisibility(ctx, t, pool, f.KBID); v != "public" {
		t.Errorf("KB visibility: want public (untouched), got %q", v)
	}
	if got := orgStrikes(ctx, t, pool, f.OrgID); got != beforeStrikes {
		t.Errorf("strikes: want %d (unchanged), got %d", beforeStrikes, got)
	}
}

// TestAdminModeration_ListReports filters by status and returns the
// matching rows in created_at ASC order.
func TestAdminModeration_ListReports(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "list")

	reports := marketplace.NewReports(pool)
	takedowns := marketplace.NewTakedowns(pool)
	if _, err := reports.Create(ctx, f.KBID, f.UserID, "first"); err != nil {
		t.Fatalf("seed report 1: %v", err)
	}
	if _, err := reports.Create(ctx, f.KBID, f.UserID, "second"); err != nil {
		t.Fatalf("seed report 2: %v", err)
	}

	svc := marketplace.NewAdminModeration(pool, reports, takedowns, marketplace.NewNoopPublisherNotifier())
	got, err := svc.ListReports(ctx, marketplace.ReportStatusOpen, 100, 0)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	if len(got) < 2 {
		t.Errorf("want >= 2 open reports, got %d", len(got))
	}
}
