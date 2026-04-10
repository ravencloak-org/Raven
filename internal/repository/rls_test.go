package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	kbRepo := repository.NewKBRepository(pool)

	// Create a KB for org-A using a raw transaction.
	txA, err := pool.Begin(ctx)
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
	_, err = txVerifyA.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgA)
	require.NoError(t, err)
	kbsA, err := kbRepo.ListByWorkspace(ctx, txVerifyA, orgA, wsID)
	require.NoError(t, err)
	assert.Len(t, kbsA, 1, "org-A must see its own KB")
	_ = txVerifyA.Rollback(ctx)

	// Verify org-B cannot see org-A's KB (RLS isolation).
	txB, err := pool.Begin(ctx)
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

	kbRepo := repository.NewKBRepository(pool)

	// Create a KB for org-A.
	txA, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = txA.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgA)
	require.NoError(t, err)
	kb, err := kbRepo.Create(ctx, txA, orgA, wsID, "Secret KB", slug, "")
	require.NoError(t, err)
	require.NoError(t, txA.Commit(ctx))

	// org-B tries to fetch org-A's KB by ID — RLS must prevent access.
	txB, err := pool.Begin(ctx)
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

	// All 32 SQL migrations must be applied.
	var maxVersion int64
	err := pool.QueryRow(ctx,
		"SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true",
	).Scan(&maxVersion)
	require.NoError(t, err)
	assert.EqualValues(t, 32, maxVersion, "all 32 migrations must be applied cleanly")

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
