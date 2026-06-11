// Integration tests for marketplace.LookupKBSlugHold. Exercises the
// resolution chain (org-slug → org-id → kb_slug_holds probe) against
// a real Postgres so the 410-Gone helper consumed by #731 has a
// pinned contract.

package marketplace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

func TestLookupKBSlugHold_PresentAndActive_ReturnsHeld(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedPublicKBForUnpublish(ctx, t, pool, "active")

	// Run the unpublish path to land an active hold. Avoids leaking
	// SQL-shaped fixtures into a test of the resolver itself.
	svc := marketplace.NewUnpublishService(pool, noopGate)
	if _, err := svc.UnpublishKB(ctx, f.OrgID.String(), f.WSID.String(), f.KBID.String()); err != nil {
		t.Fatalf("UnpublishKB: %v", err)
	}

	// Resolve the org slug back from the row so the test does not
	// hard-code the suffix logic in seedPublicKBForUnpublish.
	var orgSlug string
	if err := pool.QueryRow(ctx,
		`SELECT slug FROM organizations WHERE id = $1`,
		f.OrgID,
	).Scan(&orgSlug); err != nil {
		t.Fatalf("read org slug: %v", err)
	}

	status, err := marketplace.LookupKBSlugHold(ctx, pool, orgSlug, f.KBSlug)
	if err != nil {
		t.Fatalf("LookupKBSlugHold: %v", err)
	}
	if !status.IsHeld {
		t.Error("status.IsHeld: want true")
	}
	if status.OrgID != f.OrgID {
		t.Errorf("status.OrgID: got %v want %v", status.OrgID, f.OrgID)
	}
	if status.KBID != f.KBID {
		t.Errorf("status.KBID: got %v want %v", status.KBID, f.KBID)
	}
}

func TestLookupKBSlugHold_PresentButExpired_ReturnsNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedPublicKBForUnpublish(ctx, t, pool, "expired")

	var orgSlug string
	if err := pool.QueryRow(ctx,
		`SELECT slug FROM organizations WHERE id = $1`,
		f.OrgID,
	).Scan(&orgSlug); err != nil {
		t.Fatalf("read org slug: %v", err)
	}

	// Seed a hold that already expired. Using NULL kb_id avoids the FK
	// to the still-public KB row.
	if _, err := pool.Exec(ctx,
		`INSERT INTO kb_slug_holds (org_id, slug, kb_id, held_until)
		 VALUES ($1, $2, NULL, NOW() - interval '1 hour')`,
		f.OrgID, f.KBSlug,
	); err != nil {
		t.Fatalf("seed expired hold: %v", err)
	}

	_, err := marketplace.LookupKBSlugHold(ctx, pool, orgSlug, f.KBSlug)
	if !errors.Is(err, marketplace.ErrKBSlugNotFound) {
		t.Errorf("expired hold: want ErrKBSlugNotFound, got %v", err)
	}
}

func TestLookupKBSlugHold_NoSuchOrg_ReturnsNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	_, err := marketplace.LookupKBSlugHold(ctx, pool, "ghost-org", "ghost-kb")
	if !errors.Is(err, marketplace.ErrKBSlugNotFound) {
		t.Errorf("missing org: want ErrKBSlugNotFound, got %v", err)
	}
}

func TestLookupKBSlugHold_OrgPresentNoHold_ReturnsNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedPublicKBForUnpublish(ctx, t, pool, "noheld")

	var orgSlug string
	if err := pool.QueryRow(ctx,
		`SELECT slug FROM organizations WHERE id = $1`,
		f.OrgID,
	).Scan(&orgSlug); err != nil {
		t.Fatalf("read org slug: %v", err)
	}

	status, err := marketplace.LookupKBSlugHold(ctx, pool, orgSlug, "never-existed")
	if !errors.Is(err, marketplace.ErrKBSlugNotFound) {
		t.Errorf("org-without-hold: want ErrKBSlugNotFound, got %v", err)
	}
	// Pin the contract: when the Org resolves but the KB slug does not
	// have an active hold, the returned status must still carry the
	// resolved OrgID so callers can disambiguate "we found the Org but
	// not the slug" from a wholly-unknown Org slug.
	if status.OrgID != f.OrgID {
		t.Errorf("status.OrgID: got %v want %v", status.OrgID, f.OrgID)
	}
}

func TestLookupKBSlugHold_InvalidSlugShape_ReturnsNotFound(t *testing.T) {
	// Cheap input guard — no DB needed beyond the testutil bootstrap.
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	_, err := marketplace.LookupKBSlugHold(ctx, pool, "NOT VALID", "also bad")
	if !errors.Is(err, marketplace.ErrKBSlugNotFound) {
		t.Errorf("invalid shape: want ErrKBSlugNotFound, got %v", err)
	}
}

func TestLookupKBSlugHold_NullKBID_OrgIDPopulatedKBIDZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedPublicKBForUnpublish(ctx, t, pool, "nullkb")

	var orgSlug string
	if err := pool.QueryRow(ctx,
		`SELECT slug FROM organizations WHERE id = $1`,
		f.OrgID,
	).Scan(&orgSlug); err != nil {
		t.Fatalf("read org slug: %v", err)
	}

	// Hold with NULL kb_id (the KB was hard-deleted after the hold
	// landed). Resolver must still report IsHeld=true.
	if _, err := pool.Exec(ctx,
		`INSERT INTO kb_slug_holds (org_id, slug, kb_id, held_until)
		 VALUES ($1, $2, NULL, NOW() + interval '30 days')`,
		f.OrgID, "deleted-slug",
	); err != nil {
		t.Fatalf("seed null-kb hold: %v", err)
	}

	status, err := marketplace.LookupKBSlugHold(ctx, pool, orgSlug, "deleted-slug")
	if err != nil {
		t.Fatalf("LookupKBSlugHold: %v", err)
	}
	if !status.IsHeld {
		t.Error("status.IsHeld: want true")
	}
	if status.KBID != uuid.Nil {
		t.Errorf("status.KBID: want uuid.Nil for NULL kb_id, got %v", status.KBID)
	}
	if status.OrgID != f.OrgID {
		t.Errorf("status.OrgID: got %v want %v", status.OrgID, f.OrgID)
	}
}
