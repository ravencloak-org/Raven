package marketplace_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// TestReImporter_Constructor asserts the public constructor returns a usable
// value even with a nil pool. Matches the convention in kb_test.go.
func TestReImporter_Constructor(t *testing.T) {
	r := marketplace.NewReImporter(nil)
	if r == nil {
		t.Fatal("NewReImporter must return non-nil")
	}
	// Chainable helpers must return self so callers can compose at boot.
	if got := r.WithProjection(nil); got == nil {
		t.Fatal("WithProjection must return the receiver")
	}
	if got := r.WithClock(nil); got == nil {
		t.Fatal("WithClock must return the receiver")
	}
}

// TestProjectFromPublicKB_StubSentinel locks the stub contract: the
// production projection returns the sentinel error so a misconfigured
// deployment fails loudly. #729 will rewrite this assertion to exercise
// the real projection; the import sentinel test keeps the stub honest until
// then.
func TestProjectFromPublicKB_StubSentinel(t *testing.T) {
	_, err := marketplace.ProjectFromPublicKB(context.Background(), nil, "", "", "")
	if !errors.Is(err, marketplace.ErrProjectionNotImplemented) {
		t.Fatalf("expected ErrProjectionNotImplemented, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Integration tests — exercise the real ReImporter against a Postgres
// container started by testutil.NewTestDB. Gated by testing.Short() so the
// fast unit suite stays fast.
// ---------------------------------------------------------------------------

// reimportFixture holds the row ids for a freshly-seeded publisher + importer
// pair so individual t.Run blocks below stay short.
type reimportFixture struct {
	pubOrgID   string
	pubWSID    string
	pubKBID    string
	impOrgID   string
	impWSID    string
	impKBID    string
	pubLastMod time.Time
}

// seedReimportFixture creates two orgs (publisher + importer), one workspace
// in each, a public KB in the publisher with one document/source/chunk, and
// an imported KB in the importer with stale content and a single chat
// session pointing at it (so the runtime-preservation assertion can run).
func seedReimportFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool) reimportFixture {
	t.Helper()
	f := reimportFixture{
		pubOrgID: uuid.NewString(),
		pubWSID:  uuid.NewString(),
		pubKBID:  uuid.NewString(),
		impOrgID: uuid.NewString(),
		impWSID:  uuid.NewString(),
		impKBID:  uuid.NewString(),
	}

	// Two orgs with distinct slugs (the organizations table has a UNIQUE
	// constraint on slug per migration 00048).
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		f.pubOrgID, "Publisher Org", "pub-"+f.pubOrgID[:8],
		f.impOrgID, "Importer Org", "imp-"+f.impOrgID[:8],
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO workspaces (id, org_id, name, slug) VALUES ($1, $2, $3, $4), ($5, $6, $7, $8)`,
		f.pubWSID, f.pubOrgID, "Pub WS", "pub-ws",
		f.impWSID, f.impOrgID, "Imp WS", "imp-ws",
	)
	require.NoError(t, err)

	// Public KB on the publisher org. visibility='public' is the
	// pre-condition every Re-import lookup verifies.
	_, err = pool.Exec(ctx,
		`INSERT INTO knowledge_bases (id, org_id, workspace_id, name, slug, visibility, license_spdx_id, last_modified_at)
		 VALUES ($1, $2, $3, 'Public KB', 'public-kb', 'public', 'MIT', NOW())`,
		f.pubKBID, f.pubOrgID, f.pubWSID,
	)
	require.NoError(t, err)

	// Publisher content — one row in each child table so the wipe step has
	// something to delete.
	_, err = pool.Exec(ctx,
		`INSERT INTO documents (id, org_id, knowledge_base_id, filename, file_size, mime_type)
		 VALUES ($1, $2, $3, 'pub.pdf', 100, 'application/pdf')`,
		uuid.NewString(), f.pubOrgID, f.pubKBID,
	)
	require.NoError(t, err)

	// Importer KB — already-imported, with stale content (one chunk that the
	// re-import will delete). source_public_kb_id points at the publisher KB,
	// imported_from_revision_at is well in the past so we can assert it moves
	// forward on success.
	stale := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO knowledge_bases (id, org_id, workspace_id, name, slug,
		    source_public_kb_id, imported_from_revision_at, last_modified_at)
		 VALUES ($1, $2, $3, 'Imp KB', 'imp-kb', $4, $5, $5)`,
		f.impKBID, f.impOrgID, f.impWSID, f.pubKBID, stale,
	)
	require.NoError(t, err)

	staleDocID := uuid.NewString()
	staleSrcID := uuid.NewString()
	_, err = pool.Exec(ctx,
		`INSERT INTO documents (id, org_id, knowledge_base_id, filename, file_size, mime_type)
		 VALUES ($1, $2, $3, 'stale.pdf', 50, 'application/pdf')`,
		staleDocID, f.impOrgID, f.impKBID,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO sources (id, org_id, knowledge_base_id, url, processing_status)
		 VALUES ($1, $2, $3, 'https://stale.example/', 'completed')`,
		staleSrcID, f.impOrgID, f.impKBID,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO chunks (id, org_id, knowledge_base_id, document_id, content, chunk_index)
		 VALUES ($1, $2, $3, $4, 'stale text', 0)`,
		uuid.NewString(), f.impOrgID, f.impKBID, staleDocID,
	)
	require.NoError(t, err)

	// Now re-read the publisher's last_modified_at: the document insert
	// above triggered the bump trigger and the value is the canonical
	// publisher revision we expect imported_from_revision_at to land on.
	err = pool.QueryRow(ctx,
		`SELECT last_modified_at FROM knowledge_bases WHERE id = $1`, f.pubKBID,
	).Scan(&f.pubLastMod)
	require.NoError(t, err)

	return f
}

// fakeProjection is the deterministic projection threaded into the tests so
// we exercise the wipe + tx semantics of ReImporter without depending on
// #729's allow-list / embedding-pipeline machinery.
func fakeProjection(content string) marketplace.ProjectionFunc {
	return func(ctx context.Context, tx pgx.Tx, destOrgID, destKBID, _ string) (int, error) {
		newDocID := uuid.NewString()
		if _, err := tx.Exec(ctx,
			`INSERT INTO documents (id, org_id, knowledge_base_id, filename, file_size, mime_type)
			 VALUES ($1, $2, $3, 'reimported.pdf', 1, 'application/pdf')`,
			newDocID, destOrgID, destKBID,
		); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO chunks (id, org_id, knowledge_base_id, document_id, content, chunk_index)
			 VALUES ($1, $2, $3, $4, $5, 0)`,
			uuid.NewString(), destOrgID, destKBID, newDocID, content,
		); err != nil {
			return 0, err
		}
		return 1, nil
	}
}

// TestReImporter_HappyPath_PreservesKBIdAndRefreshesContent covers the core
// ADR-0007 §7b contract: re-import preserves the KB id (and by extension
// every runtime artefact that FKs to it — exercised via chat_sessions here)
// and replaces the content wholesale.
func TestReImporter_HappyPath_PreservesKBIdAndRefreshesContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping marketplace re-import integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedReimportFixture(ctx, t, pool)

	// Seed a chat session bound to the importer KB id. Re-import MUST NOT
	// touch this row — the assertion is the live demo of "id preserved →
	// runtime artefacts survive".
	chatID := uuid.NewString()
	_, err := pool.Exec(ctx,
		`INSERT INTO chat_sessions (id, org_id, knowledge_base_id, session_token)
		 VALUES ($1, $2, $3, $4)`,
		chatID, f.impOrgID, f.impKBID, "tok-"+chatID[:8],
	)
	require.NoError(t, err)

	r := marketplace.NewReImporter(pool).WithProjection(fakeProjection("fresh text"))

	res, err := r.ReImport(ctx, f.impOrgID, f.impKBID, uuid.NewString())
	require.NoError(t, err)

	// Same id back.
	assert.Equal(t, f.impKBID, res.KBID)
	// Pinned to publisher's revision, not now() — the value should equal
	// the publisher KB's last_modified_at at the moment we read it.
	assert.WithinDuration(t, f.pubLastMod, res.ImportedFromRevisionAt, time.Second)
	assert.Equal(t, 1, res.ChunksProjected)

	// Chat session is intact.
	var chatCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM chat_sessions WHERE id = $1 AND knowledge_base_id = $2`,
		chatID, f.impKBID,
	).Scan(&chatCount))
	assert.Equal(t, 1, chatCount, "chat_sessions row must survive re-import")

	// Stale chunk is gone; fresh chunk is present.
	var chunkContents []string
	rows, err := pool.Query(ctx,
		`SELECT content FROM chunks WHERE knowledge_base_id = $1 ORDER BY chunk_index`,
		f.impKBID,
	)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		chunkContents = append(chunkContents, c)
	}
	assert.Equal(t, []string{"fresh text"}, chunkContents)
}

