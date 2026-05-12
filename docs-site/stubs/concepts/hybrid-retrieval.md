---
title: "Hybrid Retrieval"
---

# Hybrid Retrieval

Raven combines two complementary retrievers behind a single query: dense
vector similarity (pgvector / HNSW) and sparse lexical scoring (PostgreSQL
`tsvector` + `ts_rank_cd`). Vectors are good at semantic recall — they find
chunks that mean the same thing as the query even when the wording differs.
Lexical ranking is good at precision on rare tokens — proper nouns, error
codes, API symbols, identifiers — that an embedding model often smears
together. Running both and merging the rankings with Reciprocal Rank Fusion
(RRF) reliably beats either retriever alone on heterogeneous knowledge
bases.

## Pipeline

A retrieval request follows a fixed pipeline. The two main entry points are
the Go HTTP service for synchronous hybrid search and the Python
`ai-worker` for the RAG streaming RPC.

1. **Embed the query.** The Python worker calls the tenant's embedding
   provider (`await embedding_provider.embed(query_text)`). For the
   synchronous `HybridSearch` Go endpoint, the embedding is precomputed by
   the caller and passed in. Embeddings are always 1536 dimensions (see
   [Data Model](/concepts/data-model)).

2. **Run vector and BM25 searches.** Both retrievers are launched in
   parallel and constrained to the requesting org's RLS scope. In the Go
   service the two queries run inside one RLS-scoped transaction:

   ```go
   err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
       if len(embedding) > 0 {
           vectorResults, vErr = s.repo.VectorSearch(ctx, tx, kbID, embedding, candidateK)
       }
       if q != "" {
           bm25Results, bErr = s.repo.BM25Search(ctx, tx, kbID, q, candidateK)
       }
       return nil
   })
   ```

   In the Python worker the two queries are awaited concurrently with
   `asyncio.gather` on a single connection:

   ```python
   vector_results, bm25_results = await asyncio.gather(
       vector_search(conn, query_embedding, org_id, kb_ids),
       bm25_search(conn, query_text, org_id, kb_ids),
   )
   ```

3. **Fuse the rankings with RRF.** The two ranked lists are merged into one
   ordered list by Reciprocal Rank Fusion. The Go path computes the fused
   score inline in `fuseRRF`; the Python path uses the shared helper in
   `raven_worker.retrieval.rrf`.

4. **(Optional) Rerank.** On the RAG path the top-K fused chunks may be
   passed to Cohere Rerank when the caller opts in via the
   `filters["rerank"] == "cohere"` flag and the org has a Cohere BYOK key.

5. **Return top-K.** The Go service returns the fused list directly. The
   RAG service uses the fused (or reranked) chunks to build a context
   string, then streams the LLM completion back as `RAGChunk` proto
   messages over `rpc QueryRAG (RAGRequest) returns (stream RAGChunk);`
   (defined in `proto/ai_worker.proto`). The Go API talks to the ai-worker
   via gRPC on `localhost:50051` (see `grpc.worker_addr` in
   `internal/config/config.go`).

The Go orchestrator for the synchronous hybrid endpoint lives in
`internal/service/search.go` as `SearchService.HybridSearch`. The streaming
RAG orchestrator lives in `ai-worker/raven_worker/services/rag.py`.

## Vector search (pgvector + HNSW)

Embeddings are stored in the `embeddings` table, with one row per
`(chunk_id, model_name)` pair. Migration `00010_chunks_and_embeddings.sql`
creates the table and an HNSW index over the cosine operator class:

```sql
CREATE TABLE embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    embedding vector(1536) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    model_version VARCHAR(50),
    dimensions INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chunk_id, model_name)
);

CREATE INDEX idx_embeddings_hnsw
    ON embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
```

The `vector_cosine_ops` operator class fixes the distance function: pgvector
exposes it as the `<=>` operator, which returns cosine *distance* in the
range `[0, 2]`. Raven converts that to cosine *similarity* by computing
`1 - (<=>)` so higher scores mean more similar. The HNSW build parameters
are the pgvector defaults: `m = 16` (max neighbours per layer) and
`ef_construction = 64` (candidate-list size during index build).

The vector leg, defined in `internal/repository/search.go` as
`SearchRepository.VectorSearch`, projects cosine distance back to
similarity:

```sql
SELECT c.id, …, 1 - (e.embedding <=> $2) AS score
FROM embeddings e
JOIN chunks c ON c.id = e.chunk_id
WHERE c.knowledge_base_id = $1
ORDER BY e.embedding <=> $2
LIMIT $3
```

The Python equivalent in `ai-worker/raven_worker/retrieval/vector_search.py`
binds the query embedding as a pgvector literal string:

