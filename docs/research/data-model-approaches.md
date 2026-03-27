# Raven Data Model Design -- Approaches & Trade-offs

**Date:** 2026-03-27
**Status:** Research draft -- no code written

---

## Table of Contents

1. [Platform Hierarchy Recap](#1-platform-hierarchy-recap)
2. [Multi-Tenancy Approaches](#2-multi-tenancy-approaches)
3. [Core Entity Design](#3-core-entity-design)
4. [Strapi Content Type Mapping](#4-strapi-content-type-mapping)
5. [pgvector Table Design & Chunking Strategy](#5-pgvector-table-design--chunking-strategy)
6. [Relationships & Access Control](#6-relationships--access-control)
7. [Document Processing State Machine](#7-document-processing-state-machine)
8. [Recommendation](#8-recommendation)

---

## 1. Platform Hierarchy Recap

```
Application (top-level tenant -- e.g., a SaaS customer)
  └── Organization (team/department within an application)
       └── Knowledge Base (a collection of documents/sources)
            ├── Document (uploaded file: PDF, DOCX, image)
            ├── Source (URL: web page, sitemap, RSS feed)
            └── Chunks → Embeddings (stored in pgvector)

Users belong to Organizations (many-to-many via roles).
Sessions (Chat, Voice) are scoped to a Knowledge Base + User.
```

**Key insight:** The "Application" is the true tenant boundary. Organizations are sub-divisions within an application. Multi-tenancy isolation must happen at the Application level.

---

## 2. Multi-Tenancy Approaches

### Approach A: Shared Schema with Row-Level `application_id` (Recommended)

All tenants share the same database, same schema, same tables. Every table includes an `application_id` column. Isolation is enforced at the application layer and optionally via PostgreSQL Row-Level Security (RLS).

```sql
-- Every tenant-scoped table includes:
application_id UUID NOT NULL REFERENCES applications(id),

-- RLS policy (optional but recommended as defense-in-depth)
ALTER TABLE knowledge_bases ENABLE ROW LEVEL SECURITY;
CREATE POLICY kb_tenant_isolation ON knowledge_bases
  USING (application_id = current_setting('app.current_application_id')::uuid);
```

**Pros:**
- Simplest to implement, deploy, and maintain
- Single database connection pool; efficient resource usage
- Easy cross-tenant analytics/admin queries when needed
- Strapi natively supports this pattern (just add a relation field)
- pgvector indexes shared across tenants (single HNSW index)
- ParadeDB BM25 index works naturally (filter by `application_id` in query)
- Schema migrations are atomic -- one migration applies to all tenants
- Works well up to ~1,000 tenants / 10TB (ParadeDB's proven range)

**Cons:**
- Noisy neighbor risk: one tenant's large ingestion can slow others
- Must be disciplined about always filtering by `application_id`
- RLS adds ~2-5% query overhead (but prevents catastrophic data leaks)
- pgvector HNSW index covers all tenants; very large indexes may degrade
- No physical data isolation (may not satisfy some compliance requirements)

**Mitigation strategies:**
- Use RLS as defense-in-depth (not sole enforcement)
- Add `application_id` to all indexes (composite indexes)
- Connection middleware sets `app.current_application_id` on every request
- Rate-limit ingestion per application
- Partition the embeddings table by `application_id` if scale demands it

---

### Approach B: Schema-Per-Tenant

Each Application gets its own PostgreSQL schema. Tables within each schema are identical, but physically separated.

```sql
-- On application creation:
CREATE SCHEMA app_abc123;

-- Tables live under the schema:
CREATE TABLE app_abc123.knowledge_bases (...);
CREATE TABLE app_abc123.documents (...);
CREATE TABLE app_abc123.embeddings (...);

-- Query routing via search_path:
SET search_path TO app_abc123, public;
```

**Pros:**
- Strong logical isolation between tenants
- No risk of cross-tenant data leakage at the DB level
- Each tenant's pgvector HNSW index is sized only for their data
- Easy to backup/restore/export a single tenant
- Can drop an entire schema to offboard a tenant cleanly
- Satisfies compliance requirements that demand data separation
- No need for `application_id` on every table -- isolation is structural

**Cons:**
- Schema proliferation: 1,000 tenants = 1,000 schemas = tens of thousands of tables
- PostgreSQL catalog bloat (pg_class, pg_attribute grow large)
- Schema migrations must be applied to EVERY schema (migration tooling needed)
- Connection pool management becomes complex (schema switching per request)
- Strapi does NOT natively support schema-per-tenant; requires custom plugin or bypassing Strapi for tenant-scoped tables
- ParadeDB BM25 indexes duplicated per schema (more memory, more maintenance)
- Cross-tenant admin queries require UNION across schemas or separate admin views
- pgvector HNSW indexes per schema: each is smaller (good for query) but more total memory used

**When this makes sense:**
- Strict compliance requirements (healthcare, finance, government)
- Small number of tenants (<50) with large data per tenant
- Tenants need the ability to export/import their own data independently

---

### Approach C: Database-Per-Tenant

Each Application gets its own PostgreSQL database. Maximum isolation.

```
raven_app_abc123  (database)
  └── public schema
       ├── knowledge_bases
       ├── documents
       ├── embeddings
       └── ...

raven_app_def456  (database)
  └── public schema
       └── ...
```

**Pros:**
- Complete physical isolation
- Independent backup, restore, and scaling per tenant
- Can place high-value tenants on dedicated hardware
- No catalog bloat within any single database
- Simplest per-tenant data management (drop database = full cleanup)

**Cons:**
- Highest operational complexity by far
- Separate connection pools per database (connection explosion)
- Schema migrations must be coordinated across all databases
- Strapi cannot manage multiple databases natively
- Keycloak integration becomes more complex (realm-per-database?)
- No cross-tenant queries without foreign data wrappers
- pgvector and ParadeDB extensions must be installed per database
- Resource waste: idle tenants still consume connection slots
- Significantly harder to implement in cloud-managed Postgres (RDS, Cloud SQL)

**When this makes sense:**
- Enterprise on-premises deployments where tenants demand physical separation
- Very few tenants (<10) with extremely large datasets
- Regulated industries with data residency requirements (different databases in different regions)

---

### Approach Comparison Matrix

| Criterion | A: Shared + RLS | B: Schema-per-tenant | C: DB-per-tenant |
|-----------|----------------|---------------------|------------------|
| **Implementation effort** | Low | Medium | High |
| **Strapi compatibility** | Native | Requires custom work | Not feasible |
| **Keycloak integration** | Simple | Medium | Complex |
| **Data isolation** | Logical (RLS) | Logical (schema) | Physical |
| **Migration complexity** | Single migration | Per-schema migration | Per-database migration |
| **pgvector performance** | Single large index | Per-tenant indexes | Per-tenant indexes |
| **ParadeDB BM25** | Single index, filter | Per-schema indexes | Per-database indexes |
| **Max tenants** | 1,000+ | ~50-100 | ~10-20 |
| **Noisy neighbor risk** | Medium | Low | None |
| **Cross-tenant admin** | Easy (SQL) | Possible (UNION) | Hard (FDW) |
| **Tenant offboarding** | DELETE WHERE | DROP SCHEMA | DROP DATABASE |
| **Compliance** | Standard | Enhanced | Maximum |
| **Operational cost** | Lowest | Medium | Highest |

---

## 3. Core Entity Design

All designs below assume **Approach A (shared schema with RLS)**, as it is the recommended starting point. Adjust by removing `application_id` and using schema qualification if you choose Approach B.

### 3.1 Applications

The top-level tenant. Created when a new SaaS customer signs up. Maps 1:1 with a Keycloak realm (or a dedicated client within a shared realm).

```sql
CREATE TABLE applications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(100) NOT NULL UNIQUE,  -- URL-friendly identifier
    status          VARCHAR(20) NOT NULL DEFAULT 'active',  -- active, suspended, deactivated
    settings        JSONB DEFAULT '{}',  -- per-app config (rate limits, feature flags, etc.)
    keycloak_realm  VARCHAR(255),  -- Keycloak realm or client ID
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.2 Organizations

Sub-divisions within an Application. Think "teams" or "departments."

```sql
CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(100) NOT NULL,
    settings        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (application_id, slug)
);

CREATE INDEX idx_orgs_app ON organizations(application_id);
```

### 3.3 Users

Users are primarily managed by Keycloak. The local `users` table is a mirror/cache of Keycloak user data, synced via the custom SPI or webhooks.

```sql
CREATE TABLE users (
    id              UUID PRIMARY KEY,  -- same UUID as Keycloak user ID
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    email           VARCHAR(255) NOT NULL,
    display_name    VARCHAR(255),
    avatar_url      TEXT,
    keycloak_sub    VARCHAR(255) NOT NULL,  -- Keycloak subject identifier
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (application_id, email),
    UNIQUE (keycloak_sub)
);
```

### 3.4 User-Organization Membership (many-to-many with roles)

```sql
CREATE TABLE organization_members (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            VARCHAR(50) NOT NULL DEFAULT 'member',  -- owner, admin, member, viewer
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (organization_id, user_id)
);

CREATE INDEX idx_orgmembers_user ON organization_members(user_id);
CREATE INDEX idx_orgmembers_org ON organization_members(organization_id);
```

### 3.5 Knowledge Bases

A knowledge base is a collection of documents and sources within an organization. It is the primary unit of RAG retrieval.

```sql
CREATE TABLE knowledge_bases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(100) NOT NULL,
    description     TEXT,
    settings        JSONB DEFAULT '{}',  -- chunk size, overlap, embedding model, etc.
    status          VARCHAR(20) NOT NULL DEFAULT 'active',  -- active, archived
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (organization_id, slug)
);

CREATE INDEX idx_kb_app ON knowledge_bases(application_id);
CREATE INDEX idx_kb_org ON knowledge_bases(organization_id);
```

### 3.6 Documents (uploaded files)

Represents a file uploaded to a knowledge base (PDF, DOCX, images, etc.). Processed by LiteParse.

```sql
CREATE TABLE documents (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id      UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    knowledge_base_id   UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,

    -- File metadata
    file_name           VARCHAR(500) NOT NULL,
    file_type           VARCHAR(50) NOT NULL,   -- pdf, docx, xlsx, pptx, png, jpg, etc.
    file_size_bytes     BIGINT,
    file_hash           VARCHAR(128),           -- SHA-256 for deduplication
    storage_path        TEXT NOT NULL,           -- S3/MinIO path or local path

    -- Processing state (see Section 7)
    processing_status   VARCHAR(20) NOT NULL DEFAULT 'queued',
    processing_error    TEXT,
    processing_started_at TIMESTAMPTZ,
    processing_completed_at TIMESTAMPTZ,

    -- Extracted content
    title               VARCHAR(500),           -- extracted or user-provided
    page_count          INTEGER,
    word_count          INTEGER,
    language            VARCHAR(10),            -- detected language (ISO 639-1)

    -- Metadata
    metadata            JSONB DEFAULT '{}',     -- arbitrary key-value pairs
    uploaded_by         UUID REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_docs_kb ON documents(knowledge_base_id);
CREATE INDEX idx_docs_app ON documents(application_id);
CREATE INDEX idx_docs_status ON documents(processing_status);
CREATE INDEX idx_docs_hash ON documents(file_hash);
```

### 3.7 Sources (URLs / web content)

Represents a web source to be scraped and ingested. Handled by the separate web scraping tool.

```sql
CREATE TABLE sources (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id      UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    knowledge_base_id   UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,

    -- Source definition
    source_type         VARCHAR(20) NOT NULL,   -- url, sitemap, rss_feed
    url                 TEXT NOT NULL,
    crawl_depth         INTEGER DEFAULT 1,      -- for sitemaps: how deep to crawl
    crawl_frequency     VARCHAR(20),            -- manual, daily, weekly, monthly

    -- Processing state
    processing_status   VARCHAR(20) NOT NULL DEFAULT 'queued',
    processing_error    TEXT,
    last_crawled_at     TIMESTAMPTZ,

    -- Extracted content summary
    title               VARCHAR(500),
    pages_crawled       INTEGER DEFAULT 0,

    -- Metadata
    metadata            JSONB DEFAULT '{}',
    created_by          UUID REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sources_kb ON sources(knowledge_base_id);
CREATE INDEX idx_sources_app ON sources(application_id);
CREATE INDEX idx_sources_status ON sources(processing_status);
```

### 3.8 Chunks (the unit of retrieval)

A chunk is a segment of text extracted from a Document or Source. This is the fundamental unit that gets embedded and retrieved during RAG.

```sql
CREATE TABLE chunks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id      UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    knowledge_base_id   UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,

    -- Parent reference (polymorphic: either a document or a source)
    document_id         UUID REFERENCES documents(id) ON DELETE CASCADE,
    source_id           UUID REFERENCES sources(id) ON DELETE CASCADE,

    -- Chunk content
    content             TEXT NOT NULL,           -- the actual text of the chunk
    chunk_index         INTEGER NOT NULL,        -- ordering within the parent
    token_count         INTEGER,                 -- token count for the chunk

    -- Position in original document
    page_number         INTEGER,                 -- for PDFs
    start_char          INTEGER,                 -- character offset in original
    end_char            INTEGER,

    -- Structural metadata
    heading             VARCHAR(500),            -- nearest heading/section title
    chunk_type          VARCHAR(50) DEFAULT 'text',  -- text, table, image_caption, code

    -- Metadata for filtering
    metadata            JSONB DEFAULT '{}',

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Ensure chunk belongs to exactly one parent
    CONSTRAINT chk_parent CHECK (
        (document_id IS NOT NULL AND source_id IS NULL) OR
        (document_id IS NULL AND source_id IS NOT NULL)
    )
);

CREATE INDEX idx_chunks_kb ON chunks(knowledge_base_id);
CREATE INDEX idx_chunks_app ON chunks(application_id);
CREATE INDEX idx_chunks_doc ON chunks(document_id);
CREATE INDEX idx_chunks_source ON chunks(source_id);

-- ParadeDB BM25 index for full-text search
-- (Requires pg_search extension)
CALL paradedb.create_bm25(
    index_name => 'idx_chunks_bm25',
    table_name => 'chunks',
    key_field  => 'id',
    text_fields => paradedb.field('content', tokenizer => paradedb.tokenizer('default'))
                   || paradedb.field('heading')
                   || paradedb.field('chunk_type'),
    json_fields => paradedb.field('metadata')
);
```

### 3.9 Embeddings (pgvector)

Separated from chunks to allow multiple embedding models and easy re-embedding.

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE embeddings (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id      UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    chunk_id            UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,

    -- Embedding vector
    embedding           vector(1536),           -- dimension depends on model
                                                -- OpenAI text-embedding-3-small: 1536
                                                -- Cohere embed-v3: 1024
                                                -- BGE-large: 1024

    -- Model metadata (allows re-embedding with different models)
    model_name          VARCHAR(100) NOT NULL,   -- e.g., 'text-embedding-3-small'
    model_version       VARCHAR(50),
    dimensions          INTEGER NOT NULL,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (chunk_id, model_name)  -- one embedding per chunk per model
);

-- HNSW index for fast approximate nearest neighbor search
-- m = 16, ef_construction = 64 are good defaults; tune based on dataset size
CREATE INDEX idx_embeddings_hnsw ON embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

CREATE INDEX idx_embeddings_app ON embeddings(application_id);
CREATE INDEX idx_embeddings_chunk ON embeddings(chunk_id);
```

### 3.10 Chat Sessions

```sql
CREATE TABLE chat_sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id      UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    knowledge_base_id   UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    title               VARCHAR(500),           -- auto-generated or user-provided
    status              VARCHAR(20) NOT NULL DEFAULT 'active',  -- active, archived
    settings            JSONB DEFAULT '{}',     -- model, temperature, system prompt, etc.

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chatsessions_kb ON chat_sessions(knowledge_base_id);
CREATE INDEX idx_chatsessions_user ON chat_sessions(user_id);
CREATE INDEX idx_chatsessions_app ON chat_sessions(application_id);
```

### 3.11 Chat Messages

```sql
CREATE TABLE chat_messages (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_session_id     UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,

    role                VARCHAR(20) NOT NULL,    -- user, assistant, system
    content             TEXT NOT NULL,
    token_count         INTEGER,

    -- RAG context: which chunks were retrieved for this message
    retrieved_chunk_ids UUID[],                  -- array of chunk IDs used as context
    retrieval_scores    JSONB,                   -- {chunk_id: score} for transparency

    -- Model metadata
    model_name          VARCHAR(100),
    model_latency_ms    INTEGER,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chatmessages_session ON chat_messages(chat_session_id);
CREATE INDEX idx_chatmessages_created ON chat_messages(chat_session_id, created_at);
```

### 3.12 Voice Sessions

```sql
CREATE TABLE voice_sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id      UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    knowledge_base_id   UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Voice-specific metadata
    channel             VARCHAR(20) NOT NULL,    -- browser_webrtc, whatsapp
    transport           VARCHAR(50),             -- livekit, pipecat, daily
    room_id             VARCHAR(255),            -- LiveKit room ID or equivalent
    call_id             VARCHAR(255),            -- WhatsApp call_id if applicable

    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    -- active, completed, failed, dropped
    started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at            TIMESTAMPTZ,
    duration_seconds    INTEGER,

    -- Settings
    settings            JSONB DEFAULT '{}',     -- STT model, TTS voice, LLM config

    -- Optional: linked chat session for transcript
    chat_session_id     UUID REFERENCES chat_sessions(id),

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_voicesessions_kb ON voice_sessions(knowledge_base_id);
CREATE INDEX idx_voicesessions_user ON voice_sessions(user_id);
CREATE INDEX idx_voicesessions_app ON voice_sessions(application_id);
```

### 3.13 Voice Turns (transcript of voice interactions)

```sql
CREATE TABLE voice_turns (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    voice_session_id    UUID NOT NULL REFERENCES voice_sessions(id) ON DELETE CASCADE,

    role                VARCHAR(20) NOT NULL,    -- user, assistant
    transcript          TEXT NOT NULL,           -- STT output (user) or TTS input (assistant)
    audio_url           TEXT,                    -- optional: stored audio segment

    -- Timing
    started_at          TIMESTAMPTZ NOT NULL,
    ended_at            TIMESTAMPTZ,
    duration_ms         INTEGER,

    -- Latency tracking
    stt_latency_ms      INTEGER,
    llm_latency_ms      INTEGER,
    tts_latency_ms      INTEGER,

    -- RAG context
    retrieved_chunk_ids UUID[],

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_voiceturns_session ON voice_turns(voice_session_id);
```

---

## 4. Strapi Content Type Mapping

### What Strapi Manages vs. What PostgreSQL Manages Directly

Strapi acts as the CMS for **configuration and metadata** -- the entities that admins create and manage through a UI. The high-throughput, embedding-heavy tables are managed directly in PostgreSQL (not through Strapi).

| Entity | Managed By | Reason |
|--------|-----------|--------|
| Application | **Strapi** | Admin CRUD, low volume, needs UI |
| Organization | **Strapi** | Admin CRUD, low volume, needs UI |
| User (profile mirror) | **Strapi** | Synced from Keycloak, displayed in admin UI |
| Knowledge Base | **Strapi** | Admin CRUD, settings management, needs UI |
| Document (metadata) | **Strapi** | File upload UI, metadata editing |
| Source (URL config) | **Strapi** | URL management UI, crawl settings |
| Chunk | **PostgreSQL directly** | High volume, created by processing pipeline, no admin UI needed |
| Embedding | **PostgreSQL directly** | pgvector, high volume, created by pipeline |
| Chat Session | **PostgreSQL directly** | High throughput, real-time |
| Chat Message | **PostgreSQL directly** | High throughput, real-time |
| Voice Session | **PostgreSQL directly** | High throughput, real-time |
| Voice Turn | **PostgreSQL directly** | High throughput, real-time |
| Organization Member | **Strapi** | Role management UI |

### Strapi Content Type Definitions

```javascript
// api/application/content-types/application/schema.json
{
  "kind": "collectionType",
  "collectionName": "applications",
  "attributes": {
    "name": { "type": "string", "required": true },
    "slug": { "type": "uid", "targetField": "name" },
    "status": { "type": "enumeration", "enum": ["active", "suspended", "deactivated"], "default": "active" },
    "settings": { "type": "json" },
    "keycloak_realm": { "type": "string" },
    "organizations": { "type": "relation", "relation": "oneToMany", "target": "api::organization.organization", "mappedBy": "application" }
  }
}

// api/organization/content-types/organization/schema.json
{
  "kind": "collectionType",
  "collectionName": "organizations",
  "attributes": {
    "name": { "type": "string", "required": true },
    "slug": { "type": "uid", "targetField": "name" },
    "settings": { "type": "json" },
    "application": { "type": "relation", "relation": "manyToOne", "target": "api::application.application", "inversedBy": "organizations" },
    "knowledge_bases": { "type": "relation", "relation": "oneToMany", "target": "api::knowledge-base.knowledge-base", "mappedBy": "organization" },
    "members": { "type": "relation", "relation": "manyToMany", "target": "plugin::users-permissions.user" }
  }
}

// api/knowledge-base/content-types/knowledge-base/schema.json
{
  "kind": "collectionType",
  "collectionName": "knowledge_bases",
  "attributes": {
    "name": { "type": "string", "required": true },
    "slug": { "type": "uid", "targetField": "name" },
    "description": { "type": "text" },
    "settings": { "type": "json" },
    "status": { "type": "enumeration", "enum": ["active", "archived"], "default": "active" },
    "organization": { "type": "relation", "relation": "manyToOne", "target": "api::organization.organization", "inversedBy": "knowledge_bases" },
    "application": { "type": "relation", "relation": "manyToOne", "target": "api::application.application" },
    "documents": { "type": "relation", "relation": "oneToMany", "target": "api::document.document", "mappedBy": "knowledge_base" },
    "sources": { "type": "relation", "relation": "oneToMany", "target": "api::source.source", "mappedBy": "knowledge_base" }
  }
}

// api/document/content-types/document/schema.json
{
  "kind": "collectionType",
  "collectionName": "documents",
  "attributes": {
    "file_name": { "type": "string", "required": true },
    "file": { "type": "media", "allowedTypes": ["files", "images"] },
    "file_type": { "type": "string" },
    "file_size_bytes": { "type": "biginteger" },
    "file_hash": { "type": "string" },
    "processing_status": {
      "type": "enumeration",
      "enum": ["queued", "parsing", "chunking", "embedding", "ready", "failed", "reprocessing"],
      "default": "queued"
    },
    "processing_error": { "type": "text" },
    "title": { "type": "string" },
    "page_count": { "type": "integer" },
    "word_count": { "type": "integer" },
    "language": { "type": "string" },
    "metadata": { "type": "json" },
    "knowledge_base": { "type": "relation", "relation": "manyToOne", "target": "api::knowledge-base.knowledge-base", "inversedBy": "documents" },
    "application": { "type": "relation", "relation": "manyToOne", "target": "api::application.application" },
    "uploaded_by": { "type": "relation", "relation": "manyToOne", "target": "plugin::users-permissions.user" }
  }
}

// api/source/content-types/source/schema.json
{
  "kind": "collectionType",
  "collectionName": "sources",
  "attributes": {
    "source_type": { "type": "enumeration", "enum": ["url", "sitemap", "rss_feed"], "required": true },
    "url": { "type": "string", "required": true },
    "crawl_depth": { "type": "integer", "default": 1 },
    "crawl_frequency": { "type": "enumeration", "enum": ["manual", "daily", "weekly", "monthly"] },
    "processing_status": {
      "type": "enumeration",
      "enum": ["queued", "crawling", "parsing", "chunking", "embedding", "ready", "failed"],
      "default": "queued"
    },
    "processing_error": { "type": "text" },
    "title": { "type": "string" },
    "pages_crawled": { "type": "integer", "default": 0 },
    "metadata": { "type": "json" },
    "knowledge_base": { "type": "relation", "relation": "manyToOne", "target": "api::knowledge-base.knowledge-base", "inversedBy": "sources" },
    "application": { "type": "relation", "relation": "manyToOne", "target": "api::application.application" },
    "created_by_user": { "type": "relation", "relation": "manyToOne", "target": "plugin::users-permissions.user" }
  }
}
```

### Strapi-to-PostgreSQL Bridge Pattern

Strapi manages the CRUD lifecycle for documents and sources. When a document is created in Strapi, a lifecycle hook triggers the processing pipeline:

```javascript
// api/document/content-types/document/lifecycles.js
module.exports = {
  async afterCreate(event) {
    const { result } = event;
    // Publish to processing queue (Redis/BullMQ, RabbitMQ, or Postgres NOTIFY)
    await queue.add('process-document', {
      documentId: result.id,
      applicationId: result.application.id,
      knowledgeBaseId: result.knowledge_base.id,
    });
  },
};
```

The processing worker writes chunks and embeddings directly to PostgreSQL (bypassing Strapi's ORM for performance).

---

## 5. pgvector Table Design & Chunking Strategy

### 5.1 Chunking Strategy

The chunking strategy depends on the document type and the use case:

| Strategy | Chunk Size | Overlap | Best For |
|----------|-----------|---------|----------|
| **Fixed-size token** | 512 tokens | 50 tokens (~10%) | General-purpose, simple |
| **Semantic (paragraph)** | Variable (100-800 tokens) | Sentence overlap | Well-structured documents |
| **Recursive character** | 1000 chars | 200 chars | Markdown, code |
| **Heading-based** | Variable (section) | None | Documents with clear headings |
| **Page-based** | 1 page | None | PDFs where page boundaries matter |
| **Table-aware** | 1 table | None | Spreadsheets, data-heavy docs |

**Recommended default:** Fixed-size token chunking at 512 tokens with 50-token overlap. This is the most predictable for embedding quality and retrieval. Allow per-knowledge-base override via `knowledge_bases.settings`:

```json
{
  "chunking": {
    "strategy": "fixed_token",
    "chunk_size": 512,
    "overlap": 50,
    "respect_headings": true
  },
  "embedding": {
    "model": "text-embedding-3-small",
    "dimensions": 1536
  }
}
```

### 5.2 Embedding Model Considerations

| Model | Dimensions | Cost | Quality | Notes |
|-------|-----------|------|---------|-------|
| `text-embedding-3-small` (OpenAI) | 1536 | $0.02/1M tokens | Good | Best cost/quality ratio |
| `text-embedding-3-large` (OpenAI) | 3072 | $0.13/1M tokens | Best | Supports dimension reduction |
| `embed-v3` (Cohere) | 1024 | $0.10/1M tokens | Very good | Supports multilingual natively |
| `BGE-large-en-v1.5` | 1024 | Free (self-hosted) | Good | Self-hosted option |
| `nomic-embed-text` | 768 | Free (self-hosted) | Good | Lightweight, fast |

**Design decision:** The `embeddings` table stores the model name and dimensions, allowing different knowledge bases to use different models. The HNSW index is dimension-specific, so if you support multiple dimensions, you need separate indexes or use a fixed dimension with padding.

**Practical recommendation:** Standardize on one model (e.g., `text-embedding-3-small` at 1536 dimensions) for the initial release. The schema supports future model additions.

### 5.3 Retrieval Query Pattern

Hybrid search combining pgvector (semantic) + ParadeDB (keyword):

```sql
-- Step 1: Semantic search via pgvector
WITH semantic AS (
    SELECT
        c.id AS chunk_id,
        c.content,
        c.heading,
        c.metadata,
        1 - (e.embedding <=> $1::vector) AS semantic_score,  -- $1 = query embedding
        ROW_NUMBER() OVER (ORDER BY e.embedding <=> $1::vector) AS semantic_rank
    FROM embeddings e
    JOIN chunks c ON c.id = e.chunk_id
    WHERE e.application_id = $2    -- tenant filter
      AND c.knowledge_base_id = $3 -- scope to knowledge base
      AND e.model_name = 'text-embedding-3-small'
    ORDER BY e.embedding <=> $1::vector
    LIMIT 20
),

-- Step 2: Keyword search via ParadeDB BM25
keyword AS (
    SELECT
        c.id AS chunk_id,
        c.content,
        c.heading,
        c.metadata,
        paradedb.score(c.id) AS keyword_score,
        ROW_NUMBER() OVER (ORDER BY paradedb.score(c.id) DESC) AS keyword_rank
    FROM chunks c
    WHERE c.application_id = $2
      AND c.knowledge_base_id = $3
      AND c.id @@@ paradedb.parse($4)  -- $4 = keyword query
    LIMIT 20
),

-- Step 3: Reciprocal Rank Fusion (RRF)
combined AS (
    SELECT
        COALESCE(s.chunk_id, k.chunk_id) AS chunk_id,
        COALESCE(s.content, k.content) AS content,
        COALESCE(s.heading, k.heading) AS heading,
        COALESCE(s.metadata, k.metadata) AS metadata,
        -- RRF formula: score = sum(1 / (k + rank)) where k = 60 (constant)
        COALESCE(1.0 / (60 + s.semantic_rank), 0) +
        COALESCE(1.0 / (60 + k.keyword_rank), 0) AS rrf_score
    FROM semantic s
    FULL OUTER JOIN keyword k ON s.chunk_id = k.chunk_id
)
SELECT chunk_id, content, heading, metadata, rrf_score
FROM combined
ORDER BY rrf_score DESC
LIMIT 10;
```

### 5.4 Index Tuning Notes

For HNSW indexes:
- **m** (max connections per node): 16 is good default. Increase to 32-64 for higher recall at cost of memory.
- **ef_construction**: 64 is good default. Increase for better index quality at cost of build time.
- **ef_search** (set at query time): Default 40. Increase for better recall: `SET hnsw.ef_search = 100;`

For large datasets (>1M embeddings):
- Consider partitioning the `embeddings` table by `application_id` (if one tenant dominates)
- Or partition by `knowledge_base_id` if knowledge bases are very large
- Each partition gets its own HNSW index, keeping index size manageable

```sql
-- Partitioned embeddings table (if needed at scale)
CREATE TABLE embeddings (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL,
    chunk_id        UUID NOT NULL,
    embedding       vector(1536),
    model_name      VARCHAR(100) NOT NULL,
    model_version   VARCHAR(50),
    dimensions      INTEGER NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
) PARTITION BY HASH (application_id);

-- Create partitions
CREATE TABLE embeddings_p0 PARTITION OF embeddings FOR VALUES WITH (MODULUS 8, REMAINDER 0);
CREATE TABLE embeddings_p1 PARTITION OF embeddings FOR VALUES WITH (MODULUS 8, REMAINDER 1);
-- ... up to p7

-- HNSW index on each partition (auto-created if defined on parent in PG 15+)
```

---

## 6. Relationships & Access Control

### 6.1 Entity Relationship Diagram (text representation)

```
Application (1) ──────────── (*) Organization
    │                              │
    │                              ├── (*) OrganizationMember ──── (*) User
    │                              │         (role: owner/admin/member/viewer)
    │                              │
    │                              └── (*) KnowledgeBase
    │                                       │
    │                                       ├── (*) Document
    │                                       │       └── (*) Chunk
    │                                       │              └── (*) Embedding
    │                                       │
    │                                       ├── (*) Source
    │                                       │       └── (*) Chunk
    │                                       │              └── (*) Embedding
    │                                       │
    │                                       ├── (*) ChatSession
    │                                       │       └── (*) ChatMessage
    │                                       │
    │                                       └── (*) VoiceSession
    │                                               └── (*) VoiceTurn
    │
    └── (*) User
```

### 6.2 Access Control Layers

Access control is enforced at **three layers**:

#### Layer 1: Authentication (Keycloak)

Keycloak handles identity verification. The JWT token contains:

```json
{
  "sub": "user-uuid",
  "realm_access": { "roles": ["app_admin", "user"] },
  "resource_access": {
    "raven-api": { "roles": ["kb_read", "kb_write", "admin"] }
  },
  "application_id": "app-uuid",      // custom claim via SPI
  "organization_ids": ["org-uuid-1"]  // custom claim via SPI
}
```

#### Layer 2: Application-Level Authorization (API middleware)

Every API request is scoped to an `application_id` (from the JWT). The middleware:
1. Extracts `application_id` from the JWT
2. Sets `SET app.current_application_id = 'uuid'` on the database connection (for RLS)
3. Validates that the requested resource belongs to the user's application

#### Layer 3: Organization/KB-Level Authorization (business logic)

Within an application, access is further scoped:
- **Organization scope:** User can only access KBs in orgs they belong to
- **Role-based access:**
  - `owner`: Full control (CRUD on org, KB, members)
  - `admin`: Manage KB, documents, sources; cannot delete org
  - `member`: Read KB, chat, voice; upload documents
  - `viewer`: Read-only access to KB content and chat history

```python
# Pseudocode for access check
def check_kb_access(user, knowledge_base, required_role='viewer'):
    membership = get_membership(user.id, knowledge_base.organization_id)
    if not membership:
        raise Forbidden("Not a member of this organization")
    if ROLE_HIERARCHY[membership.role] < ROLE_HIERARCHY[required_role]:
        raise Forbidden(f"Requires {required_role} role")
```

#### Layer 4: Row-Level Security (defense-in-depth)

RLS policies on all tenant-scoped tables ensure that even if application code has a bug, cross-tenant data access is impossible:

```sql
-- Applied to all tenant-scoped tables
ALTER TABLE knowledge_bases ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE chunks ENABLE ROW LEVEL SECURITY;
ALTER TABLE embeddings ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE voice_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE voice_turns ENABLE ROW LEVEL SECURITY;

-- Example policy (same pattern for all tables)
CREATE POLICY tenant_isolation ON knowledge_bases
    FOR ALL
    USING (application_id = current_setting('app.current_application_id')::uuid)
    WITH CHECK (application_id = current_setting('app.current_application_id')::uuid);

-- Admin bypass role (for cross-tenant admin operations)
CREATE ROLE raven_admin;
GRANT ALL ON ALL TABLES IN SCHEMA public TO raven_admin;
ALTER TABLE knowledge_bases ENABLE ROW LEVEL SECURITY;
CREATE POLICY admin_bypass ON knowledge_bases
    FOR ALL
    TO raven_admin
    USING (true);
```

### 6.3 API Key Access (for programmatic access)

Applications may also need API key access for integrations:

```sql
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,  -- optional: scope to org
    knowledge_base_id UUID REFERENCES knowledge_bases(id) ON DELETE CASCADE,  -- optional: scope to KB

    key_hash        VARCHAR(128) NOT NULL,  -- SHA-256 of the actual key (never store plaintext)
    key_prefix      VARCHAR(10) NOT NULL,   -- first 8 chars for identification (e.g., "rvn_abc1")
    name            VARCHAR(255) NOT NULL,
    permissions     VARCHAR(50)[] NOT NULL,  -- ['kb:read', 'kb:write', 'chat:create']
    expires_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (key_hash)
);

CREATE INDEX idx_apikeys_hash ON api_keys(key_hash);
CREATE INDEX idx_apikeys_app ON api_keys(application_id);
```

---

## 7. Document Processing State Machine

### 7.1 State Diagram

```
                    ┌─────────────────────────┐
                    │                         │
                    v                         │
  ┌─────────┐   ┌────────┐   ┌──────────┐   │   ┌───────────┐   ┌───────┐
  │ queued  │──>│parsing │──>│ chunking │──>│──>│ embedding │──>│ ready │
  └─────────┘   └────────┘   └──────────┘   │   └───────────┘   └───────┘
       │             │             │         │         │
       │             │             │         │         │
       │             v             v         │         v
       │        ┌────────┐   ┌────────┐     │    ┌────────┐
       │        │ failed │   │ failed │     │    │ failed │
       │        └────────┘   └────────┘     │    └────────┘
       │             │             │         │         │
       │             └─────────────┘─────────┘─────────┘
       │                           │
       │                           v
       │                    ┌──────────────┐
       └───────────────────>│ reprocessing │ (manual retry)
                            └──────────────┘
```

### 7.2 Processing States

| State | Description | Next States |
|-------|-------------|-------------|
| `queued` | Document/source created, waiting in processing queue | `parsing`, `crawling` (sources), `reprocessing` |
| `crawling` | (Sources only) Web scraper is fetching the URL | `parsing`, `failed` |
| `parsing` | LiteParse is extracting text from the document | `chunking`, `failed` |
| `chunking` | Text is being split into chunks | `embedding`, `failed` |
| `embedding` | Chunks are being embedded via the embedding model | `ready`, `failed` |
| `ready` | All processing complete, chunks and embeddings available for retrieval | `reprocessing` |
| `failed` | Processing failed at some stage; `processing_error` contains details | `reprocessing` |
| `reprocessing` | Manual retry triggered; re-enters pipeline from the beginning | `parsing`, `crawling` |

### 7.3 Processing Pipeline Implementation Pattern

```python
# Worker pseudocode (BullMQ, Celery, or custom)

async def process_document(job):
    doc_id = job.data['documentId']

    try:
        # Stage 1: Parse
        await update_status(doc_id, 'parsing')
        parsed = await liteparse.parse(doc_id)  # PDF -> text, images -> OCR, etc.

        # Stage 2: Chunk
        await update_status(doc_id, 'chunking')
        chunks = await chunker.chunk(parsed, settings=get_kb_settings(doc_id))
        await bulk_insert_chunks(chunks)

        # Stage 3: Embed
        await update_status(doc_id, 'embedding')
        embeddings = await embed_chunks(chunks, model=get_kb_settings(doc_id).embedding.model)
        await bulk_insert_embeddings(embeddings)

        # Done
        await update_status(doc_id, 'ready')

    except Exception as e:
        await update_status(doc_id, 'failed', error=str(e))
        raise  # Let the job queue handle retries
```

### 7.4 Processing Events Table (audit log)

```sql
CREATE TABLE processing_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL,

    -- Polymorphic reference
    document_id     UUID REFERENCES documents(id) ON DELETE CASCADE,
    source_id       UUID REFERENCES sources(id) ON DELETE CASCADE,

    -- Event details
    from_status     VARCHAR(20),
    to_status       VARCHAR(20) NOT NULL,
    error_message   TEXT,
    metadata        JSONB DEFAULT '{}',   -- timing, chunk count, token count, etc.

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_processing_parent CHECK (
        (document_id IS NOT NULL AND source_id IS NULL) OR
        (document_id IS NULL AND source_id IS NOT NULL)
    )
);

CREATE INDEX idx_processing_events_doc ON processing_events(document_id);
CREATE INDEX idx_processing_events_source ON processing_events(source_id);
```

---

## 8. Recommendation

### Recommended Starting Point

**Multi-tenancy: Approach A (Shared Schema with RLS)**

Rationale:
- Simplest to implement and operationally cheapest
- Native Strapi compatibility (no custom schema routing)
- Single pgvector HNSW index, single ParadeDB BM25 index
- RLS provides defense-in-depth against cross-tenant data leaks
- Can evolve to Approach B later if compliance demands arise (the schema is compatible)
- Scales to 1,000+ tenants within PostgreSQL's comfortable range

### Migration Path (if needed later)

```
Phase 1 (now):     Approach A -- Shared schema + RLS
Phase 2 (if needed): Approach A with partitioned embeddings (by application_id)
Phase 3 (if needed): Approach B for high-security tenants (hybrid: most tenants on shared, premium on dedicated schema)
```

### Key Design Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Multi-tenancy | Shared schema + RLS | Simplest, Strapi-compatible |
| Tenant column | `application_id` on all tables | Consistent, indexable |
| Chunks separate from embeddings | Yes | Allows re-embedding, multiple models |
| Strapi scope | Config/metadata entities only | High-throughput tables bypass Strapi |
| Processing state | Status column + events table | Simple state machine + audit trail |
| Default chunking | 512 tokens, 50 overlap | Predictable, configurable per KB |
| Default embedding | text-embedding-3-small (1536d) | Good cost/quality, single HNSW index |
| Hybrid search | RRF fusion of pgvector + ParadeDB | Until ParadeDB ships native hybrid |
| Access control | Keycloak JWT + API middleware + RLS | Defense in depth |
