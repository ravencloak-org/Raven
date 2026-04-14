# Integration Test Suite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a comprehensive integration test suite that validates Raven's ingestion pipeline, search (BM25/vector/hybrid), caching, benchmarks, and RLS tenant isolation against a real PostgreSQL instance.

**Architecture:** testcontainers-go spins up a `pgvector/pgvector:0.8.0-pg18` Postgres container in `TestMain`. Goose runs all migrations. Each test suite seeds its own org/workspace/KB. A gRPC mock returns pre-computed chunks and embeddings from fixture data. Two cache subsystems tested separately: Valkey (SHA256 exact-match via miniredis) and Postgres `response_cache` table.

**Tech Stack:** Go 1.25, testcontainers-go, pgxpool, goose, miniredis, gRPC mock, testify

**Spec:** `docs/superpowers/specs/2026-04-13-integration-tests-design.md`

---

## File Structure

```
internal/integration/
├── testdata/
│   ├── sample_markdown.md          # ~2KB, produces 8 chunks
│   ├── sample_technical.md         # ~5KB, produces 20 chunks
│   ├── embeddings.json             # Pre-computed 1536-dim vectors per chunk + queries
│   └── expected_chunks.json        # Expected boundaries, token counts, headings
├── setup_test.go                   # TestMain, container lifecycle, migration, seed helpers
├── helpers_test.go                 # Shared test helpers (seed org, KB, insert chunks, etc.)
├── mock_aiworker_test.go           # In-process gRPC AIWorker mock
├── ingestion_test.go               # File upload, source creation, document lifecycle
├── search_test.go                  # BM25, vector, hybrid RRF search
├── cache_test.go                   # Valkey cache + Postgres response_cache
├── benchmark_test.go               # Latency thresholds, throughput baselines
└── rls_test.go                     # Tenant isolation across all tables
```

All files use build tag `//go:build integration`.

**Key dependencies (from codebase):**
- `db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error)` — sets RLS context via `set_config('app.current_org_id', orgID, true)`
- `repository.NewSearchRepository(pool)` — BM25, vector, hybrid queries
- `repository.NewSemanticCacheRepository(pool)` — `InvalidateKB()`, `Stats()`
- `service.NewSearchService(repo, pool)` — `TextSearch`, `TextSearchWithFilters`, `HybridSearch`
- `service.NewUploadService(repo, pool, store, maxSize, allowedTypes)` — file upload with SHA256 dedup
- `service.NewSourceService(repo, pool)` — source CRUD with type validation
- `cache.Key(kbID, query)` — `SHA256(kbID:normalizeQuery(q))` where `normalizeQuery` = `strings.ToLower(strings.TrimSpace(q))`
- Document status transitions (full 8-state machine): `queued→{crawling,failed}`, `crawling→{parsing,failed}`, `parsing→{chunking,failed}`, `chunking→{embedding,failed}`, `embedding→{ready,failed}`, `failed→{queued,reprocessing}`, `ready→reprocessing`, `reprocessing→{crawling,failed}`

---

### Task 1: Test Fixtures — Sample Documents and Expected Data

**Files:**
- Create: `internal/integration/testdata/sample_markdown.md`
- Create: `internal/integration/testdata/sample_technical.md`
- Create: `internal/integration/testdata/expected_chunks.json`
- Create: `internal/integration/testdata/embeddings.json`

- [ ] **Step 1: Create sample_markdown.md**

A ~2KB markdown document with 8 clearly delimited sections (each section becomes one chunk). Use distinct headings and unique keywords per section for targeted BM25 search assertions.

```markdown
# Introduction to Raven Platform

Raven is a retrieval-augmented generation platform designed for enterprise knowledge management.

## Architecture Overview

The system uses a microservices architecture with Go API servers, Python gRPC workers,
and PostgreSQL with pgvector for hybrid search capabilities.

## Data Ingestion Pipeline

Documents are ingested through file uploads, web crawling, and RSS feeds.
Each document goes through parsing, chunking, and embedding stages.

## Chunk Processing

Text is split into semantic chunks using heading-aware splitting.
Each chunk preserves its heading context and page number for citation.

## Vector Embeddings

Chunks are embedded using 1536-dimensional vectors via OpenAI ada-002 model.
Embeddings enable semantic similarity search across the knowledge base.

## BM25 Full-Text Search

PostgreSQL tsvector indexes enable keyword-based retrieval with ts_rank_cd scoring.
Combined with vector search via Reciprocal Rank Fusion for hybrid retrieval.

## Response Caching

Frequently asked queries are cached using SHA256 hashing for exact-match lookups.
A vector similarity index on the response_cache table enables future semantic caching.

## Tenant Isolation

Row-level security policies enforce strict data isolation between organizations.
Each query runs within a transaction scoped to the requesting org_id.
```

- [ ] **Step 2: Create sample_technical.md**

A ~5KB technical document with 20 sections. Include terms that overlap with `sample_markdown.md` (for cross-document search tests) and unique terms (for filter tests).

Content should cover: API endpoints, authentication, rate limiting, webhooks, streaming, error handling, SDKs, deployment, monitoring, scaling, database schema, migrations, testing, CI/CD, security, logging, configuration, environment variables, Docker setup, Kubernetes deployment.

Each section ~250 words with a unique H2 heading.

- [ ] **Step 3: Create expected_chunks.json**

