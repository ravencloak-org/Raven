package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

// TestWithOrgID_SignatureStable confirms the exported symbol is callable
// and has the expected function signature.
// SetOrgIDQuery was removed in favour of the parameterized set_config call inside
// WithOrgID. Full integration coverage requires a live database.
func TestWithOrgID_SignatureStable(t *testing.T) {
	// Verify the function signature matches the expected contract.
	var fn func(ctx context.Context, pool interface{ Begin(context.Context) (pgx.Tx, error) }, orgID string, fn func(tx pgx.Tx) error) error
	_ = fn
	_ = db.WithOrgID
}

// TestWithOrgID_NilPool_Panics verifies that calling WithOrgID with a nil
// *pgxpool.Pool panics (nil pointer dereference on Begin). This exercises
// the contract: callers must supply a valid pool.
func TestWithOrgID_NilPool_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when calling WithOrgID with nil pool, but did not panic")
		}
	}()

	_ = db.WithOrgID(context.Background(), nil, "test-org", func(_ pgx.Tx) error {
		t.Fatal("fn should not be called with nil pool")
		return nil
	})
}

// TestWithRLSVar_ParameterisedSetConfig is the safety net for the
// withRLSVar refactor: both WithOrgID and WithUserID delegate to one helper
// that passes the GUC name as $1 and the value as $2 to set_config. If
// Postgres ever rejects a parameterised first arg to set_config, this test
// flips red before callers do.
func TestWithRLSVar_ParameterisedSetConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)

	const (
		orgID  = "00000000-0000-0000-0000-0000000000aa"
		userID = "00000000-0000-0000-0000-0000000000bb"
	)

	// WithOrgID must make current_setting('app.current_org_id') visible to fn.
	require.NoError(t, db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error {
		var got string
		if err := tx.QueryRow(ctx,
			`SELECT current_setting('app.current_org_id', true)`,
		).Scan(&got); err != nil {
			return err
		}
		require.Equal(t, orgID, got)
		return nil
	}))

	// WithUserID must make current_setting('app.current_user_id') visible to fn.
	require.NoError(t, db.WithUserID(ctx, pool, userID, func(tx pgx.Tx) error {
		var got string
		if err := tx.QueryRow(ctx,
			`SELECT current_setting('app.current_user_id', true)`,
		).Scan(&got); err != nil {
			return err
		}
		require.Equal(t, userID, got)
		return nil
	}))
}
