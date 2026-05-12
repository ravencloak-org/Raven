---
title: "Retrieval"
---

# Retrieval

This page is the operator's how-to for querying a Raven knowledge base. It
covers the public HTTP surface, the request and response shapes, the knobs
you can turn, and how to debug a result list that does not look right.

For the *why* and *how* of the retrieval pipeline itself — pgvector +
PostgreSQL `tsvector` running in parallel, fused with Reciprocal Rank
Fusion, optionally reranked — read
[Concepts → Hybrid Retrieval](/concepts/hybrid-retrieval) first. This
page assumes you already know what those words mean.

Raven exposes retrieval through two endpoints:

| Use case | Endpoint | Pipeline |
|---|---|---|
| Get raw chunks ranked by relevance | `GET …/knowledge-bases/{kb_id}/search` | BM25 only (`ts_rank_cd`) |
| Ask a question, get a streamed LLM answer with citations | `POST /api/v1/chat/{kb_id}/completions` | Embed → vector + BM25 → RRF → (optional rerank) → LLM |

The standalone hybrid search service (`SearchService.HybridSearch` in
`internal/service/search.go`) exists in the Go service but is not yet
wired into a public HTTP route — see `main.go`:
`// TODO: wire into chat handler for enterprise hybrid search`. For now,
hybrid search is reached *through* the chat completions endpoint, which
runs the full RAG pipeline in `ai-worker`.

## Search endpoint

Source: `internal/handler/search.go`, mounted in `cmd/api/main.go`:

```
GET /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/search
```

This is a **GET** with query parameters — not a POST with a JSON body.
It runs PostgreSQL full-text search (`to_tsvector('english', …)` over the
heading and content columns, ranked with `ts_rank_cd`) inside the
requesting org's RLS scope.

```bash
curl -s \
  -H "Authorization: Bearer $RAVEN_TOKEN" \
  --get "https://api.example.com/api/v1/orgs/$ORG/workspaces/$WS/knowledge-bases/$KB/search" \
  --data-urlencode "q=refund window for enterprise customers" \
  --data-urlencode "limit=20"
```

To narrow the search to specific documents, repeat the `doc_ids` parameter:

```bash
curl -s \
  -H "Authorization: Bearer $RAVEN_TOKEN" \
  --get "https://api.example.com/api/v1/orgs/$ORG/workspaces/$WS/knowledge-bases/$KB/search" \
  --data-urlencode "q=refund window" \
  --data-urlencode "doc_ids=01H..." \
  --data-urlencode "doc_ids=01J..."
```

## Request

The search endpoint takes three query parameters
(see `SearchHandler.Search`):

| Field | Type | Default | Notes |
|---|---|---|---|
| `q` | string | required | The user query. Trimmed and whitespace-collapsed by `sanitizeQuery`. |
| `limit` | int | `10` | Top-K. Clamped to `[1, 100]` by `clampLimit`. |
| `doc_ids` | repeated string | — | Restrict to specific document IDs. Maps to `TextSearchWithFilters` in the service. |

The chat-completions endpoint takes a JSON body. From
`internal/model/chat.go`:

