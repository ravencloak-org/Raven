# Raven Platform -- Design Specification

**Status:** Draft
**Date:** 2026-03-27
**Authors:** Jobin Lawrance, Claude

---

## Table of Contents

1. [Overview](#1-overview)
2. [System Architecture](#2-system-architecture)
3. [Data Model](#3-data-model)
4. [Ingestion Pipeline](#4-ingestion-pipeline)
5. [Interaction Layer](#5-interaction-layer)
6. [Deployment & Auth](#6-deployment--auth)
7. [MVP Scope & Phasing](#7-mvp-scope--phasing)

---

## 1. Overview

Raven is a multi-tenant knowledge-base platform. Organizations ingest documents (PDF, DOCX, images) and web content into knowledge bases, which are then queryable via:

> **Note on research docs:** The research documents in `docs/research/` were drafted before key decisions were finalized. Two significant changes were made during brainstorming:
> 1. **Hierarchy:** Research used Application -> Organization -> Knowledge Base (3-tier). After discussion, this was simplified to **Organization -> Workspace -> Knowledge Base** (2-tier), with Organization as the tenant boundary and Workspace replacing the old Organization concept.
> 2. **Backend language:** Research recommended Node.js + Python hybrid. The user chose **Go + Python hybrid** instead, for performance, concurrency, and personal preference. This changes LiteParse integration (subprocess in Python container), job queue (plain Redis instead of BullMQ), and removes the shared-TypeScript benefit with Strapi.

1. **Embeddable Chatbot** (MVP) -- a `<raven-chat>` web component for any website
2. **Voice Agent** (Phase 2) -- voice interface via LiveKit Agents
3. **WebRTC / WhatsApp** (Phase 3) -- real-time voice via WhatsApp Business Calling API

### Core Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend API** | Go (Gin or FastHTTP) |
| **AI Workers** | Python (gRPC server) |
| **Frontend** | Vue.js + Tailwind Plus |
| **CMS** | Strapi (headless, content management) |
| **Auth** | Keycloak + reavencloak custom SPI |
| **Database** | PostgreSQL + pgvector + ParadeDB |
| **Document Parsing** | LiteParse (Apache 2.0, local) |
| **Web Scraping** | Crawl4AI (Apache 2.0) |
| **Job Queue** | Redis |
| **Object Storage** | MinIO (S3-compatible) |
| **LLM** | Multi-provider BYOK (Anthropic Claude primary) |

### Hierarchy

```
Organization (tenant boundary -- billing, auth, data isolation)
  +-- Workspace (sub-unit -- e.g., Google, Chrome, Android)
       +-- Knowledge Base (collection of documents for RAG retrieval)
            +-- Document (uploaded file)
            +-- Source (web URL / sitemap / RSS)
            +-- Chunks -> Embeddings
```

---

## 2. System Architecture

### 2.1 High-Level Component Diagram

```
+-------------------------------------------------------------------------+
|                            CLIENTS                                      |
|                  Vue.js + Tailwind Plus (SPA)                           |
+----------------------------+--------------------------------------------+
                             | HTTPS
                             v
+--------------------------------------------------------------------------+
|                        REVERSE PROXY (Traefik/Caddy)                     |
|          /api/* --> Go API    /cms/* --> Strapi    /auth/* --> Keycloak   |
+----+------------------+--------------------+------------------+----------+
     |                  |                    |                  |
     v                  v                    v                  v
+---------+    +--------------+    +--------------+   +--------------+
| Keycloak|    |  Go API      |    |   Strapi     |   |   Redis      |
| + reaven|<---|  Server      |--->|   CMS        |   |  (Job Queue) |
|   cloak |    |  (Gin/Fast)  |    |              |   |              |
|   SPI   |    +--+-+-+-------+    +------+-------+   +------+-------+
+---------+       | | |                   |                  |
                  | | |  gRPC             |                  |
                  | | v                   |                  |
                  | | +-------------------+------------------+---+
                  | | |         Python AI Worker                 |
                  | | |  +----------+  +-------------------+    |
                  | | |  | Embedding|  |   RAG Query       |    |
                  | | |  | Pipeline |  |   Engine          |    |
                  | | |  +----+-----+  +-------------------+    |
                  | | |       | subprocess/CLI                  |
                  | | |  +----v-----------------+               |
                  | | |  | LiteParse (Node.js)  |               |
                  | | |  | Document Parsing     |               |
                  | | |  +----------------------+               |
                  | | +------------------------------------------+
                  | |             |
                  v v             v
              +-----------------------------+
              |       PostgreSQL             |
              |  + pgvector (embeddings)     |
              |  + ParadeDB (full-text/BM25) |
              +-----------------------------+
```

### 2.2 Service Breakdown

| Service | Role | Exposed |
|---------|------|---------|
| **Go API** | Primary API gateway. JWT validation, routing, tenant resolution, orchestration. REST API for CRUD, enqueues async jobs, delegates AI to Python via gRPC. | Yes (:8080) |
| **Python AI Worker** | All AI/ML workloads. gRPC server for RAG queries, embedding generation. Consumes Redis jobs for async document processing. | No (internal gRPC) |
| **LiteParse** | Document-to-text extraction (PDF, DOCX, images/OCR). Invoked by Python worker as subprocess. | No (co-located) |
| **Strapi** | Headless CMS for platform marketing content (landing pages, docs, help articles) and as a quick admin UI for seed data/content management during early development. Not in the critical request path -- Go API owns all tenant CRUD. Strapi's value is editorial content + rapid admin tooling, not core business logic. Can be dropped if the Vue.js admin dashboard covers all needs. | Yes (:1337) |
| **Keycloak** | Identity provider. OIDC/OAuth2, user management, reavencloak SPI for custom claims. | Yes (:8443) |
| **PostgreSQL** | Primary datastore. pgvector for embeddings, ParadeDB for BM25 full-text. RLS for tenant isolation. **Note:** ParadeDB Community Edition lacks WAL support -- data loss possible on crash. For production, either use ParadeDB Enterprise (WAL-enabled) or fall back to PostgreSQL's built-in `tsvector` full-text search as a simpler BM25 alternative until ParadeDB matures. | No (internal) |
| **Redis** | Job queue for async processing, rate limiting, caching. | No (internal) |
| **MinIO** | S3-compatible object storage for uploaded files. | No (internal) |

### 2.3 Go <-> Python gRPC Interface

```protobuf
service AIWorker {
  rpc ParseAndEmbed (ParseRequest)      returns (ParseResponse);    // sync, small docs
  rpc QueryRAG      (RAGRequest)        returns (stream RAGChunk);  // server-streaming
  rpc GetEmbedding  (EmbeddingRequest)  returns (EmbeddingResponse);
}

message RAGRequest {
  string query           = 1;
  string org_id          = 2;
  repeated string kb_ids = 3;
  map<string, string> filters = 4;
}

message RAGChunk {
  string text            = 1;
  bool   is_final        = 2;
  repeated Source sources = 3;
}
```

**Interaction model:**
- **Synchronous** (`GetEmbedding`): Go blocks on gRPC call for lightweight operations.
- **Server-streaming** (`QueryRAG`): Python streams LLM tokens; Go forwards each chunk to client via SSE.
- **Async via queue**: For document uploads, Go enqueues to Redis (not gRPC), avoiding backpressure on the API server.

### 2.4 LiteParse Integration

Python worker invokes LiteParse as a subprocess:

```
Python AI Worker --subprocess--> liteparse --input /path/to/doc.pdf --format json
                 <--stdout(JSON)-- { pages: [...], full_text: "...", metadata: {...} }
```

Deployed into the Python worker Docker image (`apt-get install nodejs` + `npm install @llamaindex/liteparse`). Optional optimization: HTTP sidecar daemon on localhost for amortizing Node.js cold start.

### 2.5 Request Flows

**User Login:**
```
Client --> Keycloak (OIDC Authorization Code + PKCE)
       <-- { access_token (JWT w/ org_id, roles), refresh_token }
Client --> Go API /api/v1/me [Bearer token]
  Go API: validate JWT via JWKS, extract org_id/roles
       <-- { user profile, org context }
```

**Upload Document:**
```
Client --> Go API POST /api/v1/orgs/{org}/documents [multipart]
  Go API: validate JWT + org membership
  Go API: store file to MinIO
  Go API: INSERT document (status: "queued") into PostgreSQL
  Go API: ENQUEUE job into Redis
       <-- 202 { doc_id, status: "queued" }

--- Async (Python Worker) ---
  Worker: DEQUEUE job
  Worker: invoke LiteParse CLI --> JSON text output
  Worker: chunk text (512 tokens, 50 overlap)
  Worker: embed chunks via org's configured provider (BYOK)
  Worker: INSERT chunks + embeddings into PostgreSQL
  Worker: UPDATE document status --> "ready"
```

**Chat / RAG Query:**
```
Client --> Go API POST /api/v1/orgs/{org}/chat { query, conversation_id }
  Go API: validate JWT
  Go API: gRPC server-streaming call to Python Worker

  Python Worker:
    1. Embed query --> query_vector
    2. Hybrid search: pgvector (cosine) + ParadeDB (BM25)
    3. RRF fusion to merge results
    4. Rerank top-K
    5. Stream LLM completion (org's BYOK provider)

  Go API: forward tokens as SSE events --> Client
```

### 2.6 Job Queue Design (Redis)

| Queue | Consumer | Purpose |
|-------|----------|---------|
| `raven:jobs:document_process` | Python Worker | Parse, chunk, embed, index |
| `raven:jobs:document_process:failed` | -- | Dead-letter queue |
| `raven:jobs:reindex` | Python Worker | Re-embed on model change |

Job lifecycle: `pending --> processing --> done` (or `failed` after 3 retries with exponential backoff). Visibility timeout: 300s. Max TTL: 30 minutes.

---

## 3. Data Model

### 3.1 Hierarchy Naming

**Organization -> Workspace -> Knowledge Base**

- **Organization** = tenant boundary (billing, auth, data isolation). Example: Alphabet.
- **Workspace** = sub-unit within org. Example: Google, Chrome, Android. Implies autonomy with shared infrastructure. Aligns with Slack/Notion conventions.
- **Knowledge Base** = collection of documents/sources for RAG retrieval.

### 3.2 Core Entities

All IDs are UUIDs. All timestamps are `TIMESTAMPTZ`. Every tenant-scoped table carries `org_id` for RLS.

**Organizations** -- top-level tenant

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `name` | VARCHAR(255) | |
| `slug` | VARCHAR(100) UNIQUE | URL-friendly |
| `status` | ENUM | `active`, `suspended`, `deactivated` |
| `settings` | JSONB | Rate limits, feature flags |
| `keycloak_realm` | VARCHAR(255) | Keycloak realm/client ID |

**Workspaces** -- sub-units within org

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | Tenant boundary |
| `name` | VARCHAR(255) | |
| `slug` | VARCHAR(100) | Unique within org |
| `settings` | JSONB | LLM provider selection, etc. |

**Users** -- Keycloak mirror

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | Same as Keycloak user ID |
| `org_id` | UUID FK | |
| `email` | VARCHAR(255) | Unique within org |
| `display_name` | VARCHAR(255) | |
| `keycloak_sub` | VARCHAR(255) UNIQUE | |

**Workspace Members** -- join table with roles

| Column | Type | Notes |
|--------|------|-------|
| `workspace_id` | UUID FK | |
| `user_id` | UUID FK | |
| `role` | ENUM | `owner`, `admin`, `member`, `viewer` |
| | | UNIQUE(workspace_id, user_id) |

**Knowledge Bases**

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `workspace_id` | UUID FK | |
| `name` | VARCHAR(255) | |
| `description` | TEXT | |
| `settings` | JSONB | Chunk size, overlap, embedding model |

**Documents** -- uploaded files

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `file_name` | VARCHAR(500) | |
| `file_type` | VARCHAR(50) | pdf, docx, etc. |
| `file_size_bytes` | BIGINT | |
| `file_hash` | VARCHAR(128) | SHA-256 dedup |
| `storage_path` | TEXT | MinIO object path |
| `processing_status` | ENUM | See state machine |
| `processing_error` | TEXT | |

**Sources** -- web URLs, sitemaps, RSS

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `source_type` | ENUM | `url`, `sitemap`, `rss_feed` |
| `url` | TEXT | |
| `crawl_depth` | INTEGER | |
| `crawl_frequency` | ENUM | `manual`, `daily`, `weekly`, `monthly` |
| `processing_status` | ENUM | |

**Chunks** -- fundamental unit of retrieval

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `document_id` | UUID FK (nullable) | One of document_id or source_id |
| `source_id` | UUID FK (nullable) | |
| `content` | TEXT | ParadeDB BM25 indexed |
| `chunk_index` | INTEGER | Order within parent |
| `token_count` | INTEGER | |
| `heading` | VARCHAR(500) | Nearest section title |

**Embeddings** -- separate from chunks for multi-model support

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `chunk_id` | UUID FK | |
| `embedding` | vector(N) | pgvector, dimension depends on model |
| `model_name` | VARCHAR(100) | e.g. `text-embedding-3-small` |
| `dimensions` | INTEGER | e.g. 1536 |
| | | UNIQUE(chunk_id, model_name) |

HNSW index: `CREATE INDEX ON embeddings USING hnsw (embedding vector_cosine_ops) WITH (m=16, ef_construction=64);`

**LLM Provider Configs** -- BYOK encrypted key storage

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `provider` | ENUM | `openai`, `anthropic`, `cohere`, `google`, etc. |
| `api_key_encrypted` | BYTEA | AES-256-GCM encrypted |
| `api_key_hint` | VARCHAR(20) | Last 4 chars for UI |
| `base_url` | TEXT | Override for custom endpoints |
| `config` | JSONB | Provider-specific settings |
| `is_default` | BOOLEAN | |

Encryption: AES-256-GCM with master key in secrets manager. Per-org data encryption keys (DEKs). Keys never logged or returned in API responses.

**Chat Sessions / Messages, Voice Sessions / Turns** -- high-throughput tables managed directly by PostgreSQL (not Strapi). `user_id` is **nullable** to support anonymous chatbot widget users (who authenticate via API key, not user login). Anonymous sessions use a client-generated `session_id` with TTL-based cleanup (default 24h). Authenticated users get persistent history.

### 3.3 Multi-Tenancy via RLS

Shared schema with Row-Level Security. All tenants share one database.

```sql
ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON workspaces
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
```

Go API middleware sets `SET app.current_org_id = '<uuid>'` on every request from JWT claims. Admin bypass role (`raven_admin`) for cross-tenant operations.

### 3.4 Document Processing State Machine

```
queued --> crawling* --> parsing --> chunking --> embedding --> ready
  |           |            |           |             |
  +-----------+------------+-----------+-------------+--> failed
                                                          |
                                                          v
                                                     reprocessing --> parsing
```
*`crawling` only for Sources (web scraper fetches URLs first).

Audit trail: `processing_events` table records every state transition.

### 3.5 Access Control

| Role | Scope | Permissions |
|------|-------|-------------|
| **owner** | Workspace | Full control, delete workspace, transfer ownership |
| **admin** | Workspace | Manage KBs, documents, members (except owners) |
| **member** | Workspace | Read KBs, upload documents, create chat sessions |
| **viewer** | Workspace | Read-only access |

Four-layer enforcement: Keycloak (authn) -> API middleware (tenant scoping) -> Business logic (role checks) -> PostgreSQL RLS (defense-in-depth).

---

## 4. Ingestion Pipeline

### 4.1 Pipeline Stages

```
Upload/URL --> Queue --> Parse/Scrape --> Chunk --> Embed --> Index --> Ready
  Go API      Redis     Python Worker   Python   Ext API   pgvector+
                                        Worker   (BYOK)    ParadeDB
```

### 4.2 Supported Input Types

| Category | Formats |
|----------|---------|
| Documents | PDF, DOCX, XLSX, PPTX, Markdown, images (PNG, JPG, TIFF with OCR) |
| Web | Any public URL (rendered via Crawl4AI) |

### 4.3 File Processing Path

1. Go API receives multipart upload, validates type/size, stores in MinIO.
2. Creates document record (status: `queued`), enqueues Redis job.
3. Python worker dequeues, calls LiteParse CLI: `liteparse --input <file> --format json`.
4. Chunks extracted text, embeds via org's BYOK provider, stores in pgvector + ParadeDB.

### 4.4 URL Processing Path

1. Go API receives URL, validates, enqueues Redis scrape job.
2. Python worker calls Crawl4AI (Apache 2.0, Python-native) to render and extract markdown.
3. Same chunking/embedding flow as file processing.

### 4.5 Chunking Strategy

- **Method:** Recursive text splitting
- **Target:** ~512 tokens per chunk
- **Overlap:** 50 tokens between consecutive chunks
- **Split hierarchy:** paragraph -> sentence -> word boundaries
- **Metadata preserved:** source document ID, chunk index, section heading, character offsets

### 4.6 Multi-Provider Embeddings (BYOK)

Each org configures embedding provider in `llm_provider_configs`. Python worker reads config, dispatches to appropriate provider via adapter interface:

```python
class EmbeddingProvider(Protocol):
    def embed(self, texts: list[str]) -> list[list[float]]: ...
```

**Constraint:** All documents within a single **knowledge base** must use the same embedding model/dimensions. The HNSW index is built per-model-dimension pair. If an org switches embedding providers, existing KBs keep their current index and a re-index job is required. Different KBs within the same org may use different models since embeddings are filtered by `knowledge_base_id` at query time. The `embeddings.dimensions` column allows the system to validate consistency at insert time.

### 4.7 Hybrid Retrieval (Vector + BM25 via RRF)

1. **Semantic search:** query embedding vs. stored embeddings via pgvector `<=>` operator
2. **Keyword search:** query text vs. chunk content via ParadeDB `@@@` BM25 operator
3. **RRF fusion:** `score = SUM(1 / (k + rank))` with k=60

### 4.8 Error Handling

| Failure | Behavior |
|---------|----------|
| Parse failure (corrupt file) | Mark `failed`, no retry |
| Scrape failure (timeout, 4xx/5xx) | Retry 3x with exponential backoff |
| Embedding API error (rate limit) | Retry 5x with exponential backoff |
| Embedding API auth error (bad key) | Fail immediately, notify user |
| Partial failure | Resume from last checkpoint on retry |

### 4.9 Web Scraper: Crawl4AI

Chosen over Firecrawl for **Apache 2.0 license** (no AGPL risk for SaaS). Python-native, integrates directly into the worker. Playwright-based JS rendering. If scrape quality proves insufficient, Firecrawl's cloud API can be added as an optional premium tier.

---

## 5. Interaction Layer

### 5.1 Phase 1: Embeddable Chatbot (MVP)

**Web Component** (`<raven-chat>`) -- framework-agnostic, Shadow DOM for style isolation:

```html
<script src="https://cdn.raven.dev/chat.js"></script>
<raven-chat kb="kb_abc123" api-key="rk_live_..."></raven-chat>
```

**Authentication:**
- Publishable API keys per knowledge base (`rk_live_...`), domain-scoped.
- No end-user login required. Rate limiting per key.
- Admin operations use separate secret keys via Bearer tokens.

**Chat API -- SSE streaming:**
- `POST /v1/chat/{kb}/completions` with `Content-Type: text/event-stream`
- Events: `token`, `source` (citations), `error`, `done`
- Go API opens gRPC server-streaming call to Python worker, forwards tokens as SSE.

**RAG Query Flow:**
```
User message --> Go API (auth, rate-limit, load history)
  --> gRPC to Python Worker
    1. Embed query
    2. Hybrid search (pgvector + ParadeDB)
    3. RRF fusion
    4. Rerank top-K
    5. Stream LLM completion (BYOK provider)
  --> SSE stream --> Client
```

**Conversation History:**
- UUID-based `conversation_id`, returned on first response.
- Last N turns loaded as context (configurable, default 10).
- Sliding window with token-budget awareness.
- 24h TTL for anonymous sessions. Persistent history optional with JWT pass-through.

**Admin Dashboard (Vue.js):**
- Chatbot configurator with live preview (theme, avatar, welcome text)
- Test sandbox (staging key, test against KB before going live)
- Analytics (conversation volume, top queries, source-hit frequency)
- API key management (create/revoke, domain allow-lists, rate limits)

### 5.2 Phase 2: Voice Agent (Deferred)

- **Framework:** LiveKit Agents (Python worker alongside existing AI worker)
- **STT:** faster-whisper (self-hosted) or Deepgram (managed) -- configurable per org
- **TTS:** Piper (open-source) or Cartesia (low-latency streaming) -- configurable per org
- Same RAG pipeline underneath. LiveKit handles media transport and room state.

### 5.3 Phase 3: WebRTC / WhatsApp (Deferred)

- **WhatsApp:** WhatsApp Business Calling API (WebRTC native). Inbound calls routed to LiveKit room where Raven voice agent joins as participant.
- **Browser WebRTC:** LiveKit room token endpoint for "call the assistant" button in chatbot widget.
- **Room bridging:** Lightweight Go service manages LiveKit room lifecycle.

---

## 6. Deployment & Auth

### 6.1 Docker Compose Setup

| Service | Image | Exposed |
|---------|-------|---------|
| `go-api` | Custom build | Yes (:8080) |
| `python-worker` | Custom build | No |
| `strapi` | Custom build | Yes (:1337) |
| `keycloak` | `quay.io/keycloak/keycloak` | Yes (:8443) |
| `postgres` | `pgvector/pgvector:pg16` + ParadeDB | No |
| `redis` | `redis:7-alpine` | No |
| `minio` | `minio/minio` | No |
| `nginx` | `nginx` or Traefik | Yes (:80/:443) |

**Network:** All on `raven-internal` bridge. Only go-api, strapi, keycloak, nginx bind host ports.

**Volumes:** `pg-data`, `kc-config` (realm exports, SPI JARs), `uploads` (MinIO data), `redis-data`.

**Environment:** `.env` for non-secrets, `.env.secrets` (git-ignored) for credentials. `raven init` CLI scaffolds `.env.secrets` interactively.

### 6.2 Keycloak + reavencloak

- OIDC Authorization Code flow with PKCE
- **reavencloak SPI** injects custom JWT claims: `org_id`, `org_role`, `kb_permissions[]`
- Event listener propagates user lifecycle events to Go API via internal webhook
- Deployed as JAR in `kc-config` volume

### 6.3 JWT Validation (Go API Middleware)

1. Extract Bearer token from `Authorization` header
2. Validate signature against Keycloak JWKS (cached with TTL)
3. Check `iss`, `aud`, `exp`, `nbf` claims
4. Extract `org_id`, `org_role` into request context
5. Set `app.current_org_id` on PostgreSQL connection for RLS

### 6.4 API Key Auth (Embeddable Chatbot)

- Scoped to specific knowledge base, permits only `query` operations
- SHA-256 hashed in Postgres, plaintext shown once at creation
- `X-API-Key` header, validated by Go API with Origin/Referer check
- Rate limited via Redis

### 6.5 Strapi Auth

- Own admin auth for CMS administrators (separate from Keycloak)
- REST/GraphQL API consumed only by Go API internally via service account API token
- End users never interact with Strapi auth directly

### 6.6 Future Cloud Deployment (High-Level)

```
Route 53 --> CloudFront --> ALB
  +-- ECS Fargate: go-api (auto-scaled)
  +-- ECS Fargate: strapi
  +-- ECS Fargate: keycloak
  +-- ECS Fargate (private): python-worker
  +-- RDS PostgreSQL (pgvector, Multi-AZ)
  +-- ElastiCache Redis
  +-- S3 (uploads)
  +-- Secrets Manager
```

IaC via Terraform (modular: networking, ecs, rds, keycloak). Environment promotion: dev -> staging -> prod.

---

## 7. MVP Scope & Phasing

### Phase 1 -- MVP (Chatbot)

- Organization + Workspace + Knowledge Base CRUD
- User auth via Keycloak + reavencloak
- File upload (PDF, DOCX, images) + URL ingestion
- LiteParse document parsing + Crawl4AI web scraping
- Chunking + embedding (BYOK multi-provider)
- Hybrid search (pgvector + ParadeDB + RRF)
- Embeddable `<raven-chat>` web component with SSE streaming
- Admin dashboard (Vue.js): configurator, test sandbox, analytics, API keys
- Docker Compose deployment

### Phase 2 -- Voice Agent

- LiveKit Agents integration
- STT (faster-whisper / Deepgram) + TTS (Piper / Cartesia)
- Same RAG pipeline, voice-optimized (sentence-boundary TTS dispatch)
- Voice session management

### Phase 3 -- WebRTC / WhatsApp

- WhatsApp Business Calling API integration
- LiveKit room bridging
- Browser "call the assistant" button
- WebRTC session management

### Phase 4 -- Knowledge Graph (Future)

- Neo4j or Qdrant integration
- LlamaIndex PropertyGraphIndex for multi-hop relational queries
- Graph-enhanced retrieval alongside existing hybrid search

### Phase 5 -- Cloud Managed

- AWS deployment scripts (Terraform)
- Hosted cloud offering
- Pricing strategy (break-even first)
- Multi-region support
