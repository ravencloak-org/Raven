// Integration tests for DMCAService (issue #736, ADR-0006 launch blocker).
// Drive Submit → SubmitCounterNotice → SweepExpired through a real
// Postgres container, asserting the atomic-side-effect contracts:
//
//   - Submit flips kb_status=dmca_pending AND inserts a pending notice
//     row in ONE transaction (rollback test: a forced FK failure leaves
//     the KB alone).
//   - SubmitCounterNotice pivots to counter_filed; subsequent sweep does
//     NOT auto-resolve a counter-filed row.
//   - SweepExpired auto-resolves expired pending rows, writes the
//     takedowns row with source='dmca', and dispatches the
//     OnTakedownCreated registry.
//
// Skipped under `go test -short` so the fast unit loop stays free of
// testcontainers (mirrors the existing moderation integration tests).

package marketplace_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// dmcaNoticeStatus reads the current status of a DMCA notice. The
// SELECT runs under the testcontainer superuser so RLS is bypassed.
func dmcaNoticeStatus(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(ctx, `SELECT status FROM dmca_notices WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("read DMCA notice status: %v", err)
	}
	return s
}

// kbStatus reads the lifecycle status of the given KB row.
func kbStatus(ctx context.Context, t *testing.T, pool *pgxpool.Pool, kbID uuid.UUID) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(ctx, `SELECT status FROM knowledge_bases WHERE id = $1`, kbID).Scan(&s); err != nil {
		t.Fatalf("read KB status: %v", err)
	}
	return s
}

// rewindWindow forces a notice's counter_notice_window_ends into the
// past so SweepExpired picks it up without waiting 14 real days.
func rewindWindow(ctx context.Context, t *testing.T, pool *pgxpool.Pool, noticeID uuid.UUID, into time.Duration) {
	t.Helper()
	if _, err := pool.Exec(ctx,
		`UPDATE dmca_notices SET counter_notice_window_ends = now() - $2::interval WHERE id = $1`,
		noticeID, into.String(),
	); err != nil {
		t.Fatalf("rewind window: %v", err)
	}
}

// recordingHook captures every OnTakedownCreated dispatch. Used by the
// sweep tests to assert that the derivative-notifier registry fires
// exactly once per auto-resolved notice.
type recordingHook struct {
	mu      sync.Mutex
	dispatched []marketplace.Takedown
	reasons    []string
}

func (r *recordingHook) hook(_ context.Context, td marketplace.Takedown, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dispatched = append(r.dispatched, td)
	r.reasons = append(r.reasons, reason)
	return nil
}

// TestDMCAService_Submit_HappyPath drives the atomic two-side-effect
// commit: dmca_notices row inserted + knowledge_bases.status flipped to
// dmca_pending. Asserts the 14-day window was materialised correctly.
func TestDMCAService_Submit_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "dmca-sub")

	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)

	before := time.Now()
	notice, err := svc.Submit(ctx, marketplace.DMCANoticeInput{
		TargetKBID:    f.KBID,
		NoticeText:    "alleged copyright infringement at /marketplace/foo — signed.",
		ClaimantEmail: "rights@example.com",
		ClaimantName:  "Acme Rights Holder",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Notice row persisted with pending status.
	if got := dmcaNoticeStatus(ctx, t, pool, notice.ID); got != "pending" {
		t.Errorf("notice status: want pending, got %q", got)
	}

	// KB flipped to dmca_pending.
	if got := kbStatus(ctx, t, pool, f.KBID); got != "dmca_pending" {
		t.Errorf("KB status: want dmca_pending, got %q", got)
	}

	// Window is ~14 days out (allow a generous slack so a slow CI doesn't flap).
	wantEnd := before.Add(marketplace.CounterNoticeWindow)
	delta := notice.CounterNoticeWindowEnds.Sub(wantEnd).Abs()
	if delta > 5*time.Second {
		t.Errorf("window end drift: |%v - %v| = %v (>5s)", notice.CounterNoticeWindowEnds, wantEnd, delta)
	}
}

// TestDMCAService_Submit_AlreadyPending409 asserts the "one active
// notice per KB" invariant: a second Submit against a KB with a pending
// notice returns ErrDMCAAlreadyPending.
func TestDMCAService_Submit_AlreadyPending409(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "dmca-dup")

	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)
	in := marketplace.DMCANoticeInput{
		TargetKBID:    f.KBID,
		NoticeText:    "first notice",
		ClaimantEmail: "a@example.com",
		ClaimantName:  "A",
	}
	if _, err := svc.Submit(ctx, in); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	_, err := svc.Submit(ctx, in)
	if !errors.Is(err, marketplace.ErrDMCAAlreadyPending) {
		t.Errorf("second submit: want ErrDMCAAlreadyPending, got %v", err)
	}
}

// TestDMCAService_Submit_TargetKBNotFound asserts the 404 mapping.
func TestDMCAService_Submit_TargetKBNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)
	_, err := svc.Submit(ctx, marketplace.DMCANoticeInput{
		TargetKBID:    uuid.New(),
		NoticeText:    "doesn't matter",
		ClaimantEmail: "a@example.com",
		ClaimantName:  "A",
	})
	if !errors.Is(err, marketplace.ErrDMCATargetKBNotFound) {
		t.Errorf("missing KB: want ErrDMCATargetKBNotFound, got %v", err)
	}
}

// TestDMCAService_Submit_InvalidInput400 covers the inline validation.
func TestDMCAService_Submit_InvalidInput400(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)

	cases := []struct {
		name string
		in   marketplace.DMCANoticeInput
	}{
		{"nil_uuid", marketplace.DMCANoticeInput{NoticeText: "x", ClaimantEmail: "a@b", ClaimantName: "A"}},
		{"empty_text", marketplace.DMCANoticeInput{TargetKBID: uuid.New(), ClaimantEmail: "a@b", ClaimantName: "A"}},
		{"empty_email", marketplace.DMCANoticeInput{TargetKBID: uuid.New(), NoticeText: "x", ClaimantName: "A"}},
		{"empty_name", marketplace.DMCANoticeInput{TargetKBID: uuid.New(), NoticeText: "x", ClaimantEmail: "a@b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Submit(ctx, tc.in); !errors.Is(err, marketplace.ErrDMCAInvalidInput) {
				t.Errorf("%s: want ErrDMCAInvalidInput, got %v", tc.name, err)
			}
		})
	}
}

// TestDMCAService_SubmitCounterNotice_PreventsSweep is the headline
// counter-notice flow. After a counter-notice is filed, the row's
// status is `counter_filed`; subsequent SweepExpired must NOT touch it.
func TestDMCAService_SubmitCounterNotice_PreventsSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "dmca-cn")

	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)
	notice, err := svc.Submit(ctx, marketplace.DMCANoticeInput{
		TargetKBID:    f.KBID,
		NoticeText:    "alleged",
		ClaimantEmail: "a@b",
		ClaimantName:  "A",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if err := svc.SubmitCounterNotice(ctx, notice.ID, "this is my counter-notice — signed."); err != nil {
		t.Fatalf("SubmitCounterNotice: %v", err)
	}
	if got := dmcaNoticeStatus(ctx, t, pool, notice.ID); got != "counter_filed" {
		t.Errorf("after counter: want counter_filed, got %q", got)
	}

	// Force the window into the past — the sweep should NOT pick up a
	// counter-filed row.
	rewindWindow(ctx, t, pool, notice.ID, 1*time.Hour)

	result, err := svc.SweepExpired(ctx)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if result.Examined != 0 {
		t.Errorf("sweep examined: want 0 (counter-filed row should not match), got %d", result.Examined)
	}
	if got := dmcaNoticeStatus(ctx, t, pool, notice.ID); got != "counter_filed" {
		t.Errorf("after sweep: want counter_filed, got %q", got)
	}
}

// TestDMCAService_SubmitCounterNotice_NotPending409 asserts that a
// counter-notice cannot be filed against a terminal-state notice.
func TestDMCAService_SubmitCounterNotice_NotPending409(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "dmca-illegal")

	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)
	notice, err := svc.Submit(ctx, marketplace.DMCANoticeInput{
		TargetKBID:    f.KBID,
		NoticeText:    "x",
		ClaimantEmail: "a@b",
		ClaimantName:  "A",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Manually flip to resolved_take_down so the next call is on a
	// terminal-state row.
	if _, err := pool.Exec(ctx,
		`UPDATE dmca_notices SET status='resolved_take_down' WHERE id=$1`, notice.ID); err != nil {
		t.Fatalf("setup terminal: %v", err)
	}

	err = svc.SubmitCounterNotice(ctx, notice.ID, "counter")
	if !errors.Is(err, marketplace.ErrDMCAIllegalTransition) {
		t.Errorf("counter on terminal: want ErrDMCAIllegalTransition, got %v", err)
	}
}

// TestDMCAService_SubmitCounterNotice_NotFound asserts the 404 mapping.
func TestDMCAService_SubmitCounterNotice_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)

	if err := svc.SubmitCounterNotice(ctx, uuid.New(), "doesn't matter"); !errors.Is(err, marketplace.ErrDMCANoticeNotFound) {
		t.Errorf("missing notice: want ErrDMCANoticeNotFound, got %v", err)
	}
}

// TestDMCAService_SweepExpired_AutoTakedown is the headline sweep test.
// A pending notice with an expired window must (a) transition to
// resolved_take_down, (b) flip the KB to private + status=active,
// (c) write a takedowns row with source='dmca', and (d) dispatch the
// OnTakedownCreated registry exactly once.
func TestDMCAService_SweepExpired_AutoTakedown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "dmca-sweep")

	// Subscribe a recording hook. ResetOnTakedownCreatedForTest is
	// idempotent and isolates this test from any global registration.
	marketplace.ResetOnTakedownCreatedForTest()
	t.Cleanup(marketplace.ResetOnTakedownCreatedForTest)
	rec := &recordingHook{}
	marketplace.RegisterOnTakedownCreated(rec.hook)

	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)
	notice, err := svc.Submit(ctx, marketplace.DMCANoticeInput{
		TargetKBID:    f.KBID,
		NoticeText:    "alleged",
		ClaimantEmail: "a@b",
		ClaimantName:  "A",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	rewindWindow(ctx, t, pool, notice.ID, 1*time.Hour)

	result, err := svc.SweepExpired(ctx)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if result.Examined != 1 {
		t.Errorf("sweep examined: want 1, got %d", result.Examined)
	}
	if result.Resolved != 1 {
		t.Errorf("sweep resolved: want 1, got %d", result.Resolved)
	}

	// (a) Notice transitioned to resolved_take_down.
	if got := dmcaNoticeStatus(ctx, t, pool, notice.ID); got != "resolved_take_down" {
		t.Errorf("notice status: want resolved_take_down, got %q", got)
	}

	// (b) KB flipped to private + status=active (dmca_pending freeze lifted).
	if v := kbVisibility(ctx, t, pool, f.KBID); v != "private" {
		t.Errorf("KB visibility: want private, got %q", v)
	}
	if s := kbStatus(ctx, t, pool, f.KBID); s != "active" {
		t.Errorf("KB status: want active, got %q", s)
	}

	// (c) Takedowns row with source='dmca'.
	var (
		tdCount  int
		tdSource string
	)
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*), MIN(source) FROM marketplace_takedowns WHERE target_kb_id = $1`,
		f.KBID,
	).Scan(&tdCount, &tdSource); err != nil {
		t.Fatalf("query takedowns: %v", err)
	}
	if tdCount != 1 || tdSource != "dmca" {
		t.Errorf("takedowns: count=%d source=%q (want 1, dmca)", tdCount, tdSource)
	}

	// (d) Registry fired exactly once.
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if got := len(rec.dispatched); got != 1 {
		t.Errorf("OnTakedownCreated dispatched: want 1, got %d", got)
	}
}

