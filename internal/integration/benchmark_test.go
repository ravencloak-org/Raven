//go:build integration

package integration

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
)

// loremFragments provides varied text for realistic chunk content.
var loremFragments = []string{
	"The quick brown fox jumps over the lazy dog near the riverbank",
	"Machine learning algorithms process natural language understanding tasks",
	"Distributed systems require careful consideration of network partitions",
	"Kubernetes orchestrates containerized workloads across multiple nodes",
	"PostgreSQL supports advanced indexing strategies for full-text search",
	"Vector embeddings capture semantic meaning in high-dimensional space",
	"Retrieval augmented generation combines search with language models",
	"Graph databases excel at traversing complex relationship networks",
	"Event-driven architecture decouples producers from consumers effectively",
	"Observability platforms aggregate metrics traces and structured logs",
	"Microservices communicate through well-defined API contracts and protocols",
	"Caching layers reduce database load by storing frequently accessed data",
	"Consensus algorithms ensure data consistency across distributed replicas",
	"Security best practices include encryption at rest and in transit",
	"Load balancers distribute incoming traffic across healthy backend servers",
	"Continuous integration pipelines validate code changes automatically before merge",
	"Data pipelines transform raw events into structured analytical datasets",
	"Message queues buffer workloads during peak traffic processing periods",
	"Service mesh provides observability and traffic management for microservices",
	"Infrastructure as code enables reproducible environment provisioning quickly",
}

// seedBenchmarkData inserts numDocs documents each with chunksPerDoc chunks and embeddings.
// Returns the total chunk count.
func seedBenchmarkData(t *testing.T, ctx context.Context, orgID, kbID, userID string, numDocs, chunksPerDoc int) int {
	t.Helper()

	const tokenCount = 128
	globalIndex := 0

	for d := range numDocs {
		docID := insertDocument(t, ctx, orgID, kbID, userID,
			fmt.Sprintf("bench-doc-%d.md", d), "ready")

		for c := range chunksPerDoc {
			content := fmt.Sprintf("%s. Document %d chunk %d with additional context about %s.",
				loremFragments[globalIndex%len(loremFragments)], d, c,
				loremFragments[(globalIndex+7)%len(loremFragments)])

			chunkID := insertChunk(t, ctx, orgID, kbID, docID, c, content,
				fmt.Sprintf("Section %d.%d", d, c), tokenCount)

			insertEmbedding(t, ctx, orgID, chunkID, generateEmbedding(globalIndex))
			globalIndex++
		}
	}

	return numDocs * chunksPerDoc
}

// seedBenchmarkDataRaw inserts data using a raw admin connection, suitable for
// calling from both *testing.T and *testing.B contexts via seedDataViaAdmin.
func seedBenchmarkDataRaw(ctx context.Context, conn *pgxpool.Conn, orgID, kbID, userID string, numDocs, chunksPerDoc int) error {
	const tokenCount = 128
	globalIndex := 0

	for d := range numDocs {
		docID := uuid.NewString()
		_, err := conn.Exec(ctx, `
			INSERT INTO documents (id, org_id, knowledge_base_id, file_name, file_type, file_size_bytes, file_hash, storage_path, processing_status, uploaded_by)
			VALUES ($1, $2, $3, $4, 'text/markdown', 2048, $5, '/test/path', 'ready'::processing_status, $6::uuid)`,
			docID, orgID, kbID, fmt.Sprintf("bench-doc-%d.md", d), uuid.NewString(), userID)
		if err != nil {
			return fmt.Errorf("insert document %d: %w", d, err)
		}

		for c := range chunksPerDoc {
			content := fmt.Sprintf("%s. Document %d chunk %d with additional context about %s.",
				loremFragments[globalIndex%len(loremFragments)], d, c,
				loremFragments[(globalIndex+7)%len(loremFragments)])

			chunkID := uuid.NewString()
			_, err := conn.Exec(ctx, `
				INSERT INTO chunks (id, org_id, knowledge_base_id, document_id, content, chunk_index, token_count, page_number, heading, chunk_type)
				VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, 'text')`,
				chunkID, orgID, kbID, docID, content, c, tokenCount, fmt.Sprintf("Section %d.%d", d, c))
			if err != nil {
				return fmt.Errorf("insert chunk %d.%d: %w", d, c, err)
			}

			_, err = conn.Exec(ctx, `
				INSERT INTO embeddings (id, org_id, chunk_id, embedding, model_name, dimensions)
				VALUES ($1, $2, $3, $4::vector, 'text-embedding-3-small', 1536)`,
				uuid.NewString(), orgID, chunkID, vectorToString(generateEmbedding(globalIndex)))
			if err != nil {
				return fmt.Errorf("insert embedding %d.%d: %w", d, c, err)
			}

			globalIndex++
		}
	}

	return nil
}