```python
_VECTOR_SEARCH_SQL = """
SELECT c.id::text, 1 - (e.embedding <=> $1::vector) AS score
FROM chunks c
JOIN embeddings e ON e.chunk_id = c.id
WHERE c.org_id = $2::uuid
  AND c.knowledge_base_id = ANY($3::uuid[])
ORDER BY e.embedding <=> $1::vector
LIMIT $4
"""
```

Both queries rely on Row-Level Security to enforce org isolation: the Go
service sets `app.current_org_id` via `db.WithOrgID`, and the Python worker
does the same with `SELECT set_config('app.current_org_id', $1, false)`
before the query runs.

## Keyword search (PostgreSQL `tsvector` + `ts_rank_cd`)

Raven's "BM25" leg is implemented with vanilla PostgreSQL full-text search:
`to_tsvector('english', …)` for indexing, `plainto_tsquery('english', …)`
for query parsing, and `ts_rank_cd` for relevance scoring. There is no
external BM25 extension (`pg_search`, `paradedb`, etc.) in the stack — the
name "BM25" in the README refers to BM25-*style* lexical ranking via
`ts_rank_cd`, which uses cover-density ranking that approximates BM25's
behaviour for short documents.

The indexes are created across two migrations.
`00010_chunks_and_embeddings.sql` creates the baseline GIN index on
`chunks.content`:

```sql
CREATE INDEX idx_chunks_content_fts
    ON chunks USING gin(to_tsvector('english', content));
```

`00017_bm25_heading_index.sql` adds the heading-only and combined indexes
that the BM25 query actually targets:

```sql
CREATE INDEX idx_chunks_heading_fts
    ON chunks USING gin(to_tsvector('english', heading))
    WHERE heading IS NOT NULL;

CREATE INDEX idx_chunks_content_heading_fts
    ON chunks USING gin(to_tsvector('english',
        coalesce(heading, '') || ' ' || content));
```

The lexical leg, defined in `internal/repository/search.go` as
`SearchRepository.BM25Search`:

```sql
SELECT c.id, …,
       ts_rank_cd(to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content), q) AS rank
FROM chunks c, plainto_tsquery('english', $1) q
WHERE c.knowledge_base_id = $2
  AND to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content) @@ q
ORDER BY rank DESC
LIMIT $3
```

The Python sibling in `ai-worker/raven_worker/retrieval/bm25_search.py` uses
the same `ts_rank_cd` formulation, scoped to a list of knowledge bases via
`c.knowledge_base_id = ANY($3::uuid[])`.

The `@@` operator filters chunks whose composite `tsvector` matches the
parsed query; rows are then scored by `ts_rank_cd`, which weights both term
proximity and term frequency over the document.

## Reciprocal Rank Fusion

Once both retrievers return their candidate lists, Raven merges them with
Reciprocal Rank Fusion. The formula is the canonical one from Cormack et
al. (2009):

```
score(d) = Σ 1 / (k + rank_i(d))
```

where `rank_i(d)` is the 1-based position of document `d` in retriever
`i`'s ranked list, and `k` is a smoothing constant. **Raven defaults `k`
to 60** in both implementations — the value recommended in the original
paper — and exposes it as `RAVEN_RETRIEVAL_RRF_K` for tuning. Per-leg
weights are exposed as `RAVEN_RETRIEVAL_HYBRID_VECTOR_WEIGHT` and
`RAVEN_RETRIEVAL_HYBRID_BM25_WEIGHT` (both default `1.0`).

The Go implementation lives in `internal/service/search.go` as
`fuseRRFWith`, with the default declared as a package constant and the
runtime-effective value flowing in from `config.RetrievalConfig`:

```go
// rrfK is the compile-time default; the runtime value comes from
// cfg.RRFK (RAVEN_RETRIEVAL_RRF_K). k=60 is the standard value from the
// original RRF paper (Cormack et al., 2009).
const rrfK = 60

// fuseRRFWith merges vector and BM25 result lists using weighted RRF.
// For each document appearing in either list:
//   score = vectorWeight * 1/(k + rank_vec) + bm25Weight * 1/(k + rank_bm25)
// where rank_* is the 1-based position in each retriever.
```

The Python implementation in `ai-worker/raven_worker/retrieval/rrf.py` is
the canonical one — both paths produce identical fusion ordering:

```python
def reciprocal_rank_fusion(
    ranked_lists: list[list[tuple[str, float]]],
    k: int = 60,
    top_n: int = 10,
) -> list[tuple[str, float]]:
    rrf_scores: dict[str, float] = {}
    for ranked in ranked_lists:
        for rank_idx, (item_id, _) in enumerate(ranked):
            rank = rank_idx + 1  # 1-based
            rrf_scores[item_id] = rrf_scores.get(item_id, 0.0) + 1.0 / (k + rank)
    return sorted(rrf_scores.items(), key=lambda x: x[1], reverse=True)[:top_n]
```

