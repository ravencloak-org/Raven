---
title: "First Knowledge Base"
---

# Your First Knowledge Base

This tutorial picks up where the [Quickstart](/get-started/installation) leaves
off. By the end of this page you will have:

- A workspace under your organisation.
- A knowledge base with at least one ingested document.
- A working full-text search call against that knowledge base.
- A streaming chat reply grounded in your document.

## Prerequisites

You should already be at the point where:

- Raven is running locally (`docker compose up -d` from the Quickstart succeeded).
- You have signed in through Keycloak and Raven created your user.
- The **Onboarding Wizard** has finished. The wizard auto-creates your first
  organisation and, by default, a workspace named **Default**. You can use
  those, or follow along below to create a fresh workspace.

All API examples below assume:

```bash
export RAVEN_API=http://localhost:8080/api/v1
export ORG_ID="<your-org-uuid>"     # visible in the URL of the dashboard
export WS_ID="<your-workspace-uuid>"
export KB_ID="<filled-in-after-step-3>"
export TOKEN="<your-keycloak-access-token>"   # see Authentication below
```

> **Authentication.** Dashboard sessions use cookies — the browser is already
> signed in. For curl, paste a Keycloak access token (`Authorization: Bearer
> $TOKEN`) or call the `/api/v1/me` endpoint from the browser's DevTools to
> grab the session cookie and pass it with `--cookie`. The embeddable chat
> widget uses long-lived API keys instead; see
> [Embed the Chat Widget](/get-started/embed-the-chat-widget).

## Concepts in 60 seconds

Raven groups data in a strict hierarchy. Every row in the database carries an
`org_id`, and Postgres Row-Level Security blocks any query that crosses orgs.

| Concept           | What it is                                                                                  |
|-------------------|---------------------------------------------------------------------------------------------|
| **Organisation**  | The top-level tenant boundary. One billing account, one Keycloak realm worth of users.      |
| **Workspace**     | A team within the organisation. Holds knowledge bases, members, and notification settings.  |
| **Knowledge Base**| A scoped document store. The unit you point a chatbot, API key, or voice agent at.          |
| **Source**        | A URL (web page, sitemap, RSS feed) registered with a KB; the crawler fetches and re-fetches it. |
| **Document**      | A file or crawled page inside a KB. Goes through a processing pipeline before it is queryable. |
| **Chunk**         | A passage of a document — text plus an optional heading and page number. The retrieval unit. |
| **Embedding**     | A vector representation of a chunk. Powers semantic similarity in hybrid search.            |

For the deeper picture, read [Data Model](/concepts/data-model) and
[Multi-Tenancy](/concepts/multi-tenancy).

## Create a workspace

### Dashboard

1. From the dashboard sidebar choose **Workspaces → New Workspace**.
2. Enter a name (2–255 characters) and submit.
3. You land on the new workspace's detail page; copy its UUID from the URL.

### API

The dashboard call hits:

```http
POST /api/v1/orgs/{org_id}/workspaces
Content-Type: application/json
```

Body (validated by `model.CreateWorkspaceRequest` — `name` is required,
2–255 characters):

```json
{ "name": "My Team" }
```

Curl:

```bash
curl -s -X POST "$RAVEN_API/orgs/$ORG_ID/workspaces" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My Team"}'
```

A `201 Created` response returns the full `Workspace` object (`id`, `org_id`,
`name`, `slug`, `settings`, timestamps). Save the `id` as `$WS_ID`.

> **Onboarding shortcut.** The first workspace under a new org does not
> require any org role — that is why the Onboarding Wizard is allowed to
> create it. Subsequent workspaces are unrestricted at the API; member
> management on a workspace requires `admin`.

## Create a knowledge base

### Dashboard

1. Inside your workspace, choose **Knowledge Bases**.
2. Type a name in the inline form (the `KBListPage` calls
   `useKnowledgeBasesStore.create` under the hood).
3. You are redirected to `/orgs/<org-id>/workspaces/<ws-id>/knowledge-bases/<kb-id>` —
   copy the UUID from the URL.

### API

```http
POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases
Content-Type: application/json
```

Body (`model.CreateKBRequest` — `name` required, `description` optional):

```json
{
  "name": "Product Docs",
  "description": "Customer-facing product documentation"
}
```

Curl:

```bash
curl -s -X POST "$RAVEN_API/orgs/$ORG_ID/workspaces/$WS_ID/knowledge-bases" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Product Docs","description":"Customer-facing product documentation"}'
```

