---
title: "Ingestion"
---

# Ingestion

Ingestion is the path that turns raw input — uploaded files or scraped web
pages — into chunks and embeddings that the retrieval pipeline can search.
It is multi-stage, asynchronous, and tenant-isolated end to end:

```
client → POST /documents (or /sources) → documents/sources row (queued)
       → Asynq enqueue (Valkey)
       → ai-worker: parse → chunk → embed
       → chunks + embeddings rows (status=ready)
```

Every stage writes status back to the `documents.processing_status` (or
`sources.processing_status`) column, so the UI and operators can observe
progress without polling internal queues.

## Source types

Raven ingests two source families today. The third (connectors) is wired
through the Asynq `airbyte:sync` task type but is not exposed via a
first-class REST endpoint yet.

| Type | Where the data comes from | Parser |
|------|---------------------------|--------|
| File upload (PDF) | `multipart/form-data` to `/documents/upload` → SeaweedFS | `DocumentParser` shelling out to the LiteParse CLI (`ai-worker/raven_worker/processors/parser.py`) |
| File upload (DOCX, PPTX) | Same multipart endpoint | LiteParse CLI subprocess |
| File upload (images: PNG, JPEG, TIFF, WebP) | Same multipart endpoint | LiteParse CLI with OCR |
| File upload (Markdown, HTML, plain text) | Same multipart endpoint | Handled directly in `DocumentParser` (no subprocess) |
| Web page | `POST /sources` with a URL | `WebScraper` (httpx + BeautifulSoup) in `ai-worker/raven_worker/processors/scraper.py` |
| Sitemap / RSS / website crawl | `POST /sources` with `source_type` of `sitemap`, `rss_feed`, or `web_site` (see `internal/model/source.go`) | `WebScraper`, optionally recursive up to `crawl_depth` |
| Airbyte connector sync | Internal — `queue.TypeAirbyteSync` (`airbyte:sync`) in `internal/queue/tasks.go` | Connector-specific; not yet exposed in the public API |

The allowlist that the upload handler enforces is controlled by
`RAVEN_UPLOAD_ALLOWED_TYPES` and defaults to PDF, DOCX, PPTX, HTML, MD, TXT,
and CSV. See [Configuration](/reference/configuration) for the full env-var
matrix.

## Pipeline overview

The ingestion pipeline is intentionally split between the Go API and the
Python AI worker. The Go side owns the database transaction; the Python side
owns the heavy CPU/GPU work.

1. **API receives the source** — the upload handler
   ([`internal/handler/upload.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/handler/upload.go))
   validates the MIME type, enforces the 50 MB default upload cap, and
   streams the file to SeaweedFS via the storage client
   (`internal/storage/seaweedfs.go`). For URL sources, the source handler
   ([`internal/handler/source.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/handler/source.go))
   validates the URL and persists a `sources` row.
2. **Document record created** — a `documents` row (see
   `migrations/00008_documents.sql`) is inserted with
   `processing_status = 'queued'`, the SeaweedFS `storage_path` (fid), the
   `file_hash`, and the uploader's user ID. Row-level security
   (`tenant_isolation` policy) confines all reads/writes to the caller's
   `org_id`.
3. **Asynq job enqueued** — the API calls
   `queue.Client.EnqueueDocumentProcess` (or `EnqueueURLScrape`) with the
   `org_id` / `document_id` / `knowledge_base_id` payload. The task type is
   `document:process` or `url:scrape` (see
   [`internal/queue/tasks.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/queue/tasks.go)).
   Tasks land on the `default` queue in Valkey with `MaxRetry=5`.
4. **Worker picks up** — the Go worker process
   (`internal/jobs/document_process.go`) drains the Asynq queue, fetches the
   document by ID, downloads the bytes from SeaweedFS, and updates the
   document status to `parsing`.
5. **Parser extracts text** — the Go worker invokes the AI worker over gRPC
   to parse the bytes. `DocumentParser` routes PDFs, Office files, and
   images through the LiteParse CLI subprocess and reads plain text /
   Markdown / HTML directly.
6. **Chunker splits** — once text is in hand, the worker advances status to
   `chunking` and runs the chunker (see below).
7. **Embedder writes vectors** — for each chunk the worker calls the AI
   worker's `GetEmbedding` RPC
   (`ai-worker/raven_worker/services/embedding.py`), then writes one row to
   `chunks` and one row to `embeddings` per chunk. Status flips to
   `embedding`, then `ready` on completion.
8. **On failure** — any error sets `processing_status = 'failed'` and writes
   the message to `processing_error`. Asynq keeps the task in its retry
   queue up to `MaxRetry`; after that, the task is archived.

The seed service
([`internal/service/seed.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/service/seed.go))
walks exactly this path: it renders a TMDB movie to Markdown, uploads via
`storage.Client.Upload`, inserts a `documents` row, and calls
`queue.Client.EnqueueDocumentProcess`. Reading `processMovie` is the
shortest way to internalise the end-to-end flow.

