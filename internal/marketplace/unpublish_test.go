// Integration tests for the marketplace.UnpublishService. Exercises the
// transactional unpublish path against a real Postgres so the
// `kb_slug_holds` insert, the visibility flip, and the "existing
// imports unaffected" invariant from ADR-0001 are all checked end to
// end. Skipped under `go test -short`.

package marketplace_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/testutil"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// unpubFixture is the minimum seed a single unpublish-path test needs:
// one Org, one workspace, one Public KB. Per-call UUIDs keep parallel
// tests sharing a testcontainer isolated.
type unpubFixture struct {
	OrgID  uuid.UUID
	WSID   uuid.UUID
	KBID   uuid.UUID
	KBSlug string
}

// seedPublicKBForUnpublish inserts an org + workspace + a single Public KB ready to
// be unpublished. Org slugs are suffixed with the UUID prefix so the
// CHECK regex on organizations.slug accepts them.
func seedPublicKBForUnpublish(ctx context.Context, t *testing.T, pool *pgxpool.Pool, label string) unpubFixture {
	t.Helper()
	f := unpubFixture{
		OrgID:  uuid.New(),
		WSID:   uuid.New(),
		KBID:   uuid.New(),
		KBSlug: label + "-kb-" + uuid.New().String()[:8],
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
		`INSERT INTO knowledge_bases (id, org_id, workspace_id, name, slug, visibility, license_spdx_id, published_at)
		 VALUES ($1, $2, $3, $4, $5, 'public', 'MIT', NOW())`,
		f.KBID, f.OrgID, f.WSID, label+"-kb", f.KBSlug,
	); err != nil {
		t.Fatalf("seed public knowledge_base: %v", err)
	}
	return f
}

// noopGate is a publish gate that always allows the action. Used by
// the happy-path tests so we don't pull in KBStatusGate plumbing.
func noopGate(_ context.Context, _ pgx.Tx, _, _ string) error { return nil }

