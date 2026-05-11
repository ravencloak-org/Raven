---
title: "AI Worker Design"
---

# AI Worker Design

Raven runs two long-lived backend processes: a Go API server and a Python
service called the **AI worker**. The split lets each language do what it's
best at — Go handles HTTP, auth, multi-tenant request routing, Postgres,
Asynq queues; Python handles document parsing, embedding clients, LLM SDKs,
re-ranking, and the LiveKit voice agent. The seam between them is gRPC.

The worker is published as the Python package `raven-worker` (see
[`ai-worker/pyproject.toml`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/pyproject.toml))
and is started by `python -m raven_worker` — see
[`raven_worker/__main__.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/__main__.py),
which just calls `asyncio.run(serve())`.

## Service boundaries

The worker exposes two network surfaces:

| Surface | Protocol | Default port | Setting | Consumer |
|---------|----------|--------------|---------|----------|
| Public RPC | gRPC (HTTP/2) | `50051` | `RAVEN_GRPC_PORT` | Go API server |
| Internal HTTP | FastAPI on HTTP/1.1 | `8090` | `RAVEN_HTTP_PORT` | Go Asynq workers (same host/pod) |

The gRPC service is the canonical surface; its contract lives in
[`proto/ai_worker.proto`](https://github.com/ravencloak-org/Raven/blob/main/proto/ai_worker.proto)
under the `raven.ai.v1` package. The Go side imports the generated stubs
from `github.com/ravencloak-org/Raven/internal/grpc/pb` (set via the proto's
`go_package` option) and the Python side from `raven_worker.generated`.

The internal HTTP listener binds to `127.0.0.1` by default — see
`http_bind_host` in
[`raven_worker/config.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/config.py).
It exists for one endpoint that doesn't fit the streaming gRPC contract:
the M9 post-session email summariser (`POST /internal/summarize`), called
by the Go-side Asynq worker after a conversation_sessions row is ready.

## RPC surface

The `AIWorker` service in `proto/ai_worker.proto` exposes **three** RPCs:

- **`ParseAndEmbed(ParseRequest) → ParseResponse`** — accepts a document
  blob (bytes + MIME type) and pushes parsed-and-embedded chunks into
  Postgres. The proto defines the contract; the Python servicer currently
  returns `UNIMPLEMENTED` and is being wired into LiteParse.
- **`QueryRAG(RAGRequest) → stream RAGChunk`** — server-streaming RPC.
  Each `RAGChunk` carries either a token of the assistant's reply or, on
  the final message (`is_final = true`), the list of source citations. The
  request carries `org_id`, `kb_ids`, optional `session_id`, free-form
  `filters`, and the model/provider selection.
- **`GetEmbedding(EmbeddingRequest) → EmbeddingResponse`** — one-shot
  embedding for a single text. Used by the Go API for query-time vector
  search and for re-embedding tasks scheduled via Asynq.

