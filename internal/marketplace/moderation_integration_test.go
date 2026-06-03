// Integration tests for the moderation repositories. These spin up a real
// Postgres via testutil.NewTestDB and exercise marketplace_reports +
// marketplace_takedowns through the Reports / Takedowns repositories.
//
// Tests are skipped under `go test -short` so the fast unit-test loop
// stays free of testcontainers.

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

// modFixture is the minimal set of seeded IDs every moderation integration
// test needs: a Public-KB target, a user to act as reporter, and the
// org+workspace they live under (FK chain on knowledge_bases / users).
//
// All four IDs are freshly generated per call so tests are isolated from
// each other when they share the same testcontainer.
type modFixture struct {
	OrgID  uuid.UUID
	WSID   uuid.UUID
	UserID uuid.UUID
	KBID   uuid.UUID
}

// seedModFixture inserts an org, workspace, user, and KB. Slugs are
// derived from the generated UUID prefix so parallel tests under the
// same DB cannot collide on the unique slug index.
//
// Runs as the testcontainer superuser — RLS is bypassed for fixtures.
func seedModFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool, label string) modFixture {
	t.Helper()
	f := modFixture{
		OrgID:  uuid.New(),
		WSID:   uuid.New(),
		UserID: uuid.New(),
		KBID:   uuid.New(),
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug)
		 VALUES ($1, $2, $2 || '-' || substring($1::text from 1 for 8))`,
		f.OrgID, label+"-org",
	); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspaces (id, org_id, name, slug)
		 VALUES ($1, $2, $3, $3 || '-' || substring($1::text from 1 for 8))`,
		f.WSID, f.OrgID, label+"-ws",
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, org_id, email, status)
		 VALUES ($1, $2, $3 || '-' || substring($1::text from 1 for 8) || '@example.com', 'active')`,
		f.UserID, f.OrgID, label,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO knowledge_bases (id, org_id, workspace_id, name, slug)
		 VALUES ($1, $2, $3, $4, $4 || '-' || substring($1::text from 1 for 8))`,
		f.KBID, f.OrgID, f.WSID, label+"-kb",
	); err != nil {
		t.Fatalf("seed knowledge_base: %v", err)
	}
	return f
}

// TestReportsRepository_CreateAndRateLimit verifies the per-user rate
// limiter blocks the 6th open report.
func TestReportsRepository_CreateAndRateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "rate")

	repo := marketplace.NewReports(pool)

	// Fill the cap (5 reports). Each must succeed.
	for i := 0; i < marketplace.MaxOpenReportsPerUser; i++ {
		if _, err := repo.Create(ctx, f.KBID, f.UserID, "spam attempt"); err != nil {
			t.Fatalf("create report %d: %v", i+1, err)
		}
	}

	// 6th must hit the rate-limit gate.
	_, err := repo.Create(ctx, f.KBID, f.UserID, "one too many")
	if !errors.Is(err, marketplace.ErrRateLimit) {
		t.Errorf("6th report: want ErrRateLimit, got %v", err)
	}

	// CountOpenForUser should report exactly the cap (the rejected attempt
	// was never inserted).
	n, err := repo.CountOpenForUser(ctx, f.UserID)
	if err != nil {
		t.Fatalf("CountOpenForUser: %v", err)
	}
	if n != marketplace.MaxOpenReportsPerUser {
		t.Errorf("CountOpenForUser: want %d, got %d", marketplace.MaxOpenReportsPerUser, n)
	}
}

// TestReportsRepository_InvalidReason verifies the empty / oversize reason
// guard fires before any SQL round-trip.
func TestReportsRepository_InvalidReason(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := marketplace.NewReports(pool)

	// Empty.
	_, err := repo.Create(ctx, uuid.New(), uuid.New(), "")
	if !errors.Is(err, marketplace.ErrInvalidReason) {
		t.Errorf("empty reason: want ErrInvalidReason, got %v", err)
	}

	// Oversize: 4001 chars.
	oversize := make([]byte, 4001)
	for i := range oversize {
		oversize[i] = 'a'
	}
	_, err = repo.Create(ctx, uuid.New(), uuid.New(), string(oversize))
	if !errors.Is(err, marketplace.ErrInvalidReason) {
		t.Errorf("oversize reason: want ErrInvalidReason, got %v", err)
	}
}

// TestReportsRepository_TransitionStateMachine drives a report through
// the legal chain (open -> reviewing -> resolved) and asserts the illegal
// shortcut (open -> resolved) is rejected.
func TestReportsRepository_TransitionStateMachine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "trans")

	repo := marketplace.NewReports(pool)
	rep, err := repo.Create(ctx, f.KBID, f.UserID, "transition test")
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	if rep.Status != marketplace.ReportStatusOpen {
		t.Fatalf("initial status: want %q, got %q", marketplace.ReportStatusOpen, rep.Status)
	}

	// Illegal shortcut.
	err = repo.Transition(ctx, rep.ID, marketplace.ReportStatusResolved)
	if !errors.Is(err, marketplace.ErrIllegalTransition) {
		t.Errorf("open -> resolved: want ErrIllegalTransition, got %v", err)
	}

	// Legal chain.
	if err := repo.Transition(ctx, rep.ID, marketplace.ReportStatusReviewing); err != nil {
		t.Fatalf("open -> reviewing: %v", err)
	}
	if err := repo.Transition(ctx, rep.ID, marketplace.ReportStatusResolved); err != nil {
		t.Fatalf("reviewing -> resolved: %v", err)
	}

	// Terminal state: any further transition is illegal.
	err = repo.Transition(ctx, rep.ID, marketplace.ReportStatusDismissed)
	if !errors.Is(err, marketplace.ErrIllegalTransition) {
		t.Errorf("resolved -> dismissed: want ErrIllegalTransition, got %v", err)
	}

	// Unknown id.
	err = repo.Transition(ctx, uuid.New(), marketplace.ReportStatusReviewing)
	if !errors.Is(err, marketplace.ErrReportNotFound) {
		t.Errorf("unknown id: want ErrReportNotFound, got %v", err)
	}

	// Invalid newStatus.
	err = repo.Transition(ctx, rep.ID, marketplace.ReportStatus("garbage"))
	if !errors.Is(err, marketplace.ErrInvalidReportStatus) {
		t.Errorf("invalid status: want ErrInvalidReportStatus, got %v", err)
	}
}

