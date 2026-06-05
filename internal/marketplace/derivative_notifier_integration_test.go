// Integration tests for the derivative-owner notifier walker + the
// takedown audit-log pagination. Both touch the SQL that lives in
// derivative_notifier.go / takedowns.go (ListAudit) — split into a
// separate file so the unit tests in derivative_notifier_test.go stay
// fast.
//
// All tests skip under `go test -short` so the fast inner loop stays
// free of testcontainers.

package marketplace_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// derivFixture mirrors modFixture but extends it with two importer
// orgs (each holding a private derivative KB that imported from the
// upstream Public KB), an extra importer org whose KB is a derivative
// of a derivative (the "second hop" we must NOT notify), and a
// workspace_admin user per importer Org.
type derivFixture struct {
	UpstreamOrgID uuid.UUID
	UpstreamKBID  uuid.UUID

	Importer1OrgID    uuid.UUID
	Importer1WSID    uuid.UUID
	Importer1KBID    uuid.UUID
	Importer1AdminID uuid.UUID

	Importer2OrgID    uuid.UUID
	Importer2WSID    uuid.UUID
	Importer2KBID    uuid.UUID
	Importer2AdminID uuid.UUID

	Importer3OrgID    uuid.UUID
	Importer3WSID    uuid.UUID
	Importer3KBID    uuid.UUID
	Importer3AdminID uuid.UUID

	// "Second hop" derivative whose source is Importer1KBID, not the
	// taken-down upstream. Must NOT receive a notice.
	SecondHopOrgID    uuid.UUID
	SecondHopWSID    uuid.UUID
	SecondHopKBID    uuid.UUID
	SecondHopAdminID uuid.UUID
}

// recordingNotifierInt is the integration-side recorder; we don't share
// recordingNotifier with the unit-test file because go's internal
// linker won't let us cross test packages — each file in a _test
// package compiles separately when picked by `go test -run`.
type recordingNotifierInt struct {
	got []marketplace.DerivativeNotice
}

func (r *recordingNotifierInt) NotifyDerivativeTakedown(_ context.Context, n marketplace.DerivativeNotice) error {
	r.got = append(r.got, n)
	return nil
}

