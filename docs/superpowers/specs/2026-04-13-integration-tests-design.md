# Integration Test Suite Design — Raven Core Pipeline

**Date**: 2026-04-13  
**Status**: Approved  
**Scope**: Go API integration tests against real PostgreSQL (pgvector + ParadeDB)

## Overview

Integration tests for Raven's core features: data ingestion (file uploads, URL sources, DB connectors), cataloguing (document lifecycle, chunk/embedding storage), search (BM25, vector, hybrid RRF), response caching, token accounting, and tenant isolation (RLS). All tests run against a real Postgres instance via testcontainers-go.

## Architecture

### Test Database Lifecycle

- **testcontainers-go** spins up a `paradedb/paradedb:latest` container per test package in `TestMain`
- Goose migrations run on startup to create the full schema
- Each test suite seeds a fresh org/workspace/KB
- Cleanup between test functions via `DELETE FROM` scoped to the test org
- Container torn down in `TestMain` teardown

### gRPC AI Worker Mock

In-process mock implementing the `AIWorker` gRPC interface (`raven.ai.v1`):

- `ParseAndEmbed` — returns pre-computed chunks from fixture data
- `QueryRAG` — returns streaming response from fixtures
- `GetEmbedding` — returns pre-computed 1536-dim vectors from `embeddings.json`
- Supports configurable latency injection and error simulation

### Fixture Data (`internal/integration/testdata/`)

| File | Purpose |
|------|---------|
| `sample_markdown.md` | ~2KB doc, produces exactly 8 chunks |
| `sample_technical.md` | ~5KB doc, produces exactly 20 chunks with known headings |
| `embeddings.json` | Pre-computed 1536-dim vectors for each chunk + test queries |
| `expected_chunks.json` | Expected chunk boundaries, token counts, headings per document |

### Package Structure

```
internal/integration/
├── testdata/
│   ├── sample_markdown.md
│   ├── sample_technical.md
│   ├── embeddings.json
│   └── expected_chunks.json
├── setup_test.go          # TestMain, container, migrations, helpers
├── ingestion_test.go      # Ingestion pipeline tests
├── search_test.go         # BM25, vector, hybrid search tests
├── cache_test.go          # Response cache tests
├── benchmark_test.go      # Latency & throughput benchmarks
└── rls_test.go            # Tenant isolation tests
```

All files use build tag `//go:build integration`.

## Test Suites

### 1. Ingestion Pipeline (`ingestion_test.go`)

Pre-condition: testcontainer running, migrations applied, gRPC mock started.

| # | Test Case | Assertion |
|---|-----------|-----------|
| 1 | File upload → document lifecycle | Document transitions `queued → processing → succeeded`; mock receives correct `ParseRequest` (content, mime_type, org_id, kb_id) |
| 2 | Chunk storage correctness | After ingestion: exactly 8 chunks for `sample_markdown.md` with correct `heading`, `page_number`, `token_count` matching `expected_chunks.json` |
| 3 | Embedding storage | One 1536-dim vector per chunk in `embeddings` table, values match `embeddings.json` |
| 4 | URL source ingestion | Source record created with correct URL metadata and status (Airbyte sync mocked at HTTP boundary) |
| 5 | DB connector source | Connector type, credentials storage, and sync trigger path validated |
| 6 | Duplicate detection | Upload same document twice to same KB; assert correct handling (reject or new version per current behavior) |
| 7 | Failure path | gRPC mock returns error on `ParseAndEmbed`; document transitions to `failed` with persisted error message |
| 8 | Token count accuracy | `SUM(chunks.token_count)` for `sample_technical.md` (20 chunks) within ±5% of known expected total |

### 2. Search (`search_test.go`)

Pre-condition: Both sample documents ingested, chunks and embeddings populated from fixtures.

**BM25 full-text search:**

| # | Test Case | Assertion |
|---|-----------|-----------|
| 1 | Exact keyword match | Term in exactly 3 chunks → 3 results, BM25 score descending |
| 2 | Phrase search | Multi-word phrase ranks higher than partial matches |
| 3 | No results | Non-existent term → empty results, `total: 0` |
| 4 | Document filter | `TextSearchWithFilters` with one doc ID → only that doc's chunks |
| 5 | Limit clamping | limit=0 defaults to 10; limit=200 clamps to 100 |

**Vector similarity search:**

| # | Test Case | Assertion |
|---|-----------|-----------|
| 6 | Nearest neighbor | Pre-computed embedding closest to chunk #5 → chunk #5 is top result |
| 7 | Dimension mismatch | Wrong-dimension embedding → graceful error, no panic |

**Hybrid search (RRF):**

| # | Test Case | Assertion |
|---|-----------|-----------|
| 8 | Fusion correctness | Chunk appearing in both BM25 and vector results ranks highest |
| 9 | BM25-only fallback | Empty embedding + valid text → BM25 results only, `VectorScore`/`VectorRank` zero |
| 10 | Vector-only fallback | Empty query + valid embedding → vector results only, `BM25Score`/`BM25Rank` zero |
| 11 | topK respected | topK=3 on large candidate set → exactly 3 results |