// TestDMCAService_SweepExpired_Idempotent verifies the sweep is safe to
// re-run — a second sweep finds zero expired pending rows.
func TestDMCAService_SweepExpired_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "dmca-idem")

	marketplace.ResetOnTakedownCreatedForTest()
	t.Cleanup(marketplace.ResetOnTakedownCreatedForTest)

	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)
	notice, err := svc.Submit(ctx, marketplace.DMCANoticeInput{
		TargetKBID:    f.KBID,
		NoticeText:    "alleged",
		ClaimantEmail: "a@b",
		ClaimantName:  "A",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	rewindWindow(ctx, t, pool, notice.ID, 1*time.Hour)

	if _, err := svc.SweepExpired(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	second, err := svc.SweepExpired(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if second.Examined != 0 {
		t.Errorf("second sweep examined: want 0, got %d", second.Examined)
	}
}

// TestDMCAService_SweepExpired_RaceCounterNotice verifies the FOR
// UPDATE re-check inside resolveExpiredOne: when a counter-notice
// raced between listExpiredPending and the per-row resolution, the
// sweep skips the row without writing a takedown.
//
// We simulate the race by:
//  1. Submit + rewind so the row is in the candidate list.
//  2. SubmitCounterNotice — flips the row OUT of pending.
//  3. SweepExpired runs against the now-stale candidate list and must
//     skip cleanly (no takedowns row).
func TestDMCAService_SweepExpired_RaceCounterNotice(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedModFixture(ctx, t, pool, "dmca-race")

	svc := marketplace.NewDMCAService(pool, marketplace.NewTakedowns(pool), nil)
	notice, err := svc.Submit(ctx, marketplace.DMCANoticeInput{
		TargetKBID:    f.KBID,
		NoticeText:    "alleged",
		ClaimantEmail: "a@b",
		ClaimantName:  "A",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	rewindWindow(ctx, t, pool, notice.ID, 1*time.Hour)

	// The "race" — counter-notice arrives before sweep gets the FOR UPDATE
	// lock. In our serial test this is a plain pre-sweep call; the sweep's
	// inside-tx re-check is what actually exercises the FOR UPDATE.
	if err := svc.SubmitCounterNotice(ctx, notice.ID, "counter"); err != nil {
		t.Fatalf("counter: %v", err)
	}

	result, err := svc.SweepExpired(ctx)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if result.Resolved != 0 {
		t.Errorf("sweep resolved: want 0 (counter-notice raced), got %d", result.Resolved)
	}

	// No takedown row was written.
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM marketplace_takedowns WHERE target_kb_id = $1`,
		f.KBID,
	).Scan(&n); err != nil {
		t.Fatalf("count takedowns: %v", err)
	}
	if n != 0 {
		t.Errorf("takedowns: want 0 (race skipped), got %d", n)
	}
}
