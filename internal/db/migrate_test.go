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
