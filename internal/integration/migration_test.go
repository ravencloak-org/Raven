// Package integration_test provides end-to-end integration tests for the Raven
// backend that require real external services (PostgreSQL via testcontainers).
package integration_test

import (
	"context"
	"testing"

	"github.com/ravencloak-org/Raven/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationCorrectness_AllMigrationsApply verifies that all SQL migrations
// apply cleanly and create the expected schema.
func TestMigrationCorrectness_AllMigrationsApply(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	// Verify at least the baseline number of migrations are applied.
	const minExpectedMigrations = 32
	var maxVersion int64
	err := pool.QueryRow(ctx,
		"SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true",
	).Scan(&maxVersion)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, int(maxVersion), minExpectedMigrations,
		"expected at least %d migrations to be applied, got %d", minExpectedMigrations, maxVersion)

	// Spot-check key tables exist.
	for _, table := range []string{
		"organizations",
		"workspaces",
		"knowledge_bases",
		"documents",
		"chunks",
		"embeddings",
		"chat_sessions",
		"chat_messages",
		"api_keys",
	} {
		var count int
		queryErr := pool.QueryRow(ctx,
			"SELECT count(*) FROM information_schema.tables WHERE table_name = $1 AND table_schema = 'public'",
			table,
		).Scan(&count)
		require.NoError(t, queryErr)
		assert.Equal(t, 1, count, "table %s must exist after migrations", table)
	}
}

// TestMigrationCorrectness_RLSEnabled verifies Row-Level Security is active on
// all tables that contain org_id.
func TestMigrationCorrectness_RLSEnabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	rlsTables := []string{
		"knowledge_bases",
		"documents",
		"chunks",
		"chat_sessions",
		"chat_messages",
		"api_keys",
	}

	for _, table := range rlsTables {
		t.Run(table, func(t *testing.T) {
			var rlsEnabled bool
			err := pool.QueryRow(ctx,
				"SELECT relrowsecurity FROM pg_class WHERE relname = $1",
				table,
			).Scan(&rlsEnabled)
			require.NoError(t, err)
			assert.True(t, rlsEnabled,
				"RLS must be enabled on table %s", table)
		})
	}
}

// TestMigrationCorrectness_VectorExtension verifies the pgvector extension
// and HNSW index are present.
func TestMigrationCorrectness_VectorExtension(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	// Check pgvector extension.
	var extExists bool
	err := pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')",
	).Scan(&extExists)
	require.NoError(t, err)
	assert.True(t, extExists, "pgvector extension must be installed")

	// Check HNSW index on embeddings.
	var idxExists bool
	err = pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename = 'embeddings' AND indexname = 'idx_embeddings_hnsw')",
	).Scan(&idxExists)
	require.NoError(t, err)
	assert.True(t, idxExists, "HNSW index must exist on embeddings table")
}