// seedOrgRaw creates a full org hierarchy using raven_admin role on a raw connection.
// Returns testOrg or error.
func seedOrgRaw(ctx context.Context, conn *pgxpool.Conn, name string) (testOrg, error) {
	org := testOrg{
		OrgID:       uuid.NewString(),
		WorkspaceID: uuid.NewString(),
		KBID:        uuid.NewString(),
		UserID:      uuid.NewString(),
	}

	if _, err := conn.Exec(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)`,
		org.OrgID, name, name); err != nil {
		return org, fmt.Errorf("insert org: %w", err)
	}

	if _, err := conn.Exec(ctx, `INSERT INTO users (id, org_id, email, display_name) VALUES ($1, $2, $3, $4)`,
		org.UserID, org.OrgID, name+"@test.local", name+"-user"); err != nil {
		return org, fmt.Errorf("insert user: %w", err)
	}

	if _, err := conn.Exec(ctx, `INSERT INTO workspaces (id, org_id, name, slug) VALUES ($1, $2, $3, $4)`,
		org.WorkspaceID, org.OrgID, name+"-ws", name+"-ws"); err != nil {
		return org, fmt.Errorf("insert workspace: %w", err)
	}

	if _, err := conn.Exec(ctx, `INSERT INTO knowledge_bases (id, org_id, workspace_id, name, slug) VALUES ($1, $2, $3, $4, $5)`,
		org.KBID, org.OrgID, org.WorkspaceID, name+"-kb", name+"-kb"); err != nil {
		return org, fmt.Errorf("insert kb: %w", err)
	}

	return org, nil
}

// collectLatencies runs fn n times and returns the latency of each invocation.
func collectLatencies(n int, fn func()) []time.Duration {
	latencies := make([]time.Duration, n)
	for i := range n {
		start := time.Now()
		fn()
		latencies[i] = time.Since(start)
	}
	return latencies
}

// p95 returns the 95th percentile latency from the given slice.
func p95(latencies []time.Duration) time.Duration {
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	idx := int(float64(len(latencies)) * 0.95)
	if idx >= len(latencies) {
		idx = len(latencies) - 1
	}
	return latencies[idx]
}

// ---------------------------------------------------------------------------
// Shared 1K and 10K datasets seeded once via sync.Once
// ---------------------------------------------------------------------------

var (
	bench1KOnce sync.Once
	bench1KOrg  testOrg
	bench1KErr  error

	bench10KOnce sync.Once
	bench10KOrg  testOrg
	bench10KErr  error
)

// ensureBench1K lazily seeds 1,000 chunks (50 docs x 20 chunks) once.
// Safe to call from both *testing.T and *testing.B.
func ensureBench1K(tb testing.TB) testOrg {
	tb.Helper()
	bench1KOnce.Do(func() {
		ctx := context.Background()
		conn, err := testPool.Acquire(ctx)
		if err != nil {
			bench1KErr = fmt.Errorf("acquire conn: %w", err)
			return
		}
		defer conn.Release()

		if _, err := conn.Exec(ctx, "SET ROLE raven_admin"); err != nil {
			bench1KErr = fmt.Errorf("set role: %w", err)
			return
		}
		defer func() { _, _ = conn.Exec(ctx, "RESET ROLE") }()

		bench1KOrg, bench1KErr = seedOrgRaw(ctx, conn, "bench-1k")
		if bench1KErr != nil {
			return
		}

		bench1KErr = seedBenchmarkDataRaw(ctx, conn, bench1KOrg.OrgID, bench1KOrg.KBID, bench1KOrg.UserID, 50, 20)
	})
	require.NoError(tb, bench1KErr, "failed to seed 1K benchmark data")
	return bench1KOrg
}

// ensureBench10K lazily seeds 10,000 chunks (500 docs x 20 chunks) once.
// Safe to call from both *testing.T and *testing.B.
func ensureBench10K(tb testing.TB) testOrg {
	tb.Helper()
	bench10KOnce.Do(func() {
		ctx := context.Background()
		conn, err := testPool.Acquire(ctx)
		if err != nil {
			bench10KErr = fmt.Errorf("acquire conn: %w", err)
			return
		}
		defer conn.Release()

		if _, err := conn.Exec(ctx, "SET ROLE raven_admin"); err != nil {
			bench10KErr = fmt.Errorf("set role: %w", err)
			return
		}
		defer func() { _, _ = conn.Exec(ctx, "RESET ROLE") }()

		bench10KOrg, bench10KErr = seedOrgRaw(ctx, conn, "bench-10k")
		if bench10KErr != nil {
			return
		}

		bench10KErr = seedBenchmarkDataRaw(ctx, conn, bench10KOrg.OrgID, bench10KOrg.KBID, bench10KOrg.UserID, 500, 20)
	})
	require.NoError(tb, bench10KErr, "failed to seed 10K benchmark data")
	return bench10KOrg
}

// ---------------------------------------------------------------------------
// Benchmarks (testing.B)
// ---------------------------------------------------------------------------

// BenchmarkBM25Search1K benchmarks TextSearch on 1,000 chunks (50 docs x 20 chunks).
func BenchmarkBM25Search1K(b *testing.B) {
	ctx := context.Background()
	org := ensureBench1K(b)

	b.ResetTimer()
	for b.Loop() {
		resp, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "machine learning algorithms", 10)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp
	}
}

// BenchmarkHybridSearch1K benchmarks HybridSearch on 1,000 chunks (50 docs x 20 chunks).
func BenchmarkHybridSearch1K(b *testing.B) {
	ctx := context.Background()
	org := ensureBench1K(b)
	queryEmbedding := generateEmbedding(0)

	b.ResetTimer()
	for b.Loop() {
		resp, err := testSearchSvc.HybridSearch(ctx, org.OrgID, org.KBID,
			"distributed systems network", queryEmbedding, 10)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp
	}
}

// BenchmarkBM25Search10K benchmarks TextSearch on 10,000 chunks (500 docs x 20 chunks).
// This is a baseline benchmark; no latency threshold is enforced.
func BenchmarkBM25Search10K(b *testing.B) {
	ctx := context.Background()
	org := ensureBench10K(b)

	b.ResetTimer()
	for b.Loop() {
		resp, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "kubernetes orchestration workloads", 10)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp
	}
}

// ---------------------------------------------------------------------------
// Threshold assertion tests (testing.T)
// ---------------------------------------------------------------------------

// TestBenchmarkBM25Threshold runs 50 iterations of TextSearch on 1K chunks
// and asserts that p95 latency is under 100ms.
func TestBenchmarkBM25Threshold(t *testing.T) {
	ctx := context.Background()
	org := ensureBench1K(t)

	latencies := collectLatencies(50, func() {
		_, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "vector embeddings semantic search", 10)
		require.NoError(t, err)
	})

	p95Lat := p95(latencies)
	t.Logf("BM25 1K p95 latency: %v", p95Lat)
	assert.Less(t, p95Lat, 100*time.Millisecond,
		"BM25 search p95 latency on 1K chunks must be under 100ms, got %v", p95Lat)
}

// TestBenchmarkHybridThreshold runs 50 iterations of HybridSearch on 1K chunks
// and asserts that p95 latency is under 200ms.
func TestBenchmarkHybridThreshold(t *testing.T) {
	ctx := context.Background()
	org := ensureBench1K(t)
	queryEmbedding := generateEmbedding(0)

	latencies := collectLatencies(50, func() {
		_, err := testSearchSvc.HybridSearch(ctx, org.OrgID, org.KBID,
			"graph databases relationship networks", queryEmbedding, 10)
		require.NoError(t, err)
	})

	p95Lat := p95(latencies)
	t.Logf("Hybrid 1K p95 latency: %v", p95Lat)
	assert.Less(t, p95Lat, 200*time.Millisecond,
		"Hybrid search p95 latency on 1K chunks must be under 200ms, got %v", p95Lat)
}

// TestBenchmarkIngestionThroughput verifies that inserting 1 document + 20 chunks +
// 20 embeddings and then querying them completes in under 2 seconds.
func TestBenchmarkIngestionThroughput(t *testing.T) {
	ctx := context.Background()
	org := seedOrg(t, ctx, "bench-ingest")
	t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

	start := time.Now()

	// Insert 1 doc with 20 chunks + embeddings.
	docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "ingest-bench.md", "ready")
	for i := range 20 {
		content := fmt.Sprintf("Ingestion benchmark chunk %d: %s",
			i, loremFragments[i%len(loremFragments)])
		chunkID := insertChunk(t, ctx, org.OrgID, org.KBID, docID, i, content,
			fmt.Sprintf("Heading %d", i), 128)
		insertEmbedding(t, ctx, org.OrgID, chunkID, generateEmbedding(i))
	}

	// Verify the data is queryable via TextSearch.
	resp, err := testSearchSvc.TextSearch(ctx, org.OrgID, org.KBID, "ingestion benchmark", 10)
	require.NoError(t, err)
	require.Greater(t, resp.Total, 0, "ingested chunks must be searchable")

	elapsed := time.Since(start)
	t.Logf("Ingestion throughput: 1 doc + 20 chunks + 20 embeddings + query in %v", elapsed)
	assert.Less(t, elapsed, 2*time.Second,
		"ingestion + verification must complete in under 2s, took %v", elapsed)
}

// TestBenchmarkTokenCountConsistency seeds 1,000 chunks with a known token_count
// and verifies that SUM(token_count) matches the expected total exactly.
func TestBenchmarkTokenCountConsistency(t *testing.T) {
	ctx := context.Background()
	org := seedOrg(t, ctx, "bench-tokens")
	t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

	const (
		numDocs      = 50
		chunksPerDoc = 20
		tokenCount   = 128
	)

	totalChunks := seedBenchmarkData(t, ctx, org.OrgID, org.KBID, org.UserID, numDocs, chunksPerDoc)
	expectedTotal := totalChunks * tokenCount

	var actualTotal int64
	err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			"SELECT COALESCE(SUM(token_count), 0) FROM chunks WHERE org_id = $1 AND knowledge_base_id = $2",
			org.OrgID, org.KBID,
		).Scan(&actualTotal)
	})
	require.NoError(t, err)

	assert.Equal(t, int64(expectedTotal), actualTotal,
		"SUM(token_count) must equal %d (chunks=%d x tokens=%d), got %d",
		expectedTotal, totalChunks, tokenCount, actualTotal)
}
