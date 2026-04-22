//go:build integration

package repository_test

// Benchmark scaffold for the M9 semantic-cache lookup acceptance criterion:
// “Cache lookup adds ≤ 10ms to p99 latency on a miss.” This file runs only
// under the `integration` build tag because it needs a live pgvector DB via
// testutil.NewTestDB.
//
// Running locally:
//   go test -tags=integration -bench='BenchmarkLookupSimilar' \
//     -benchtime=1000x ./internal/repository/...
//
// The benchmark seeds `seedSize` random 1536-dim embeddings for one KB,
// then measures LookupSimilar on misses (random query vectors that don't
// match anything). Use `go test -benchmem -cpuprofile=...` to investigate
// regressions. A CI gate that fails above 10 ms will be added in #256
// follow-up sub-issue once baseline numbers are collected.

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/testutil"
	"github.com/jackc/pgx/v5"
)

const (
	dims     = 1536
	seedSize = 5000
)

func randomEmbedding() []float32 {
	v := make([]float32, dims)
	for i := range v {
		v[i] = rand.Float32()*2 - 1
	}
	return v
}

func BenchmarkLookupSimilar(b *testing.B) {
	pool := testutil.NewTestDB(&testing.T{})
	orgID := uuid.NewString()
	kbID := uuid.NewString()

	// Minimal RLS-compatible fixtures. The integration cache_test.go already
	// exercises these so we reuse its seeding pattern loosely.
	ctx := context.Background()
	_ = db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error {
		_, _ = tx.Exec(ctx,
			`INSERT INTO organizations (id, name, slug) VALUES ($1, 'b', 'b')
			 ON CONFLICT DO NOTHING`, orgID)
		return nil
	})

	repo := repository.NewSemanticCacheRepository(pool)

	// Seed seedSize cache entries with random embeddings.
	for i := 0; i < seedSize; i++ {
		_, _ = repo.Store(ctx, orgID, kbID, "q", "a", randomEmbedding(), nil)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = repo.LookupSimilar(ctx, orgID, kbID, randomEmbedding(), 0.99)
	}
}