### 3. Cache (`cache_test.go`)

**SHA256 exact-match cache (current implementation via `cache/cache.go`):**

| # | Test Case | Assertion |
|---|-----------|-----------|
| 1 | Cache miss → store → hit | Query misses, store response, identical query hits with matching response |
| 2 | Normalized matching | Extra whitespace + case difference → cache hit (SHA256 on normalized text) |
| 3 | Different query → miss | Stored query A, query B → cache miss |
| 4 | Hit count increment | 5 hits on same entry → `hit_count = 5` in `response_cache` table |
| 5 | TTL expiration | Entry with `expires_at` in the past → cache miss |
| 6 | KB invalidation | 3 entries for KB-1, 2 for KB-2; invalidate KB-1 → KB-1 deleted, KB-2 untouched |
| 7 | Cache stats | Varying hit counts → correct `count` and `avg_hits` from `Stats()` |

**Response cache Postgres table (future semantic path):**

| # | Test Case | Assertion |
|---|-----------|-----------|
| 8 | HNSW index exercised | 50 entries with distinct embeddings; `ORDER BY query_embedding <=> $1 LIMIT 1` returns closest; `EXPLAIN ANALYZE` confirms HNSW index scan |
| 9 | RLS on cache | Org-A's cache invisible when querying as Org-B |

### 4. Benchmarks (`benchmark_test.go`)

Using Go's `testing.B` with companion `Test` functions for threshold assertions.

| # | Benchmark | Dataset | Threshold |
|---|-----------|---------|-----------|
| 1 | BM25 search latency | 1,000 chunks / 50 docs | p95 < 100ms |
| 2 | BM25 scale test | 10,000 chunks / 500 docs | Baseline record (no hard threshold yet) |
| 3 | Hybrid search (RRF) latency | 1,000 chunks | p95 < 200ms |
| 4 | SHA256 cache hit | Populated cache, 1,000 iterations | p95 < 5ms |
| 5 | HNSW vector cache scan | 100 cache entries | p95 < 50ms |
| 6 | Ingestion throughput | `sample_technical.md` (20 chunks) | Document `succeeded` with correct chunk count in < 2s |
| 7 | Token count consistency | 1,000 synthetic chunks | `SUM(token_count)` matches expected total exactly |

**Benchmark baseline**: Results written to `internal/integration/testdata/benchmark_baseline.json` on first run. CI compares against baseline — regressions > 20% fail the build.

### 5. RLS Tenant Isolation (`rls_test.go`)

Setup: Two orgs (Org-A, Org-B), each with own KB, documents, chunks, embeddings, cache entries. **Zero tolerance — all hard pass/fail.**

| # | Test Case | Assertion |
|---|-----------|-----------|
| 1 | Document isolation | Org-A sees only own documents; Org-B sees only own documents |
| 2 | Chunk isolation | BM25 search as Org-A for term in both orgs → only Org-A's chunks |
| 3 | Embedding isolation | Vector search as Org-A with embedding closest to Org-B chunk → Org-B chunk NOT returned |
| 4 | Cache isolation | Org-A caches entry; Org-B queries identical text → cache miss |
| 5 | Cache invalidation scoping | Org-A invalidates KB cache → Org-B cache untouched |
| 6 | Source isolation | List sources as Org-A → only Org-A's sources |
| 7 | Cross-org KB access | Org-A searches Org-B's KB ID → empty results or forbidden, never Org-B's data |
| 8 | Admin bypass | `raven_admin` role → all orgs' data visible |

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `testcontainers-go` | Programmatic Postgres container lifecycle |
| `paradedb/paradedb:latest` | Docker image with pgvector + pg_bm25 extensions |
| `pressly/goose` | Migration runner (already in use) |
| `google.golang.org/grpc` | gRPC mock server (already in use) |

## Running

```bash
# Run all integration tests
go test -tags=integration ./internal/integration/ -v -timeout 5m

# Run benchmarks only
go test -tags=integration ./internal/integration/ -bench=. -benchmem -timeout 10m

# Run specific suite
go test -tags=integration ./internal/integration/ -run TestRLS -v
```

## CI Integration

- Integration tests run in a separate CI job (requires Docker)
- Build tag `integration` ensures they never run during `go test ./...`
- Benchmark baseline checked into repo; CI compares and fails on > 20% regression
- Test results published as CI artifacts

## Design Decisions

1. **testcontainers-go over Docker Compose** — self-contained, no external orchestration needed, works identically local and CI
2. **SHA256 cache tested as-is** — the current implementation is exact-match, not semantic similarity. HNSW tests validate the Postgres index works for when the semantic path is wired up
3. **Pre-computed embeddings** — avoids calling real embedding APIs in tests, makes assertions deterministic
4. **RRF fusion tested at service layer** — the `fuseRRF` function is pure logic, but we test it through the full `HybridSearch` path to also exercise the repository queries
5. **Benchmark baselines** — first run establishes the record; subsequent runs detect regressions. No hard thresholds on the 10k scale test until we have production data
