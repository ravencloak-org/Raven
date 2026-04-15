//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/cache"
	"github.com/ravencloak-org/Raven/internal/db"
)

// ---------- Subsystem A: Valkey SHA256 exact-match (miniredis) ----------

func TestCacheValkey(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	rc := cache.NewResponseCache(client, 5*time.Minute)
	ctx := context.Background()

	t.Run("E2E_miss_store_hit", func(t *testing.T) {
		kbID := "kb-e2e-1"
		query := "what is raven"

		// Miss.
		got, err := rc.Get(ctx, kbID, query)
		require.NoError(t, err)
		assert.Nil(t, got, "expected cache miss on first Get")

		// Store.
		resp := &cache.CachedResponse{
			Text: "Raven is a RAG platform.",
			Sources: []cache.CachedSource{
				{
					DocumentID:   "doc-1",
					DocumentName: "intro.md",
					ChunkText:    "Raven provides retrieval-augmented generation.",
					Score:        0.92,
				},
			},
			Model:    "gpt-4o",
			CachedAt: time.Now().UTC().Truncate(time.Second),
		}
		require.NoError(t, rc.Set(ctx, kbID, query, resp))

		// Hit.
		got, err = rc.Get(ctx, kbID, query)
		require.NoError(t, err)
		require.NotNil(t, got, "expected cache hit after Set")

		assert.Equal(t, resp.Text, got.Text)
		assert.Equal(t, resp.Model, got.Model)
		require.Len(t, got.Sources, 1)
		assert.Equal(t, resp.Sources[0].DocumentID, got.Sources[0].DocumentID)
		assert.Equal(t, resp.Sources[0].DocumentName, got.Sources[0].DocumentName)
		assert.Equal(t, resp.Sources[0].ChunkText, got.Sources[0].ChunkText)
		assert.InDelta(t, float64(resp.Sources[0].Score), float64(got.Sources[0].Score), 0.001)
	})

	t.Run("Normalized_matching", func(t *testing.T) {
		kbID := "kb-norm-1"
		resp := &cache.CachedResponse{
			Text:     "normalized answer",
			CachedAt: time.Now().UTC(),
		}

		// Set with clean query.
		require.NoError(t, rc.Set(ctx, kbID, "what is raven", resp))

		// Get with different casing and whitespace (TrimSpace + ToLower).
		got, err := rc.Get(ctx, kbID, " What Is RAVEN ")
		require.NoError(t, err)
		require.NotNil(t, got, "expected hit: normalization should match")
		assert.Equal(t, "normalized answer", got.Text)
	})

	t.Run("Internal_whitespace_NOT_normalized", func(t *testing.T) {
		kbID := "kb-ws-1"
		resp := &cache.CachedResponse{
			Text:     "single space answer",
			CachedAt: time.Now().UTC(),
		}

		// Set with single space.
		require.NoError(t, rc.Set(ctx, kbID, "hello world", resp))

		// Get with double space -- different SHA256.
		got, err := rc.Get(ctx, kbID, "hello  world")
		require.NoError(t, err)
		assert.Nil(t, got, "expected miss: internal whitespace changes SHA256")
	})

	t.Run("KB_invalidation", func(t *testing.T) {
		kb1 := "kb-inv-1"
		kb2 := "kb-inv-2"
		resp := &cache.CachedResponse{Text: "answer", CachedAt: time.Now().UTC()}

		// 3 entries for KB-1.
		require.NoError(t, rc.Set(ctx, kb1, "q1", resp))
		require.NoError(t, rc.Set(ctx, kb1, "q2", resp))
		require.NoError(t, rc.Set(ctx, kb1, "q3", resp))

		// 2 entries for KB-2.
		require.NoError(t, rc.Set(ctx, kb2, "q1", resp))
		require.NoError(t, rc.Set(ctx, kb2, "q2", resp))

		// Invalidate KB-1.
		require.NoError(t, rc.InvalidateKB(ctx, kb1))

		// KB-1 entries should be gone.
		for _, q := range []string{"q1", "q2", "q3"} {
			got, err := rc.Get(ctx, kb1, q)
			require.NoError(t, err)
			assert.Nil(t, got, "expected miss for kb1/%s after invalidation", q)
		}

		// KB-2 entries should still exist.
		for _, q := range []string{"q1", "q2"} {
			got, err := rc.Get(ctx, kb2, q)
			require.NoError(t, err)
			assert.NotNil(t, got, "expected hit for kb2/%s", q)
		}
	})
}