The Python entry point is
[`raven_worker/server.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/server.py).
It composes a single `AIWorkerServicer` from `EmbeddingServicer`
(in `raven_worker/services/embedding.py`) and `RAGServicer` (in
`raven_worker/services/rag.py`), registers gRPC reflection plus the
standard health-checking service, wires the OTel interceptor, and blocks
on a signal handler for graceful shutdown.

## Module layout

| Directory / file | Responsibility | Key files |
|------------------|----------------|-----------|
| `raven_worker/server.py` | gRPC bootstrap, health, reflection, signals | `AIWorkerServicer`, `serve()` |
| `raven_worker/__main__.py` | `python -m raven_worker` entry point | `main()` |
| `raven_worker/config.py` | Pydantic settings, `RAVEN_*` env vars | `Settings`, `settings` |
| `raven_worker/services/` | gRPC servicer implementations | `embedding.py`, `rag.py`, `semantic_cache.py` |
| `raven_worker/providers/` | LLM / embedding SDK adapters | `base.py`, `registry.py`, `openai_provider.py`, `anthropic_provider.py`, `cohere_provider.py`, `ollama_provider.py` |
| `raven_worker/processors/` | Document ingestion pipeline | `scraper.py`, `parser.py`, `chunker.py` |
| `raven_worker/retrieval/` | RAG helpers used by `services/rag.py` | `vector_search.py`, `bm25_search.py`, `rrf.py`, `reranker.py`, `cache_repo.py` |
| `raven_worker/memory/` | M9 per-session memory tool | `store.py` (`MemoryStore`, `MEMORY_TOOL`) |
| `raven_worker/http_internal/` | FastAPI app for internal HTTP endpoints | `app.py`, `summarize.py` |
| `raven_worker/agent.py` | Standalone LiveKit voice worker | `_create_worker_options()` |
| `raven_worker/telemetry.py` | OpenTelemetry tracing bootstrap | `init_telemetry()`, `get_grpc_server_interceptor()` |
| `raven_worker/crypto.py` | AES decryption of BYOK keys | `decrypt_api_key()` |
| `raven_worker/generated/` | `grpcio-tools` output from the proto | `ai_worker_pb2.py`, `ai_worker_pb2_grpc.py` |
| `raven_worker/ee/` | Enterprise-only modules — non-OSS licence | `analytics/`, `connectors/` |

## Provider pattern

All embedding providers conform to a `typing.Protocol` defined in
[`raven_worker/providers/base.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/providers/base.py):

```python
class EmbeddingProvider(Protocol):
    async def embed(self, text: str) -> list[float]: ...
    @property
    def dimensions(self) -> int: ...
    @property
    def model_name(self) -> str: ...
```

Concrete adapters live next to it — `openai_provider.py`,
`anthropic_provider.py`, `cohere_provider.py`, and `ollama_provider.py` for
local models. The set of accepted slugs is pinned in
[`raven_worker/providers/registry.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/providers/registry.py):

```python
_SUPPORTED_PROVIDERS = {"openai", "cohere", "anthropic", "ollama"}
```

Selection is **per-request**, not per-deployment. Each `EmbeddingRequest`
and `RAGRequest` carries `provider` and `model` fields, so a single worker
serves multiple tenants with different provider mixes simultaneously. The
registry resolves the request via `get_provider_for_request(org_id,
provider_name, model)`, looks up the row in the `llm_provider_configs`
table, decrypts the BYOK key with `decrypt_api_key()`, and caches the live
client keyed by `(org_id, provider_name, model)`.

The one env var that ties the whole flow together is
`RAVEN_ENCRYPTION_KEY` (a base64-encoded 32-byte AES key — see
`config.py`). Without it, BYOK decryption fails and the worker rejects
provider requests. Local-only providers in `_NO_API_KEY_PROVIDERS`
(currently `ollama`) skip decryption so Raven Local can run without
configuring tenants.

## Document processing pipeline

`raven_worker/processors/` is the ingestion side of the worker, designed
to run inside the future `ParseAndEmbed` implementation:

1. **`scraper.py` — `WebScraper`** uses `httpx` plus `beautifulsoup4`
   to fetch a URL, follow redirects, parse the robots.txt rules, and
   extract title + clean text. It raises `RobotsTxtDisallowedError` when
   crawling is forbidden.
2. **`parser.py` — `DocumentParser`** shells out to the `liteparse` CLI
   (path configurable via `RAVEN_LITEPARSE_PATH`) for PDFs, DOCX, PPTX
   and images, and handles plain text / Markdown / HTML inline. Errors
   surface as `LiteParseError`.
3. **`chunker.py` — `TextChunker`** wraps LangChain's
   `RecursiveCharacterTextSplitter`. Defaults are 512 tokens per chunk
   with 50 token overlap (the `~4 chars per token` heuristic is encoded
   in the file), and the separator list is ordered coarsest-to-finest.
4. **Embedding** — each `Chunk` is passed through the provider returned
   by the registry; the resulting vector is written to the
   `document_chunks` table with the chunk text and metadata.

The Go side decides when to invoke this pipeline (on upload, on URL
ingest, on re-index) and queues the work through Asynq; the worker just
serves the RPC.

## RAG retrieval

`services/rag.py` is the streaming `QueryRAG` implementation. Its module
docstring spells out the eight-step pipeline:

1. Exact-match response cache in Valkey.
2. Embed the user query via the BYOK provider.
3. Run pgvector cosine search (`retrieval/vector_search.py`) and BM25
   full-text search (`retrieval/bm25_search.py`) **in parallel**.
4. Merge the two ranked lists with Reciprocal Rank Fusion
   (`retrieval/rrf.py`).
5. Optionally re-rank the top chunks with the Cohere Rerank API —
   `retrieval/reranker.py` calls `cohere.AsyncClientV2` and returns
   `None` on any failure so the pipeline can degrade gracefully.
6. Build a context string from the surviving chunks.
7. Stream tokens from the configured LLM (Anthropic or OpenAI) back as
   `RAGChunk` proto messages.
8. Write the completed response to the semantic + exact-match cache so
   future identical queries skip steps 2–7.

The semantic cache layer
([`services/semantic_cache.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/services/semantic_cache.py))
sits behind the Valkey exact-match cache and uses pgvector to find
near-identical earlier queries. It's gated by
`RAVEN_SEMANTIC_CACHE_ENABLED` so operators can disable the second layer
without losing the first.

