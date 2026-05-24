package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

func TestVerifyMigrationsState_AllAppliedPasses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

	require.NoError(t, db.VerifyMigrationsState(ctx, dsn))
}

func TestVerifyMigrationsState_MismatchReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

	_, err := pool.Exec(ctx,
		`DELETE FROM goose_db_version WHERE version_id = (
			SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true
		)`,
	)
	require.NoError(t, err)

	err = db.VerifyMigrationsState(ctx, dsn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migration state mismatch")
}

func TestVerifyMigrationsState_RollbackThenReapplyPasses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

	// Simulate a rollback then re-apply of the latest migration by appending
	// two extra rows for its version_id — one with is_applied=false (the
	// rollback event), one with is_applied=true (the re-apply). Goose's table
	// is append-only; VerifyMigrationsState must look at the LATEST row per
	// version_id, not COUNT(*) of is_applied=true rows.
	var latestVersion int64
	err := pool.QueryRow(ctx,
		`SELECT MAX(version_id) FROM goose_db_version WHERE version_id > 0`,
	).Scan(&latestVersion)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO goose_db_version (version_id, is_applied, tstamp)
		 VALUES ($1, false, now()), ($1, true, now())`,
		latestVersion,
	)
	require.NoError(t, err)

	require.NoError(t, db.VerifyMigrationsState(ctx, dsn))
}