```json
{
  "sample_markdown.md": {
    "total_chunks": 8,
    "total_token_count": 380,
    "chunks": [
      {"index": 0, "heading": "Introduction to Raven Platform", "page_number": 1, "token_count": 28, "unique_keyword": "retrieval-augmented"},
      {"index": 1, "heading": "Architecture Overview", "page_number": 1, "token_count": 42, "unique_keyword": "microservices"},
      {"index": 2, "heading": "Data Ingestion Pipeline", "page_number": 1, "token_count": 38, "unique_keyword": "ingested"},
      {"index": 3, "heading": "Chunk Processing", "page_number": 1, "token_count": 40, "unique_keyword": "heading-aware"},
      {"index": 4, "heading": "Vector Embeddings", "page_number": 1, "token_count": 44, "unique_keyword": "ada-002"},
      {"index": 5, "heading": "BM25 Full-Text Search", "page_number": 1, "token_count": 48, "unique_keyword": "ts_rank_cd"},
      {"index": 6, "heading": "Response Caching", "page_number": 1, "token_count": 46, "unique_keyword": "SHA256"},
      {"index": 7, "heading": "Tenant Isolation", "page_number": 1, "token_count": 44, "unique_keyword": "row-level"}
    ]
  },
  "sample_technical.md": {
    "total_chunks": 20,
    "total_token_count": 2400,
    "chunks": []
  }
}
```

Note: Token counts are approximate targets. The actual values will be set after running the real tokenizer on the fixture content during test development.

- [ ] **Step 4: Create embeddings.json**

Pre-computed 1536-dimensional vectors. For test purposes, use orthogonal-ish synthetic vectors that give predictable cosine similarity results:

```json
{
  "chunks": {
    "sample_markdown_0": [0.1, 0.2, ...],
    "sample_markdown_1": [0.3, 0.1, ...],
    ...
  },
  "queries": {
    "exact_match_chunk5": {
      "text": "BM25 full-text search ranking",
      "embedding": [0.15, 0.25, ...],
      "expected_nearest_chunk": "sample_markdown_5"
    },
    "cross_document": {
      "text": "architecture and deployment",
      "embedding": [0.22, 0.18, ...],
      "expected_top3": ["sample_markdown_1", "sample_technical_9", "sample_technical_10"]
    }
  }
}
```

Generate actual 1536-dim vectors using a simple deterministic scheme: for chunk i, vector[j] = sin(i * 0.1 + j * 0.01). Query vectors are set to be close to their expected nearest neighbor.

- [ ] **Step 5: Commit**

```bash
git add internal/integration/testdata/
git commit -m "test: add integration test fixture data"
```

---

### Task 2: Test Infrastructure — Setup, Helpers, gRPC Mock

**Files:**
- Create: `internal/integration/setup_test.go`
- Create: `internal/integration/helpers_test.go`
- Create: `internal/integration/mock_aiworker_test.go`

**References:**
- Existing testutil: `internal/testutil/db.go` — extends testcontainer pattern
- DB connection: `internal/db/db.go` — `db.New()`, `db.WithOrgID()`
- Migrations dir: `migrations/`
- gRPC proto: `proto/ai_worker.proto` — `AIWorker` service definition
- Generated pb: `internal/grpc/pb/`

- [ ] **Step 1: Write setup_test.go**

```go
//go:build integration

package integration

import (
    "context"
    "database/sql"
    "fmt"
    "os"
    "path/filepath"
    "runtime"
    "testing"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    _ "github.com/lib/pq"
    "github.com/pressly/goose/v3"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"

    "github.com/ravencloak-org/Raven/internal/repository"
    "github.com/ravencloak-org/Raven/internal/service"
)

var (
    testPool       *pgxpool.Pool
    testSearchSvc  *service.SearchService
    testDocSvc     *service.DocumentService
    testSourceSvc  *service.SourceService
    testCacheRepo  *repository.SemanticCacheRepository
    testSearchRepo *repository.SearchRepository
    testDocRepo    *repository.DocumentRepository
    testSourceRepo *repository.SourceRepository
)

func TestMain(m *testing.M) {
    ctx := context.Background()

    // Start Postgres container with pgvector
    req := testcontainers.ContainerRequest{
        Image:        "pgvector/pgvector:0.8.0-pg18",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_USER":     "raven_test",
            "POSTGRES_PASSWORD": "raven_test",
            "POSTGRES_DB":       "raven_test",
        },
        WaitingFor: wait.ForLog("database system is ready to accept connections").
            WithOccurrence(2).
            WithStartupTimeout(60 * time.Second),
    }

    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    if err != nil {
        panic(fmt.Sprintf("failed to start container: %v", err))
    }
    defer container.Terminate(ctx)

    host, _ := container.Host(ctx)
    port, _ := container.MappedPort(ctx, "5432")

    dsn := fmt.Sprintf("postgres://raven_test:raven_test@%s:%s/raven_test?sslmode=disable", host, port.Port())

    // Resolve migration path relative to this source file (robust across working dirs)
    _, thisFile, _, _ := runtime.Caller(0)
    migDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")

    // Run goose migrations
    sqlDB, err := sql.Open("postgres", dsn)
    if err != nil {
        panic(fmt.Sprintf("failed to open sql.DB: %v", err))
    }
    if err := goose.Up(sqlDB, migDir); err != nil {
        panic(fmt.Sprintf("failed to run migrations: %v", err))
    }
    sqlDB.Close()

    // Create pgxpool for tests
    testPool, err = pgxpool.New(ctx, dsn)
    if err != nil {
        panic(fmt.Sprintf("failed to create pool: %v", err))
    }
    defer testPool.Close()

    // Initialize repositories
    testSearchRepo = repository.NewSearchRepository(testPool)
    testCacheRepo = repository.NewSemanticCacheRepository(testPool)
    testDocRepo = repository.NewDocumentRepository(testPool)
    testSourceRepo = repository.NewSourceRepository(testPool)

    // Initialize services
    testSearchSvc = service.NewSearchService(testSearchRepo, testPool)
    testDocSvc = service.NewDocumentService(testDocRepo, testPool)
    testSourceSvc = service.NewSourceService(testSourceRepo, testPool)

    os.Exit(m.Run())
}
```

