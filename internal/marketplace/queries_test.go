package marketplace_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// resolveMigrationsDir walks up from this test file to the repo root
// (`migrations/` is a sibling of `internal/`). Without an override the
// default testutil resolver looks two levels up from internal/testutil/;
// from internal/marketplace/ the layout is identical so the same offset
// works — but we resolve explicitly here so a future package move can't
// silently break the lookup.
func resolveMigrationsDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/marketplace/queries_test.go -> repo root/migrations
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}

// seedPublisherOrg inserts an org + workspace and returns their ids. The
// caller can then attach KBs to (orgID, wsID) below. We use unique slugs
// per call so the test can be run in parallel later without collisions on
// the global org-slug index.
func seedPublisherOrg(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgSlug, orgName, wsSlug string) (uuid.UUID, uuid.UUID) {
	t.Helper()

	var orgID, wsID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (id, name, slug)
		 VALUES (uuid_generate_v4(), $1, $2)
		 RETURNING id`, orgName, orgSlug).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces (id, org_id, name, slug)
		 VALUES (uuid_generate_v4(), $1, $2, $3)
		 RETURNING id`, orgID, orgName+" WS", wsSlug).Scan(&wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return orgID, wsID
}

// seedPublicKB inserts a KB with the given metadata and (optionally) some
// chunks attached via a single source row. Returns the KB id. The
// last_modified_at column has its trigger fire when chunks are inserted,
// so the returned row's freshness reflects the seed time.
func seedPublicKB(ctx context.Context, t *testing.T, pool *pgxpool.Pool,
	orgID, wsID uuid.UUID, slug, name, license string, chunkCount int,
) uuid.UUID {
	t.Helper()

	var kbID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO knowledge_bases
		   (org_id, workspace_id, name, slug, description,
		    visibility, license_spdx_id, published_at)
		 VALUES ($1, $2, $3, $4, $5, 'public', $6, NOW())
		 RETURNING id`,
		orgID, wsID, name, slug, "desc for "+name, license).Scan(&kbID); err != nil {
		t.Fatalf("seed public KB: %v", err)
	}

	if chunkCount > 0 {
		// A document row is required by the chunks CHECK
		// (document_id OR source_id NOT NULL). Document is simpler than
		// source — fewer required columns.
		var docID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO documents (org_id, knowledge_base_id, file_name)
			 VALUES ($1, $2, $3)
			 RETURNING id`, orgID, kbID, name+".txt").Scan(&docID); err != nil {
			t.Fatalf("seed document: %v", err)
		}
		for i := 0; i < chunkCount; i++ {
			if _, err := pool.Exec(ctx,
				`INSERT INTO chunks
				   (org_id, knowledge_base_id, document_id, content, chunk_index)
				 VALUES ($1, $2, $3, $4, $5)`,
				orgID, kbID, docID, name+" chunk content "+string(rune('A'+i)), i,
			); err != nil {
				t.Fatalf("seed chunk %d: %v", i, err)
			}
		}
	}
	return kbID
}