For the dual-index design behind steps 3 and 4, see
[/concepts/hybrid-retrieval](/concepts/hybrid-retrieval).

## Voice agent

`raven_worker/agent.py` is a **separate** long-lived process (not a gRPC
servicer). It uses the `livekit` and `livekit-agents` packages — both
pinned to `>=1.0.0` in `pyproject.toml` — to join LiveKit rooms,
subscribe to audio tracks, and route the transcript through the same RAG
pipeline. STT/TTS plugin selection is tracked in issues #59 and #60
respectively. Configuration comes from `RAVEN_LIVEKIT_URL`,
`RAVEN_LIVEKIT_API_KEY`, and `RAVEN_LIVEKIT_API_SECRET`.

Detailed setup will land at [/guides/voice](/guides/voice) once the STT
and TTS plugins are wired.

## Memory

The M9 cross-channel memory subsystem lives in `raven_worker/memory/`.
It exposes two pieces of public surface:

- **`MemoryStore`** — a per-`(org_id, session_id)` file-backed sandbox
  rooted at `RAVEN_MEMORY_DIR`. Every path operation is sandboxed to
  prevent directory traversal between tenants or sessions.
- **`MEMORY_TOOL`** — the JSON tool definition passed to the Anthropic
  client. Claude calls into it via four commands — `view`, `create`,
  `str_replace`, `delete` — to persist user preferences and topic
  context across sessions, regardless of which channel the user is on.

The store is opt-in: set `RAVEN_MEMORY_DIR` to an empty string (the
default) to disable it entirely. More on the user-facing behaviour at
[/guides/retrieval](/guides/retrieval).

## Post-session summariser