- [ ] **Step 2: Write helpers_test.go**

```go
//go:build integration

package integration

import (
    "context"
    "encoding/json"
    "fmt"
    "math"
    "os"
    "testing"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/stretchr/testify/require"

    "github.com/ravencloak-org/Raven/internal/db"
)

// testOrg holds IDs for a seeded test organization.
type testOrg struct {
    OrgID       string
    WorkspaceID string
    KBID        string
    UserID      string
}

// seedOrg creates a full org → workspace → knowledge_base chain and returns the IDs.
func seedOrg(t *testing.T, ctx context.Context, name string) testOrg {
    t.Helper()
    org := testOrg{
        OrgID:       uuid.NewString(),
        WorkspaceID: uuid.NewString(),
        KBID:        uuid.NewString(),
        UserID:      uuid.NewString(),
    }

    // Insert org, workspace, KB using admin connection (bypasses RLS)
    conn, err := testPool.Acquire(ctx)
    require.NoError(t, err)
    defer conn.Release()

    _, err = conn.Exec(ctx, "SET ROLE raven_admin")
    require.NoError(t, err)

    _, err = conn.Exec(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)`,
        org.OrgID, name, name)
    require.NoError(t, err)

    _, err = conn.Exec(ctx, `INSERT INTO workspaces (id, org_id, name, slug) VALUES ($1, $2, $3, $4)`,
        org.WorkspaceID, org.OrgID, name+"-ws", name+"-ws")
    require.NoError(t, err)

    _, err = conn.Exec(ctx, `INSERT INTO knowledge_bases (id, org_id, workspace_id, name) VALUES ($1, $2, $3, $4)`,
        org.KBID, org.OrgID, org.WorkspaceID, name+"-kb")
    require.NoError(t, err)

    _, err = conn.Exec(ctx, "RESET ROLE")
    require.NoError(t, err)

    return org
}

// insertDocument inserts a document record and returns its ID.
func insertDocument(t *testing.T, ctx context.Context, orgID, kbID, fileName, status string) string {
    t.Helper()
    docID := uuid.NewString()
    err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
        _, err := tx.Exec(ctx, `
            INSERT INTO documents (id, org_id, knowledge_base_id, file_name, file_type, file_size_bytes, file_hash, storage_path, processing_status, uploaded_by)
            VALUES ($1, $2, $3, $4, 'text/markdown', 2048, $5, '/test/path', $6::processing_status, $7::uuid)`,
            docID, orgID, kbID, fileName, uuid.NewString(), status, uuid.NewString())
        return err
    })
    require.NoError(t, err)
    return docID
}

// insertChunk inserts a chunk and returns its ID.
func insertChunk(t *testing.T, ctx context.Context, orgID, kbID, docID string, index int, content, heading string, tokenCount int) string {
    t.Helper()
    chunkID := uuid.NewString()
    err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
        _, err := tx.Exec(ctx, `
            INSERT INTO chunks (id, org_id, knowledge_base_id, document_id, content, chunk_index, token_count, page_number, heading, chunk_type)
            VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, 'text')`,
            chunkID, orgID, kbID, docID, content, index, tokenCount, heading)
        return err
    })
    require.NoError(t, err)
    return chunkID
}

// insertEmbedding inserts an embedding vector for a chunk.
func insertEmbedding(t *testing.T, ctx context.Context, orgID, chunkID string, embedding []float32) {
    t.Helper()
    err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
        _, err := tx.Exec(ctx, `
            INSERT INTO embeddings (id, org_id, chunk_id, embedding)
            VALUES ($1, $2, $3, $4)`,
            uuid.NewString(), orgID, chunkID, embedding)
        return err
    })
    require.NoError(t, err)
}

// insertCacheEntry inserts a response_cache row with default expires_at (1 hour from now).
func insertCacheEntry(t *testing.T, ctx context.Context, orgID, kbID, queryText string, embedding []float32, hitCount int) string {
    t.Helper()
    id := uuid.NewString()
    err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
        _, err := tx.Exec(ctx, `
            INSERT INTO response_cache (id, org_id, kb_id, query_text, query_embedding, response_text, sources, model_name, hit_count, expires_at)
            VALUES ($1, $2, $3, $4, $5, 'cached response for: ' || $4, '[]', 'gpt-4', $6, NOW() + INTERVAL '1 hour')`,
            id, orgID, kbID, queryText, embedding, hitCount)
        return err
    })
    require.NoError(t, err)
    return id
}

