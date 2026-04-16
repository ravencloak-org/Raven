// Package repository_test contains integration tests for the repository layer.
package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedRLSFixtures inserts the org-A, org-B, and workspace records required by
// FK constraints on knowledge_bases. Runs as superuser (no RLS applies).
// It also grants raven_app the minimum privileges needed to exercise RLS.
func seedRLSFixtures(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgA, orgB, wsID string) {
	t.Helper()

	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		orgA, "Org A", "org-a-"+orgA[:8],
		orgB, "Org B", "org-b-"+orgB[:8],
	)
	require.NoError(t, err, "seed organizations")

	// workspaces has RLS enabled but raven_test is a superuser → bypasses policy.
	_, err = pool.Exec(ctx,
		`INSERT INTO workspaces (id, org_id, name, slug) VALUES ($1, $2, $3, $4)`,
		wsID, orgA, "WS", "ws-"+wsID[:8],
	)
	require.NoError(t, err, "seed workspace")

	// Grant raven_app the minimum permissions it needs so SET LOCAL ROLE raven_app
	// can read and write knowledge_bases during RLS enforcement.
	for _, stmt := range []string{
		`GRANT USAGE ON SCHEMA public TO raven_app`,
		`GRANT SELECT, INSERT ON knowledge_bases TO raven_app`,
	} {
		_, err = pool.Exec(ctx, stmt)
		require.NoError(t, err, "grant: %s", stmt)
	}

	// Revoke grants on cleanup so parallel tests don't leak privileges.
	t.Cleanup(func() {
		for _, stmt := range []string{
			`REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM raven_app`,
			`REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM raven_app`,
			`REVOKE USAGE ON SCHEMA public FROM raven_app`,
		} {
			_, _ = pool.Exec(ctx, stmt)
		}
	})
}

// TestRLS_CrossOrgKB_ReturnsZeroRows verifies that org-B cannot read
// knowledge bases that belong to org-A, even when using the same workspace ID.
func TestRLS_CrossOrgKB_ReturnsZeroRows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping RLS integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	orgA := uuid.NewString()
	orgB := uuid.NewString()
	wsID := uuid.NewString()
	slug := uuid.NewString()[:8]

	seedRLSFixtures(ctx, t, pool, orgA, orgB, wsID)

	kbRepo := repository.NewKBRepository(pool)

	// Create a KB for org-A as raven_app (RLS enforced) so the insert respects
	// the WITH CHECK policy and the row is visible under org-A's context.
	txA, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txA.Rollback(ctx) }()
	_, err = txA.Exec(ctx, "SET LOCAL ROLE raven_app")
	require.NoError(t, err)
	_, err = txA.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgA)
	require.NoError(t, err)
	kb, err := kbRepo.Create(ctx, txA, orgA, wsID, "Org-A KB", slug, "")
	require.NoError(t, err)
	require.NoError(t, txA.Commit(ctx))
	require.NotEmpty(t, kb.ID)

	// Verify org-A can read its own KB.
	txVerifyA, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txVerifyA.Rollback(ctx) }()
	_, err = txVerifyA.Exec(ctx, "SET LOCAL ROLE raven_app")
	require.NoError(t, err)
	_, err = txVerifyA.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgA)
	require.NoError(t, err)
	kbsA, err := kbRepo.ListByWorkspace(ctx, txVerifyA, orgA, wsID)
	require.NoError(t, err)
	assert.Len(t, kbsA, 1, "org-A must see its own KB")
	_ = txVerifyA.Rollback(ctx)

	// Verify org-B cannot see org-A's KB (RLS isolation).
	// With app.current_org_id = orgB, the USING policy filters rows where org_id ≠ orgB.
	txB, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txB.Rollback(ctx) }()
	_, err = txB.Exec(ctx, "SET LOCAL ROLE raven_app")
	require.NoError(t, err)
	_, err = txB.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgB)
	require.NoError(t, err)
	kbsB, err := kbRepo.ListByWorkspace(ctx, txB, orgA, wsID)
	require.NoError(t, err)
	assert.Empty(t, kbsB, "org-B must not see org-A's knowledge bases (RLS violation)")
	_ = txB.Rollback(ctx)
}

// TestRLS_CrossOrgKB_GetByID_ReturnsError verifies that org-B cannot fetch
// a specific KB that belongs to org-A by its ID.
func TestRLS_CrossOrgKB_GetByID_ReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping RLS integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	orgA := uuid.NewString()
	orgB := uuid.NewString()
	wsID := uuid.NewString()
	slug := uuid.NewString()[:8]

	seedRLSFixtures(ctx, t, pool, orgA, orgB, wsID)

	kbRepo := repository.NewKBRepository(pool)

	// Create a KB for org-A.
	txA, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txA.Rollback(ctx) }()
	_, err = txA.Exec(ctx, "SET LOCAL ROLE raven_app")
	require.NoError(t, err)
	_, err = txA.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgA)
	require.NoError(t, err)
	kb, err := kbRepo.Create(ctx, txA, orgA, wsID, "Secret KB", slug, "")
	require.NoError(t, err)
	require.NoError(t, txA.Commit(ctx))

	// org-B tries to fetch org-A's KB by ID — RLS must prevent access.
	txB, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txB.Rollback(ctx) }()
	_, err = txB.Exec(ctx, "SET LOCAL ROLE raven_app")
	require.NoError(t, err)
	_, err = txB.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgB)
	require.NoError(t, err)
	_, err = kbRepo.GetByID(ctx, txB, orgA, kb.ID)
	assert.Error(t, err, "org-B must not be able to fetch org-A's KB by ID (RLS violation)")
	_ = txB.Rollback(ctx)
}

// TestRLS_MigrationVersion_AllApplied verifies all migrations applied after NewTestDB.
func TestRLS_MigrationVersion_AllApplied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping migration version test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	// All 34 SQL migrations must be applied.
	var maxVersion int64
	err := pool.QueryRow(ctx,
		"SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true",
	).Scan(&maxVersion)
	require.NoError(t, err)
	assert.EqualValues(t, 35, maxVersion, "all 35 migrations must be applied cleanly")

	// Spot-check critical tables exist.
	for _, table := range []string{
		"organizations", "workspaces", "knowledge_bases",
		"documents", "chunks", "chat_sessions", "chat_messages", "api_keys",
	} {
		var count int
		err := pool.QueryRow(ctx,
			"SELECT count(*) FROM information_schema.tables WHERE table_name = $1 AND table_schema = 'public'",
			table,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "table %s must exist after migrations", table)
	}
}
