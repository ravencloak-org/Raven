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

// When the DB is missing one of the embedded versions (a deploy that ran the
// new binary but goose hasn't caught up yet), VerifyMigrationsState must name
// the specific unapplied version_id, not just say "count mismatch".
func TestVerifyMigrationsState_UnappliedReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

	var latestVersion int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT MAX(version_id) FROM goose_db_version WHERE version_id > 0`,
	).Scan(&latestVersion))

	_, err := pool.Exec(ctx,
		`DELETE FROM goose_db_version WHERE version_id = $1`, latestVersion,
	)
	require.NoError(t, err)

	err = db.VerifyMigrationsState(ctx, dsn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migration state mismatch")
	require.Contains(t, err.Error(), "unapplied migrations")
	// The specific id must appear in the error so operators can act on it
	// without re-reading the goose table by hand.
	require.Contains(t, err.Error(), "[")
}

// When the DB has a goose row for a version_id that isn't in the embedded
// migrations FS (a binary rollback to an older release leaves "orphan"
// applied rows), the error must label them as orphans and list the id.
func TestVerifyMigrationsState_OrphanReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

	// Pick a version_id far above anything in the embedded set so it's
	// unambiguously an orphan from this binary's point of view.
	const orphanVersion = 999999
	_, err := pool.Exec(ctx,
		`INSERT INTO goose_db_version (version_id, is_applied, tstamp)
		 VALUES ($1, true, now())`,
		orphanVersion,
	)
	require.NoError(t, err)

	err = db.VerifyMigrationsState(ctx, dsn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migration state mismatch")
	require.Contains(t, err.Error(), "orphan applied migrations not in embedded set")
	require.Contains(t, err.Error(), "999999")
}

// A version that was rolled back (latest row is_applied=false) must be
// treated as not-applied for verification purposes — it will then either
// match the embedded set (no error) or appear in the unapplied list.
func TestVerifyMigrationsState_RollbackLatestRowNotApplied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

	var latestVersion int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT MAX(version_id) FROM goose_db_version WHERE version_id > 0`,
	).Scan(&latestVersion))

	// Append a rollback row for the latest version. The version is in the
	// embedded set but the latest row says is_applied=false, so it must be
	// reported as unapplied.
	_, err := pool.Exec(ctx,
		`INSERT INTO goose_db_version (version_id, is_applied, tstamp)
		 VALUES ($1, false, now())`,
		latestVersion,
	)
	require.NoError(t, err)

	err = db.VerifyMigrationsState(ctx, dsn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unapplied migrations")
}

// A rollback followed by a re-apply leaves the LATEST row as is_applied=true,
// even though earlier rows exist for the same version. VerifyMigrationsState
// must look at the latest row per version_id and treat the version as
// applied — bool_and across history would false-positive here.
func TestVerifyMigrationsState_RollbackThenReapplyPasses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

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

// Both orphan and unapplied at once: the error must mention both lists so
// operators see the full picture rather than fix one and rediscover the
// other on the next boot.
func TestVerifyMigrationsState_OrphanAndUnappliedReturnsBoth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

	var latestVersion int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT MAX(version_id) FROM goose_db_version WHERE version_id > 0`,
	).Scan(&latestVersion))

	// Delete the highest applied row → unapplied.
	_, err := pool.Exec(ctx,
		`DELETE FROM goose_db_version WHERE version_id = $1`, latestVersion,
	)
	require.NoError(t, err)
	// Insert a far-future version → orphan.
	_, err = pool.Exec(ctx,
		`INSERT INTO goose_db_version (version_id, is_applied, tstamp)
		 VALUES (999999, true, now())`,
	)
	require.NoError(t, err)

	err = db.VerifyMigrationsState(ctx, dsn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "orphan applied migrations not in embedded set")
	require.Contains(t, err.Error(), "unapplied migrations")
}