// seedDerivFixture wires the full lineage graph in a single transaction
// to keep the test deterministic. Slugs are generated with the same
// `label || substring(id,1,8)` shape as seedModFixture so we don't
// collide on the unique slug constraints when this test runs alongside
// the moderation suite.
func seedDerivFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool) derivFixture {
	t.Helper()
	f := derivFixture{
		UpstreamOrgID:    uuid.New(),
		UpstreamKBID:     uuid.New(),
		Importer1OrgID:   uuid.New(),
		Importer1WSID:    uuid.New(),
		Importer1KBID:    uuid.New(),
		Importer1AdminID: uuid.New(),
		Importer2OrgID:   uuid.New(),
		Importer2WSID:    uuid.New(),
		Importer2KBID:    uuid.New(),
		Importer2AdminID: uuid.New(),
		Importer3OrgID:   uuid.New(),
		Importer3WSID:    uuid.New(),
		Importer3KBID:    uuid.New(),
		Importer3AdminID: uuid.New(),
		SecondHopOrgID:   uuid.New(),
		SecondHopWSID:    uuid.New(),
		SecondHopKBID:    uuid.New(),
		SecondHopAdminID: uuid.New(),
	}
	// Upstream Org + upstream Public KB (no workspace required for the
	// test — the upstream is what is being taken down; we only need its
	// org name to populate SourceOrgDisplayName in the notice).
	if _, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug)
		 VALUES ($1, 'Upstream Org', 'upstream-' || substring($1::text from 1 for 8))`,
		f.UpstreamOrgID,
	); err != nil {
		t.Fatalf("seed upstream org: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO knowledge_bases (id, org_id, workspace_id, name, slug)
		 VALUES ($1, $2, NULL, 'Upstream KB', 'upstream-kb-' || substring($1::text from 1 for 8))`,
		f.UpstreamKBID, f.UpstreamOrgID,
	); err != nil {
		t.Fatalf("seed upstream kb: %v", err)
	}

	seedImporter := func(name string, orgID, wsID, kbID, adminID, sourceKBID uuid.UUID) {
		t.Helper()
		if _, err := pool.Exec(ctx,
			`INSERT INTO organizations (id, name, slug)
			 VALUES ($1, $2, $2 || '-' || substring($1::text from 1 for 8))`,
			orgID, name,
		); err != nil {
			t.Fatalf("seed %s org: %v", name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO workspaces (id, org_id, name, slug)
			 VALUES ($1, $2, $3, $3 || '-' || substring($1::text from 1 for 8))`,
			wsID, orgID, name+"-ws",
		); err != nil {
			t.Fatalf("seed %s workspace: %v", name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO users (id, org_id, email, status)
			 VALUES ($1, $2, $3 || '-' || substring($1::text from 1 for 8) || '@example.com', 'active')`,
			adminID, orgID, name+"-admin",
		); err != nil {
			t.Fatalf("seed %s admin user: %v", name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO workspace_members (workspace_id, user_id, role, org_id)
			 VALUES ($1, $2, 'admin', $3)`,
			wsID, adminID, orgID,
		); err != nil {
			t.Fatalf("seed %s workspace_members: %v", name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO knowledge_bases (id, org_id, workspace_id, name, slug, source_public_kb_id)
			 VALUES ($1, $2, $3, $4, $4 || '-' || substring($1::text from 1 for 8), $5)`,
			kbID, orgID, wsID, name+"-kb", sourceKBID,
		); err != nil {
			t.Fatalf("seed %s kb: %v", name, err)
		}
	}

	seedImporter("importer1", f.Importer1OrgID, f.Importer1WSID, f.Importer1KBID, f.Importer1AdminID, f.UpstreamKBID)
	seedImporter("importer2", f.Importer2OrgID, f.Importer2WSID, f.Importer2KBID, f.Importer2AdminID, f.UpstreamKBID)
	seedImporter("importer3", f.Importer3OrgID, f.Importer3WSID, f.Importer3KBID, f.Importer3AdminID, f.UpstreamKBID)
	// Second-hop derivative: imports from Importer1KBID, NOT from the
	// upstream. Must NOT show up in the notifier sweep.
	seedImporter("secondhop", f.SecondHopOrgID, f.SecondHopWSID, f.SecondHopKBID, f.SecondHopAdminID, f.Importer1KBID)
	return f
}

// TestDerivativeNotifier_OneHopFanOut asserts the SQL walker contacts
// every direct-derivative admin exactly once and skips the second-hop
// derivative.
func TestDerivativeNotifier_OneHopFanOut(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedDerivFixture(ctx, t, pool)

	recorder := &recordingNotifierInt{}
	d := marketplace.NewDerivativeNotifier(pool, recorder)
	if err := d.NotifyDerivativeOwners(ctx, f.UpstreamKBID, "trademark dispute"); err != nil {
		t.Fatalf("NotifyDerivativeOwners: %v", err)
	}

	if got, want := len(recorder.got), 3; got != want {
		t.Fatalf("notices: want %d, got %d: %+v", want, got, recorder.got)
	}

	// Stable comparison: sort by recipient email and assert the set.
	sort.Slice(recorder.got, func(i, j int) bool {
		return recorder.got[i].RecipientEmail < recorder.got[j].RecipientEmail
	})

	for _, n := range recorder.got {
		if n.SourceKBID != f.UpstreamKBID {
			t.Errorf("SourceKBID mismatch: got %v want %v", n.SourceKBID, f.UpstreamKBID)
		}
		if n.SourceOrgDisplayName != "Upstream Org" {
			t.Errorf("SourceOrgDisplayName: %q", n.SourceOrgDisplayName)
		}
		if n.SourceKBName != "Upstream KB" {
			t.Errorf("SourceKBName: %q", n.SourceKBName)
		}
		if n.Reason != "trademark dispute" {
			t.Errorf("Reason: %q", n.Reason)
		}
		if n.RecipientEmail == "" {
			t.Errorf("empty RecipientEmail in %+v", n)
		}
	}

	// Verify second-hop is NOT contacted: no notice should reference
	// the second-hop derivative KB.
	for _, n := range recorder.got {
		if n.DerivativeKBID == f.SecondHopKBID {
			t.Errorf("second-hop derivative leaked into notices: %+v", n)
		}
	}
}

// TestDerivativeNotifier_NoDerivatives asserts the walker no-ops cleanly
// when nothing imports from the taken-down KB. The contract: zero
// notifier calls, no error.
func TestDerivativeNotifier_NoDerivatives(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	orgID := uuid.New()
	kbID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug)
		 VALUES ($1, 'Lonely Org', 'lonely-' || substring($1::text from 1 for 8))`, orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO knowledge_bases (id, org_id, name, slug)
		 VALUES ($1, $2, 'Lonely KB', 'lonely-kb-' || substring($1::text from 1 for 8))`, kbID, orgID); err != nil {
		t.Fatalf("seed kb: %v", err)
	}

	rec := &recordingNotifierInt{}
	d := marketplace.NewDerivativeNotifier(pool, rec)
	if err := d.NotifyDerivativeOwners(ctx, kbID, "any"); err != nil {
		t.Fatalf("NotifyDerivativeOwners: %v", err)
	}
	if len(rec.got) != 0 {
		t.Errorf("notices: want 0, got %d", len(rec.got))
	}
}

