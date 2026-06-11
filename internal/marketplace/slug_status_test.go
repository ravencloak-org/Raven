package marketplace_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

func slugStatusMigrationsDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}

func seedSlugStatusOrg(ctx context.Context, t *testing.T, pool *pgxpool.Pool, slug, name string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var orgID, wsID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (id, name, slug)
		 VALUES (uuid_generate_v4(), $1, $2)
		 RETURNING id`, name, slug,
	).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (id, org_id, name, slug)
		 VALUES (uuid_generate_v4(), $1, $2, $3)
		 RETURNING id`, orgID, name+" WS", slug+"-ws",
	).Scan(&wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return orgID, wsID
}

func TestMarketplaceSlugStatus_ResolvesPublicKB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := testutil.NewTestDB(t, testutil.WithMigrationsDir(slugStatusMigrationsDir(t)))

	orgID, wsID := seedSlugStatusOrg(ctx, t, pool, "ss-acme", "SS Acme")
	var kbID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO knowledge_bases
		   (org_id, workspace_id, name, slug, visibility, license_spdx_id, published_at)
		 VALUES ($1, $2, 'Alpha', 'alpha', 'public', 'MIT', NOW())
		 RETURNING id`, orgID, wsID,
	).Scan(&kbID); err != nil {
		t.Fatalf("seed public kb: %v", err)
	}

	status, err := marketplace.MarketplaceSlugStatus(ctx, pool, "ss-acme", "alpha")
	if err != nil {
		t.Fatalf("MarketplaceSlugStatus: %v", err)
	}
	if status.IsHeld {
		t.Errorf("expected IsHeld=false for live public KB")
	}
	if status.Detail == nil {
		t.Fatalf("expected non-nil Detail")
	}
	if status.KBID != kbID {
		t.Errorf("expected kb id %s, got %s", kbID, status.KBID)
	}
	if status.Detail.OrgSlug != "ss-acme" || status.Detail.KBSlug != "alpha" {
		t.Errorf("unexpected detail row: %+v", status.Detail)
	}
}

func TestMarketplaceSlugStatus_MissReturnsZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := testutil.NewTestDB(t, testutil.WithMigrationsDir(slugStatusMigrationsDir(t)))

	// Seed a public KB so the listing is non-empty; the resolver
	// should still miss on a wrong slug pair.
	orgID, wsID := seedSlugStatusOrg(ctx, t, pool, "ss-miss-org", "SS Miss")
	if _, err := pool.Exec(ctx,
		`INSERT INTO knowledge_bases
		   (org_id, workspace_id, name, slug, visibility, license_spdx_id, published_at)
		 VALUES ($1, $2, 'Beta', 'beta', 'public', 'MIT', NOW())`,
		orgID, wsID); err != nil {
		t.Fatalf("seed public kb: %v", err)
	}

	status, err := marketplace.MarketplaceSlugStatus(ctx, pool, "ss-miss-org", "nonexistent")
	if err != nil {
		t.Fatalf("MarketplaceSlugStatus: %v", err)
	}
	if status.IsHeld {
		t.Errorf("expected IsHeld=false on miss")
	}
	if status.Detail != nil {
		t.Errorf("expected Detail nil on miss, got %+v", status.Detail)
	}
}

func TestMarketplaceSlugStatus_HeldShortCircuits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := testutil.NewTestDB(t, testutil.WithMigrationsDir(slugStatusMigrationsDir(t)))

	orgID, wsID := seedSlugStatusOrg(ctx, t, pool, "ss-held-org", "SS Held")
	// Seed a Public KB under a different slug; the held lookup must
	// return IsHeld=true for the held slug irrespective of any live
	// KB row for that org.
	if _, err := pool.Exec(ctx,
		`INSERT INTO knowledge_bases
		   (org_id, workspace_id, name, slug, visibility, license_spdx_id, published_at)
		 VALUES ($1, $2, 'Live', 'live', 'public', 'MIT', NOW())`,
		orgID, wsID); err != nil {
		t.Fatalf("seed public kb: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO kb_slug_holds (org_id, slug, kb_id, held_until)
		 VALUES ($1, $2, NULL, $3)`,
		orgID, "withdrawn", time.Now().Add(7*24*time.Hour),
	); err != nil {
		t.Fatalf("seed slug hold: %v", err)
	}

	status, err := marketplace.MarketplaceSlugStatus(ctx, pool, "ss-held-org", "withdrawn")
	if err != nil {
		t.Fatalf("MarketplaceSlugStatus: %v", err)
	}
	if !status.IsHeld {
		t.Errorf("expected IsHeld=true for active hold")
	}
	if status.Detail != nil {
		t.Errorf("expected Detail nil when held, got %+v", status.Detail)
	}
}

func TestMarketplaceSlugStatus_ExpiredHoldIsMiss(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool := testutil.NewTestDB(t, testutil.WithMigrationsDir(slugStatusMigrationsDir(t)))

	orgID, _ := seedSlugStatusOrg(ctx, t, pool, "ss-exp-org", "SS Exp")
	if _, err := pool.Exec(ctx,
		`INSERT INTO kb_slug_holds (org_id, slug, kb_id, held_until)
		 VALUES ($1, $2, NULL, $3)`,
		orgID, "expired", time.Now().Add(-24*time.Hour),
	); err != nil {
		t.Fatalf("seed slug hold: %v", err)
	}

	status, err := marketplace.MarketplaceSlugStatus(ctx, pool, "ss-exp-org", "expired")
	if err != nil {
		t.Fatalf("MarketplaceSlugStatus: %v", err)
	}
	if status.IsHeld {
		t.Errorf("expected IsHeld=false for expired hold, got true")
	}
	if status.Detail != nil {
		t.Errorf("expected Detail nil for expired hold (and no live KB), got %+v", status.Detail)
	}
}
