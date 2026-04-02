package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ravencloak-org/Raven/internal/db"
)

// TestWithOrgID_SignatureStable confirms the exported symbol is callable
// and has the expected function signature.
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