// TestReImporter_Refuses_NonImport asserts the 409 path: a KB whose
// source_public_kb_id is NULL cannot be re-imported.
func TestReImporter_Refuses_NonImport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping marketplace re-import integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedReimportFixture(ctx, t, pool)

	// Wipe the lineage pointer so the importer KB looks like a locally
	// authored KB.
	_, err := pool.Exec(ctx,
		`UPDATE knowledge_bases SET source_public_kb_id = NULL WHERE id = $1`,
		f.impKBID,
	)
	require.NoError(t, err)

	r := marketplace.NewReImporter(pool).WithProjection(fakeProjection("never"))
	_, err = r.ReImport(ctx, f.impOrgID, f.impKBID, uuid.NewString())
	assert.ErrorIs(t, err, marketplace.ErrNotAnImport)
}

// TestReImporter_Refuses_UnpublishedSource asserts the 410 path: a KB whose
// upstream source has been unpublished (visibility='private') cannot be
// re-imported.
func TestReImporter_Refuses_UnpublishedSource(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping marketplace re-import integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedReimportFixture(ctx, t, pool)

	// Flip the publisher KB back to private — the Re-import path must
	// surface 410, not silently re-import private content.
	_, err := pool.Exec(ctx,
		`UPDATE knowledge_bases SET visibility = 'private' WHERE id = $1`,
		f.pubKBID,
	)
	require.NoError(t, err)

	r := marketplace.NewReImporter(pool).WithProjection(fakeProjection("never"))
	_, err = r.ReImport(ctx, f.impOrgID, f.impKBID, uuid.NewString())
	assert.ErrorIs(t, err, marketplace.ErrSourceUnpublished)
}