A `201 Created` response returns the full `KnowledgeBase`:

```json
{
  "id": "5f5a...",
  "org_id": "1a2b...",
  "workspace_id": "9c8d...",
  "name": "Product Docs",
  "slug": "product-docs",
  "description": "Customer-facing product documentation",
  "settings": {},
  "status": "active",
  "cache_enabled": false,
  "cache_similarity_threshold": 0.90,
  "created_at": "2026-05-08T10:00:00Z",
  "updated_at": "2026-05-08T10:00:00Z"
}
```

Save `id` as `$KB_ID`. (Like workspaces, the first KB under a workspace skips
the role check — onboarding-friendly.)

## Ingest your first document

You have three ways to put content into a KB. Each one ends in the same place:
a row in the `documents` table with `processing_status = "queued"` and a job
on Asynq for the AI worker to pick up.

### Option 1 — Upload a file from the dashboard

1. Open the KB detail page.
2. Drag and drop a PDF, Markdown, or text file onto the drop zone, or click
   **Upload**.
3. The page polls and updates the status badge from `queued` to `ready`
   (usually a few seconds for a short document).

The frontend calls the same multipart endpoint as the curl example below.

### Option 2 — POST a file to the documents endpoint

```http
POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/documents/upload
Content-Type: multipart/form-data
```

The handler (`internal/handler/upload.go`) accepts a single `file` form field,
validates the MIME type and size, hashes the bytes for deduplication, writes
the blob to SeaweedFS, creates a `Document` row, and enqueues a processing
job. The leaf has a 60-second deadline so large PDFs do not get cut off by
the global API budget.

Curl:

```bash
curl -s -X POST \
  "$RAVEN_API/orgs/$ORG_ID/workspaces/$WS_ID/knowledge-bases/$KB_ID/documents/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./getting-started.md"
```

`201 Created` returns `model.UploadDocumentResponse`:

```json
{
  "id": "f3e2...",
  "org_id": "1a2b...",
  "knowledge_base_id": "5f5a...",
  "file_name": "getting-started.md",
  "file_type": "text/markdown",
  "file_size_bytes": 4321,
  "file_hash": "sha256:...",
  "storage_path": "3,01a47a1234",
  "processing_status": "queued",
  "uploaded_by": "user-uuid",
  "created_at": "2026-05-08T10:01:00Z"
}
```

Save the `id` if you want to poll a specific document.

### Option 3 — Add a web source (URL)

For pages you do not want to download locally, register them as a **source**.
Raven's crawler fetches the URL, parses it, and creates one or more documents.

```http
POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/sources
Content-Type: application/json
```

Body (`model.CreateSourceRequest`):

```json
{
  "source_type": "web_page",
  "url": "https://example.com/docs/intro",
  "crawl_frequency": "manual"
}
```

`source_type` accepts `web_page`, `web_site`, `sitemap`, or `rss_feed`.
`crawl_frequency` accepts `manual`, `daily`, `weekly`, or `monthly`.

Curl:

```bash
curl -s -X POST \
  "$RAVEN_API/orgs/$ORG_ID/workspaces/$WS_ID/knowledge-bases/$KB_ID/sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
        "source_type":"web_page",
        "url":"https://example.com/docs/intro",
        "crawl_frequency":"manual"
      }'
```

The response includes a `processing_status` field that walks the source from
`queued` to `ready` the same way a file upload does. Creating a source
requires the workspace `member` role.

## Watch the pipeline

A document moves through a state machine defined in
`internal/model/document.go`:

| `processing_status` | Meaning                                                              |
|---------------------|----------------------------------------------------------------------|
| `queued`            | Job is on the Asynq queue, waiting for the AI worker.                |
| `crawling`          | (Source-only) the crawler is fetching the URL.                       |
| `parsing`           | Extracting text from the file format (PDF, HTML, Markdown, …).       |
| `chunking`          | Splitting into passages with headings and page anchors.              |
| `embedding`         | Calling the embedding model for each chunk.                          |
| `ready`             | Chunks and vectors are written; the document is queryable.           |
| `failed`            | Pipeline error; see the `processing_error` field for the cause.      |
| `reprocessing`      | A re-ingestion is in flight (for example after a settings change).   |