func TestUnpublishKB_Success_FlipsVisibilityAndRegistersHold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedPublicKBForUnpublish(ctx, t, pool, "happy")

	svc := marketplace.NewUnpublishService(pool, noopGate)

	start := time.Now()
	res, err := svc.UnpublishKB(ctx, f.OrgID.String(), f.KBID.String())
	if err != nil {
		t.Fatalf("UnpublishKB: %v", err)
	}

	if res.Visibility != model.KBVisibilityPrivate {
		t.Errorf("visibility: got %q want private", res.Visibility)
	}
	wantApprox := start.Add(marketplace.UnpublishHold)
	if res.SlugHeldUntil.Before(wantApprox.Add(-1*time.Minute)) ||
		res.SlugHeldUntil.After(wantApprox.Add(1*time.Minute)) {
		t.Errorf("SlugHeldUntil: %v should be ~%v (within 1m)", res.SlugHeldUntil, wantApprox)
	}

	// Database row must reflect the flip.
	var vis string
	var publishedAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT visibility, published_at FROM knowledge_bases WHERE id = $1`,
		f.KBID,
	).Scan(&vis, &publishedAt); err != nil {
		t.Fatalf("read kb row: %v", err)
	}
	if vis != "private" {
		t.Errorf("kb.visibility: got %q want private", vis)
	}
	if publishedAt != nil {
		t.Errorf("kb.published_at should be NULL after unpublish, got %v", publishedAt)
	}

	// Hold row must exist with the right (org_id, slug, kb_id) and a
	// held_until in the future.
	var heldKBID *uuid.UUID
	var heldUntil time.Time
	if err := pool.QueryRow(ctx,
		`SELECT kb_id, held_until FROM kb_slug_holds WHERE org_id = $1 AND slug = $2`,
		f.OrgID, f.KBSlug,
	).Scan(&heldKBID, &heldUntil); err != nil {
		t.Fatalf("read kb_slug_holds: %v", err)
	}
	if heldKBID == nil || *heldKBID != f.KBID {
		t.Errorf("hold kb_id: got %v want %v", heldKBID, f.KBID)
	}
	if !heldUntil.After(time.Now()) {
		t.Errorf("hold held_until %v should be in the future", heldUntil)
	}
}

func TestUnpublishKB_NotFound_Returns404(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedPublicKBForUnpublish(ctx, t, pool, "notfound")

	svc := marketplace.NewUnpublishService(pool, noopGate)

	// Unknown KB id under the right org → 404.
	_, err := svc.UnpublishKB(ctx, f.OrgID.String(), uuid.New().String())
	var appErr *apierror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("unknown kb id: want AppError, got %v", err)
	}
	if appErr.Code != 404 {
		t.Errorf("unknown kb id: want 404, got %d", appErr.Code)
	}
}

func TestUnpublishKB_AlreadyPrivate_Returns404(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedPublicKBForUnpublish(ctx, t, pool, "alreadypriv")

	// Force it private before the call. The service must report 404
	// rather than silently succeed — the contract distinguishes "I
	// took it down" from "it was already down".
	if _, err := pool.Exec(ctx,
		`UPDATE knowledge_bases SET visibility = 'private', published_at = NULL WHERE id = $1`,
		f.KBID,
	); err != nil {
		t.Fatalf("force-private: %v", err)
	}

	svc := marketplace.NewUnpublishService(pool, noopGate)
	_, err := svc.UnpublishKB(ctx, f.OrgID.String(), f.KBID.String())
	var appErr *apierror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("already-private kb: want AppError, got %v", err)
	}
	if appErr.Code != 404 {
		t.Errorf("already-private kb: want 404, got %d", appErr.Code)
	}
}

func TestUnpublishKB_GateRejects_Returns409(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedPublicKBForUnpublish(ctx, t, pool, "frozen")

	// Gate that always refuses with kb_frozen — simulates a Free Plan
	// downgrade-freeze trying to toggle visibility.
	frozenGate := func(_ context.Context, _ pgx.Tx, _, _ string) error {
		return apierror.NewKBFrozen("frozen")
	}
	svc := marketplace.NewUnpublishService(pool, frozenGate)

	_, err := svc.UnpublishKB(ctx, f.OrgID.String(), f.KBID.String())
	var appErr *apierror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("gate refusal: want AppError, got %v", err)
	}
	if appErr.Code != 409 || appErr.ErrorCode != "kb_frozen" {
		t.Errorf("gate refusal: want 409 kb_frozen, got %d %s", appErr.Code, appErr.ErrorCode)
	}

	// Row must NOT have flipped — the gate ran before the UPDATE.
	var vis string
	if err := pool.QueryRow(ctx,
		`SELECT visibility FROM knowledge_bases WHERE id = $1`,
		f.KBID,
	).Scan(&vis); err != nil {
		t.Fatalf("read kb row: %v", err)
	}
	if vis != "public" {
		t.Errorf("kb.visibility should remain public on gate refusal, got %q", vis)
	}
}

// TestUnpublishKB_ExistingImportsUnaffected verifies ADR-0001's
// guarantee: a Public KB's downstream imports keep their
// `source_public_kb_id` FK pointing at the publisher row even after
// the publisher unpublishes. Visibility is not a column the FK
// references, so toggling it must not cascade.
func TestUnpublishKB_ExistingImportsUnaffected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	publisher := seedPublicKBForUnpublish(ctx, t, pool, "publisher")
	importer := seedPublicKBForUnpublish(ctx, t, pool, "importer")

	// Mark the importer's KB as an import of the publisher's. Use a
	// distinct slug + private visibility so it doesn't conflict with
	// the publisher's public row.
	if _, err := pool.Exec(ctx,
		`UPDATE knowledge_bases
		    SET source_public_kb_id    = $1,
		        visibility             = 'private',
		        published_at           = NULL,
		        imported_from_revision_at = NOW()
		  WHERE id = $2`,
		publisher.KBID, importer.KBID,
	); err != nil {
		t.Fatalf("mark importer: %v", err)
	}

	svc := marketplace.NewUnpublishService(pool, noopGate)
	if _, err := svc.UnpublishKB(ctx, publisher.OrgID.String(), publisher.KBID.String()); err != nil {
		t.Fatalf("UnpublishKB: %v", err)
	}

	// Importer row must still point at the publisher row.
	var src *uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT source_public_kb_id FROM knowledge_bases WHERE id = $1`,
		importer.KBID,
	).Scan(&src); err != nil {
		t.Fatalf("read importer.source_public_kb_id: %v", err)
	}
	if src == nil || *src != publisher.KBID {
		t.Errorf("importer.source_public_kb_id: got %v want %v", src, publisher.KBID)
	}

	// And the publisher row must still exist (no cascade-delete from
	// unpublish). The FK is ON DELETE SET NULL, but we never deleted.
	var publisherExists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM knowledge_bases WHERE id = $1)`,
		publisher.KBID,
	).Scan(&publisherExists); err != nil {
		t.Fatalf("check publisher exists: %v", err)
	}
	if !publisherExists {
		t.Error("publisher KB row should still exist after unpublish")
	}
}

// TestUnpublishKB_RefreshesExistingHold confirms ON CONFLICT keeps the
// (org_id, slug) pair pinned to the newer window. If a slug is held,
// reused, then unpublished again before the first window elapses, the
// second window must start from the second unpublish — not chain off
// the first.
func TestUnpublishKB_RefreshesExistingHold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedPublicKBForUnpublish(ctx, t, pool, "refresh")

	// Pre-seed a near-expired hold for the same (org_id, slug). The
	// service must overwrite both kb_id and held_until.
	stale := time.Now().Add(1 * time.Hour)
	staleKB := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO kb_slug_holds (org_id, slug, kb_id, held_until)
		 VALUES ($1, $2, $3, $4)`,
		f.OrgID, f.KBSlug, staleKB, stale,
	); err != nil {
		// FK to knowledge_bases — use NULL since the row doesn't exist.
		// Retry without kb_id.
		if !strings.Contains(err.Error(), "foreign key") {
			t.Fatalf("seed stale hold: %v", err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO kb_slug_holds (org_id, slug, kb_id, held_until)
			 VALUES ($1, $2, NULL, $3)`,
			f.OrgID, f.KBSlug, stale,
		); err != nil {
			t.Fatalf("seed stale hold (null kb_id): %v", err)
		}
	}

	svc := marketplace.NewUnpublishService(pool, noopGate)
	res, err := svc.UnpublishKB(ctx, f.OrgID.String(), f.KBID.String())
	if err != nil {
		t.Fatalf("UnpublishKB: %v", err)
	}

	// New hold should be substantially after the stale one (90 days vs 1 hour).
	if !res.SlugHeldUntil.After(stale.Add(80 * 24 * time.Hour)) {
		t.Errorf("SlugHeldUntil should refresh to ~90d window: got %v vs stale %v",
			res.SlugHeldUntil, stale)
	}

	var heldKBID *uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT kb_id FROM kb_slug_holds WHERE org_id = $1 AND slug = $2`,
		f.OrgID, f.KBSlug,
	).Scan(&heldKBID); err != nil {
		t.Fatalf("read kb_slug_holds: %v", err)
	}
	if heldKBID == nil || *heldKBID != f.KBID {
		t.Errorf("hold kb_id should refresh to the new KB: got %v want %v", heldKBID, f.KBID)
	}
}