// TestReImporter_Refuses_DeletedSource asserts the 410 path for outright
// deletion: a KB whose source row no longer exists also returns 410.
func TestReImporter_Refuses_DeletedSource(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping marketplace re-import integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedReimportFixture(ctx, t, pool)

	// Clear the FK first so a hard DELETE on the publisher KB is allowed
	// — the importer's source_public_kb_id has ON DELETE SET NULL, but
	// the test wants to exercise the "row missing" branch directly.
	_, err := pool.Exec(ctx,
		`DELETE FROM documents WHERE knowledge_base_id = $1`, f.pubKBID,
	)
	require.NoError(t, err)
	// The Importer keeps the lineage pointer even after the publisher row
	// vanishes (ON DELETE SET NULL would null it; we instead UPDATE to a
	// random uuid so the lookup misses with ErrNoRows).
	bogus := uuid.NewString()
	_, err = pool.Exec(ctx,
		`UPDATE knowledge_bases SET source_public_kb_id = $1 WHERE id = $2`,
		bogus, f.impKBID,
	)
	require.NoError(t, err)

	r := marketplace.NewReImporter(pool).WithProjection(fakeProjection("never"))
	_, err = r.ReImport(ctx, f.impOrgID, f.impKBID, uuid.NewString())
	assert.ErrorIs(t, err, marketplace.ErrSourceUnpublished)
}