A few properties to note:

- **Documents appearing in both lists win.** Each list contributes an
  independent `1 / (k + rank)` term, so a chunk that ranks reasonably
  well in *both* retrievers will outrank a chunk that ranks slightly
  higher in only one. The integration test in
  `internal/integration/search_test.go` asserts exactly this:
  > Chunk A should rank highest because it appears in BOTH BM25 and
  > vector results.
- **The original scores are discarded.** RRF uses only the rank position,
  which means it is robust to the very different score distributions of
  pgvector cosine similarity and `ts_rank_cd`.
- **Candidate-set expansion.** Each leg is queried with
  `candidateK = topK * RAVEN_RETRIEVAL_CANDIDATE_MULTIPLIER` (default
  multiplier `3`, capped at `RAVEN_RETRIEVAL_MAX_LIMIT` which defaults
  to `100`) so RRF has enough overlap signal to work with; only the
  top-K survive after fusion.

The fused `HybridSearchResult` carries all the intermediate signals back to
the caller — `VectorScore`, `BM25Score`, `VectorRank`, `BM25Rank`, and the
final `RRFScore` — which is what makes the endpoint debuggable.

## Reranking (optional)

The RAG path in `ai-worker/raven_worker/services/rag.py` may apply a
cross-encoder rerank on top of the RRF-fused list. The provider call lives
in `ai-worker/raven_worker/retrieval/reranker.py`:

```python
_DEFAULT_RERANK_MODEL = "rerank-english-v3.0"

async def cohere_rerank(
    query: str,
    chunks: list[dict],
    api_key: str,
    model: str = _DEFAULT_RERANK_MODEL,
    top_n: int = 5,
) -> list[dict] | None:
    client = cohere.AsyncClientV2(api_key=api_key)
    resp = await client.rerank(
        model=model,
        query=query,
        documents=[c["content"] for c in chunks],
        top_n=top_n,
    )
```

The default model is **Cohere `rerank-english-v3.0`**, called via the
`cohere.AsyncClientV2` SDK. The function is BYOK — it pulls the org's
Cohere API key from `llm_provider_configs` — and degrades gracefully:

- If the `cohere` package is not installed it logs
  `cohere_rerank_unavailable` and returns `None`.
- If the API call raises, it logs `cohere_rerank_failed` and returns
  `None`.

In both fallback cases the caller in `rag.py` keeps the RRF-fused list
unchanged:

```python
use_cohere_rerank = filters.get("rerank") == "cohere"
if use_cohere_rerank and chunk_records:
    cohere_api_key = await self._get_llm_api_key(org_id, "cohere")
    if cohere_api_key:
        reranked = await cohere_rerank(
            query=query_text,
            chunks=chunk_records,
            api_key=cohere_api_key,
            top_n=5,
        )
        if reranked is not None:
            chunk_records = reranked
```

Reranking is therefore **off by default**. It runs only when the gRPC
caller sets `RAGRequest.filters["rerank"] = "cohere"` and the org has a
Cohere BYOK key configured. The synchronous `SearchService.HybridSearch` Go
endpoint never reranks — there is a `TODO` placeholder noting that a future
cross-encoder rerank could be added on top of RRF.

## Configuration