Poll the document list endpoint to watch progress:

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "$RAVEN_API/orgs/$ORG_ID/workspaces/$WS_ID/knowledge-bases/$KB_ID/documents" \
  | jq '.documents[] | {file_name, processing_status, processing_error}'
```

Wait until your document shows `"processing_status": "ready"` before searching.

## Run your first search

Search uses Postgres `tsvector` with BM25-style ranking, scoped to a single
KB. It is exposed as a `GET` (the search handler reads the query from `?q=`),
not a `POST` — see `internal/handler/search.go`.

```http
GET /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/search?q=...&limit=...
```

Curl:

```bash
curl -s -G -H "Authorization: Bearer $TOKEN" \
  "$RAVEN_API/orgs/$ORG_ID/workspaces/$WS_ID/knowledge-bases/$KB_ID/search" \
  --data-urlencode "q=how do i get started" \
  --data-urlencode "limit=5" \
  | jq
```

Sample `model.SearchResponse`:

```json
{
  "results": [
    {
      "id": "c1...",
      "org_id": "1a2b...",
      "knowledge_base_id": "5f5a...",
      "document_id": "f3e2...",
      "content": "To get started, install Raven with `docker compose up -d`…",
      "chunk_index": 0,
      "heading": "Quickstart",
      "chunk_type": "text",
      "rank": 0.812,
      "highlight": "To <mark>get started</mark>, install Raven…",
      "created_at": "2026-05-08T10:02:00Z"
    }
  ],
  "total": 1
}
```

The `rank` is the BM25 score; `highlight` is a snippet wrapped in `<mark>`
tags. Higher `rank` means more relevant. Use the `doc_ids` query parameter
(repeatable) to filter to a subset of documents within the KB.

> **Hybrid search.** The dashboard chat path also uses dense vector
> similarity. The hybrid retriever lives inside the AI worker and is invoked
> by the chat completion endpoint below. See
> [Hybrid Retrieval](/concepts/hybrid-retrieval) for the full design.

## Run your first chat

Chat completions stream tokens via Server-Sent Events. The session-authenticated
endpoint (used by the dashboard) is:

```http
POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/completions
Accept: text/event-stream
Content-Type: application/json
```

Body (`model.ChatCompletionRequest` — `session_id` is optional; omit it on
the first turn and Raven creates one, then echoes the `session_id` back so
you can continue the conversation):

```json
{
  "messages": [
    { "role": "user", "content": "How do I get started?" }
  ]
}
```

Curl (`-N` disables buffering so you see tokens as they stream):

```bash
curl -N -X POST \
  "$RAVEN_API/orgs/$ORG_ID/workspaces/$WS_ID/knowledge-bases/$KB_ID/completions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"How do I get started?"}]}'
```

You will see SSE frames roughly like:

```text
event: session
data: {"session_id":"sess_01H...","message_id":"msg_01H..."}

event: token
data: {"text":"To "}

event: token
data: {"text":"get started, "}

event: sources
data: {"sources":[{"document_id":"f3e2...","heading":"Quickstart","rank":0.81}]}

event: done
data: {"finish_reason":"stop"}
```

The streaming budget is capped at 30 seconds at the route level. Past
conversations can be replayed from
`GET /api/v1/orgs/{org_id}/kbs/{kb_id}/conversations` (cross-channel session
history — see `internal/handler/conversation.go`).

## What to try next

- **Tune ingestion** — chunk size, parser overrides, deduplication: see
  [Ingestion](/guides/ingestion).
- **Tune retrieval** — top-K, similarity thresholds, filtering by document or
  metadata: see [Retrieval](/guides/retrieval).
- **Ship it to a website** — issue an API key and drop the widget on any page:
  [Embed the Chat Widget](/get-started/embed-the-chat-widget).
- **Compare your KB to the canonical example** — `internal/service/seed.go`
  builds the `Raven Demo` org → `Movies` workspace → `Movie Database` KB and
  ingests TMDB movie pages. It is the reference for what a populated KB looks
  like at every layer (org, workspace, KB, document, chunk, embedding).

## Quick checklist

- [ ] Workspace created.
- [ ] Knowledge base created.
- [ ] At least one document uploaded (or one source crawled) and `processing_status = "ready"`.
- [ ] Search returns at least one result for a query you know is in the document.
- [ ] Chat returns a streaming reply that cites your document.

When all five boxes are ticked, you have a fully working Raven knowledge base
end to end. From here, every other guide on this site assumes this baseline.