// ---------- Subsystem B: Postgres response_cache table ----------

func TestCachePostgres(t *testing.T) {
	ctx := context.Background()

	t.Run("Hit_count_increment", func(t *testing.T) {
		org := seedOrg(t, ctx, "cache-hitcount")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		emb := generateEmbedding(1)
		entryID := insertCacheEntry(t, ctx, org.OrgID, org.KBID, "hit count query", emb, 0)

		// Increment hit_count 5 times.
		for i := 0; i < 5; i++ {
			err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx,
					`UPDATE response_cache SET hit_count = hit_count + 1 WHERE id = $1`,
					entryID)
				return err
			})
			require.NoError(t, err)
		}

		// Assert hit_count = 5.
		var hitCount int
		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT hit_count FROM response_cache WHERE id = $1`, entryID,
			).Scan(&hitCount)
		})
		require.NoError(t, err)
		assert.Equal(t, 5, hitCount)
	})

	t.Run("TTL_expiration", func(t *testing.T) {
		org := seedOrg(t, ctx, "cache-ttl")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		emb := generateEmbedding(2)

		// Insert an expired entry.
		insertExpiredCacheEntry(t, ctx, org.OrgID, org.KBID, "expired query", emb)

		// Query for non-expired entries -- should find 0.
		var expiredCount int
		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM response_cache WHERE kb_id = $1 AND expires_at > NOW()`,
				org.KBID,
			).Scan(&expiredCount)
		})
		require.NoError(t, err)
		assert.Equal(t, 0, expiredCount, "expired entry should not appear in active query")

		// Insert a valid (non-expired) entry.
		insertCacheEntry(t, ctx, org.OrgID, org.KBID, "valid query", generateEmbedding(3), 0)

		// Query again -- should find 1.
		var validCount int
		err = db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM response_cache WHERE kb_id = $1 AND expires_at > NOW()`,
				org.KBID,
			).Scan(&validCount)
		})
		require.NoError(t, err)
		assert.Equal(t, 1, validCount, "valid entry should appear in active query")
	})

	t.Run("KB_invalidation", func(t *testing.T) {
		org := seedOrg(t, ctx, "cache-kbinv")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		kb2ID := seedKB(t, ctx, org.OrgID, org.WorkspaceID, "cache-kbinv-kb2")

		// Insert 3 entries for KB-1 (org.KBID).
		for i := 0; i < 3; i++ {
			insertCacheEntry(t, ctx, org.OrgID, org.KBID,
				fmt.Sprintf("kb1 query %d", i), generateEmbedding(100+i), 0)
		}
		// Insert 2 entries for KB-2.
		for i := 0; i < 2; i++ {
			insertCacheEntry(t, ctx, org.OrgID, kb2ID,
				fmt.Sprintf("kb2 query %d", i), generateEmbedding(200+i), 0)
		}

		// Invalidate KB-1.
		deleted, err := testCacheRepo.InvalidateKB(ctx, org.OrgID, org.KBID)
		require.NoError(t, err)
		assert.Equal(t, int64(3), deleted, "should delete 3 KB-1 entries")

		// KB-2 should be untouched.
		var kb2Count int
		err = db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM response_cache WHERE kb_id = $1`, kb2ID,
			).Scan(&kb2Count)
		})
		require.NoError(t, err)
		assert.Equal(t, 2, kb2Count, "KB-2 entries should be untouched")
	})

	t.Run("Stats", func(t *testing.T) {
		org := seedOrg(t, ctx, "cache-stats")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		// Insert entries with hit_counts [2, 4, 6].
		insertCacheEntry(t, ctx, org.OrgID, org.KBID, "stats q1", generateEmbedding(10), 2)
		insertCacheEntry(t, ctx, org.OrgID, org.KBID, "stats q2", generateEmbedding(11), 4)
		insertCacheEntry(t, ctx, org.OrgID, org.KBID, "stats q3", generateEmbedding(12), 6)

		count, avgHits, err := testCacheRepo.Stats(ctx, org.OrgID, org.KBID)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
		assert.InDelta(t, 4.0, avgHits, 0.001)
	})

	t.Run("HNSW_index", func(t *testing.T) {
		org := seedOrg(t, ctx, "cache-hnsw")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		const numEntries = 1000

		// Insert 1000+ cache entries with distinct embeddings.
		for i := 0; i < numEntries; i++ {
			insertCacheEntry(t, ctx, org.OrgID, org.KBID,
				fmt.Sprintf("hnsw query %d", i), generateEmbedding(i), 0)
		}

		// Query for the nearest neighbor of embedding(42).
		queryVec := generateEmbedding(42)
		var closestQuery string
		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			// Disable sequential scan to force index usage.
			if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
				return err
			}
			return tx.QueryRow(ctx,
				`SELECT query_text FROM response_cache
				 WHERE kb_id = $1
				 ORDER BY query_embedding <=> $2
				 LIMIT 1`,
				org.KBID, pgvector.NewVector(queryVec),
			).Scan(&closestQuery)
		})
		require.NoError(t, err)
		assert.Equal(t, "hnsw query 42", closestQuery,
			"nearest neighbor should be the entry with the same seed")

		// Verify HNSW index is used via EXPLAIN ANALYZE.
		var explainOutput string
		err = db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
				return err
			}
			rows, err := tx.Query(ctx,
				`EXPLAIN ANALYZE SELECT query_text FROM response_cache
				 WHERE kb_id = $1
				 ORDER BY query_embedding <=> $2
				 LIMIT 1`,
				org.KBID, pgvector.NewVector(queryVec),
			)
			if err != nil {
				return err
			}
			defer rows.Close()
			var sb strings.Builder
			for rows.Next() {
				var line string
				if err := rows.Scan(&line); err != nil {
					return err
				}
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			explainOutput = sb.String()
			return rows.Err()
		})
		require.NoError(t, err)
		assert.Contains(t, strings.ToLower(explainOutput), "index scan",
			"EXPLAIN ANALYZE should show index scan (HNSW), got:\n%s", explainOutput)
	})

	t.Run("RLS_on_cache", func(t *testing.T) {
		orgA := seedOrg(t, ctx, "cache-rls-a")
		t.Cleanup(func() { cleanupOrg(t, ctx, orgA.OrgID) })

		orgB := seedOrg(t, ctx, "cache-rls-b")
		t.Cleanup(func() { cleanupOrg(t, ctx, orgB.OrgID) })

		// Insert a cache entry for Org-A.
		insertCacheEntry(t, ctx, orgA.OrgID, orgA.KBID, "rls query", generateEmbedding(99), 0)

		// Query as Org-B -- should see 0 rows.
		var countB int
		err := db.WithOrgID(ctx, testPool, orgB.OrgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM response_cache`,
			).Scan(&countB)
		})
		require.NoError(t, err)
		assert.Equal(t, 0, countB, "Org-B should not see Org-A's cache entries")

		// Query as Org-A -- should see 1 row.
		var countA int
		err = db.WithOrgID(ctx, testPool, orgA.OrgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM response_cache`,
			).Scan(&countA)
		})
		require.NoError(t, err)
		assert.Equal(t, 1, countA, "Org-A should see its own cache entry")
	})
}