// insertExpiredCacheEntry inserts a response_cache row that has already expired.
func insertExpiredCacheEntry(t *testing.T, ctx context.Context, orgID, kbID, queryText string, embedding []float32) string {
    t.Helper()
    id := uuid.NewString()
    err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
        _, err := tx.Exec(ctx, `
            INSERT INTO response_cache (id, org_id, kb_id, query_text, query_embedding, response_text, sources, model_name, hit_count, expires_at)
            VALUES ($1, $2, $3, $4, $5, 'expired response', '[]', 'gpt-4', 0, NOW() - INTERVAL '1 second')`,
            id, orgID, kbID, queryText, embedding)
        return err
    })
    require.NoError(t, err)
    return id
}

// cleanupOrg removes all data for a test org.
func cleanupOrg(t *testing.T, ctx context.Context, orgID string) {
    t.Helper()
    conn, err := testPool.Acquire(ctx)
    require.NoError(t, err)
    defer conn.Release()

    _, err = conn.Exec(ctx, "SET ROLE raven_admin")
    require.NoError(t, err)

    // Cascade deletes handle everything from org level
    _, err = conn.Exec(ctx, "DELETE FROM organizations WHERE id = $1", orgID)
    require.NoError(t, err)

    _, err = conn.Exec(ctx, "RESET ROLE")
    require.NoError(t, err)
}

// loadFixture reads and unmarshals a JSON fixture file.
func loadFixture[T any](t *testing.T, filename string) T {
    t.Helper()
    data, err := os.ReadFile("testdata/" + filename)
    require.NoError(t, err)
    var result T
    require.NoError(t, json.Unmarshal(data, &result))
    return result
}

// generateEmbedding creates a deterministic 1536-dim vector: vector[j] = sin(seed*0.1 + j*0.01).
func generateEmbedding(seed int) []float32 {
    vec := make([]float32, 1536)
    for j := range vec {
        vec[j] = float32(math.Sin(float64(seed)*0.1 + float64(j)*0.01))
    }
    return vec
}
```

- [ ] **Step 3: Write mock_aiworker_test.go**

```go
//go:build integration

package integration