func TestQueriesListPublicKBs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pool := testutil.NewTestDB(t, testutil.WithMigrationsDir(resolveMigrationsDir(t)))
	q := marketplace.NewQueries(pool)

	// Two publishers, three Public KBs across them, each with a distinct
	// license so the filter is observable.
	orgAID, wsAID := seedPublisherOrg(ctx, t, pool, "list-acme", "Acme", "list-acme-ws")
	orgBID, wsBID := seedPublisherOrg(ctx, t, pool, "list-globex", "Globex", "list-globex-ws")
	kbA1 := seedPublicKB(ctx, t, pool, orgAID, wsAID, "alpha", "Alpha", "MIT", 0)
	kbA2 := seedPublicKB(ctx, t, pool, orgAID, wsAID, "beta", "Beta", "Apache-2.0", 0)
	kbB1 := seedPublicKB(ctx, t, pool, orgBID, wsBID, "gamma", "Gamma", "MIT", 0)

	// A private KB on org A — must never appear in any listing result.
	var privateKB uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO knowledge_bases (org_id, workspace_id, name, slug, visibility)
		 VALUES ($1, $2, 'Secret', 'secret', 'private')
		 RETURNING id`, orgAID, wsAID).Scan(&privateKB); err != nil {
		t.Fatalf("seed private KB: %v", err)
	}

	t.Run("returns_all_public_kbs_no_filter", func(t *testing.T) {
		items, err := q.ListPublicKBs(ctx, marketplace.ListFilters{
			Sort:  marketplace.SortAlphabetic,
			Limit: 50,
		})
		if err != nil {
			t.Fatalf("ListPublicKBs: %v", err)
		}
		got := map[uuid.UUID]bool{}
		for _, it := range items {
			got[it.KBID] = true
		}
		for _, want := range []uuid.UUID{kbA1, kbA2, kbB1} {
			if !got[want] {
				t.Errorf("expected KB %s in listing", want)
			}
		}
		if got[privateKB] {
			t.Errorf("private KB %s leaked into listing", privateKB)
		}
	})

	t.Run("license_filter_narrows_results", func(t *testing.T) {
		items, err := q.ListPublicKBs(ctx, marketplace.ListFilters{
			Sort:     marketplace.SortAlphabetic,
			Licenses: []string{"MIT"},
			Limit:    50,
		})
		if err != nil {
			t.Fatalf("ListPublicKBs MIT: %v", err)
		}
		seen := map[uuid.UUID]bool{}
		for _, it := range items {
			seen[it.KBID] = true
			if it.LicenseSPDXID == nil || *it.LicenseSPDXID != "MIT" {
				t.Errorf("KB %s leaked through license filter (got %v)", it.KBID, it.LicenseSPDXID)
			}
		}
		if !seen[kbA1] || !seen[kbB1] {
			t.Errorf("MIT filter dropped expected KBs; got %d items", len(items))
		}
		if seen[kbA2] {
			t.Errorf("Apache-2.0 KB %s leaked into MIT filter", kbA2)
		}
	})

	t.Run("alphabetic_sort_is_deterministic", func(t *testing.T) {
		items, err := q.ListPublicKBs(ctx, marketplace.ListFilters{
			Sort:  marketplace.SortAlphabetic,
			Limit: 50,
		})
		if err != nil {
			t.Fatalf("ListPublicKBs alphabetic: %v", err)
		}
		// All three names should appear in ascending order. We don't check
		// every name in the listing — other tests may seed more — just that
		// our three appear in the expected relative order.
		idxA, idxB, idxG := -1, -1, -1
		for i, it := range items {
			switch it.KBID {
			case kbA1:
				idxA = i
			case kbA2:
				idxB = i
			case kbB1:
				idxG = i
			}
		}
		if idxA >= idxB || idxB >= idxG {
			t.Errorf("alphabetic order broken: Alpha=%d Beta=%d Gamma=%d", idxA, idxB, idxG)
		}
	})

	t.Run("limit_clamp_to_50_in_function", func(t *testing.T) {
		// Even though we ask for 1000, the SQL function clamps. We can't
		// observe the clamp directly without seeding 51+ rows, but we can
		// at least confirm the request succeeds and returns no more than
		// the cap.
		items, err := q.ListPublicKBs(ctx, marketplace.ListFilters{
			Sort:  marketplace.SortNewest,
			Limit: 1000,
		})
		if err != nil {
			t.Fatalf("ListPublicKBs limit=1000: %v", err)
		}
		if len(items) > 50 {
			t.Errorf("expected ≤50 items after clamp, got %d", len(items))
		}
	})

	t.Run("unknown_sort_returns_typed_error", func(t *testing.T) {
		_, err := q.ListPublicKBs(ctx, marketplace.ListFilters{
			Sort:  marketplace.ListSort("brand-new-sort"),
			Limit: 5,
		})
		if !errors.Is(err, marketplace.ErrUnknownSort) {
			t.Errorf("expected ErrUnknownSort, got %v", err)
		}
	})
}

func TestQueriesPreviewKB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pool := testutil.NewTestDB(t, testutil.WithMigrationsDir(resolveMigrationsDir(t)))
	q := marketplace.NewQueries(pool)

	orgID, wsID := seedPublisherOrg(ctx, t, pool, "preview-acme", "Acme P", "preview-acme-ws")

	// Seed a Public KB with 5 chunks — the function must clip to 3.
	kbID := seedPublicKB(ctx, t, pool, orgID, wsID, "five-chunks", "Five Chunks", "MIT", 5)

	t.Run("returns_at_most_three_chunks_in_order", func(t *testing.T) {
		chunks, err := q.PreviewKB(ctx, kbID)
		if err != nil {
			t.Fatalf("PreviewKB: %v", err)
		}
		if len(chunks) != 3 {
			t.Fatalf("expected exactly 3 preview chunks (cap), got %d", len(chunks))
		}
		// Ordinal must be ascending and start at 0 (chunk_index assigned by
		// the seed helper in insertion order).
		for i, c := range chunks {
			if c.Ordinal != i {
				t.Errorf("chunk %d: ordinal = %d, want %d", i, c.Ordinal, i)
			}
			if !strings.Contains(c.Text, "Five Chunks chunk content") {
				t.Errorf("chunk %d: text shape unexpected: %q", i, c.Text)
			}
		}
	})

	t.Run("preview_count_bumps_on_success", func(t *testing.T) {
		var before, after int
		if err := pool.QueryRow(ctx,
			`SELECT preview_count FROM knowledge_bases WHERE id = $1`, kbID).Scan(&before); err != nil {
			t.Fatalf("read preview_count pre: %v", err)
		}
		if _, err := q.PreviewKB(ctx, kbID); err != nil {
			t.Fatalf("PreviewKB second call: %v", err)
		}
		if err := pool.QueryRow(ctx,
			`SELECT preview_count FROM knowledge_bases WHERE id = $1`, kbID).Scan(&after); err != nil {
			t.Fatalf("read preview_count post: %v", err)
		}
		if after != before+1 {
			t.Errorf("preview_count: before=%d after=%d, want delta=1", before, after)
		}
	})

	t.Run("private_kb_returns_ErrKBNotPublic", func(t *testing.T) {
		var privID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO knowledge_bases (org_id, workspace_id, name, slug, visibility)
			 VALUES ($1, $2, 'Hidden', 'hidden', 'private')
			 RETURNING id`, orgID, wsID).Scan(&privID); err != nil {
			t.Fatalf("seed private KB: %v", err)
		}
		_, err := q.PreviewKB(ctx, privID)
		if !errors.Is(err, marketplace.ErrKBNotPublic) {
			t.Errorf("private KB: expected ErrKBNotPublic, got %v", err)
		}
	})

	t.Run("unknown_id_returns_ErrKBNotPublic", func(t *testing.T) {
		_, err := q.PreviewKB(ctx, uuid.New())
		if !errors.Is(err, marketplace.ErrKBNotPublic) {
			t.Errorf("unknown KB: expected ErrKBNotPublic, got %v", err)
		}
	})
}
