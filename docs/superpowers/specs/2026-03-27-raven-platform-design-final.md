# Raven Platform -- Final Design Specification

**Status:** Final
**Date:** 2026-03-27
**Authors:** Jobin Lawrance, Claude
**Version:** 1.0

> This is the single source of truth for the Raven platform. All GitHub wiki pages, issues, milestones, and roadmaps derive from this document.

---

## Table of Contents

1. [Overview and Vision](#1-overview-and-vision)
2. [Tech Stack](#2-tech-stack)
3. [System Architecture](#3-system-architecture)
4. [Data Model](#4-data-model)
5. [Ingestion Pipeline](#5-ingestion-pipeline)
6. [Interaction Layer](#6-interaction-layer)
7. [Deployment](#7-deployment)
8. [Authentication and Authorization](#8-authentication-and-authorization)
9. [Compliance and Security](#9-compliance-and-security)
10. [Developer Tooling](#10-developer-tooling)
11. [Competitive Positioning](#11-competitive-positioning)
12. [MVP Roadmap and Phasing](#12-mvp-roadmap-and-phasing)

---

## 1. Overview and Vision

Raven is a **multi-tenant knowledge-base platform** that enables organizations to ingest documents and web content into searchable knowledge bases, then query them through multiple channels:

1. **Embeddable Chatbot** (MVP) -- a `<raven-chat>` web component for any website
2. **Voice Agent** (Phase 2) -- real-time voice interface via LiveKit Agents
3. **WebRTC / WhatsApp** (Phase 3) -- real-time voice via WhatsApp Business Calling API and browser WebRTC

### Vision

No existing platform combines document ingestion + RAG chatbot + voice agent + WebRTC + WhatsApp calling in a single, self-hostable, multi-tenant package. Raven fills this gap with production-grade hybrid retrieval (vector + BM25), BYOK LLM support, and an architecture optimized for edge deployment.

### Hierarchy

```
Organization (tenant boundary -- billing, auth, data isolation)
  +-- Workspace (sub-unit -- e.g., Google, Chrome, Android)
       +-- Knowledge Base (collection of documents for RAG retrieval)
            +-- Document (uploaded file)
            +-- Source (web URL / sitemap / RSS)
            +-- Chunks -> Embeddings
```

- **Organization** = tenant boundary (billing, auth, data isolation). Example: Alphabet.
- **Workspace** = sub-unit within org with operational autonomy. Example: Google, Chrome, Android. Aligns with Slack/Notion conventions.
- **Knowledge Base** = collection of documents/sources for RAG retrieval.

---

## 2. Tech Stack

### Decision Rationale

**Go + Echo** was chosen over Node.js and Kotlin/JVM for the backend API based on:
- Native compilation to single static binary: 10-25 MB Docker images on distroless
- Startup time under 50ms, RAM usage 5-10 MB at idle
- Trivial ARM64 cross-compilation (`GOOS=linux GOARCH=arm64`) for Raspberry Pi / edge deployment
- gRPC is Go-native (grpc-go is the reference implementation)
- 1-5 second compile times, 1-3 minute CI pipelines
- Goroutines handle thousands of concurrent SSE/WebSocket connections trivially

**Python AI Worker** runs separately, connected via gRPC, to access the full ML/AI ecosystem (LangChain, LlamaIndex, sentence-transformers, faster-whisper, etc.).

### Complete Dependency Table

| # | Component | Version | License | Purpose | SaaS-Safe? |
|---|-----------|---------|---------|---------|------------|
| 1 | **Go** | 1.23.x | BSD-3-Clause + Patent Grant | Backend API language | YES |
| 2 | **Echo** | v4.13.x | MIT | Go HTTP framework (routing, middleware, grouping) | YES |
| 3 | **grpc-go** | 1.70.x | Apache 2.0 | gRPC client/server for Go <-> Python communication | YES |
| 4 | **pgx** | v5.7.x | MIT | PostgreSQL driver for Go (connection pooling built-in) | YES |
| 5 | **sqlc** | 1.28.x | MIT | Type-safe Go code generation from SQL queries | YES |
| 6 | **go-redis** | v9.7.x | BSD-2-Clause | Valkey/Redis client for Go (job queue, caching) | YES |
| 7 | **coder/websocket** | 1.8.x | ISC | WebSocket library for Go (context.Context aware) | YES |
| 8 | **goose** | v3.24.x | MIT | Database migrations | YES |
| 9 | **viper** | 1.20.x | MIT | Configuration management | YES |
| 10 | **Python** | 3.12.x | PSF License | AI worker language | YES |
| 11 | **grpcio** | 1.69.x | Apache 2.0 | gRPC for Python AI worker | YES |
| 12 | **Vue.js** | 3.5.x | MIT | Frontend SPA framework | YES |
| 13 | **Tailwind CSS** | 4.x | MIT | Utility-first CSS framework | YES |
| 14 | **Tailwind Plus** | Latest | Commercial (paid) | Premium UI component library | YES |
| 15 | **PostgreSQL** | 18.x | PostgreSQL License (BSD-like) | Primary database | YES |
| 16 | **pgvector** | 0.8.x | PostgreSQL License (BSD-like) | Vector similarity search (HNSW, IVFFlat) | YES |
| 17 | **ParadeDB (pg_search)** | 0.22.x | AGPL-3.0 | BM25 full-text search in PostgreSQL | **CAUTION** |
| 18 | **Valkey** | 8.1.x | BSD-3-Clause | Job queue, caching, rate limiting (Redis replacement) | YES |
| 19 | **SeaweedFS** | 3.82.x | Apache 2.0 | S3-compatible object storage (MinIO replacement) | YES |
| 20 | **Keycloak** | 26.x | Apache 2.0 | Identity provider (OIDC/OAuth2, user management) | YES |
| 21 | **Strapi** (Community) | 5.x | MIT | Headless CMS for marketing content and admin tooling | YES |
| 22 | **LiteParse** | Latest | Apache 2.0 | Document-to-text extraction (PDF, DOCX, OCR) | YES |
| 23 | **Crawl4AI** | 0.6.x | Apache 2.0 | Web scraping with Playwright (Firecrawl replacement) | YES |
| 24 | **LiveKit Server** | 2.3.x | Apache 2.0 | WebRTC SFU for voice/video transport | YES |
| 25 | **LiveKit Agents** | 1.1.x | Apache 2.0 | Voice agent framework (STT/LLM/TTS pipeline) | YES |
| 26 | **faster-whisper** | 1.1.x | MIT | Self-hosted STT (CTranslate2-based Whisper) | YES |
| 27 | **Piper TTS** | 1.2.x (MIT archived) | MIT | Self-hosted text-to-speech | YES |
| 28 | **Silero VAD** | 5.1.x | MIT | Voice Activity Detection | YES |
| 29 | **Traefik** | 3.3.x | MIT | Reverse proxy with auto-TLS | YES |
| 30 | **Docker Engine** | 27.x | Apache 2.0 | Container runtime | YES |
| 31 | **Docker Compose** | 2.32.x | Apache 2.0 | Multi-container orchestration | YES |
| 32 | **Tesseract.js** | 5.x | Apache 2.0 | OCR engine (used by LiteParse) | YES |
| 33 | **OpenTelemetry Go** | 1.34.x | Apache 2.0 | Observability (tracing, metrics) | YES |
| 34 | **Anthropic Claude** | API | Proprietary (API usage) | Primary LLM provider (BYOK) | YES |
| 35 | **OpenAI** | API | Proprietary (API usage) | Embedding + fallback LLM provider (BYOK) | YES |
| 36 | **Cohere** | API | Proprietary (API usage) | Reranking provider (BYOK) | YES |

### License Risk Notes

**ParadeDB (AGPL-3.0):** Using ParadeDB in a SaaS product triggers AGPL copyleft obligations. Mitigation options:
1. Purchase a ParadeDB commercial license (recommended for production)
2. Fall back to PostgreSQL native `tsvector` + `ts_rank` for BM25-equivalent full-text search (no ParadeDB dependency)
3. Use ParadeDB in development/staging, PostgreSQL native tsvector in production until commercial license is obtained

The codebase MUST abstract the full-text search layer behind an interface so the ParadeDB implementation can be swapped for native tsvector without code changes.

**Redis replaced by Valkey:** Redis adopted tri-licensing (RSALv2/SSPLv1/AGPLv3) since v8.0. Valkey (BSD-3-Clause, Linux Foundation fork of Redis 7.2) is a drop-in API-compatible replacement with no licensing risk.

**MinIO replaced by SeaweedFS:** MinIO is AGPL-3.0. SeaweedFS (Apache 2.0) provides S3-compatible object storage. For simple deployments, local filesystem storage is also supported.

**Firecrawl replaced by Crawl4AI:** Firecrawl is AGPL-3.0. Crawl4AI (Apache 2.0) is Python-native, integrates directly into the Python AI worker, and has built-in chunking strategies.

**Coqui TTS replaced by Piper TTS:** Coqui XTTS model weights are non-commercial only (CPML) and the company is defunct. Piper TTS (MIT, archived rhasspy/piper version) is the license-safe replacement. The active fork (OHF-Voice/piper1-gpl) is GPL-3.0 which is acceptable for server-side SaaS use (not triggered by network access, only distribution).

**TEN Framework REJECTED:** License includes anti-competition clauses and restrictions on enabling third parties. Agora RTC is the only WebRTC transport (proprietary, paid). LiveKit (Apache 2.0, fully self-hostable) is used instead.

---

## 3. System Architecture

### 3.1 High-Level Component Diagram

```
+-------------------------------------------------------------------------+
|                            CLIENTS                                      |
|       Vue.js + Tailwind Plus (SPA)  |  <raven-chat> Web Component      |
+----------------------------+--------------------------------------------+
                             | HTTPS
                             v
+--------------------------------------------------------------------------+
|                        REVERSE PROXY (Traefik)                           |
|    /api/* --> Go API   /cms/* --> Strapi   /auth/* --> Keycloak           |
+----+------------------+--------------------+------------------+----------+
     |                  |                    |                  |
     v                  v                    v                  v
+---------+    +--------------+    +--------------+   +--------------+
| Keycloak|    |  Go API      |    |   Strapi     |   |   Valkey     |
| + reaven|<---|  Server      |--->|   CMS        |   |  (Job Queue) |
|   cloak |    |  (Echo)      |    |              |   |              |
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
              +-----------------------------+     +------------------+
              |       PostgreSQL 18         |     |    SeaweedFS     |
              |  + pgvector (embeddings)    |     |  (Object Store)  |
              |  + ParadeDB (full-text/BM25)|     +------------------+
              +-----------------------------+
```

### 3.2 Service Breakdown

| Service | Role | Exposed |
|---------|------|---------|
| **Go API** | Primary API gateway. JWT validation, routing, tenant resolution, orchestration. REST API for CRUD, enqueues async jobs, delegates AI to Python via gRPC. | Yes (:8080) |
| **Python AI Worker** | All AI/ML workloads. gRPC server for RAG queries, embedding generation. Consumes Valkey jobs for async document processing. Runs Crawl4AI for web scraping. | No (internal gRPC) |
| **LiteParse** | Document-to-text extraction (PDF, DOCX, images/OCR). Invoked by Python worker as subprocess. | No (co-located) |
| **Strapi** | Headless CMS for platform marketing content (landing pages, docs, help articles) and as a quick admin UI for seed data during early development. Not in the critical path. Can be dropped if Vue.js admin dashboard covers all needs. | Yes (:1337) |
| **Keycloak** | Identity provider. OIDC/OAuth2, user management, reavencloak SPI for custom claims. | Yes (:8443) |
| **PostgreSQL 18** | Primary datastore. pgvector for embeddings, ParadeDB for BM25 full-text (or native tsvector as fallback). RLS for tenant isolation. | No (internal) |
| **Valkey** | Job queue for async processing, rate limiting, caching. Drop-in Redis replacement, BSD-3-Clause. | No (internal) |
| **SeaweedFS** | S3-compatible object storage for uploaded files. Apache 2.0, replaces MinIO. | No (internal) |

### 3.3 Edge Deployment Mode (Split Architecture)

For Raspberry Pi and edge devices, Raven runs in a split configuration:

```
+---------------------------+          +---------------------------+
|   EDGE DEVICE (Pi 5)     |          |   REMOTE SERVER (Cloud)   |
|                           |   gRPC   |                           |
|  Go API Server (Echo)     |<-------->|  Python AI Worker         |
|  - JWT validation         |  over    |  - Embedding generation   |
|  - REST API serving       |  network |  - RAG query engine       |
|  - Valkey (embedded/tiny) |          |  - LiteParse subprocess   |
|  - PostgreSQL 18 (local)  |          |  - Crawl4AI scraping      |
|    + pgvector             |          |  - LLM API calls (BYOK)  |
|    + tsvector (no ParadeDB|          |                           |
|      on ARM64)            |          |  LiveKit Server (if voice)|
|                           |          |  SeaweedFS (object store) |
|  Docker image: ~25 MB     |          |                           |
|  RAM: 5-10 MB idle        |          +---------------------------+
|  Startup: <50ms           |
+---------------------------+
```

**Key design constraints for edge mode:**
- Go API binary cross-compiled for ARM64: `GOOS=linux GOARCH=arm64 go build`
- PostgreSQL runs locally on edge; pgvector is ARM64-compatible; ParadeDB may not be (fall back to native tsvector)
- Python AI worker runs on a remote server with GPU access
- gRPC connection between edge and cloud over TLS with mutual authentication
- Valkey runs locally on edge in minimal-memory mode
- SeaweedFS or local filesystem for file storage on edge

### 3.4 Go <-> Python gRPC Interface

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
- **Async via queue**: For document uploads, Go enqueues to Valkey (not gRPC), avoiding backpressure on the API server.

### 3.5 Request Flows

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
  Go API: store file to SeaweedFS
  Go API: INSERT document (status: "queued") into PostgreSQL
  Go API: ENQUEUE job into Valkey
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
    2. Hybrid search: pgvector (cosine) + ParadeDB (BM25) or tsvector
    3. RRF fusion to merge results
    4. Rerank top-K
    5. Stream LLM completion (org's BYOK provider)

  Go API: forward tokens as SSE events --> Client
```

### 3.6 Job Queue Design (Valkey)

| Queue | Consumer | Purpose |
|-------|----------|---------|
| `raven:jobs:document_process` | Python Worker | Parse, chunk, embed, index |
| `raven:jobs:document_process:failed` | -- | Dead-letter queue |
| `raven:jobs:web_scrape` | Python Worker | Crawl4AI web scraping |
| `raven:jobs:reindex` | Python Worker | Re-embed on model change |

Job lifecycle: `pending --> processing --> done` (or `failed` after 3 retries with exponential backoff). Visibility timeout: 300s. Max TTL: 30 minutes.

---

## 4. Data Model

### 4.1 Hierarchy

**Organization -> Workspace -> Knowledge Base**

All IDs are UUIDs. All timestamps are `TIMESTAMPTZ`. Every tenant-scoped table carries `org_id` for RLS.

### 4.2 Core Entities

**Organizations** -- top-level tenant

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `name` | VARCHAR(255) | |
| `slug` | VARCHAR(100) UNIQUE | URL-friendly |
| `status` | ENUM | `active`, `suspended`, `deactivated` |
| `settings` | JSONB | Rate limits, feature flags |
| `keycloak_realm` | VARCHAR(255) | Keycloak realm/client ID |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

**Workspaces** -- sub-units within org

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | Tenant boundary |
| `name` | VARCHAR(255) | |
| `slug` | VARCHAR(100) | Unique within org |
| `settings` | JSONB | LLM provider selection, etc. |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

**Users** -- Keycloak mirror

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | Same as Keycloak user ID |
| `org_id` | UUID FK | |
| `email` | VARCHAR(255) | Unique within org |
| `display_name` | VARCHAR(255) | |
| `keycloak_sub` | VARCHAR(255) UNIQUE | |
| `status` | ENUM | `active`, `disabled` |
| `last_login_at` | TIMESTAMPTZ | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

**Workspace Members** -- join table with roles

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `workspace_id` | UUID FK | |
| `user_id` | UUID FK | |
| `role` | ENUM | `owner`, `admin`, `member`, `viewer` |
| `created_at` | TIMESTAMPTZ | |
| | | UNIQUE(workspace_id, user_id) |

**Knowledge Bases**

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `workspace_id` | UUID FK | |
| `name` | VARCHAR(255) | |
| `slug` | VARCHAR(100) | Unique within workspace |
| `description` | TEXT | |
| `settings` | JSONB | Chunk size, overlap, embedding model |
| `status` | ENUM | `active`, `archived` |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

**Documents** -- uploaded files

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `file_name` | VARCHAR(500) | |
| `file_type` | VARCHAR(50) | pdf, docx, xlsx, pptx, etc. |
| `file_size_bytes` | BIGINT | |
| `file_hash` | VARCHAR(128) | SHA-256 dedup |
| `storage_path` | TEXT | SeaweedFS object path |
| `processing_status` | ENUM | See state machine |
| `processing_error` | TEXT | |
| `title` | VARCHAR(500) | Extracted or user-provided |
| `page_count` | INTEGER | |
| `metadata` | JSONB | Arbitrary key-value pairs |
| `uploaded_by` | UUID FK -> users | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

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
| `processing_error` | TEXT | |
| `title` | VARCHAR(500) | |
| `pages_crawled` | INTEGER | |
| `metadata` | JSONB | |
| `created_by` | UUID FK -> users | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

**Chunks** -- fundamental unit of retrieval

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `document_id` | UUID FK (nullable) | One of document_id or source_id |
| `source_id` | UUID FK (nullable) | |
| `content` | TEXT | BM25 indexed (ParadeDB or tsvector) |
| `chunk_index` | INTEGER | Order within parent |
| `token_count` | INTEGER | |
| `page_number` | INTEGER | For PDFs |
| `heading` | VARCHAR(500) | Nearest section title |
| `chunk_type` | ENUM | `text`, `table`, `image_caption`, `code` |
| `metadata` | JSONB | |
| `created_at` | TIMESTAMPTZ | |

**Embeddings** -- separate from chunks for multi-model support

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `chunk_id` | UUID FK | |
| `embedding` | vector(N) | pgvector, dimension depends on model |
| `model_name` | VARCHAR(100) | e.g. `text-embedding-3-small` |
| `model_version` | VARCHAR(50) | |
| `dimensions` | INTEGER | e.g. 1536 |
| `created_at` | TIMESTAMPTZ | |
| | | UNIQUE(chunk_id, model_name) |

Index: `CREATE INDEX ON embeddings USING hnsw (embedding vector_cosine_ops) WITH (m=16, ef_construction=64);`

**LLM Provider Configs** -- BYOK encrypted key storage

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `provider` | ENUM | `openai`, `anthropic`, `cohere`, `google`, `azure_openai`, `custom` |
| `display_name` | VARCHAR(255) | User-facing label |
| `api_key_encrypted` | BYTEA | AES-256-GCM encrypted |
| `api_key_iv` | BYTEA | Initialization vector |
| `api_key_hint` | VARCHAR(20) | Last 4 chars for UI |
| `base_url` | TEXT | Override for custom endpoints |
| `config` | JSONB | Provider-specific settings |
| `is_default` | BOOLEAN | |
| `status` | ENUM | `active`, `revoked`, `expired` |
| `created_by` | UUID FK -> users | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

Encryption: AES-256-GCM with master key in secrets manager (AWS Secrets Manager, HashiCorp Vault, or env var fallback). Per-org data encryption keys (DEKs). Keys never logged or returned in API responses.

**API Keys** -- for embeddable chatbot authentication

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `key_hash` | VARCHAR(128) | SHA-256 hash of the key |
| `key_prefix` | VARCHAR(20) | `rk_live_...` prefix for identification |
| `name` | VARCHAR(255) | User-given label |
| `allowed_domains` | TEXT[] | Domain allow-list for CORS |
| `rate_limit` | INTEGER | Requests per minute |
| `status` | ENUM | `active`, `revoked` |
| `created_by` | UUID FK -> users | |
| `created_at` / `expires_at` | TIMESTAMPTZ | |

**Chat Sessions / Messages** -- high-throughput

| Column (Session) | Type | Notes |
|-------------------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `user_id` | UUID FK (nullable) | Nullable for anonymous chatbot users |
| `session_token` | VARCHAR(255) | Client-generated for anonymous |
| `metadata` | JSONB | Channel, user-agent, etc. |
| `created_at` / `expires_at` | TIMESTAMPTZ | 24h TTL for anonymous |

| Column (Message) | Type | Notes |
|-------------------|------|-------|
| `id` | UUID PK | |
| `session_id` | UUID FK | |
| `org_id` | UUID FK | |
| `role` | ENUM | `user`, `assistant`, `system` |
| `content` | TEXT | |
| `token_count` | INTEGER | |
| `chunk_ids` | UUID[] | Retrieved chunks for this response |
| `model_name` | VARCHAR(100) | LLM model used |
| `latency_ms` | INTEGER | Response generation time |
| `created_at` | TIMESTAMPTZ | |

**Voice Sessions / Turns** -- structured similarly to chat, with additional fields for audio duration, STT/TTS latency, and LiveKit room ID.

**Processing Events** -- audit log for document processing

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `document_id` | UUID FK (nullable) | |
| `source_id` | UUID FK (nullable) | |
| `from_status` | VARCHAR(20) | |
| `to_status` | VARCHAR(20) | |
| `error_message` | TEXT | |
| `duration_ms` | INTEGER | Time in this state |
| `created_at` | TIMESTAMPTZ | |

### 4.3 Multi-Tenancy via RLS

Shared schema with Row-Level Security. All tenants share one database.

```sql
ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON workspaces
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

-- Admin bypass for cross-tenant operations
CREATE POLICY admin_bypass ON workspaces
    FOR ALL TO raven_admin
    USING (true);
```

Go API middleware sets `SET app.current_org_id = '<uuid>'` on every request from JWT claims.

**Tables with RLS:** `workspaces`, `users`, `workspace_members`, `knowledge_bases`, `documents`, `sources`, `chunks`, `embeddings`, `llm_provider_configs`, `chat_sessions`, `chat_messages`, `voice_sessions`, `voice_turns`, `api_keys`.

### 4.4 Document Processing State Machine

```
queued --> crawling* --> parsing --> chunking --> embedding --> ready
  |           |            |           |             |
  +-----------+------------+-----------+-------------+--> failed
                                                          |
                                                          v
                                                     reprocessing --> parsing
```
*`crawling` only for Sources (web scraper fetches URLs first).

| State | Description | Valid transitions |
|-------|-------------|-------------------|
| `queued` | Created, waiting in processing queue | `parsing`, `crawling` (sources only) |
| `crawling` | Web scraper fetching URL content | `parsing`, `failed` |
| `parsing` | Extracting text from file | `chunking`, `failed` |
| `chunking` | Splitting extracted text into chunks | `embedding`, `failed` |
| `embedding` | Generating vector embeddings | `ready`, `failed` |
| `ready` | Complete -- available for retrieval | `reprocessing` |
| `failed` | Error occurred | `reprocessing` |
| `reprocessing` | Manual retry; clears old chunks | `parsing`, `crawling` |

### 4.5 Access Control

| Role | Scope | Permissions |
|------|-------|-------------|
| **org_admin** | Organization | Full control over all workspaces, billing, org settings |
| **owner** | Workspace | Full control, delete workspace, transfer ownership |
| **admin** | Workspace | Manage KBs, documents, members (except owners) |
| **member** | Workspace | Read KBs, upload documents, create chat sessions |
| **viewer** | Workspace | Read-only access |

Four-layer enforcement: Keycloak (authn) -> API middleware (tenant scoping) -> Business logic (role checks) -> PostgreSQL RLS (defense-in-depth).

### 4.6 Entity Relationship Summary

```
Organization (tenant boundary)
  +-- User (Keycloak mirror)
  +-- LLMProviderConfig (encrypted BYOK keys)
  +-- APIKey (chatbot widget auth)
  +-- Workspace
       +-- WorkspaceMember (user + role)
       +-- KnowledgeBase
            +-- Document --> Chunk --> Embedding
            +-- Source   --> Chunk --> Embedding
            +-- ChatSession --> ChatMessage
            +-- VoiceSession --> VoiceTurn
```

---

## 5. Ingestion Pipeline

### 5.1 Pipeline Stages

```
Upload/URL --> Queue --> Parse/Scrape --> Chunk --> Embed --> Index --> Ready
  Go API      Valkey    Python Worker   Python   Ext API   pgvector+
                                        Worker   (BYOK)    ParadeDB/tsvector
```

### 5.2 Supported Input Types

| Category | Formats |
|----------|---------|
| Documents | PDF, DOCX, XLSX, PPTX, Markdown, images (PNG, JPG, TIFF with OCR) |
| Web | Any public URL (rendered via Crawl4AI with Playwright) |

### 5.3 File Processing Path

1. Go API receives multipart upload, validates type/size, stores in SeaweedFS (or local filesystem).
2. Creates document record (status: `queued`), enqueues Valkey job.
3. Python worker dequeues, calls LiteParse CLI: `liteparse --input <file> --format json`.
4. Chunks extracted text, embeds via org's BYOK provider, stores in pgvector + ParadeDB/tsvector.

### 5.4 URL Processing Path

1. Go API receives URL, validates, enqueues Valkey scrape job.
2. Python worker calls Crawl4AI (Apache 2.0, Python-native) with Playwright to render and extract markdown.
3. Crawl4AI features: async-first, built-in content filtering (removes navbars/footers/ads), BFS deep crawl strategy, configurable depth/page limits.
4. Same chunking/embedding flow as file processing.

### 5.5 Chunking Strategy

- **Method:** Document-structure-aware with recursive fallback
- **Target:** ~512 tokens per chunk
- **Overlap:** 50 tokens between consecutive chunks
- **Split hierarchy:** LiteParse structural elements (headings, tables, lists) -> paragraph -> sentence -> word boundaries
- **Tables:** Each table becomes its own chunk with caption/heading as prefix
- **Metadata preserved:** document_id, org_id, chunk_index, section heading, page_number, character offsets

### 5.6 Multi-Provider Embeddings (BYOK)

Each org configures embedding provider in `llm_provider_configs`. Python worker reads config, dispatches to appropriate provider:

```python
class EmbeddingProvider(Protocol):
    def embed(self, texts: list[str]) -> list[list[float]]: ...
```

**Constraint:** All documents within a single knowledge base must use the same embedding model/dimensions. The HNSW index is built per-model-dimension pair. Different KBs within the same org may use different models.

### 5.7 Hybrid Retrieval (Vector + BM25 via RRF)

1. **Semantic search:** query embedding vs. stored embeddings via pgvector `<=>` operator
2. **Keyword search:** query text vs. chunk content via ParadeDB `@@@` BM25 operator (or PostgreSQL native `ts_rank` with `tsvector` as fallback)
3. **RRF fusion:** `score = SUM(1 / (k + rank))` with k=60
4. **Reranking:** Top 20-30 RRF results passed to Cohere Rerank v3 (or self-hosted BGE reranker), returning top 5-8 for LLM context

### 5.8 Error Handling

| Failure | Behavior |
|---------|----------|
| Parse failure (corrupt file) | Mark `failed`, no retry |
| Scrape failure (timeout, 4xx/5xx) | Retry 3x with exponential backoff |
| Embedding API error (rate limit) | Retry 5x with exponential backoff |
| Embedding API auth error (bad key) | Fail immediately, notify user |
| Partial failure | Resume from last checkpoint on retry |

---

## 6. Interaction Layer

### 6.1 Phase 1: Embeddable Chatbot (MVP)

**Web Component** (`<raven-chat>`) -- framework-agnostic, Shadow DOM for style isolation:

```html
<script src="https://cdn.raven.dev/chat.js"></script>
<raven-chat kb="kb_abc123" api-key="rk_live_..."></raven-chat>
```

**Authentication:**
- Publishable API keys per knowledge base (`rk_live_...`), domain-scoped.
- No end-user login required. Rate limiting per key via Valkey.
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
    2. Hybrid search (pgvector + ParadeDB/tsvector)
    3. RRF fusion
    4. Rerank top-K
    5. Stream LLM completion (BYOK provider)
  --> SSE stream --> Client
```

**Conversation History:**
- UUID-based `conversation_id`, returned on first response.
- Last N turns loaded as context (configurable, default 10).
- Sliding window with token-budget awareness.
- 24h TTL for anonymous sessions. Persistent history for authenticated users.

**Admin Dashboard (Vue.js + Tailwind Plus):**
- Mobile-first, responsive, PWA-capable
- Chatbot configurator with live preview (theme, avatar, welcome text)
- Test sandbox (staging key, test against KB before going live)
- Analytics (conversation volume, top queries, source-hit frequency)
- API key management (create/revoke, domain allow-lists, rate limits)
- Knowledge base management (upload, scrape, monitor processing status)

### 6.2 Phase 2: Voice Agent

**Framework:** LiveKit Agents (Python, runs alongside existing AI worker)

**Architecture:**
```
Browser/Mobile
    |
    | WebRTC (audio via livekit-client-sdk-js)
    v
LiveKit SFU (Room)
    |
    | Audio frames
    v
LiveKit Agent (Python process, joins Room as participant)
    |
    +---> Silero VAD (detect speech start/end)
    +---> STT: faster-whisper (self-hosted) or Deepgram Nova-3 (managed)
    +---> Raven RAG Service (gRPC stream, same pipeline as chatbot)
    +---> TTS: Piper (self-hosted, MIT) or Cartesia Sonic (managed)
    +---> Audio out -> LiveKit Room -> User
```

**STT/TTS selection by phase:**

| Phase | STT | TTS | Rationale |
|-------|-----|-----|-----------|
| MVP (Phase 2) | Deepgram Nova-3 (API) | Cartesia Sonic (API) | Fastest to production, lowest latency |
| Scale | faster-whisper (self-hosted) | Piper TTS (self-hosted, MIT) | Cost control, data residency |
| Premium tier | Deepgram Nova-3 | ElevenLabs or Cartesia | Best quality for paying customers |

**Latency budget (target):**
| Stage | Target | Notes |
|-------|--------|-------|
| VAD detection | ~30ms | Silero, 30ms chunks |
| STT | 200-400ms | faster-whisper streaming |
| RAG + LLM (first token) | 300-600ms | Hybrid retrieval + LLM TTFT |
| TTS (first audio chunk) | 50-150ms | Cartesia streaming |
| WebRTC transport | 50-100ms | Jitter buffer + network |
| **Total (speech-to-speech)** | **630-1280ms** | Target: <1s for 80th percentile |

**Key optimization:** Sentence-boundary TTS dispatch -- buffer LLM tokens until a sentence boundary, then send to TTS immediately while LLM continues generating. Reduces perceived latency by 40-60%.

### 6.3 Phase 3: WebRTC / WhatsApp

**WhatsApp:** WhatsApp Business Calling API (WebRTC native). Direct WebRTC bridge from Meta's Graph API webhooks into a LiveKit Room:
1. Raven WhatsApp Bridge receives SDP offer from Meta webhook
2. Creates RTCPeerConnection, sends SDP answer back via Graph API
3. Bridges WebRTC media stream into LiveKit Room
4. Voice agent handles call identically to browser calls

**Browser WebRTC:** LiveKit room token endpoint for "call the assistant" button in chatbot widget.

**Room bridging:** Lightweight Go service manages LiveKit room lifecycle.

### 6.4 Shared RAG Interface (All Surfaces)

All three surfaces call the same RAG service via a unified gRPC interface:

```protobuf
service RavenRAG {
  rpc Query(QueryRequest) returns (stream QueryResponse);
  rpc CreateSession(CreateSessionRequest) returns (Session);
  rpc GetSessionHistory(SessionHistoryRequest) returns (SessionHistory);
}

message QueryRequest {
  string tenant_id = 1;
  string session_id = 2;
  string query_text = 3;    // typed message or STT transcription
  map<string, string> metadata = 4;
}

message QueryResponse {
  string token = 1;
  repeated Source sources = 2;
  bool is_final = 3;
}
```

| Aspect | Chatbot | Voice Agent | WhatsApp |
|--------|---------|-------------|----------|
| Input | User-typed text | STT transcription | STT transcription |
| Output | Streamed text (SSE) | Text -> TTS -> audio | Text -> TTS -> audio |
| Session | Cookie/JWT + session ID | LiveKit Room metadata | WhatsApp call_id |
| Latency tolerance | 1-3s acceptable | <1s target | <1s target |

---

## 7. Deployment

### 7.1 Docker Compose Setup (Standard)

| Service | Image | Exposed |
|---------|-------|---------|
| `go-api` | Custom build (~25 MB) | Yes (:8080) |
| `python-worker` | Custom build | No |
| `strapi` | Custom build | Yes (:1337) |
| `keycloak` | `quay.io/keycloak/keycloak:26` | Yes (:8443) |
| `postgres` | `pgvector/pgvector:pg18` + ParadeDB | No |
| `valkey` | `valkey/valkey:8.1-alpine` | No |
| `seaweedfs` | `chrislusf/seaweedfs` | No |
| `traefik` | `traefik:v3.3` | Yes (:80/:443) |

**Network:** All on `raven-internal` bridge. Only go-api, strapi, keycloak, traefik bind host ports.

**Volumes:** `pg-data`, `kc-config` (realm exports, SPI JARs), `uploads` (SeaweedFS data), `valkey-data`.

**Environment:** `.env` for non-secrets, `.env.secrets` (git-ignored) for credentials. `raven init` CLI scaffolds `.env.secrets` interactively.

### 7.2 Edge Deployment (Raspberry Pi / ARM64)

```yaml
# docker-compose.edge.yml (simplified)
services:
  go-api:
    image: raven/api:latest-arm64
    platform: linux/arm64
    ports: ["8080:8080"]
    environment:
      RAVEN_AI_WORKER_GRPC_ADDR: "cloud-server:50051"  # Remote Python worker
      RAVEN_DB_HOST: postgres
      RAVEN_VALKEY_HOST: valkey
  postgres:
    image: pgvector/pgvector:pg18
    platform: linux/arm64
    volumes: ["pg-data:/var/lib/postgresql/data"]
  valkey:
    image: valkey/valkey:8.1-alpine
    platform: linux/arm64
    command: ["valkey-server", "--maxmemory", "64mb"]
```

Go API binary: 10-25 MB, <50ms startup, 5-10 MB RAM. Cross-compiled: `GOOS=linux GOARCH=arm64 go build -ldflags="-s -w"`.

### 7.3 Cloud Deployment (Future)

```
Route 53 --> CloudFront --> ALB
  +-- ECS Fargate: go-api (auto-scaled)
  +-- ECS Fargate: strapi
  +-- ECS Fargate: keycloak
  +-- ECS Fargate (private): python-worker
  +-- RDS PostgreSQL 18 (pgvector, Multi-AZ)
  +-- ElastiCache (Valkey-compatible)
  +-- S3 (uploads, replaces SeaweedFS)
  +-- Secrets Manager
  +-- LiveKit Cloud (or self-hosted on ECS)
```

IaC via Terraform (modular: networking, ecs, rds, keycloak). Environment promotion: dev -> staging -> prod.

---

## 8. Authentication and Authorization

### 8.1 Keycloak + reavencloak SPI

- OIDC Authorization Code flow with PKCE
- **reavencloak SPI** injects custom JWT claims: `org_id`, `org_role`, `workspace_ids[]`, `kb_permissions[]`
- Event listener propagates user lifecycle events to Go API via internal webhook
- Deployed as JAR in `kc-config` volume

### 8.2 JWT Validation (Go API Middleware)

1. Extract Bearer token from `Authorization` header
2. Validate signature against Keycloak JWKS (cached with TTL)
3. Check `iss`, `aud`, `exp`, `nbf` claims
4. Extract `org_id`, `org_role` into request context
5. Set `app.current_org_id` on PostgreSQL connection for RLS

### 8.3 API Key Auth (Embeddable Chatbot)

- Scoped to specific knowledge base, permits only `query` operations
- SHA-256 hashed in Postgres, plaintext shown once at creation
- `X-API-Key` header, validated by Go API with Origin/Referer check against `allowed_domains`
- Rate limited via Valkey

### 8.4 Strapi Auth

- Own admin auth for CMS administrators (separate from Keycloak)
- REST/GraphQL API consumed only by Go API internally via service account API token
- End users never interact with Strapi auth directly

---

## 9. Compliance and Security

### 9.1 GDPR (EU General Data Protection Regulation)

| Requirement | Implementation |
|-------------|----------------|
| **Right to erasure (Art. 17)** | API endpoint `DELETE /api/v1/orgs/{org}/users/{user}` triggers cascading deletion of all user data: chat sessions, voice sessions, uploaded documents, and associated chunks/embeddings. Also triggers Keycloak user deletion via reavencloak webhook. Audit trail records the erasure event. |
| **Data portability (Art. 20)** | API endpoint `GET /api/v1/orgs/{org}/export` generates a machine-readable export (JSON/CSV) of all organization data: documents, sources, chat history, configuration. Delivered as a downloadable archive. |
| **Consent management** | Cookie consent for the `<raven-chat>` widget via configurable consent banner. Anonymous sessions use session-only storage (no persistent cookies) by default. Explicit opt-in for persistent history. |
| **Data residency** | Edge deployment mode enables data to remain on-premise. Cloud deployment uses regional infrastructure (EU-West for EU customers). Database and object storage are region-scoped. |
| **Data Processing Agreement** | Template DPA provided for enterprise customers. Lists sub-processors (LLM providers via BYOK -- customer controls which providers process their data). |
| **Privacy by design** | Minimal data collection. API keys hashed. LLM provider keys encrypted. No telemetry by default. PII detection in ingestion pipeline (configurable, opt-in). |

### 9.2 SOC 2 Readiness

| Control | Implementation |
|---------|----------------|
| **Audit logging** | All API mutations logged to `audit_log` table: who, what, when, from where (IP, user-agent). Processing events table for document pipeline. Logs retained for configurable period (default 90 days). |
| **Access controls** | Four-layer enforcement (Keycloak -> middleware -> business logic -> RLS). Role-based access per workspace. API key scoping per knowledge base. |
| **Encryption at rest** | PostgreSQL with disk encryption (LUKS/dm-crypt for self-hosted, managed encryption for RDS). LLM API keys encrypted with AES-256-GCM. SeaweedFS supports server-side encryption. |
| **Encryption in transit** | TLS 1.3 on all external endpoints (Traefik auto-TLS via Let's Encrypt). Internal gRPC with mTLS between Go API and Python worker. Valkey TLS optional (recommended for production). |
| **Change management** | Git-based deployment. CI/CD pipeline with required reviews. Infrastructure as Code (Terraform). |
| **Incident response** | Health check endpoints on all services. Alerting via OpenTelemetry metrics. Structured JSON logging for aggregation. |
| **Vulnerability management** | Dependabot for automated dependency scanning. CodeRabbit for AI code review. Container image scanning in CI. |

### 9.3 Security Measures

- **API key security:** SHA-256 hashed, shown once at creation, domain-scoped, rate-limited
- **LLM key encryption:** AES-256-GCM with master key in secrets manager, per-org DEKs
- **RLS defense-in-depth:** Even if application code has a bug, PostgreSQL RLS blocks cross-tenant access
- **Input validation:** All user inputs validated and sanitized at the Go API layer
- **CORS:** Strict origin checking for chatbot widget API keys
- **Rate limiting:** Per API key, per user, per org -- enforced via Valkey
- **Secrets management:** `.env.secrets` git-ignored, secrets manager for production

---

## 10. Developer Tooling

### 10.1 Dependabot

Automated dependency vulnerability scanning for all repositories:

```yaml
# .github/dependabot.yml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
  - package-ecosystem: "pip"
    directory: "/ai-worker"
    schedule:
      interval: "weekly"
  - package-ecosystem: "npm"
    directory: "/frontend"
    schedule:
      interval: "weekly"
  - package-ecosystem: "docker"
    directory: "/"
    schedule:
      interval: "weekly"
```

### 10.2 CodeRabbit

AI-powered code review for the public repository. Provides automated review comments on pull requests covering:
- Code quality and best practices
- Security vulnerabilities
- Performance issues
- Test coverage gaps

### 10.3 CI/CD Pipeline

```
Push/PR --> Lint --> Test --> Build --> Security Scan --> Deploy
```

| Stage | Go API | Python Worker | Frontend |
|-------|--------|---------------|----------|
| Lint | `golangci-lint` | `ruff` | `eslint` + `prettier` |
| Test | `go test ./...` | `pytest` | `vitest` |
| Build | `go build` (multi-arch) | Docker build | `vite build` |
| Security | Dependabot + `govulncheck` | Dependabot + `safety` | Dependabot + `npm audit` |
| Image scan | Trivy | Trivy | N/A (static assets) |

**Multi-architecture builds:** Go API built for `linux/amd64` and `linux/arm64` in CI. Docker multi-platform manifests via `docker buildx`.

### 10.4 Local Development

```bash
# Clone and setup
git clone https://github.com/raven-platform/raven
cd raven

# Start all services
docker compose up -d

# Run Go API in dev mode (hot reload via air)
cd api && air

# Run Python worker
cd ai-worker && python -m raven_worker

# Run frontend
cd frontend && npm run dev
```

### 10.5 Database Migrations

Managed by `goose` (Go). Migrations are versioned SQL files in `/migrations`:

```
migrations/
  001_create_organizations.sql
  002_create_workspaces.sql
  003_create_users.sql
  ...
```

Run: `goose -dir migrations postgres "$DATABASE_URL" up`

---

## 11. Competitive Positioning

### 11.1 Market Position

Raven operates at the intersection of three markets: RAG-as-a-Service, Voice AI agents, and open-source AI infrastructure. **No single platform offers the full stack** that Raven targets.

### 11.2 Feature Comparison

| Feature | Chatbase | Dify | Vapi | Retell AI | **Raven** |
|---------|----------|------|------|-----------|-----------|
| Document ingestion | Yes | Yes | No | No | **Yes** |
| Web scraping | Yes | Limited | No | No | **Yes** |
| Hybrid search (vector+BM25) | No | Partial | N/A | N/A | **Yes** |
| Reranking | No | Optional | N/A | N/A | **Yes** |
| Embeddable chatbot widget | Yes | No | No | No | **Yes** |
| Voice agent | No | No | Yes | Yes | **Yes (Phase 2)** |
| WebRTC native | No | No | Yes | Yes | **Yes (Phase 2)** |
| WhatsApp calling | No | No | No | No | **Yes (Phase 3)** |
| BYOK (multi-provider LLM) | No | Yes | Yes | Yes | **Yes** |
| Multi-tenancy | No | No | No | No | **Yes** |
| Self-hostable | No | Yes* | No | No | **Yes** |
| Edge deployment | No | No | No | No | **Yes** |

*Dify has license restrictions on hosted service.

### 11.3 Unique Value Proposition

1. **Unified multi-channel platform:** Chat + voice + WhatsApp in one self-hostable package
2. **True multi-tenancy:** Organization > Workspace > Knowledge Base with RLS
3. **Production-grade retrieval:** Hybrid search (pgvector + BM25) with RRF fusion and reranking
4. **Self-hostable with BYOK:** Data sovereignty and cost control
5. **Edge-deployable:** Go API runs on Raspberry Pi (25 MB image, <50ms startup)
6. **Go + Python hybrid:** High concurrency API + rich ML ecosystem

### 11.4 Market Opportunity

| Segment | Competition | Raven's Position |
|---------|-------------|------------------|
| Chatbot-over-docs (simple) | Very high | Differentiate on retrieval quality + multi-tenancy |
| Chatbot-over-docs (enterprise) | Medium | Strong -- self-hostable, BYOK, multi-tenant |
| Voice AI agents with RAG | Low | Strong -- integrated pipeline, self-hostable |
| Multi-channel (chat + voice + WhatsApp) | Very low | First mover potential |
| Edge AI deployment | Very low | Unique -- Go API on ARM64 |

---

## 12. MVP Roadmap and Phasing

### Phase 1 -- MVP (Chatbot) -- Target: 8-12 weeks

**Core:**
- Organization + Workspace + Knowledge Base CRUD (Go API + Echo)
- User auth via Keycloak + reavencloak SPI
- PostgreSQL 18 + pgvector + tsvector (ParadeDB optional)
- Valkey job queue
- SeaweedFS object storage (or local filesystem)

**Ingestion:**
- File upload (PDF, DOCX, images) + URL ingestion
- LiteParse document parsing (subprocess in Python worker)
- Crawl4AI web scraping (Apache 2.0)
- Document-structure-aware chunking (512 tokens, 50 overlap)
- Multi-provider embedding (BYOK)

**Retrieval:**
- Hybrid search (pgvector + BM25 + RRF fusion)
- Reranking (Cohere API or self-hosted BGE reranker)

**Interaction:**
- Embeddable `<raven-chat>` web component with SSE streaming
- API key auth with domain scoping and rate limiting

**Dashboard:**
- Vue.js + Tailwind Plus admin dashboard (mobile-first, PWA-capable)
- Chatbot configurator with live preview
- Test sandbox
- Analytics (conversation volume, top queries)
- API key management

**Deployment:**
- Docker Compose for all services
- Edge deployment mode (Go API on ARM64 + remote Python worker)

### Phase 2 -- Voice Agent -- Target: 4-6 weeks after Phase 1

- LiveKit Server deployment (self-hosted)
- LiveKit Agents integration (Python worker)
- STT: Deepgram Nova-3 (API) initially, faster-whisper (self-hosted) for scale
- TTS: Cartesia Sonic (API) initially, Piper TTS (MIT, self-hosted) for scale
- Silero VAD + LiveKit turn detection
- Same RAG pipeline, voice-optimized (sentence-boundary TTS dispatch)
- Voice session management
- "Call the assistant" button in chatbot widget

### Phase 3 -- WebRTC / WhatsApp -- Target: 4-6 weeks after Phase 2

- WhatsApp Business Calling API integration (direct WebRTC bridge)
- Meta Graph API webhook receiver
- SDP offer/answer exchange
- LiveKit Room bridging for WhatsApp calls
- Browser WebRTC via LiveKit room token endpoint
- WebRTC session management

### Phase 4 -- Knowledge Graph (Future)

- Neo4j or equivalent graph database
- LlamaIndex PropertyGraphIndex for entity extraction and multi-hop queries
- Entity-centric retrieval alongside existing hybrid search
- Graph-enhanced RAG for relational queries

### Phase 5 -- Cloud Managed

- AWS deployment scripts (Terraform modules)
- Hosted cloud offering (managed multi-tenant)
- Pricing strategy (break-even first)
- Multi-region support (EU, US, APAC)
- SOC 2 Type II certification
- GDPR compliance audit

---

## Appendix A: Go Dependency Quick Reference

```bash
go mod init github.com/raven-platform/raven

# Key dependencies
go get github.com/labstack/echo/v4          # Web framework
go get google.golang.org/grpc               # gRPC
go get github.com/jackc/pgx/v5              # PostgreSQL
go get github.com/redis/go-redis/v9         # Valkey (Redis-compatible)
go get github.com/coder/websocket           # WebSocket
go get github.com/pressly/goose/v3          # Migrations
go get github.com/spf13/viper               # Configuration
go get go.opentelemetry.io/otel             # Observability
```

## Appendix B: Full-Text Search Abstraction

The full-text search layer MUST be abstracted to allow swapping between ParadeDB and native PostgreSQL tsvector:

```go
type FullTextSearcher interface {
    IndexChunk(ctx context.Context, chunk Chunk) error
    Search(ctx context.Context, query string, orgID uuid.UUID, kbIDs []uuid.UUID, limit int) ([]SearchResult, error)
    DeleteByDocument(ctx context.Context, documentID uuid.UUID) error
}

// Implementations:
// - ParadeDBSearcher: uses ParadeDB @@ operator (AGPL risk)
// - TsvectorSearcher: uses PostgreSQL native tsvector + ts_rank (no license risk)
```

## Appendix C: LLM Provider Abstraction

```go
type LLMProvider interface {
    Complete(ctx context.Context, req CompletionRequest) (<-chan CompletionChunk, error)
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Implementations: AnthropicProvider, OpenAIProvider, CohereProvider, GoogleProvider, CustomProvider
```

Each organization selects their provider and supplies their own API key. The system supports:
- **Anthropic Claude** as primary (Claude Sonnet for chat quality, Claude Haiku for voice latency)
- **OpenAI** for embeddings and as LLM fallback
- **Cohere** for reranking
- **Google Gemini**, **Azure OpenAI**, **Custom endpoints** as additional options

---

*End of specification. This document governs all implementation work for the Raven platform.*
