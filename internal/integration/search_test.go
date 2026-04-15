//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
)

func TestSearch(t *testing.T) {
	ctx := context.Background()

	// ---------------------------------------------------------------
	// BM25 full-text search
	// ---------------------------------------------------------------

	t.Run("BM25/exact_keyword_match", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-bm25-exact")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "exact.md", "ready")

		// 3 chunks contain the keyword "photosynthesis", 2 do not.
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"Photosynthesis is the process by which green plants convert sunlight into energy.", "Biology", 20)
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 1,
			"The rate of photosynthesis depends on light intensity and carbon dioxide concentration.", "Biology", 18)
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 2,
			"Mitosis is a type of cell division that results in two identical daughter cells.", "Biology", 16)
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 3,
			"Advanced photosynthesis research explores artificial leaf technology.", "Research", 12)
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 4,
			"The mitochondria is the powerhouse of the cell.", "Biology", 10)

		resp, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "photosynthesis", 10)
		require.NoError(t, err)
		assert.Equal(t, 3, resp.Total)
		require.Len(t, resp.Results, 3)

		// Results should be ranked by score descending.
		for i := 1; i < len(resp.Results); i++ {
			assert.GreaterOrEqual(t, resp.Results[i-1].Rank, resp.Results[i].Rank,
				"results should be ordered by rank descending")
		}
	})

	t.Run("BM25/phrase_search", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-bm25-phrase")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "phrase.md", "ready")

		// Chunk with the exact phrase should rank higher.
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"Machine learning algorithms are used in natural language processing for sentiment analysis.", "ML", 18)
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 1,
			"Natural language processing enables machines to understand human language.", "NLP", 14)
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 2,
			"The language of mathematics is universal and processing data requires algorithms.", "Math", 15)

		resp, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "natural language processing", 10)
		require.NoError(t, err)
		require.GreaterOrEqual(t, resp.Total, 2, "at least 2 chunks should match")

		// The chunk with the most complete phrase match should appear first.
		assert.Contains(t, resp.Results[0].Content, "language processing",
			"top result should contain the phrase")
	})

	t.Run("BM25/no_results", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-bm25-noresults")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "empty.md", "ready")
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"This chunk talks about gardening techniques.", "Gardening", 10)

		resp, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "xylophone", 10)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Total)
		assert.Empty(t, resp.Results)
	})

	t.Run("BM25/document_filter", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-bm25-docfilter")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		doc1 := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "alpha.md", "ready")
		doc2 := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "beta.md", "ready")

		insertChunk(t, ctx, org.OrgID, org.KBID, doc1, 0,
			"Kubernetes orchestration handles container deployment.", "DevOps", 12)
		insertChunk(t, ctx, org.OrgID, org.KBID, doc2, 0,
			"Kubernetes networking uses services and ingress controllers.", "DevOps", 14)
		insertChunk(t, ctx, org.OrgID, org.KBID, doc2, 1,
			"Kubernetes pod scheduling considers resource limits.", "DevOps", 11)

		// Filter to doc1 only — should return only 1 result.
		resp, err := testSearchSvc.TextSearchWithFilters(ctx, org.OrgID, org.KBID, "kubernetes", []string{doc1}, 10)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)
		require.Len(t, resp.Results, 1)
		assert.Equal(t, &doc1, resp.Results[0].DocumentID)
	})

	t.Run("BM25/limit_clamping", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-bm25-clamp")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "clamp.md", "ready")
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"Terraform infrastructure provisioning automates cloud resources.", "IaC", 12)

		// limit=0 should default to 10 (not fail), and return the 1 matching result.
		resp, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "terraform", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)

		// limit=-1 should also default to 10.
		resp, err = testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "terraform", -1)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)

		// limit=200 should clamp to 100 — still returns the 1 matching result.
		resp, err = testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "terraform", 200)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)
	})

	t.Run("BM25/empty_knowledge_base", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-bm25-empty")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		// No documents or chunks inserted — KB is empty.
		resp, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "anything", 10)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Total)
		assert.Empty(t, resp.Results)
	})

	// ---------------------------------------------------------------
	// Vector similarity search
	// ---------------------------------------------------------------

	t.Run("Vector/nearest_neighbor", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-vec-nn")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "vector.md", "ready")

		// Seed chunks with different embeddings.
		chunk5 := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"Content matching seed five embedding.", "Vector", 8)
		insertEmbedding(t, ctx, org.OrgID, chunk5, generateEmbedding(5))

		chunkTen := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 1,
			"Content matching seed ten embedding.", "Vector", 8)
		insertEmbedding(t, ctx, org.OrgID, chunkTen, generateEmbedding(10))

		chunkFar := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 2,
			"Content matching seed ninety-nine embedding.", "Vector", 8)
		insertEmbedding(t, ctx, org.OrgID, chunkFar, generateEmbedding(99))

		// Query with generateEmbedding(5) — should return chunk5 as top result.
		queryEmb := generateEmbedding(5)
		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			vecResults, err := testSearchRepo.VectorSearch(ctx, tx, org.KBID, queryEmb, 10)
			if err != nil {
				return err
			}
			require.NotEmpty(t, vecResults, "vector search should return results")
			require.Len(t, vecResults, 3, "should return all 3 chunks")
			assert.Equal(t, chunk5, vecResults[0].ChunkID, "nearest neighbor should be the chunk with the same embedding seed")
			assert.Greater(t, vecResults[0].VectorScore, 0.99,
				"cosine similarity of identical embedding should be very high")

			return nil
		})
		require.NoError(t, err)
	})

	t.Run("Vector/dimension_mismatch", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-vec-dim")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "dim.md", "ready")
		chunkID := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"Some content for dimension test.", "Test", 8)
		insertEmbedding(t, ctx, org.OrgID, chunkID, generateEmbedding(1))

		// Pass a 512-dimensional embedding (wrong dimensions, should be 1536).
		wrongEmb := make([]float32, 512)
		for i := range wrongEmb {
			wrongEmb[i] = 0.1
		}

		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			_, err := testSearchRepo.VectorSearch(ctx, tx, org.KBID, wrongEmb, 10)
			return err
		})
		assert.Error(t, err, "dimension mismatch should return an error, not panic")
	})

	// ---------------------------------------------------------------
	// Hybrid search (RRF)
	// ---------------------------------------------------------------

	t.Run("Hybrid/fusion_correctness", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-hybrid-fuse")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "hybrid.md", "ready")

		// Chunk A: contains keyword AND has close embedding to query.
		chunkA := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"Quantum computing leverages superposition for parallel computation.", "Quantum", 12)
		insertEmbedding(t, ctx, org.OrgID, chunkA, generateEmbedding(42))

		// Chunk B: contains keyword but distant embedding.
		chunkB := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 1,
			"Quantum entanglement enables secure communication.", "Quantum", 10)
		insertEmbedding(t, ctx, org.OrgID, chunkB, generateEmbedding(999))

		// Chunk C: no keyword match but close embedding.
		chunkC := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 2,
			"Parallel computation uses multiple processors simultaneously.", "Computing", 10)
		insertEmbedding(t, ctx, org.OrgID, chunkC, generateEmbedding(43))

		// Query: text = "quantum computing" + embedding close to seed 42.
		resp, err := testSearchSvc.HybridSearch(ctx, org.OrgID, org.KBID, "quantum computing", generateEmbedding(42), 10)
		require.NoError(t, err)
		require.NotEmpty(t, resp.Results)

		// Chunk A should rank highest because it appears in BOTH BM25 and vector results.
		assert.Equal(t, chunkA, resp.Results[0].ChunkID,
			"chunk appearing in both BM25 and vector results should rank highest by RRF")
		assert.Greater(t, resp.Results[0].RRFScore, 0.0, "RRF score should be positive")
		assert.Greater(t, resp.Results[0].VectorScore, 0.0, "vector score should be set")
		assert.Greater(t, resp.Results[0].BM25Score, 0.0, "BM25 score should be set")
	})

	t.Run("Hybrid/bm25_only_fallback", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-hybrid-bm25only")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "bm25only.md", "ready")
		chunkID := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"Elasticsearch provides distributed full-text search capabilities.", "Search", 12)
		insertEmbedding(t, ctx, org.OrgID, chunkID, generateEmbedding(1))

		// Empty embedding + valid text = BM25-only search.
		resp, err := testSearchSvc.HybridSearch(ctx, org.OrgID, org.KBID, "elasticsearch", nil, 10)
		require.NoError(t, err)
		require.NotEmpty(t, resp.Results)

		for _, r := range resp.Results {
			assert.Equal(t, float64(0), r.VectorScore, "VectorScore should be 0 when no embedding provided")
			assert.Equal(t, 0, r.VectorRank, "VectorRank should be 0 when no embedding provided")
			assert.Greater(t, r.BM25Score, float64(0), "BM25Score should be positive")
		}
	})

	t.Run("Hybrid/vector_only_fallback", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-hybrid-veconly")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "veconly.md", "ready")
		chunkID := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"Content for vector-only search test.", "Test", 8)
		insertEmbedding(t, ctx, org.OrgID, chunkID, generateEmbedding(7))

		// Empty query + valid embedding = vector-only search.
		resp, err := testSearchSvc.HybridSearch(ctx, org.OrgID, org.KBID, "", generateEmbedding(7), 10)
		require.NoError(t, err)
		require.NotEmpty(t, resp.Results)

		for _, r := range resp.Results {
			assert.Equal(t, float64(0), r.BM25Score, "BM25Score should be 0 when query is empty")
			assert.Equal(t, 0, r.BM25Rank, "BM25Rank should be 0 when query is empty")
			assert.Greater(t, r.VectorScore, float64(0), "VectorScore should be positive")
		}
	})

	t.Run("Hybrid/topK_respected", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-hybrid-topk")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "topk.md", "ready")

		// Seed 6 chunks all matching the keyword "database".
		for i := 0; i < 6; i++ {
			chID := insertChunk(t, ctx, org.OrgID, org.KBID, docID, i,
				"Database optimization techniques improve query performance significantly.", "Database", 12)
			insertEmbedding(t, ctx, org.OrgID, chID, generateEmbedding(i+100))
		}

		resp, err := testSearchSvc.HybridSearch(ctx, org.OrgID, org.KBID, "database", generateEmbedding(100), 3)
		require.NoError(t, err)
		assert.Equal(t, 3, resp.Total, "total should equal topK")
		assert.Len(t, resp.Results, 3, "results should be clamped to topK=3")
	})

	// ---------------------------------------------------------------
	// Edge cases
	// ---------------------------------------------------------------

	t.Run("Edge/unicode_content", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-edge-unicode")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "unicode.md", "ready")

		// CJK characters (Chinese: "knowledge management system documentation")
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"知識管理系統的文檔資料 knowledge documentation for the platform.", "知識管理", 15)
		// Emoji content
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 1,
			"🚀 Deployment guide 📚 documentation with 🔍 search and 💾 storage features.", "🚀 Deployment", 14)
		// RTL (Arabic: "search system documentation")
		insertChunk(t, ctx, org.OrgID, org.KBID, docID, 2,
			"وثائق نظام البحث documentation for retrieval systems.", "وثائق البحث", 12)

		// BM25 search for the English term present in all chunks should not error.
		resp, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "documentation", 10)
		require.NoError(t, err, "BM25 search with unicode content should not error")
		assert.GreaterOrEqual(t, resp.Total, 1, "should find at least one result with the shared English keyword")

		// Search for a CJK term — may return 0 results with English tsvector config, but must not error.
		resp2, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "知識管理", 10)
		require.NoError(t, err, "BM25 search for CJK term should not error")
		_ = resp2 // result count depends on tsvector config
	})

	t.Run("Edge/duplicate_embeddings", func(t *testing.T) {
		org := seedOrg(t, ctx, "search-edge-dupes")
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "dupes.md", "ready")

		// Two chunks with identical embeddings but different content.
		chunk1 := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 0,
			"First chunk with duplicate embedding content.", "Dupe", 10)
		insertEmbedding(t, ctx, org.OrgID, chunk1, generateEmbedding(77))

		chunk2 := insertChunk(t, ctx, org.OrgID, org.KBID, docID, 1,
			"Second chunk with duplicate embedding content.", "Dupe", 10)
		insertEmbedding(t, ctx, org.OrgID, chunk2, generateEmbedding(77))

		// Vector search should return both chunks.
		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			results, err := testSearchRepo.VectorSearch(ctx, tx, org.KBID, generateEmbedding(77), 10)
			if err != nil {
				return err
			}
			assert.Len(t, results, 2, "vector search should return both chunks with identical embeddings")
			return nil
		})
		require.NoError(t, err)
	})
}