## Upload a file

The upload endpoint accepts a single file per request as
`multipart/form-data` with the field name `file`:

```bash
curl -X POST \
  "https://api.example.com/api/v1/orgs/${ORG_ID}/workspaces/${WS_ID}/knowledge-bases/${KB_ID}/documents/upload" \
  -H "Authorization: Bearer ${TOKEN}" \
  -F "file=@./onboarding.pdf"
```

Successful uploads return `201 Created` with `model.UploadDocumentResponse`:

```json
{
  "id": "8e7f...",
  "org_id": "...",
  "knowledge_base_id": "...",
  "file_name": "onboarding.pdf",
  "file_type": "application/pdf",
  "file_size_bytes": 1453221,
  "file_hash": "a1b2c3...",
  "storage_path": "3,01a7b9...",
  "processing_status": "queued",
  "uploaded_by": "...",
  "created_at": "2026-05-12T09:14:22Z"
}
```

A few operator-relevant details:

- The handler runs under a 60-second deadline (`middleware.Deadline(60*time.Second)`
  in `cmd/api/main.go`).
- The default size cap is 50 MB (`RAVEN_UPLOAD_MAX_SIZE_BYTES=52428800`).
- Bytes go to **SeaweedFS**, not the host filesystem. The returned
  `storage_path` is the SeaweedFS fid; downstream jobs use it to re-download
  the blob.