`raven_worker/http_internal/` is a small FastAPI app, separate from the
gRPC server. The router is built in
[`http_internal/app.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/http_internal/app.py)
with `docs_url`, `redoc_url`, and `openapi_url` all set to `None` —
there is no externally consumable schema, only the one endpoint:

- **`POST /internal/summarize`** — accepts a session transcript and
  returns a JSON `{ subject, bullets, body_html, body_text }` recap.
  Implementation in `summarize.py` calls Claude Haiku (default model
  `claude-haiku-4-5`, override via `RAVEN_SUMMARY_MODEL`); the
  transcript is wrapped in `<transcript>…</transcript>` tags as a
  prompt-injection guard and capped at 32 KB. Returns `503` when
  `ANTHROPIC_API_KEY` is not configured rather than silently producing
  a stub summary.

The Go-side Asynq worker fires this RPC once a `conversation_sessions`
row is finalised and turns the response into the outbound email body.

## EE carve-out

`raven_worker/ee/` is **not** covered by the repo's Apache 2.0 OSS
licence. The directory contains `analytics/` and `connectors/`
sub-packages used by the commercial Raven distribution and is governed by
the [Raven Enterprise License](https://github.com/ravencloak-org/Raven/blob/main/ee-LICENSE).
Production use of anything inside `raven_worker/ee/` requires a valid
licence key — see
[`ai-worker/raven_worker/ee/README.md`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/ee/README.md)
for the short version. OSS contributors should treat the directory as
read-only: changes to it ship through a different review path.

## Telemetry

`raven_worker/telemetry.py` provides two functions:

- `init_telemetry(service_name, endpoint)` — installs a
  `TracerProvider` with a `BatchSpanProcessor` exporting OTLP/gRPC to
  the configured endpoint (`RAVEN_OTEL_ENDPOINT`, e.g. `localhost:4317`).
  When `RAVEN_OTEL_ENABLED` is false or the endpoint is unset, this is
  a no-op — the rest of the worker continues with the OTel no-op
  provider and zero overhead.
- `get_grpc_server_interceptor()` — returns the
  `opentelemetry.instrumentation.grpc.aio_server_interceptor`, or
  `None` if the instrumentation package isn't installed, so the server
  setup in `server.py` can skip it without conditional imports.

Only **traces** are wired today. Structured logs are emitted via
`structlog` (console renderer in dev), and metrics emission is on the
roadmap. The Observability page in /guides covers what the Go API and
the AI worker emit end-to-end.

## Testing

Tests live in `ai-worker/tests/`, run with `pytest`, and use
`pytest-asyncio` in `asyncio_mode = "auto"` (see `pyproject.toml`).
A sample of the coverage surface:

- `test_grpc_server.py`, `test_server.py` — server bootstrap and
  reflection
- `test_embedding_servicer.py`, `test_embedding_providers.py`,
  `test_registry.py`, `test_ollama_provider.py` — embedding path
- `test_rag_service.py`, `test_cache.py` — RAG pipeline + cache layers
- `test_chunker.py`, `test_parser.py`, `test_scraper.py` — ingestion
- `test_http_internal_summarize.py` — FastAPI summariser route
- `test_voice_agent.py` — LiveKit agent options
- `test_telemetry.py` — OTel init no-op behaviour
- `tests/ee/`, `tests/processors/`, `tests/retrieval/`, `tests/smoke/` —
  scoped sub-suites including end-to-end smoke checks

Coverage is enforced at `fail_under = 70` via the `[tool.coverage.report]`
section of `pyproject.toml`. Mock HTTP traffic is recorded with `respx`,
declared in the `dev` extras.

## Packaging and deployment

The worker targets Python `>=3.12` and is pinned to a hash-locked
`requirements.txt` regenerated by `uv pip compile`. The multi-stage
[`ai-worker/Dockerfile`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/Dockerfile)
installs from `requirements.txt` with `--require-hashes` so any
mismatched archive is rejected, then copies the in-tree
`raven_worker/` package without re-resolving dependencies. The runtime
image runs as the non-root `raven` user, exposes `50051`, and entry-points
through `dotenvx` so the same image works in both bare-env and
encrypted-env deployments.

## Versioning

The worker's `version = "0.3.0"` in `pyproject.toml` is kept in lockstep
with the main repository's `CHANGELOG.md` — a single tag rolls both Go
and Python binaries forward together. Bumps happen at milestone close;
see [/reference/changelog](/reference/changelog).

## Related reading

- [/concepts/architecture](/concepts/architecture) — where the AI
  worker sits in the wider system diagram
- [/concepts/hybrid-retrieval](/concepts/hybrid-retrieval) — the
  vector + BM25 + RRF flow the worker orchestrates
- [/concepts/deployment-models](/concepts/deployment-models) — when to
  co-locate the worker vs. running it on a dedicated node
- [/concepts/multi-tenancy](/concepts/multi-tenancy) — how `org_id`
  threads through every RPC and the provider cache key
- [/reference/configuration](/reference/configuration) — full
  enumeration of `RAVEN_*` environment variables
- [/guides/voice](/guides/voice) — voice-agent setup with LiveKit
- [/guides/retrieval](/guides/retrieval) — RAG tuning and the memory
  tool
- [/guides/llm-providers](/guides/llm-providers) — provider-specific
  setup notes