// TestReImporter_UnknownKB asserts the 404 path: a kb_id the actor's org
// does not own is reported as not found.
func TestReImporter_UnknownKB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping marketplace re-import integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	// Need at least one real org so WithOrgID's set_config call works.
	orgID := uuid.NewString()
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)`,
		orgID, "Org X", "x-"+orgID[:8],
	)
	require.NoError(t, err)

	r := marketplace.NewReImporter(pool).WithProjection(fakeProjection("never"))
	_, err = r.ReImport(ctx, orgID, uuid.NewString(), uuid.NewString())
	assert.ErrorIs(t, err, marketplace.ErrKBNotFound)
}

// TestReImporter_AtomicRollback_PreservesStaleContent is the real atomicity
// check. We inject a projection that returns an error after the wipe step
// has already run inside the tx. The test asserts that the stale rows are
// still there after the call — i.e. the destructive DELETEs were rolled back
// when the projection failed. This is the "real transaction, not vibes"
// guard called out in the self-review.
func TestReImporter_AtomicRollback_PreservesStaleContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping marketplace re-import integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedReimportFixture(ctx, t, pool)

	// Sanity: there is exactly one stale chunk before we start.
	var pre int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM chunks WHERE knowledge_base_id = $1`, f.impKBID,
	).Scan(&pre))
	require.Equal(t, 1, pre)

	boom := errors.New("simulated projection failure")
	failing := func(_ context.Context, _ pgx.Tx, _, _, _ string) (int, error) {
		return 0, boom
	}

	r := marketplace.NewReImporter(pool).WithProjection(failing)
	_, err := r.ReImport(ctx, f.impOrgID, f.impKBID, uuid.NewString())
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)

	// The wipe must have rolled back: stale chunk is still present.
	var post int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM chunks WHERE knowledge_base_id = $1`, f.impKBID,
	).Scan(&post))
	assert.Equal(t, 1, post, "stale chunk must survive rolled-back re-import")

	// imported_from_revision_at must NOT have advanced.
	var rev time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT imported_from_revision_at FROM knowledge_bases WHERE id = $1`,
		f.impKBID,
	).Scan(&rev))
	assert.True(t, rev.Before(f.pubLastMod), "revision stamp must not advance on a rolled-back re-import")
}

// TestReImporter_ConcurrentReImports_Serialise documents the FOR UPDATE
// guarantee: two simultaneous re-imports against the same KB run one after
// the other, not concurrently, so the projections never interleave their
// DELETE / INSERT steps. The assertion is that both calls succeed and the
// resulting chunk count is what the LAST projection produced — not a mix.
func TestReImporter_ConcurrentReImports_Serialise(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping marketplace re-import integration test in short mode")
	}
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	f := seedReimportFixture(ctx, t, pool)

	// Use a slow projection so the second caller is guaranteed to land on
	// the FOR UPDATE wait. Both insert distinguishable content.
	makeSlow := func(label string) marketplace.ProjectionFunc {
		return func(ctx context.Context, tx pgx.Tx, destOrgID, destKBID, _ string) (int, error) {
			// Hold the row for a beat so the concurrent call must wait.
			if _, err := tx.Exec(ctx, `SELECT pg_sleep(0.2)`); err != nil {
				return 0, err
			}
			docID := uuid.NewString()
			if _, err := tx.Exec(ctx,
				`INSERT INTO documents (id, org_id, knowledge_base_id, filename, file_size, mime_type)
				 VALUES ($1, $2, $3, $4, 1, 'application/pdf')`,
				docID, destOrgID, destKBID, label+".pdf",
			); err != nil {
				return 0, err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO chunks (id, org_id, knowledge_base_id, document_id, content, chunk_index)
				 VALUES ($1, $2, $3, $4, $5, 0)`,
				uuid.NewString(), destOrgID, destKBID, docID, label,
			); err != nil {
				return 0, err
			}
			return 1, nil
		}
	}

	done := make(chan error, 2)
	go func() {
		r := marketplace.NewReImporter(pool).WithProjection(makeSlow("A"))
		_, err := r.ReImport(ctx, f.impOrgID, f.impKBID, uuid.NewString())
		done <- err
	}()
	go func() {
		r := marketplace.NewReImporter(pool).WithProjection(makeSlow("B"))
		_, err := r.ReImport(ctx, f.impOrgID, f.impKBID, uuid.NewString())
		done <- err
	}()
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("re-import timed out — FOR UPDATE may be deadlocked")
		}
	}

	// Exactly one chunk remains; either A's or B's, never both — the
	// loser's wipe step ran after the winner's insert, then the loser's
	// own insert replaced it.
	var chunks []string
	rows, err := pool.Query(ctx,
		`SELECT content FROM chunks WHERE knowledge_base_id = $1`,
		f.impKBID,
	)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		chunks = append(chunks, c)
	}
	require.Len(t, chunks, 1, "expected exactly one chunk after serialised re-imports")
	assert.Contains(t, []string{"A", "B"}, chunks[0])
}