- `file_hash` (SHA-256 of the upload) is stored on the row and indexed —
  see [Reingest / update](#reingest-update) below for how it's used.

## Add a web source

Use the source endpoint for anything reachable by URL:

```bash
curl -X POST \
  "https://api.example.com/api/v1/orgs/${ORG_ID}/workspaces/${WS_ID}/knowledge-bases/${KB_ID}/sources" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
        "source_type": "web_page",
        "url": "https://example.com/docs",
        "crawl_depth": 1,
        "crawl_frequency": "manual"
      }'
```

`source_type` is one of `web_page`, `web_site`, `sitemap`, or `rss_feed`
(see `internal/model/source.go`). `crawl_frequency` can be `manual`,
`daily`, `weekly`, or `monthly`; daily/weekly/monthly sources are re-crawled
by the scheduler.

The scraper enforces robots.txt by default. From
`ai-worker/raven_worker/processors/scraper.py`:

```python
class WebScraper:
    """Scrape web pages using httpx + BeautifulSoup4.

    Features:
    - Follows redirects (httpx default behaviour)
    - Respects robots.txt
    - Extracts clean text from HTML
    - Captures page title and meta description
    """
```

It identifies itself as `RavenBot/1.0 (+https://github.com/AumniPrime/raven)`.
If `robots.txt` disallows the path, the scraper raises
`RobotsTxtDisallowedError` and the source moves to `failed` — there is no
override flag, by design. Self-hosters who want to bypass robots.txt on
their own intranet can construct a `WebScraper` with `respect_robots=False`,
but the API never exposes that knob.

## Chunking strategy

Chunking lives in
[`ai-worker/raven_worker/processors/chunker.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/processors/chunker.py).
It wraps `langchain_text_splitters.RecursiveCharacterTextSplitter` (declared
as `langchain-text-splitters>=0.3.0` in `ai-worker/pyproject.toml`).

The defaults are intentionally conservative for RAG workloads:

```python
_CHARS_PER_TOKEN = 4
DEFAULT_CHUNK_SIZE_TOKENS = 512
DEFAULT_CHUNK_OVERLAP_TOKENS = 50
DEFAULT_CHUNK_SIZE = DEFAULT_CHUNK_SIZE_TOKENS * _CHARS_PER_TOKEN  # 2048
DEFAULT_CHUNK_OVERLAP = DEFAULT_CHUNK_OVERLAP_TOKENS * _CHARS_PER_TOKEN  # 200
```

So: **~512 tokens per chunk, ~50 tokens overlap**, measured in characters
(`length_function` is `len`, not a tokenizer). Headings are not used as
hard boundaries, but the splitter is given a separator ladder that prefers
paragraph breaks over arbitrary character splits:

```python
_DEFAULT_SEPARATORS = [
    "\n\n",  # paragraph
    "\n",    # line
    ". ",    # sentence
    ", ",    # clause
    " ",     # word
    "",      # character
]
```

`TextChunker.chunk_with_metadata` additionally attaches a `chunk_index`, the
source identifier, and a `heading` field when the first line of a chunk
looks like a heading, all of which land in `chunks.metadata` (JSONB) and on
`chunks.heading`. The chunk record itself lives in
`migrations/00010_chunks_and_embeddings.sql`:

```sql
CREATE TABLE chunks (
    ...
    content TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    token_count INTEGER,
    page_number INTEGER,
    heading VARCHAR(500),
    chunk_type chunk_type NOT NULL DEFAULT 'text',
    ...
);
CREATE INDEX idx_chunks_content_fts ON chunks USING gin(to_tsvector('english', content));
```

The GIN index over `to_tsvector('english', content)` is what backs the BM25
arm of [hybrid retrieval](/concepts/hybrid-retrieval). Chunks are emitted
even before embeddings exist, which means lexical search starts working as
soon as the chunker finishes — useful when an embedding provider is slow or
rate-limited.

## Embedding

Embeddings are bring-your-own-key. The Go side never makes the upstream
provider call; it sends each chunk to the AI worker over gRPC, and the
worker looks up the org's configured provider before calling out:

```python
class EmbeddingServicer:
    async def get_embedding(self, request, context):
        provider = await get_provider_for_request(
            request.org_id,
            request.provider,
            request.model,
        )
        embedding = await provider.embed(request.text)
        return ai_worker_pb2.EmbeddingResponse(
            embedding=embedding,
            dimensions=len(embedding),
        )
```

Provider clients live under `ai-worker/raven_worker/providers/` and cover
OpenAI, Cohere, and Anthropic (see the `openai`, `cohere`, and `anthropic`
dependencies in `ai-worker/pyproject.toml`). The default deployment uses
OpenAI `text-embedding-3-small`, which produces **1536-dimensional**
vectors — the same dimension the `embeddings.embedding` column is declared
as:

```sql
CREATE TABLE embeddings (
    ...
    embedding vector(1536) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    model_version VARCHAR(50),
    dimensions INTEGER NOT NULL,
    UNIQUE(chunk_id, model_name)
);
CREATE INDEX idx_embeddings_hnsw ON embeddings USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
```

A single chunk can carry multiple embeddings, one per `model_name`, but
each (chunk, model) pair is unique. The HNSW index is built with
`m = 16, ef_construction = 64` and uses cosine distance. If you swap to a
provider with a different vector dimension, you need a new migration that
drops and re-creates the column — Postgres rejects vectors of the wrong
size at insert time. See [Configuration](/reference/configuration) for how
to set the per-org provider and model.

## Reingest / update

There is no `reingest` endpoint today. The behaviour depends on how the
content arrives:

- **Re-uploading the same file** — a fresh row is inserted; the `file_hash`
  column makes duplicates discoverable, but the upload service does not
  silently deduplicate. If you want to replace a document, `DELETE` the old
  row (cascades to chunks and embeddings via the FK) then upload the new
  copy.
- **Editing document metadata** — `PUT /documents/{doc_id}` accepts
  `UpdateDocumentRequest` (title, metadata only). It does **not** trigger
  re-processing.
- **Recurring URL crawls** — sources with `crawl_frequency` set to
  `daily`, `weekly`, or `monthly` are re-scraped by the scheduler; each
  crawl writes fresh chunks and embeddings under the same `source_id`.
- **Knowledge-base wide reindex** — for full recomputation of embeddings
  (e.g. after changing the embedding model), use the
  `queue.TypeReindex` task (`kb:reindex`). It is enqueued via the
  `EnqueueReindex` client method on the low-priority queue. There is no
  public REST endpoint for this yet; trigger it from an admin tool.

The `processing_status` enum has a dedicated `reprocessing` state for the
in-flight case, distinct from the initial `queued` → `ready` arc.

## Bulk ingest

There is no single bulk-import endpoint. The supported patterns are:

- **Many files, scripted** — loop the upload endpoint client-side, one file
  per request. Honour the org's general rate limit
  (`RAVEN_RATELIMIT_DEFAULT_USER_LIMIT`, defaults to 1000 RPM) and the tier
  limits (`RAVEN_RATELIMIT_FREE_GENERAL_RPM=60`,
  `RAVEN_RATELIMIT_PRO_GENERAL_RPM=600`,
  `RAVEN_RATELIMIT_ENTERPRISE_GENERAL_RPM=6000`). The 60 s handler deadline
  is per-request, not per-batch.
- **Many URLs** — submit one `POST /sources` per URL. For a whole site,
  prefer `source_type = "web_site"` or `"sitemap"` with an appropriate
  `crawl_depth`; the scraper handles the fan-out inside one Asynq job.
- **Seed-style bootstrap** — the seed service is the reference for
  programmatic ingest. Self-hosters can adapt the pattern in
  `internal/service/seed.go` (render → `storage.Upload` → `docRepo.Create`
  → `qCli.EnqueueDocumentProcess`) to import data already in a tabular
  form.

Concurrency at the queue layer is governed by the AI worker's
`grpc_max_workers` (defaults to `10`, configurable via
`RAVEN_GRPC_MAX_WORKERS`). Scaling ingestion throughput beyond that is a
matter of running more AI-worker replicas — they share the Valkey-backed
queue.

## Failures

When something goes wrong, Asynq re-queues the task automatically. Each
task is enqueued with `asynq.MaxRetry(c.maxRetry)` (default `5`, see
`internal/queue/client.go`), so transient parser failures, scraper
timeouts, and embedding-provider 429s usually recover on their own.

To investigate stuck or failed ingests:

1. **Check the document row** — `processing_status` will be `failed` and
   `processing_error` will hold the last error message.
2. **Read the worker logs** — both the Go worker
   (`internal/jobs/document_process.go`) and the AI worker emit structured
   `structlog` events. Filter on `document_id` and `org_id`. Logs are
   shipped to OpenObserve in self-hosted deployments (see
   [Observability](/guides/self-hosting/observability)).
3. **Inspect Asynq directly** — Raven does not ship Asynqmon in the default
   Compose. For deep queue debugging, run `asynqmon` against the same
   Valkey instance:

   ```bash
   docker run --rm -p 8080:8080 \
     hibiken/asynqmon --redis-addr=valkey:6379
   ```

   then browse to `http://localhost:8080`. Use the `default` queue for
   document processing and URL scraping; `low` for KB reindex tasks.

After `MaxRetry` failures the task lands in Asynq's archived set. From
there you can replay it via Asynqmon's "Run" button after fixing the
underlying issue (parser misconfiguration, provider credentials, etc.).

## Storage costs

Raven splits ingested content across three stores. Rough per-document
sizing for an average 10 MB PDF:

| Where it lives | What it holds | Approximate size |
|----------------|---------------|------------------|
| SeaweedFS | The original file bytes | 10 MB (= the upload size; no compression by default) |
| Postgres `chunks` table | UTF-8 text + small metadata JSONB | ~1.5 × the extracted text; for a 10 MB PDF that's typically 200–500 KB of chunk text |
| Postgres `embeddings` table (pgvector) | One `vector(1536)` per chunk plus an HNSW entry | ~6 KB per chunk for the vector itself (1536 × 4 bytes), plus index overhead. A 200-chunk document is ~1.2 MB on disk before indexes. |

The HNSW index (`m = 16, ef_construction = 64`) materially inflates the
embeddings table — budget roughly 2–3× the raw vector size for the index.
For typical knowledge bases, **embeddings dominate the Postgres footprint
within a couple of months**; SeaweedFS dominates the overall storage
budget. Plan capacity accordingly, and run `VACUUM (ANALYZE)` after large
reingests so the planner picks the right index.

## See also

- [Hybrid retrieval](/concepts/hybrid-retrieval) — how chunks and
  embeddings are searched together.
- [Retrieval](/guides/retrieval) — the read side of the pipeline.
- [Configuration](/reference/configuration) — the env vars referenced
  throughout this guide (upload limits, rate limits, embedding provider,
  AI-worker tuning).