import (
    "context"
    "fmt"
    "sync"

    pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// mockAIWorker is an in-process gRPC mock of the AIWorker service.
type mockAIWorker struct {
    pb.UnimplementedAIWorkerServer

    mu             sync.Mutex
    parseRequests  []*pb.ParseRequest  // records received requests
    failParseEmbed bool                // if true, ParseAndEmbed returns an error
    failFromStatus string              // which status to fail from

    // Pre-computed responses keyed by document_id
    parseResponses map[string]*pb.ParseResponse
    embeddings     map[string]*pb.EmbeddingResponse
}

func newMockAIWorker() *mockAIWorker {
    return &mockAIWorker{
        parseResponses: make(map[string]*pb.ParseResponse),
        embeddings:     make(map[string]*pb.EmbeddingResponse),
    }
}

func (m *mockAIWorker) ParseAndEmbed(ctx context.Context, req *pb.ParseRequest) (*pb.ParseResponse, error) {
    m.mu.Lock()
    m.parseRequests = append(m.parseRequests, req)
    m.mu.Unlock()

    if m.failParseEmbed {
        return nil, status.Errorf(codes.Internal, "mock: simulated parse failure")
    }

    if resp, ok := m.parseResponses[req.DocumentId]; ok {
        return resp, nil
    }

    return &pb.ParseResponse{
        DocumentId: req.DocumentId,
        ChunkCount: 0,
        Status:     "ready",
    }, nil
}

func (m *mockAIWorker) GetEmbedding(ctx context.Context, req *pb.EmbeddingRequest) (*pb.EmbeddingResponse, error) {
    if resp, ok := m.embeddings[req.Text]; ok {
        return resp, nil
    }
    // Return a deterministic embedding based on text hash
    return &pb.EmbeddingResponse{
        Embedding:  make([]float32, 1536),
        Dimensions: 1536,
    }, nil
}

func (m *mockAIWorker) QueryRAG(req *pb.RAGRequest, stream pb.AIWorker_QueryRAGServer) error {
    // Send a single final chunk
    return stream.Send(&pb.RAGChunk{
        Text:    fmt.Sprintf("Mock response for: %s", req.Query),
        IsFinal: true,
        Sources: []*pb.Source{},
    })
}

func (m *mockAIWorker) getParseRequests() []*pb.ParseRequest {
    m.mu.Lock()
    defer m.mu.Unlock()
    cpy := make([]*pb.ParseRequest, len(m.parseRequests))
    copy(cpy, m.parseRequests)
    return cpy
}
```

- [ ] **Step 4: Verify the build compiles**

```bash
cd /Users/jobinlawrance/Project/raven
go build -tags=integration ./internal/integration/...
```

Expected: compiles with no errors (tests won't run yet — no test functions).

- [ ] **Step 5: Commit**

```bash
git add internal/integration/
git commit -m "test: add integration test infrastructure — setup, helpers, gRPC mock"
```

---

### Task 3: Ingestion Pipeline Tests

**Files:**
- Create: `internal/integration/ingestion_test.go`

**References:**
- Upload service: `internal/service/upload.go` — `Upload()` validates type/size, SHA256 dedup, stores file, creates doc with status `queued`
- Document service: `internal/service/document.go` — `UpdateStatus()` validates full 8-state machine: `queued→{crawling,failed}`, `crawling→{parsing,failed}`, `parsing→{chunking,failed}`, `chunking→{embedding,failed}`, `embedding→{ready,failed}`, `failed→{queued,reprocessing}`, `ready→reprocessing`, `reprocessing→{crawling,failed}`
- Source service: `internal/service/source.go` — `Create()` validates source_type ∈ {web_page, web_site, sitemap, rss_feed}, URL, crawl_depth 1-5
- Source model: `internal/model/source.go` — `SourceTypeWebPage`, `SourceTypeWebSite`, `SourceTypeSitemap`, `SourceTypeRSSFeed`

- [ ] **Step 1: Write test for document status lifecycle (full 8-state machine)**

Test the complete document status transition chain managed by `testDocSvc.UpdateStatus()`:
- **Happy path**: `queued → crawling → parsing → chunking → embedding → ready` (5 transitions)
- **Failure from each intermediate state**: `queued → failed`, `crawling → failed`, `parsing → failed`, `chunking → failed`, `embedding → failed`
- **Recovery path**: `failed → queued → crawling...` and `failed → reprocessing → crawling...`
- **Reprocessing**: `ready → reprocessing → crawling → parsing → ...`
- **Invalid transitions**: `queued → ready` (skip), `queued → parsing` (skip), `ready → queued` (invalid) — all return error

Uses `insertDocument` helper + `testDocSvc.UpdateStatus()` calls. Each sub-test creates a fresh document at the starting status.

- [ ] **Step 2: Run to verify it fails**

```bash
go test -tags=integration ./internal/integration/ -run TestIngestion/document_lifecycle -v -timeout 2m
```

Expected: FAIL (test function not found until created, then assertions fail until implementation is wired)

- [ ] **Step 3: Write test for chunk storage correctness**

After inserting a document in `ready` status and manually inserting 8 chunks matching `expected_chunks.json`:
- Query chunks by document_id
- Assert exactly 8 chunks with correct `heading`, `chunk_index`, `token_count`
- Assert content is non-empty and matches expected unique keywords

- [ ] **Step 4: Write test for embedding storage**

Insert 8 chunks + 8 embeddings (1536-dim vectors from `generateEmbedding()`):
- Query embeddings joined to chunks
- Assert 1:1 mapping, correct dimensions, values match

- [ ] **Step 5: Write test for source creation (all 4 types)**

Test `web_page`, `web_site`, `sitemap`, `rss_feed` source creation:
- Each created with valid URL and crawl config
- Assert source record persisted with correct type, URL, org_id, kb_id
- Assert invalid source type returns error

- [ ] **Step 6: Write test for duplicate document detection**

Insert a document with a known file_hash. Attempt to insert another document with the same hash in the same KB:
- Assert duplicate is detected (409 or specific error)
- Assert a document with the same hash in a *different* KB succeeds

- [ ] **Step 7: Write test for failure path**

Insert a document in `queued` status. Call `UpdateStatus` to transition to `failed` with an error message:
- Assert status is `failed`
- Assert error message is persisted

- [ ] **Step 8: Write test for token count accuracy**

Insert 20 chunks with known token_counts. Run `SUM(token_count)`:
- Assert sum matches expected total exactly (deterministic fixtures)

- [ ] **Step 9: Write test for concurrent ingestion**

Use `errgroup.Group` to upload two documents simultaneously to the same KB:
- Both insert documents + chunks + embeddings concurrently
- Assert no race conditions: both documents reach `ready` status
- Assert chunk counts are correct for both documents (no interleaving)
- Assert total chunks in KB = sum of both documents' chunks

- [ ] **Step 10: Write test for large document (500+ chunks)**

Insert a single document with 500 chunks and 500 embeddings:
- Assert all 500 chunks queryable
- Assert `SUM(token_count)` matches expected total
- Assert BM25 search returns results from these chunks

- [ ] **Step 11: Run full ingestion suite**

```bash
go test -tags=integration ./internal/integration/ -run TestIngestion -v -timeout 2m
```

Expected: all PASS

- [ ] **Step 12: Commit**

```bash
git add internal/integration/ingestion_test.go
git commit -m "test: add ingestion pipeline integration tests"
```

---

### Task 4: Search Tests — BM25, Vector, Hybrid RRF

**Files:**
- Create: `internal/integration/search_test.go`

**References:**
- Search service: `internal/service/search.go` — `TextSearch()`, `TextSearchWithFilters()`, `HybridSearch()`
- Search repo: `internal/repository/search.go` — `TextSearch()` uses `ts_rank_cd` + `plainto_tsquery`, `VectorSearch()` uses `1 - (embedding <=> $2)`, `BM25Search()` uses `ts_rank_cd`
- `clampLimit()`: `<= 0` → 10, `> 100` → 100
- `sanitizeQuery()`: `strings.Fields(strings.TrimSpace(q))` joined by space
- `fuseRRF()`: `score = sum(1/(60 + rank_i))`, sorts descending, truncates to topK

**Test data setup:** Seed an org with both sample docs, 28 chunks total (8 + 20), embeddings for all.

- [ ] **Step 1: Write BM25 exact keyword match test**

Search for a term known to exist in exactly 3 chunks (e.g., "architecture" appears in chunks with headings "Architecture Overview", "API Architecture", "Deployment Architecture"):
- Assert 3 results returned
- Assert scores are descending
- Assert each result's Content contains the search term

- [ ] **Step 2: Write BM25 phrase search test**

Search for "retrieval-augmented generation" (multi-word):
- Assert results containing the full phrase rank higher than partial matches
- Assert `Highlight` field contains the matched phrase

- [ ] **Step 3: Write BM25 no results test**

Search for "xyznonexistentterm123":
- Assert empty results slice
- Assert `Total: 0`

- [ ] **Step 4: Write BM25 document filter test**

Use `TextSearchWithFilters` for a term in both docs, restricted to doc1's ID:
- Assert all results have `DocumentID == doc1.ID`
- Assert result count < total matches across both docs

- [ ] **Step 5: Write limit clamping test**

Three sub-tests:
- `limit=0` → assert 10 results returned (if >= 10 chunks match)
- `limit=-1` → assert 10 results returned
- `limit=200` → assert at most 100 results returned

- [ ] **Step 6: Write empty KB search test**

Create a fresh KB with no documents/chunks. Search for any term:
- Assert empty results, no error

- [ ] **Step 7: Write vector nearest neighbor test**

Query with `generateEmbedding(5)` (same seed as chunk #5):
- Assert chunk #5 is the top result
- Assert `VectorScore` > 0.99 (near-identical vector)

- [ ] **Step 8: Write vector dimension mismatch test**

Query with a 512-dim embedding instead of 1536:
- Assert error returned (not a panic)

- [ ] **Step 9: Write hybrid RRF fusion correctness test**

Query with text + embedding where:
- BM25 top results: chunks [A, B, C]
- Vector top results: chunks [B, D, A]
- Expected: B ranks highest (appears in both with high ranks), then A

Assert RRF scores: `B.RRFScore > A.RRFScore > C.RRFScore` and `B.RRFScore > D.RRFScore`.

- [ ] **Step 10: Write BM25-only fallback test**

`HybridSearch` with empty embedding + valid query:
- Assert results come from BM25 only
- Assert `VectorScore == 0` and `VectorRank == 0` for all results

- [ ] **Step 11: Write vector-only fallback test**

`HybridSearch` with empty query + valid embedding:
- Assert results come from vector only
- Assert `BM25Score == 0` and `BM25Rank == 0` for all results

- [ ] **Step 12: Write topK test**

`HybridSearch` with topK=3 on a dataset with 28 chunks:
- Assert exactly 3 results returned

- [ ] **Step 13: Write unicode content test**

Insert chunks with CJK characters, emoji, and RTL text. Run BM25 search:
- Assert no error (may return 0 results for CJK since English tsvector config)

- [ ] **Step 14: Write duplicate embedding vectors test**

Insert two different chunks with identical embedding vectors (`generateEmbedding(99)` for both):
- Vector search with that same embedding → both chunks returned
- Assert result count >= 2 for those chunks

- [ ] **Step 15: Run full search suite**

```bash
go test -tags=integration ./internal/integration/ -run TestSearch -v -timeout 2m
```

Expected: all PASS

- [ ] **Step 16: Commit**

```bash
git add internal/integration/search_test.go
git commit -m "test: add search integration tests — BM25, vector, hybrid RRF"
```

---

### Task 5: Cache Tests — Valkey + Postgres response_cache

**Files:**
- Create: `internal/integration/cache_test.go`

**References:**
- Valkey cache: `internal/cache/cache.go` — `Key()`, `Get()`, `Set()`, `InvalidateKB()`, uses `normalizeQuery` = `strings.ToLower(strings.TrimSpace(q))`
- Postgres cache: `internal/repository/semantic_cache.go` — `InvalidateKB()`, `Stats()`
- Cache key: `SHA256(kbID:normalizeQuery(q))`
- Redis dep: use `github.com/alicebob/miniredis/v2` for Valkey subsystem tests
- response_cache schema: `migrations/00027_response_cache.sql` — `hit_count`, `expires_at`, `query_embedding vector(1536)`, HNSW index

**Two subsections:** Subsystem A (Valkey via miniredis), Subsystem B (Postgres response_cache table).

- [ ] **Step 1: Write Valkey end-to-end cache miss → store → hit test**

Using miniredis-backed `cache.ResponseCache`. Simulates the full request flow:
- Create a `cache.ResponseCache` connected to miniredis
- First query: `Get(kbID, "what is raven")` → nil (miss); simulate calling gRPC worker, then `Set(kbID, "what is raven", &CachedResponse{Text: "...", Sources: [...]})`
- Second query: `Get(kbID, "what is raven")` → returns cached response with matching Text and Sources
- Verify the gRPC mock would NOT be called on the second query (cache hit skips worker)

- [ ] **Step 2: Write Valkey normalized matching test**

- `Set(kbID, "what is raven", response)`
- `Get(kbID, " What Is RAVEN ")` → cache hit (TrimSpace + ToLower normalizes to same key)

- [ ] **Step 3: Write Valkey internal whitespace NOT normalized test**

- `Set(kbID, "hello world", response)`
- `Get(kbID, "hello  world")` → cache miss (double space produces different SHA256)

- [ ] **Step 4: Write Valkey KB invalidation test**

- Set 3 entries for KB-1, 2 for KB-2
- `InvalidateKB(KB-1)` → no error
- `Get` for all KB-1 entries → miss
- `Get` for all KB-2 entries → still hit

- [ ] **Step 5: Write Postgres hit_count increment test**

Insert a `response_cache` row with `hit_count = 0`:
- Run `UPDATE response_cache SET hit_count = hit_count + 1 WHERE id = $1` five times
- Query the row, assert `hit_count = 5`

- [ ] **Step 6: Write Postgres TTL expiration test**

Insert a cache entry with `expires_at = NOW() - INTERVAL '1 second'`:
- Query `WHERE expires_at > NOW()` → 0 rows
- Insert another with default `expires_at` (1 hour ahead) → 1 row

- [ ] **Step 7: Write Postgres KB invalidation test**

Insert 3 entries for KB-1, 2 for KB-2:
- Call `testCacheRepo.InvalidateKB(ctx, orgID, kb1ID)`
- Assert: 3 deleted (returned from InvalidateKB)
- Query remaining: only KB-2 entries

- [ ] **Step 8: Write Postgres Stats test**

Insert entries with hit_counts [2, 4, 6] for one KB:
- Call `testCacheRepo.Stats(ctx, orgID, kbID)`
- Assert `count = 3`, `avg_hits = 4.0`

- [ ] **Step 9: Write HNSW index test**

Insert 1,000 cache entries with distinct `generateEmbedding(i)` vectors:
- `SET enable_seqscan = off` in the test transaction
- Run `SELECT id FROM response_cache ORDER BY query_embedding <=> $1 LIMIT 1`
- Assert closest vector returned matches expected
- Run `EXPLAIN ANALYZE` on the same query, assert output contains "Index Scan using idx_response_cache_embedding"

- [ ] **Step 10: Write Postgres RLS on cache test**

Insert cache entry for Org-A:
- Query as Org-B → 0 rows
- Query as Org-A → 1 row

- [ ] **Step 11: Run full cache suite**

```bash
go test -tags=integration ./internal/integration/ -run TestCache -v -timeout 2m
```

Expected: all PASS

- [ ] **Step 12: Commit**

```bash
git add internal/integration/cache_test.go
git commit -m "test: add cache integration tests — Valkey SHA256 + Postgres response_cache"
```

---

### Task 6: Benchmark Tests

**Files:**
- Create: `internal/integration/benchmark_test.go`

**References:**
- Search service: `internal/service/search.go` — `TextSearch()`, `HybridSearch()`
- Benchmark thresholds: BM25 <100ms p95, hybrid <200ms p95, cache hit <5ms, HNSW <50ms

**Approach:** Use Go's `testing.B` for benchmarks. Companion `TestBenchmark*` functions run the benchmarks programmatically and assert p95 thresholds using `testing.Benchmark()`.

- [ ] **Step 1: Write seed helper for benchmark data**

Function `seedBenchmarkData(t, ctx, orgID, kbID, numDocs, chunksPerDoc int)` that:
- Inserts `numDocs` documents in `ready` status
- Inserts `chunksPerDoc` chunks per doc with realistic text content
- Inserts embeddings for each chunk
- Returns total chunk count

- [ ] **Step 2: Write BM25 benchmark (1,000 chunks)**

```go
func BenchmarkBM25Search1K(b *testing.B) {
    // Pre-seeded with 1,000 chunks
    ctx := context.Background()
    for b.Loop() {
        _, err := testSearchSvc.TextSearch(ctx, benchOrg.OrgID, benchOrg.KBID, "architecture deployment", 10)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

- [ ] **Step 3: Write threshold assertion for BM25**

```go
func TestBenchmarkBM25Threshold(t *testing.T) {
    result := testing.Benchmark(BenchmarkBM25Search1K)
    p95ns := result.NsPerOp() // simplified; for real p95, collect latencies in benchmark
    p95ms := float64(p95ns) / 1e6
    if p95ms > 100 {
        t.Errorf("BM25 p95 latency %vms exceeds 100ms threshold", p95ms)
    }
}
```

Note: For true p95, the benchmark function should record individual latencies in a slice and compute the percentile. The above is simplified — implementation should use a latency collector pattern.

- [ ] **Step 4: Write hybrid search benchmark (1,000 chunks)**

Same pattern as BM25 but calls `HybridSearch` with text + embedding. Threshold: p95 < 200ms.

- [ ] **Step 5: Write BM25 scale benchmark (10,000 chunks)**

Seed 10,000 chunks. Run benchmark. Record baseline only — no hard threshold.

- [ ] **Step 6: Write Valkey cache hit benchmark**

Using miniredis, pre-populate cache. Benchmark `Get()` calls. Threshold: p95 < 5ms.

- [ ] **Step 7: Write HNSW vector scan benchmark**

Pre-insert 1,000 response_cache entries. Benchmark cosine similarity query. Threshold: p95 < 50ms.

- [ ] **Step 8: Write ingestion throughput test**

Time the end-to-end path: insert document → insert 20 chunks → insert 20 embeddings → verify all queryable.
Threshold: < 2 seconds.

- [ ] **Step 9: Write token count consistency test**

Seed 1,000 chunks with known token_counts. Assert `SUM(token_count)` matches expected total exactly.

- [ ] **Step 10: Run benchmarks**

```bash
go test -tags=integration ./internal/integration/ -run TestBenchmark -bench=. -benchmem -v -timeout 10m
```

Expected: all PASS, benchmarks output ns/op

- [ ] **Step 11: Commit**

```bash
git add internal/integration/benchmark_test.go
git commit -m "test: add benchmark tests with latency thresholds"
```

---

### Task 7: RLS Tenant Isolation Tests

**Files:**
- Create: `internal/integration/rls_test.go`

**References:**
- `db.WithOrgID()`: `internal/db/db.go` — sets `app.current_org_id` GUC for RLS
- RLS policies: every table has `tenant_isolation` policy (`USING (org_id = current_setting('app.current_org_id')::uuid)`) + `admin_bypass` policy (`FOR ALL TO raven_admin USING (true)`)
- `raven_admin` role: created in `migrations/00002_roles.sql`

**Setup:** Two complete orgs (Org-A, Org-B), each with documents, chunks, embeddings, cache entries, and sources.

- [ ] **Step 1: Write test setup — seed two isolated orgs**

```go
func TestRLS(t *testing.T) {
    ctx := context.Background()
    orgA := seedOrg(t, ctx, "rls-org-a")
    orgB := seedOrg(t, ctx, "rls-org-b")
    t.Cleanup(func() {
        cleanupOrg(t, ctx, orgA.OrgID)
        cleanupOrg(t, ctx, orgB.OrgID)
    })

    // Seed documents, chunks, embeddings, cache, sources for both orgs
    // ... (using helpers)
}
```

- [ ] **Step 2: Write document isolation test**

Query documents as Org-A via `db.WithOrgID`:
- Assert only Org-A's documents returned
- Switch to Org-B, assert only Org-B's documents

- [ ] **Step 3: Write chunk isolation test (BM25)**

Insert chunks with the term "isolation-test-keyword" in both orgs. Search as Org-A:
- Assert results contain only Org-A's chunks
- Search as Org-B → only Org-B's chunks

- [ ] **Step 4: Write embedding isolation test (vector)**

Insert embeddings in both orgs. Org-A's embedding is closest to Org-B's chunk:
- Vector search as Org-A → Org-B's chunk NOT returned
- Org-A only sees own embeddings

- [ ] **Step 5: Write cache isolation test**

Insert cache entry for Org-A. Query `response_cache` as Org-B:
- Assert 0 rows
- Query as Org-A → 1 row

- [ ] **Step 6: Write cache invalidation scoping test**

Both orgs have cache entries. Org-A invalidates:
- Assert Org-B's entries untouched
- Assert Org-A's entries deleted

- [ ] **Step 7: Write source isolation test**

Create sources in both orgs. List sources as Org-A:
- Assert only Org-A's sources

- [ ] **Step 8: Write cross-org KB access test**

Org-A searches Org-B's KB ID:
- Assert empty results (RLS prevents access)
- Assert no error (query runs but returns nothing due to RLS filter)

- [ ] **Step 9: Write admin bypass test**

Connect with `SET ROLE raven_admin`. Query documents:
- Assert both Org-A and Org-B documents visible

- [ ] **Step 10: Run full RLS suite**

```bash
go test -tags=integration ./internal/integration/ -run TestRLS -v -timeout 2m
```

Expected: all PASS

- [ ] **Step 11: Commit**

```bash
git add internal/integration/rls_test.go
git commit -m "test: add RLS tenant isolation integration tests"
```

---

### Task 8: CI Integration and Final Validation

**Files:**
- Modify: `.github/workflows/ci.yml` (or equivalent CI config)
- Create: `Makefile` target (if Makefile exists)

- [ ] **Step 1: Add integration test make target**

```makefile
.PHONY: test-integration
test-integration:
	go test -tags=integration ./internal/integration/ -v -timeout 5m

.PHONY: bench-integration
bench-integration:
	go test -tags=integration ./internal/integration/ -bench=. -benchmem -timeout 10m
```

- [ ] **Step 2: Add CI job for integration tests**

Add a new job in CI config that:
- Runs on Docker-enabled runners
- Installs Go
- Runs `make test-integration`
- Uploads test results as artifacts
- Runs benchmarks and stores results

- [ ] **Step 3: Run full integration suite locally**

```bash
go test -tags=integration ./internal/integration/ -v -timeout 5m -count=1
```

Expected: all tests PASS

- [ ] **Step 4: Run benchmarks locally**

```bash
go test -tags=integration ./internal/integration/ -bench=. -benchmem -timeout 10m
```

Expected: benchmarks complete with acceptable latencies

- [ ] **Step 5: Commit CI changes**

```bash
git add Makefile .github/
git commit -m "ci: add integration test and benchmark jobs"
```

---

## Execution Order & Parallelism

Tasks must execute in this order:

1. **Task 1** (fixtures) — no dependencies, must be first
2. **Task 2** (setup/helpers/mock) — depends on Task 1 for fixture references
3. **Tasks 3-7** (test suites) — depend on Tasks 1+2, but are **independent of each other** and can run in **parallel agents**
4. **Task 8** (CI) — depends on all previous tasks

```
Task 1 → Task 2 → ┬→ Task 3 (ingestion)  ─┐
                   ├→ Task 4 (search)      ─┤
                   ├→ Task 5 (cache)       ─┼→ Task 8 (CI)
                   ├→ Task 6 (benchmarks)  ─┤
                   └→ Task 7 (RLS)         ─┘
```
