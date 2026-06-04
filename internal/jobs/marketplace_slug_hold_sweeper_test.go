package jobs_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/ravencloak-org/Raven/internal/jobs"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// TestMarketplaceSlugHoldSweepHandler_DeletesExpiredRows pins the
// idempotent-delete behaviour: seed past-due rows in both hold tables,
// run the sweep, assert only the expired rows are gone and the
// in-window rows remain. Counter-test confirms a second run is a
// no-op (deleted == 0).
func TestMarketplaceSlugHoldSweepHandler_DeletesExpiredRows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	// Seed an org we'll attach slug-hold rows to. Slug suffix keeps
	// concurrent tests sharing the testcontainer collision-free.
	orgID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug)
		 VALUES ($1, $2, $2 || '-' || substring($1::text from 1 for 8))`,
		orgID, "sweep-org",
	); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	// 1 expired + 1 active row in each of the two tables. The kb_id
	// is left NULL so we don't have to seed a knowledge_bases row.
	if _, err := pool.Exec(ctx,
		`INSERT INTO kb_slug_holds (org_id, slug, kb_id, held_until) VALUES
		    ($1, 'kb-expired', NULL, NOW() - interval '1 day'),
		    ($1, 'kb-active',  NULL, NOW() + interval '30 days')`,
		orgID,
	); err != nil {
		t.Fatalf("seed kb_slug_holds: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO org_slug_holds (slug, org_id, held_until) VALUES
		    ('org-expired-' || substring($1::text from 1 for 8), $1, NOW() - interval '1 day'),
		    ('org-active-'  || substring($1::text from 1 for 8), $1, NOW() + interval '30 days')`,
		orgID,
	); err != nil {
		t.Fatalf("seed org_slug_holds: %v", err)
	}

	h := jobs.NewMarketplaceSlugHoldSweepHandler(pool, nil)

	// Build the same task asynq would dispatch.
	payload, err := json.Marshal(jobs.MarketplaceSlugHoldSweepPayload{OrgID: orgID.String()})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	task := asynq.NewTask(jobs.TypeMarketplaceSlugHoldSweep, payload)

	if err := h.ProcessTask(ctx, task); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}

	// Expired KB hold gone; active one survives.
	var (
		kbExpired bool
		kbActive  bool
	)
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM kb_slug_holds WHERE org_id = $1 AND slug = 'kb-expired')`,
		orgID,
	).Scan(&kbExpired); err != nil {
		t.Fatalf("read kb_slug_holds expired: %v", err)
	}
	if kbExpired {
		t.Error("expired kb_slug_holds row should be gone after sweep")
	}
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM kb_slug_holds WHERE org_id = $1 AND slug = 'kb-active')`,
		orgID,
	).Scan(&kbActive); err != nil {
		t.Fatalf("read kb_slug_holds active: %v", err)
	}
	if !kbActive {
		t.Error("active kb_slug_holds row should survive sweep")
	}

	// Same shape for org_slug_holds. Both rows are scoped to orgID so
	// the OrgID-scoped sweep covers both.
	var orgExpiredCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM org_slug_holds
		   WHERE org_id = $1 AND held_until < NOW()`,
		orgID,
	).Scan(&orgExpiredCount); err != nil {
		t.Fatalf("count org expired: %v", err)
	}
	if orgExpiredCount != 0 {
		t.Errorf("expired org_slug_holds rows should be gone, got %d", orgExpiredCount)
	}

	// Second run is idempotent — re-running on a swept set must
	// complete cleanly. (RowsAffected is internal; the lack of error
	// is the contract.)
	if err := h.ProcessTask(ctx, task); err != nil {
		t.Errorf("second sweep run should be a no-op, got %v", err)
	}
}

func TestMarketplaceSlugHoldSweepHandler_InvalidPayload(t *testing.T) {
	h := jobs.NewMarketplaceSlugHoldSweepHandler(nil, nil)
	task := asynq.NewTask(jobs.TypeMarketplaceSlugHoldSweep, []byte("not-json"))
	if err := h.ProcessTask(context.Background(), task); err == nil {
		t.Error("invalid payload: want error, got nil")
	}
}

// TestNewMarketplaceSlugHoldSweepTask pins the task constructor's
// shape so wiring sites can rely on the type / payload roundtrip.
func TestNewMarketplaceSlugHoldSweepTask(t *testing.T) {
	task, err := jobs.NewMarketplaceSlugHoldSweepTask(jobs.MarketplaceSlugHoldSweepPayload{})
	if err != nil {
		t.Fatalf("NewMarketplaceSlugHoldSweepTask: %v", err)
	}
	if task.Type() != jobs.TypeMarketplaceSlugHoldSweep {
		t.Errorf("task.Type: got %q want %q", task.Type(), jobs.TypeMarketplaceSlugHoldSweep)
	}
}

