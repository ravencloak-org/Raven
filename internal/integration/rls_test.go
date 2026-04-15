//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/model"
)

// TestRLS validates that PostgreSQL Row-Level Security (RLS) policies enforce
// strict tenant isolation: no org can see, search, or mutate another org's data.
func TestRLS(t *testing.T) {
	ctx := context.Background()

	// ── Setup: two fully-isolated orgs ──────────────────────────────────
	orgA := seedOrg(t, ctx, "rls-org-a")
	orgB := seedOrg(t, ctx, "rls-org-b")
	t.Cleanup(func() {
		cleanupOrg(t, ctx, orgA.OrgID)
		cleanupOrg(t, ctx, orgB.OrgID)
	})

	// Seed documents (2 per org, "ready" status)
	docA1 := insertDocument(t, ctx, orgA.OrgID, orgA.KBID, orgA.UserID, "rls-a1.md", "ready")
	docA2 := insertDocument(t, ctx, orgA.OrgID, orgA.KBID, orgA.UserID, "rls-a2.md", "ready")
	docB1 := insertDocument(t, ctx, orgB.OrgID, orgB.KBID, orgB.UserID, "rls-b1.md", "ready")
	docB2 := insertDocument(t, ctx, orgB.OrgID, orgB.KBID, orgB.UserID, "rls-b2.md", "ready")

	// Seed chunks (2 per doc = 4 per org), all sharing "isolation-test-keyword"
	chunkA1 := insertChunk(t, ctx, orgA.OrgID, orgA.KBID, docA1, 0,
		"isolation-test-keyword alpha content for org A doc 1", "Heading A1", 50)
	chunkA2 := insertChunk(t, ctx, orgA.OrgID, orgA.KBID, docA1, 1,
		"isolation-test-keyword bravo content for org A doc 1", "Heading A1b", 50)
	chunkA3 := insertChunk(t, ctx, orgA.OrgID, orgA.KBID, docA2, 0,
		"isolation-test-keyword charlie content for org A doc 2", "Heading A2", 50)
	chunkA4 := insertChunk(t, ctx, orgA.OrgID, orgA.KBID, docA2, 1,
		"isolation-test-keyword delta content for org A doc 2", "Heading A2b", 50)

	chunkB1 := insertChunk(t, ctx, orgB.OrgID, orgB.KBID, docB1, 0,
		"isolation-test-keyword echo content for org B doc 1", "Heading B1", 50)
	chunkB2 := insertChunk(t, ctx, orgB.OrgID, orgB.KBID, docB1, 1,
		"isolation-test-keyword foxtrot content for org B doc 1", "Heading B1b", 50)
	chunkB3 := insertChunk(t, ctx, orgB.OrgID, orgB.KBID, docB2, 0,
		"isolation-test-keyword golf content for org B doc 2", "Heading B2", 50)
	chunkB4 := insertChunk(t, ctx, orgB.OrgID, orgB.KBID, docB2, 1,
		"isolation-test-keyword hotel content for org B doc 2", "Heading B2b", 50)

	// Seed embeddings (1 per chunk = 4 per org)
	// Use distinct seeds so Org-A and Org-B have different embedding vectors.
	embA1 := generateEmbedding(100)
	embA2 := generateEmbedding(101)
	embA3 := generateEmbedding(102)
	embA4 := generateEmbedding(103)
	embB1 := generateEmbedding(200)
	embB2 := generateEmbedding(201)
	embB3 := generateEmbedding(202)
	embB4 := generateEmbedding(203)

	insertEmbedding(t, ctx, orgA.OrgID, chunkA1, embA1)
	insertEmbedding(t, ctx, orgA.OrgID, chunkA2, embA2)
	insertEmbedding(t, ctx, orgA.OrgID, chunkA3, embA3)
	insertEmbedding(t, ctx, orgA.OrgID, chunkA4, embA4)
	insertEmbedding(t, ctx, orgB.OrgID, chunkB1, embB1)
	insertEmbedding(t, ctx, orgB.OrgID, chunkB2, embB2)
	insertEmbedding(t, ctx, orgB.OrgID, chunkB3, embB3)
	insertEmbedding(t, ctx, orgB.OrgID, chunkB4, embB4)

	// Seed cache entries (2 per org)
	cacheEmbA1 := generateEmbedding(300)
	cacheEmbA2 := generateEmbedding(301)
	cacheEmbB1 := generateEmbedding(400)
	cacheEmbB2 := generateEmbedding(401)
	insertCacheEntry(t, ctx, orgA.OrgID, orgA.KBID, "cache query alpha", cacheEmbA1, 5)
	insertCacheEntry(t, ctx, orgA.OrgID, orgA.KBID, "cache query bravo", cacheEmbA2, 3)
	insertCacheEntry(t, ctx, orgB.OrgID, orgB.KBID, "cache query charlie", cacheEmbB1, 7)
	insertCacheEntry(t, ctx, orgB.OrgID, orgB.KBID, "cache query delta", cacheEmbB2, 2)

	// Seed sources (1 per org)
	srcA := createSource(t, ctx, orgA.OrgID, orgA.KBID, orgA.UserID, "https://org-a.example.com")
	srcB := createSource(t, ctx, orgB.OrgID, orgB.KBID, orgB.UserID, "https://org-b.example.com")

	// All IDs above are consumed in insertion calls. Suppress any linter
	// warnings for return values only used to verify insertion succeeded.
	_ = srcA
	_ = srcB

	// ── Test cases ──────────────────────────────────────────────────────

	t.Run("document_isolation", func(t *testing.T) {
		// Org-A should see only its own documents.
		respA, err := testDocSvc.List(ctx, orgA.OrgID, orgA.KBID, 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 2, respA.Total, "Org-A should see exactly 2 documents")
		for _, doc := range respA.Documents {
			assert.Equal(t, orgA.OrgID, doc.OrgID, "Org-A document must belong to Org-A")
		}

		// Org-B should see only its own documents.
		respB, err := testDocSvc.List(ctx, orgB.OrgID, orgB.KBID, 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 2, respB.Total, "Org-B should see exactly 2 documents")
		for _, doc := range respB.Documents {
			assert.Equal(t, orgB.OrgID, doc.OrgID, "Org-B document must belong to Org-B")
		}

		// Org-A querying Org-B's KB should see nothing.
		respCross, err := testDocSvc.List(ctx, orgA.OrgID, orgB.KBID, 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 0, respCross.Total, "Org-A must not see any of Org-B's documents")
	})

	t.Run("chunk_isolation_bm25", func(t *testing.T) {
		// TextSearch as Org-A for the shared keyword → only Org-A chunks.
		respA, err := testSearchSvc.TextSearch(ctx, orgA.OrgID, orgA.KBID, "isolation-test-keyword", 50)
		require.NoError(t, err)
		assert.Equal(t, 4, respA.Total, "Org-A should find exactly 4 chunks")
		for _, chunk := range respA.Results {
			assert.Equal(t, orgA.OrgID, chunk.OrgID, "Org-A TextSearch result must belong to Org-A")
			assert.Equal(t, orgA.KBID, chunk.KnowledgeBaseID, "Org-A TextSearch result must be in Org-A's KB")
		}

		// TextSearch as Org-B for the same keyword → only Org-B chunks.
		respB, err := testSearchSvc.TextSearch(ctx, orgB.OrgID, orgB.KBID, "isolation-test-keyword", 50)
		require.NoError(t, err)
		assert.Equal(t, 4, respB.Total, "Org-B should find exactly 4 chunks")
		for _, chunk := range respB.Results {
			assert.Equal(t, orgB.OrgID, chunk.OrgID, "Org-B TextSearch result must belong to Org-B")
			assert.Equal(t, orgB.KBID, chunk.KnowledgeBaseID, "Org-B TextSearch result must be in Org-B's KB")
		}
	})

	t.Run("embedding_isolation_vector", func(t *testing.T) {
		// Use Org-B's first embedding as the query vector while searching as Org-A.
		// Org-A's HybridSearch should never return Org-B's chunks even though
		// Org-B's embedding is the closest match by cosine distance.
		respA, err := testSearchSvc.HybridSearch(ctx, orgA.OrgID, orgA.KBID, "", embB1, 10)
		require.NoError(t, err)
		for _, r := range respA.Results {
			assert.Equal(t, orgA.OrgID, r.OrgID,
				"Org-A vector search must never return Org-B's chunks (got chunk %s)", r.ChunkID)
		}

		// Conversely, search as Org-B with Org-A's embedding.
		respB, err := testSearchSvc.HybridSearch(ctx, orgB.OrgID, orgB.KBID, "", embA1, 10)
		require.NoError(t, err)
		for _, r := range respB.Results {
			assert.Equal(t, orgB.OrgID, r.OrgID,
				"Org-B vector search must never return Org-A's chunks (got chunk %s)", r.ChunkID)
		}
	})

	t.Run("cache_isolation", func(t *testing.T) {
		// Org-B queries its own cache → sees 2 entries.
		countB, _, err := testCacheRepo.Stats(ctx, orgB.OrgID, orgB.KBID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), countB, "Org-B should have exactly 2 cache entries")

		// Org-A queries its own cache → sees 2 entries.
		countA, _, err := testCacheRepo.Stats(ctx, orgA.OrgID, orgA.KBID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), countA, "Org-A should have exactly 2 cache entries")

		// Org-A queries Org-B's KB → 0 entries (RLS blocks cross-org).
		countCross, _, err := testCacheRepo.Stats(ctx, orgA.OrgID, orgB.KBID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), countCross, "Org-A must not see Org-B's cache entries")
	})

	t.Run("cache_invalidation_scoping", func(t *testing.T) {
		// Org-A invalidates its own KB cache.
		deleted, err := testCacheRepo.InvalidateKB(ctx, orgA.OrgID, orgA.KBID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), deleted, "Org-A should have invalidated 2 cache entries")

		// Org-A's cache should now be empty.
		countA, _, err := testCacheRepo.Stats(ctx, orgA.OrgID, orgA.KBID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), countA, "Org-A cache should be empty after invalidation")

		// Org-B's cache must be untouched.
		countB, _, err := testCacheRepo.Stats(ctx, orgB.OrgID, orgB.KBID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), countB, "Org-B cache must be untouched after Org-A's invalidation")
	})

	t.Run("source_isolation", func(t *testing.T) {
		// List sources as Org-A → only Org-A's source.
		respA, err := testSourceSvc.List(ctx, orgA.OrgID, orgA.KBID, 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 1, respA.Total, "Org-A should see exactly 1 source")
		for _, s := range respA.Data {
			assert.Equal(t, orgA.OrgID, s.OrgID, "Org-A source must belong to Org-A")
		}

		// List sources as Org-B → only Org-B's source.
		respB, err := testSourceSvc.List(ctx, orgB.OrgID, orgB.KBID, 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 1, respB.Total, "Org-B should see exactly 1 source")
		for _, s := range respB.Data {
			assert.Equal(t, orgB.OrgID, s.OrgID, "Org-B source must belong to Org-B")
		}

		// Org-A listing Org-B's KB sources → empty.
		respCross, err := testSourceSvc.List(ctx, orgA.OrgID, orgB.KBID, 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 0, respCross.Total, "Org-A must not see Org-B's sources")
	})

	t.Run("cross_org_kb_access", func(t *testing.T) {
		// Org-A searches Org-B's KB ID → empty results because RLS blocks it.
		resp, err := testSearchSvc.TextSearch(ctx, orgA.OrgID, orgB.KBID, "isolation-test-keyword", 50)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Total, "Org-A must get zero results when searching Org-B's KB")
		assert.Empty(t, resp.Results, "Org-A must get empty results for Org-B's KB")

		// Org-B searches Org-A's KB ID → empty.
		resp2, err := testSearchSvc.TextSearch(ctx, orgB.OrgID, orgA.KBID, "isolation-test-keyword", 50)
		require.NoError(t, err)
		assert.Equal(t, 0, resp2.Total, "Org-B must get zero results when searching Org-A's KB")
		assert.Empty(t, resp2.Results, "Org-B must get empty results for Org-A's KB")
	})

	t.Run("admin_bypass", func(t *testing.T) {
		// Connect as raven_admin and verify both orgs' documents are visible.
		conn, err := testPool.Acquire(ctx)
		require.NoError(t, err)
		defer conn.Release()

		_, err = conn.Exec(ctx, "SET ROLE raven_admin")
		require.NoError(t, err)
		defer func() { _, _ = conn.Exec(ctx, "RESET ROLE") }()

		// Count documents for both orgs (admin sees all).
		var totalDocs int
		err = conn.QueryRow(ctx,
			`SELECT COUNT(*) FROM documents WHERE org_id = ANY($1)`,
			[]string{orgA.OrgID, orgB.OrgID},
		).Scan(&totalDocs)
		require.NoError(t, err)
		assert.Equal(t, 4, totalDocs, "Admin must see all 4 documents across both orgs")

		// Count chunks for both orgs.
		var totalChunks int
		err = conn.QueryRow(ctx,
			`SELECT COUNT(*) FROM chunks WHERE org_id = ANY($1)`,
			[]string{orgA.OrgID, orgB.OrgID},
		).Scan(&totalChunks)
		require.NoError(t, err)
		assert.Equal(t, 8, totalChunks, "Admin must see all 8 chunks across both orgs")

		// Count embeddings for both orgs.
		var totalEmbeddings int
		err = conn.QueryRow(ctx,
			`SELECT COUNT(*) FROM embeddings WHERE org_id = ANY($1)`,
			[]string{orgA.OrgID, orgB.OrgID},
		).Scan(&totalEmbeddings)
		require.NoError(t, err)
		assert.Equal(t, 8, totalEmbeddings, "Admin must see all 8 embeddings across both orgs")

		// Count sources for both orgs.
		var totalSources int
		err = conn.QueryRow(ctx,
			`SELECT COUNT(*) FROM sources WHERE org_id = ANY($1)`,
			[]string{orgA.OrgID, orgB.OrgID},
		).Scan(&totalSources)
		require.NoError(t, err)
		assert.Equal(t, 2, totalSources, "Admin must see all 2 sources across both orgs")
	})
}

// createSource inserts a source via the SourceService, which applies RLS via WithOrgID.
func createSource(t *testing.T, ctx context.Context, orgID, kbID, userID, sourceURL string) string {
	t.Helper()
	depth := 1
	src, err := testSourceSvc.Create(ctx, orgID, kbID, model.CreateSourceRequest{
		SourceType: model.SourceTypeSitemap,
		URL:        sourceURL,
		CrawlDepth: &depth,
	}, userID)
	require.NoError(t, err, "failed to create source for org %s", orgID)
	return src.ID
}
