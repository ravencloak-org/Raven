# Integration Test Suite Design — Raven Core Pipeline

**Date**: 2026-04-13  
**Status**: Approved  
**Scope**: Go API integration tests against real PostgreSQL (pgvector)

## Overview

Integration tests for Raven's core features: data ingestion (file uploads, URL/web sources), cataloguing (document lifecycle with 8-state machine, chunk/embedding storage), search (BM25 via native PostgreSQL tsvector/`ts_rank_cd`, vector via pgvector, hybrid RRF), response caching (Valkey exact-match + Postgres semantic cache table), token accounting, and tenant isolation (RLS). All tests run against a real Postgres instance via testcontainers-go.

> **Note**: BM25 full-text search uses native PostgreSQL `ts_rank_cd` with `plainto_tsquery` and GIN indexes on tsvector columns — **not** ParadeDB `pg_bm25`. The `HybridRetrievalService` also supports a ClickHouse/QBit backend, but that path is excluded from this test suite (PostgreSQL-only scope).

## Architecture

### Test Database Lifecycle

- **testcontainers-go** spins up a `pgvector/pgvector:0.8.0-pg18` container per test package in `TestMain`
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
| 1 | File upload → full document lifecycle | Document transitions through the full state machine: `queued → crawling → parsing → chunking → embedding → ready`. Mock gRPC worker receives correct `ParseRequest` (content, mime_type, org_id, kb_id). Assert each intermediate status is recorded. |
| 2 | Chunk storage correctness | After ingestion reaches `ready`: exactly 8 chunks for `sample_markdown.md` with correct `heading`, `page_number`, `token_count` matching `expected_chunks.json` |
| 3 | Embedding storage | One 1536-dim vector per chunk in `embeddings` table, values match `embeddings.json` |
| 4 | Web page source ingestion | Create source with type `web_page`; assert source record with correct URL metadata and status |
| 5 | Web site / sitemap / RSS sources | Create sources with types `web_site`, `sitemap`, `rss_feed`; assert each is stored with correct type and config |
| 6 | Duplicate detection | Upload same document twice to same KB; assert correct handling (reject or new version per current behavior) |
| 7 | Failure from intermediate states | gRPC mock returns error on `ParseAndEmbed`; document transitions to `failed` from the correct intermediate state (e.g., `parsing → failed`). Assert error message is persisted. Also test failure from `crawling` and `embedding` states. |
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
| 5 | Limit clamping | limit=0 defaults to 10; limit=-1 defaults to 10; limit=200 clamps to 100 (clampLimit uses `<= 0`) |

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

Two distinct subsystems are tested here. The Valkey (Redis) layer is already well-covered by unit tests in `internal/cache/cache_test.go` (using miniredis), so integration tests focus on the end-to-end flow and the Postgres `response_cache` table.

**Subsystem A — Valkey exact-match cache (`internal/cache/cache.go`)**

Uses SHA256 of `normalizeQuery(q)` (which applies `strings.ToLower(strings.TrimSpace(q))`). TTL is Redis-native. No hit_count tracking in this layer.

| # | Test Case | Assertion |
|---|-----------|-----------|
| 1 | End-to-end cache miss → store → hit | Full request through the API; first call misses cache and hits gRPC worker, second identical call returns cached response without gRPC call |
| 2 | Normalized matching (leading/trailing whitespace + case) | " What Is RAVEN? " hits cache for "what is raven?" |
| 3 | Internal whitespace NOT normalized | "hello  world" (double space) does NOT hit cache for "hello world" — `normalizeQuery` uses `TrimSpace` only, not `strings.Fields` |
| 4 | Different query → miss | Stored query A, query B → cache miss, gRPC worker called |

**Subsystem B — Postgres `response_cache` table (`internal/repository/semantic_cache.go`)**

Manages the `response_cache` table with `hit_count`, `expires_at`, `query_embedding vector(1536)`, and HNSW index. Exposes `InvalidateKB()` and `Stats()`.

| # | Test Case | Assertion |
|---|-----------|-----------|
| 5 | Hit count increment | Insert a cache entry, increment hit_count 5 times via SQL update. Assert `hit_count = 5` |
| 6 | TTL expiration | Insert entry with `expires_at` set to past. Query with `WHERE expires_at > NOW()` → not returned |
| 7 | KB invalidation | 3 entries for KB-1, 2 for KB-2; call `InvalidateKB` for KB-1 → KB-1 deleted, KB-2 untouched |
| 8 | Cache stats | Populate entries with varying hit counts; call `Stats()` → correct `count` and `avg_hits` |
| 9 | HNSW index exercised | Insert 1,000+ cache entries with distinct embeddings (need sufficient rows to avoid seqscan); `ORDER BY query_embedding <=> $1 LIMIT 1` returns closest vector. Use `SET enable_seqscan = off` in test to force index usage, then verify via `EXPLAIN ANALYZE` |
| 10 | RLS on cache | Org-A's cache entries invisible when querying as Org-B |