```json
{
  "query": "what is the refund window?",
  "session_id": "01J3...",
  "model": "claude-3-7-sonnet-20250219",
  "provider": "anthropic",
  "filters": { "rerank": "cohere" },
  "stream": true
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `query` | string | yes | The user message. |
| `session_id` | string | no | Resume an existing chat session; omit to start a new one. |
| `model` | string | no | Provider-specific model id; falls back to the org's default. |
| `provider` | string | no | `anthropic`, `openai`, etc. Falls back to the org's default. |
| `filters` | map\<string,string\> | no | Free-form bag. Today only `rerank=cohere` is honoured (see [Reranking](#reranking)). |
| `stream` | bool | no | Whether to stream; the handler always returns SSE today. |

Two server-controlled fields are *never* read from the body and are
populated from the request context: `UserID` (from the SuperTokens JWT
`sub` claim) and `ConversationSessionID` (cross-channel memory id).
Sending them in the client body has no effect.

## Response

`/search` returns a JSON `SearchResponse` (`internal/model/search.go`):

```json
{
  "total": 3,
  "results": [
    {
      "id": "01J...",
      "org_id": "01H...",
      "knowledge_base_id": "01H...",
      "document_id": "01J...",
      "source_id": "01J...",
      "content": "Refunds are processed within 14 days...",
      "chunk_index": 7,
      "token_count": 184,
      "page_number": 3,
      "heading": "Refunds & cancellations",
      "chunk_type": "text",
      "created_at": "2026-04-19T11:22:01Z",
      "rank": 0.4187
    }
  ]
}
```

The `rank` is the raw `ts_rank_cd` score — it is *not* normalised and is
only comparable within a single response.

The hybrid pipeline (run from the chat endpoint) builds richer results
internally as `HybridSearchResult`. Each carries both leg scores plus the
fused score, which is what makes the pipeline debuggable end to end:

| Field | Source |
|---|---|
| `vector_score` | `1 - (embedding <=> query)` — cosine similarity in `[0, 1]` |
| `bm25_score` | `ts_rank_cd(...)` |
| `rrf_score` | Σ `1 / (k + rank_i)` across both lists, `k = 60` |
| `vector_rank` | 1-based position in the vector candidate list (or 0 if absent) |
| `bm25_rank` | 1-based position in the BM25 candidate list (or 0 if absent) |

For the chat endpoint, only a flattened subset surfaces in the SSE
`source` events — `document_id`, `document_name`, `chunk_text` (first 500
chars), and `score`. To see the full hybrid result shape, consume the
internal `HybridSearchResponse` directly from the Go service (no public
route — yet).

## Filters

What the codebase supports **today**:

- **By document ID** — `doc_ids` on `/search`. Implemented via
  `SearchRepository.TextSearchWithFilters`.
- **By knowledge base** — path parameter (`{kb_id}`). The hybrid pipeline
  in `ai-worker` accepts multiple KBs via `kb_ids` in the gRPC
  `RAGRequest`, but the public REST endpoint binds a single KB from the
  URL.
- **By rerank model** — `filters.rerank = "cohere"` on chat completions.

What is **planned but not wired**:

- Filter by tag, source type, or `created_at` window. The `chunks` table
  carries `chunk_type`, `source_id`, and `created_at`, but the SQL in
  `internal/repository/search.go` does not yet expose them as filter
  predicates. If you need this today, post-filter the response by
  `source_id` / `chunk_type` client-side.
- Free-form metadata key/value filters. The data model supports it (each
  chunk has a `metadata jsonb` column), but neither retriever pushes a
  predicate through.

## Reranking

The chat endpoint optionally runs a cross-encoder rerank on top of the
RRF-fused candidates. Opt in via the `filters` map:

```json
{ "query": "...", "filters": { "rerank": "cohere" } }
```

The implementation lives in
`ai-worker/raven_worker/retrieval/reranker.py`. Defaults:

- Model: `rerank-english-v3.0`.
- SDK: `cohere.AsyncClientV2`.
- Output size: top 5 chunks (`top_n=5`).

Reranking is **BYOK**. The worker pulls the org's Cohere API key from
`llm_provider_configs`. If no key is configured, or the `cohere` package
is missing, or the API call fails, the worker logs the failure and falls
back to the RRF-fused list — the request never errors because of a
rerank failure.

Reranking is skipped entirely when:

- `filters.rerank` is unset or not `"cohere"`.
- No Cohere key is registered for the org.
- The candidate list is empty.

The synchronous Go `HybridSearch` service never reranks — see the
`TODO: Rerank placeholder` comment at the bottom of
`internal/service/search.go`.

## Chat (RAG over search)

The chat endpoint is the public face of the full RAG pipeline. Source:
`internal/handler/chat.go`, mounted twice in `cmd/api/main.go`:

| Auth | Route | Audience |
|---|---|---|
| Session (SuperTokens) | `POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/completions` | Dashboard users |
| API key | `POST /api/v1/chat/{kb_id}/completions` | Embeddable widgets / 3rd-party apps |

Both call the same handler and return `text/event-stream`. The pipeline
itself runs in the Python worker
(`ai-worker/raven_worker/services/rag.py`) and is the canonical 8-step
flow:

1. Exact-match response cache lookup in Valkey (per-KB).
2. Embed the query with the org's BYOK embedding provider.
3. Vector cosine search + BM25 full-text search in parallel
   (`asyncio.gather`).
4. Merge with Reciprocal Rank Fusion (k = 60).
5. Optional Cohere rerank (see above).
6. Build a numbered context block from the top chunks.
7. Stream LLM tokens back as `RAGChunk` proto messages.
8. Store the completed answer in the response cache.

Example call against the public API-key route, consuming SSE with `curl`:

```bash
curl -N \
  -H "Authorization: Bearer $RAVEN_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{
    "query": "What is our refund window for enterprise customers?",
    "session_id": "01J3PZS...",
    "filters": { "rerank": "cohere" }
  }' \
  "https://api.example.com/api/v1/chat/$KB/completions"
```

The stream emits SSE frames in the form:

```
event: token
data: {"text":"Refunds "}

event: token
data: {"text":"are processed "}