// TestTakedowns_ListAudit_PaginatesNewestFirst seeds three takedown rows
// across two orgs, pages two at a time, and verifies (1) ordering is
// newest-first, (2) the cursor opens the second page correctly, (3) the
// strike count comes from the publisher Org.
func TestTakedowns_ListAudit_PaginatesNewestFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	// Org A: 2 takedowns, strike counter at 5.
	// Org B: 1 takedown,  strike counter at 1.
	orgA := uuid.New()
	orgB := uuid.New()
	kbA1 := uuid.New()
	kbA2 := uuid.New()
	kbB1 := uuid.New()
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec: %v\nsql=%s", err, sql)
		}
	}
	mustExec(`INSERT INTO organizations (id, name, slug, takedown_strikes)
	          VALUES ($1, 'Org A', 'orga-' || substring($1::text from 1 for 8), 5)`, orgA)
	mustExec(`INSERT INTO organizations (id, name, slug, takedown_strikes)
	          VALUES ($1, 'Org B', 'orgb-' || substring($1::text from 1 for 8), 1)`, orgB)
	mustExec(`INSERT INTO knowledge_bases (id, org_id, name, slug)
	          VALUES ($1, $2, 'A1', 'a1-' || substring($1::text from 1 for 8))`, kbA1, orgA)
	mustExec(`INSERT INTO knowledge_bases (id, org_id, name, slug)
	          VALUES ($1, $2, 'A2', 'a2-' || substring($1::text from 1 for 8))`, kbA2, orgA)
	mustExec(`INSERT INTO knowledge_bases (id, org_id, name, slug)
	          VALUES ($1, $2, 'B1', 'b1-' || substring($1::text from 1 for 8))`, kbB1, orgB)

	// Insert takedowns with explicit timestamps so we can assert the
	// sort order deterministically (default DEFAULT now() would be too
	// close together to test).
	t0 := time.Now().Add(-3 * time.Hour).UTC()
	t1 := time.Now().Add(-2 * time.Hour).UTC()
	t2 := time.Now().Add(-1 * time.Hour).UTC()
	mustExec(`INSERT INTO marketplace_takedowns (target_kb_id, source, notes, created_at)
	          VALUES ($1, 'publisher', 'self-unpublish', $2)`, kbA1, t0)
	mustExec(`INSERT INTO marketplace_takedowns (target_kb_id, source, notes, created_at)
	          VALUES ($1, 'admin', 'report confirmed', $2)`, kbB1, t1)
	mustExec(`INSERT INTO marketplace_takedowns (target_kb_id, source, notes, created_at)
	          VALUES ($1, 'dmca', 'DMCA notice 2026-06-01', $2)`, kbA2, t2)

	repo := marketplace.NewTakedowns(pool)

	// Page 1: limit=2, no cursor → newest-first = (kbA2, kbB1).
	page1, next1, err := repo.ListAudit(ctx, 2, "")
	if err != nil {
		t.Fatalf("ListAudit page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 size: want 2, got %d", len(page1))
	}
	if page1[0].TargetKBID != kbA2 {
		t.Errorf("page1[0]: want kbA2, got %v", page1[0].TargetKBID)
	}
	if page1[0].Source != marketplace.TakedownSourceDMCA {
		t.Errorf("page1[0].Source: %q", page1[0].Source)
	}
	if page1[0].StrikesAfterOrgTotal != 5 {
		t.Errorf("page1[0].StrikesAfterOrgTotal: want 5, got %d", page1[0].StrikesAfterOrgTotal)
	}
	if page1[1].TargetKBID != kbB1 {
		t.Errorf("page1[1]: want kbB1, got %v", page1[1].TargetKBID)
	}
	if page1[1].StrikesAfterOrgTotal != 1 {
		t.Errorf("page1[1].StrikesAfterOrgTotal: want 1, got %d", page1[1].StrikesAfterOrgTotal)
	}
	if next1 == "" {
		t.Fatal("expected non-empty next cursor on full page")
	}

	// Page 2: cursor from page 1 → (kbA1).
	page2, next2, err := repo.ListAudit(ctx, 2, next1)
	if err != nil {
		t.Fatalf("ListAudit page 2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("page2 size: want 1, got %d", len(page2))
	}
	if page2[0].TargetKBID != kbA1 {
		t.Errorf("page2[0]: want kbA1, got %v", page2[0].TargetKBID)
	}
	if next2 != "" {
		t.Errorf("page2 next cursor: want empty (last page), got %q", next2)
	}
}

// TestTakedowns_ListAudit_BadCursor surfaces ErrInvalidAuditCursor on
// garbage input — the handler maps this to HTTP 400.
func TestTakedowns_ListAudit_BadCursor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	repo := marketplace.NewTakedowns(pool)
	_, _, err := repo.ListAudit(context.Background(), 10, "not-a-real-cursor")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	// Sentinel comparison: handler layer relies on errors.Is.
	if !marketplaceErrIs(err, marketplace.ErrInvalidAuditCursor) {
		t.Errorf("want ErrInvalidAuditCursor in chain, got %v", err)
	}
}

func marketplaceErrIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type wrapped interface{ Unwrap() error }
		w, ok := err.(wrapped)
		if !ok {
			return false
		}
		err = w.Unwrap()
	}
	return false
}