// TestReportsRepository_CascadeOnKBDelete verifies that deleting the KB
// row hard-cascades to its reports (FK ON DELETE CASCADE).
func TestReportsRepository_CascadeOnKBDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "cascade")

	reports := marketplace.NewReports(pool)
	tdowns := marketplace.NewTakedowns(pool)

	if _, err := reports.Create(ctx, f.KBID, f.UserID, "report before cascade"); err != nil {
		t.Fatalf("create report: %v", err)
	}
	if _, err := tdowns.Create(ctx, f.KBID, marketplace.TakedownSourceAdmin, "before cascade"); err != nil {
		t.Fatalf("create takedown: %v", err)
	}

	// Delete the KB. ON DELETE CASCADE on both FKs should erase the rows.
	if _, err := pool.Exec(ctx, `DELETE FROM knowledge_bases WHERE id = $1`, f.KBID); err != nil {
		t.Fatalf("delete kb: %v", err)
	}

	var reportCount, takedownCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM marketplace_reports WHERE reported_kb_id = $1`, f.KBID,
	).Scan(&reportCount); err != nil {
		t.Fatalf("count reports after cascade: %v", err)
	}
	if reportCount != 0 {
		t.Errorf("reports after KB cascade: want 0, got %d", reportCount)
	}
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM marketplace_takedowns WHERE target_kb_id = $1`, f.KBID,
	).Scan(&takedownCount); err != nil {
		t.Fatalf("count takedowns after cascade: %v", err)
	}
	if takedownCount != 0 {
		t.Errorf("takedowns after KB cascade: want 0, got %d", takedownCount)
	}
}

// TestTakedownsRepository_CreateAndList exercises Create + ListForKB and
// verifies the source CHECK constraint via ErrInvalidTakedownSource.
func TestTakedownsRepository_CreateAndList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "td")

	repo := marketplace.NewTakedowns(pool)

	// Invalid source is rejected before any SQL round-trip.
	if _, err := repo.Create(ctx, f.KBID, marketplace.TakedownSource("badsource"), ""); !errors.Is(err, marketplace.ErrInvalidTakedownSource) {
		t.Errorf("bad source: want ErrInvalidTakedownSource, got %v", err)
	}

	// Three legal sources, one with notes, one without.
	if _, err := repo.Create(ctx, f.KBID, marketplace.TakedownSourcePublisher, ""); err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	if _, err := repo.Create(ctx, f.KBID, marketplace.TakedownSourceAdmin, "report #42 confirmed"); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := repo.Create(ctx, f.KBID, marketplace.TakedownSourceDMCA, "DMCA notice 2026-06-01"); err != nil {
		t.Fatalf("create dmca: %v", err)
	}

	tds, err := repo.ListForKB(ctx, f.KBID)
	if err != nil {
		t.Fatalf("ListForKB: %v", err)
	}
	if len(tds) != 3 {
		t.Fatalf("want 3 takedowns, got %d", len(tds))
	}

	// Order: created_at ASC. Publisher was inserted first.
	if tds[0].Source != marketplace.TakedownSourcePublisher {
		t.Errorf("tds[0].Source: want publisher, got %q", tds[0].Source)
	}
	if tds[0].Notes != "" {
		t.Errorf("tds[0].Notes: want empty, got %q", tds[0].Notes)
	}
	if tds[1].Source != marketplace.TakedownSourceAdmin {
		t.Errorf("tds[1].Source: want admin, got %q", tds[1].Source)
	}
	if tds[1].Notes != "report #42 confirmed" {
		t.Errorf("tds[1].Notes: %q", tds[1].Notes)
	}
}

// TestReportsRepository_ReporterCleanupSetsNull verifies that deleting
// the reporter user (ON DELETE SET NULL) anonymises the report rather
// than cascading it. This protects the audit trail per ADR-0006.
func TestReportsRepository_ReporterCleanupSetsNull(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "anon")

	repo := marketplace.NewReports(pool)
	rep, err := repo.Create(ctx, f.KBID, f.UserID, "anonymise me")
	if err != nil {
		t.Fatalf("create report: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, f.UserID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var reporter *uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT reporter_user_id FROM marketplace_reports WHERE id = $1`,
		rep.ID,
	).Scan(&reporter); err != nil {
		t.Fatalf("read reporter after user delete: %v", err)
	}
	if reporter != nil {
		t.Errorf("reporter_user_id: want NULL, got %v", *reporter)
	}
}