event: sources
data: {"sources":[{"document_id":"01J...","document_name":"Billing Policy","chunk_text":"Refunds are processed within 14 days…","score":0.0312}]}

event: done
data: {}
```

To pull message history for a previous chat session:

```bash
curl -s -H "Authorization: Bearer $RAVEN_API_KEY" \
  "https://api.example.com/api/v1/chat/$KB/sessions/$SESSION_ID/history?limit=50"
```

For cross-channel conversation memory (chat + voice + WebRTC fused into
one history), the dashboard route is
`GET /api/v1/orgs/{org_id}/kbs/{kb_id}/conversations` — see
`internal/handler/conversation.go`.

## Debugging a bad search

There is no `debug: true` flag on the HTTP endpoints today — the
retrievers themselves emit the trace, and the chat handler forwards it
to your structured-logs sink (OpenObserve in the default deployment).

Work down this checklist:

1. **Is BM25 finding the right chunks?** Hit `/search` with the same
   query — that handler is the pure BM25 leg, same SQL as
   `SearchRepository.BM25Search`. If the raw `rank` ranks the wrong
   chunk first, the problem is in ingestion (tokenisation, missing
   heading, wrong source attribution), not retrieval.
2. **Read the structured logs.** The worker emits one line per stage:
   `vector_search_start`/`done`, `bm25_search_start`/`done`,
   `chunks_fetched`, `rerank_applied`, `cache_hit`/`miss`. Each is
   tagged with `org_id`, `kb_ids`, `session_id`, `query_length`.
3. **Compare ranks across legs.** `HybridSearchResult` carries both
   `vector_rank` and `bm25_rank`. A chunk with `vector_rank = 1` and
   `bm25_rank = 0` is matching purely on semantics; the opposite is
   keyword-only. A chunk in neither leg cannot be fused into existence.
4. **Inspect the fused score.** RRF is bounded above by
   `2 / (k + 1) ≈ 0.0328` for a chunk ranked #1 in both legs (with
   `k = 60`). Above ~`0.025` is a strong dual-leg hit; below ~`0.008`
   is mostly single-leg. The test `internal/integration/search_test.go`
   asserts that dual-leg chunks outrank single-leg ones.
5. **Sanity-check the embedding.** If `vector_rank` is consistently 0
   for chunks you expect to match semantically, check `embedding_dims`
   in the `vector_search_start` log — it should be 1536.

## Hard limits

These are baked into the Go service and you cannot override them per
request without a code change. From `internal/service/search.go`:

| Constant | Value | Effect |
|---|---|---|
| `defaultSearchLimit` | `10` | Top-K applied when the caller omits `limit`. |
| `maxSearchLimit` | `100` | Hard cap on `limit`. Larger values are clamped. |
| `rrfK` | `60` | Smoothing constant in the RRF formula — value from the original Cormack et al. (2009) paper. |
| `candidateK` | `topK * 3` | Each leg fetches `topK * 3` candidates, capped at `maxSearchLimit`, so RRF sees enough overlap. |

The rerank `top_n` (in `services/rag.py`) is hardcoded to `5`. The
response cache TTL (`_CACHE_TTL_SECONDS`) is `3600` (1 hour, exact-match
per-KB).

## Tuning

The constants above are deliberately *not* runtime knobs. RRF is
parameterless by design, and `candidateK` was picked to give RRF enough
overlap signal without paying query latency no one benefits from.

In practice, "tuning retrieval" on Raven means tuning what *is* exposed:

- **Chunk size and overlap.** Bad chunks dominate every other knob.
  Look at chunk boundaries first — see
  [Guides → Ingestion](/guides/ingestion).
- **Source quality.** A keyword in 80% of your chunks contributes
  almost no BM25 signal. Tighten the source set or split generic
  documents into smaller, topic-scoped sources.
- **Headings.** BM25 searches
  `coalesce(heading, '') || ' ' || content`. Descriptive headings rank
  dramatically better — make sure ingestion preserves them.
- **Embedding provider.** Switching to a domain-specific model moves
  the needle more than any RRF parameter could. See
  [Guides → LLM providers](/guides/llm-providers).
- **Reranking.** For end-user chat, set `filters.rerank = "cohere"`.
  The latency cost (~100–400 ms) is usually worth the top-5 precision.

If you find yourself wanting to tune `rrfK` or `candidateK`, open an
issue first — the real fix is usually upstream.

## See also

- [Concepts → Hybrid Retrieval](/concepts/hybrid-retrieval) — the
  algorithm, the SQL, the benchmarks.
- [Guides → Ingestion](/guides/ingestion) — chunking, sources, what
  makes retrieval work or not work.
- [API Overview](/api/overview) — the canonical OpenAPI surface.