### 4. Benchmarks (`benchmark_test.go`)

Using Go's `testing.B` with companion `Test` functions for threshold assertions.

| # | Benchmark | Dataset | Threshold |
|---|-----------|---------|-----------|
| 1 | BM25 search latency | 1,000 chunks / 50 docs | p95 < 100ms |
| 2 | BM25 scale test | 10,000 chunks / 500 docs | Baseline record (no hard threshold yet) |
| 3 | Hybrid search (RRF) latency | 1,000 chunks | p95 < 200ms |
| 4 | SHA256 cache hit | Populated cache, 1,000 iterations | p95 < 5ms |
| 5 | HNSW vector cache scan | 1,000 cache entries | p95 < 50ms |
| 6 | Ingestion throughput | `sample_technical.md` (20 chunks) | Document reaches `ready` with correct chunk count in < 2s |
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
| `pgvector/pgvector:0.8.0-pg18` | Docker image with pgvector extension (pinned version for reproducible CI) |
| `pressly/goose` | Migration runner (already in use) |
| `google.golang.org/grpc` | gRPC mock server (already in use) |
| `miniredis` or testcontainer Redis | Valkey mock for cache subsystem A tests |

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

## Edge Cases (additional tests to include)

| Test | File | Assertion |
|------|------|-----------|
| Empty KB search | `search_test.go` | Search against KB with zero documents/chunks → empty results, no error |
| Concurrent ingestion | `ingestion_test.go` | Two documents uploaded simultaneously to same KB → no race conditions in chunk indexing |
| Unicode content | `search_test.go` | Chunks with CJK, emoji, RTL text → tsvector handles gracefully (may return no BM25 matches, but must not error) |
| Large document | `ingestion_test.go` | Document producing 500+ chunks → batch insert completes, correct chunk count |
| Duplicate embedding vectors | `search_test.go` | Two chunks with identical embeddings → vector search returns both |

## Setup Notes

- **`raven_admin` role**: Created by migration `00002_roles.sql`. `setup_test.go` must verify this role exists after migrations and use it for RLS test #8 (admin bypass).
- **Source type enums**: Migration `00001` defines `source_type AS ENUM ('url', 'sitemap', 'rss_feed')`. Go model uses `web_page`, `web_site`, `sitemap`, `rss_feed`. Later migrations may ALTER the enum. Tests should use the Go model constants and verify they align with the DB.
- **Benchmark baselines**: Stored as CI artifacts rather than committed to repo (avoids merge conflicts across branches). First run establishes the baseline; subsequent runs in the same CI pipeline compare against it.

## Design Decisions

1. **testcontainers-go over Docker Compose** — self-contained, no external orchestration needed, works identically local and CI
2. **Two cache subsystems tested separately** — Valkey SHA256 exact-match cache and Postgres `response_cache` table are distinct implementations with different interfaces. Integration tests for Valkey focus on end-to-end flow (unit tests already cover the Redis layer in `cache_test.go` via miniredis). Integration tests for Postgres cover `hit_count`, `expires_at`, `Stats()`, `InvalidateKB()`, and HNSW index.
3. **Native PostgreSQL tsvector, not ParadeDB** — BM25 ranking uses `ts_rank_cd` with `plainto_tsquery` and GIN indexes. No `pg_bm25` or `pg_search` extension is used. Docker image is `pgvector/pgvector` (not `paradedb`).
4. **Pre-computed embeddings** — avoids calling real embedding APIs in tests, makes assertions deterministic
5. **RRF fusion tested at service layer** — the `fuseRRF` function is pure logic, but we test it through the full `HybridSearch` path to also exercise the repository queries
6. **ClickHouse/QBit path excluded** — `HybridRetrievalService` supports a ClickHouse backend but this suite is PostgreSQL-only. A separate test suite can cover the ClickHouse path when needed.
7. **Image version pinned** — `pgvector/pgvector:0.8.0-pg18` for reproducible CI. Update intentionally, not via `:latest` drift.