The Go API exposes the hybrid-retrieval tunables as
[`RetrievalConfig`](https://github.com/ravencloak-org/Raven/blob/main/internal/config/config.go)
fields, all overridable at runtime via `RAVEN_RETRIEVAL_*` env vars. The
Python ai-worker mirrors the two values that affect fused ordering so both
code paths can be tuned together.

Go API (`internal/config/config.go` → `RetrievalConfig`, consumed by
`internal/service/search.go`):

| Env var                                  | Default | Purpose                                                                 |
| ---------------------------------------- | ------- | ----------------------------------------------------------------------- |
| `RAVEN_RETRIEVAL_DEFAULT_LIMIT`          | `10`    | `topK` returned when the caller omits `top_k`                           |
| `RAVEN_RETRIEVAL_MAX_LIMIT`              | `100`   | Hard ceiling on `topK` and `candidateK`                                 |
| `RAVEN_RETRIEVAL_RRF_K`                  | `60`    | RRF smoothing constant — score = sum(weight / (k + rank))               |
| `RAVEN_RETRIEVAL_CANDIDATE_MULTIPLIER`   | `3`     | Per-leg candidate set sized as `topK * multiplier` (then clamped)       |
| `RAVEN_RETRIEVAL_HYBRID_VECTOR_WEIGHT`   | `1.0`   | Multiplier on the vector leg's RRF contribution                         |
| `RAVEN_RETRIEVAL_HYBRID_BM25_WEIGHT`     | `1.0`   | Multiplier on the BM25 leg's RRF contribution                           |

`config.Load` validates these at startup: every integer must be positive,
weights must be non-negative, and `MaxLimit >= DefaultLimit`. A
misconfigured value causes the API binary to exit non-zero before serving
the first request. `clampLimitWith()` and `sanitizeQuery()` apply to every
hybrid call.

Python ai-worker (`ai-worker/raven_worker/config.py`), via `RAVEN_*` env
vars:

| Setting                              | Default                | Notes                                                                                            |
| ------------------------------------ | ---------------------- | ------------------------------------------------------------------------------------------------ |
| `grpc_port`                          | `50051`                | RAG and embedding RPCs                                                                           |
| `database_url`                       | local PG dev URL       | pgvector + tsvector queries run against this                                                     |
| `semantic_cache_enabled`             | `true`                 | Enables the `response_cache` lookup (separate from hybrid)                                       |
| `retrieval_rrf_k`                    | `60`                   | Mirrors `RAVEN_RETRIEVAL_RRF_K`; used by the RAG path's `reciprocal_rank_fusion` call            |
| `retrieval_default_top_n`            | `10`                   | Mirrors `RAVEN_RETRIEVAL_DEFAULT_LIMIT`; size of the fused list passed to rerank/context build   |

Rerank toggling is per-request (`filters["rerank"]`), not configuration.
The Python worker reuses the same `RAVEN_RETRIEVAL_RRF_K` env var as the Go
API so a single value tunes both code paths in lock-step.

## Benchmarks

The integration suite includes p95 latency thresholds for the BM25, hybrid,
and HNSW paths. The benchmark code lives in
`internal/integration/benchmark_test.go`:

| Benchmark                       | Target               | Test function                  |
| ------------------------------- | -------------------- | ------------------------------ |
| BM25 search, 1K chunks          | p95 < 100 ms         | `TestBenchmarkBM25Threshold`   |
| Hybrid search, 1K chunks        | p95 < 200 ms         | `TestBenchmarkHybridThreshold` |
| HNSW vector lookup (cache repo) | p95 < 50 ms          | covered by `cache_test.go`     |

Run the search benchmarks against a real Postgres:

```bash
go test -tags=integration -run='TestBenchmark(BM25|Hybrid)Threshold' \
    ./internal/integration/...

# Or, the raw Go benchmarks:
go test -tags=integration -bench='BenchmarkHybridSearch1K|BenchmarkBM25Search10K' \
    ./internal/integration/...
```

The README's numbers — *"BM25 p95 <100ms, hybrid p95 <200ms, HNSW p95
<50ms"* — come directly from these tests.

## Tradeoffs

A few practical notes for tuning hybrid retrieval on your own data.

- **Vector recall vs. lexical precision.** Embeddings can miss
  rarely-seen tokens (UUIDs, error codes, library names) because the
  encoder squashes them. `ts_rank_cd` handles those gracefully. Inversely,
  lexical search misses paraphrases entirely. RRF resolves the conflict
  by rewarding agreement, but you should expect single-leg fallbacks to
  hurt on either side: if `query == ""` only the vector leg runs; if
  `embedding == nil` only the BM25 leg runs.
- **Latency cost of reranking.** A Cohere rerank call adds one external
  HTTP round-trip per query (~100–300 ms typical) and a per-request token
  charge. The fused list is already strong on its own — keep
  `filters["rerank"]` off unless a UX evaluation shows it helps.
- **HNSW parameter conservatism.** `m = 16, ef_construction = 64` are
  pgvector's defaults and keep index build time low, which matters for
  small edge deployments (see [Deployment Models](/concepts/deployment-models)).
  Raising `ef_construction` improves recall at the cost of build time;
  query-time `hnsw.ef_search` (set per-session) trades latency for recall
  on the read side and is not currently overridden by Raven.
- **English-only analyzer.** Both the `to_tsvector` and
  `plainto_tsquery` calls hard-code the `'english'` configuration.
  Non-English content will still index and search, but stop-word removal
  and stemming will not apply correctly. CJK and RTL content is covered
  by integration tests as a smoke test only.

## See also

- [Data Model](/concepts/data-model) — `chunks` and `embeddings` tables,
  the substrate the retrievers run against.
- [AI Worker Design](/concepts/ai-worker-design) — gRPC surface,
  embedding providers, and the RAG streaming pipeline.
- [API Overview](/api/overview) — the public HTTP shape of
  `POST /v1/search/hybrid` and the gRPC `QueryRAG` RPC.
