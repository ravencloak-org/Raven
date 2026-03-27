# Raven -- Data Model

**Status:** Design document
**Last updated:** 2026-03-27

---

## 1. Hierarchy Naming: Recommendation

**Recommended: Organization -> Workspace -> Knowledge Base** (Slack-like naming)

Given the Alphabet example (top org containing Google, Chrome, Android as sub-units), here is why "Workspace" is the right name for the second tier:

- **"Workspace" implies autonomy with shared infrastructure.** Google, Chrome, and Android within Alphabet each operate as self-contained units with their own people, budgets, and knowledge -- but share a billing relationship and identity layer. "Workspace" captures this: it is a bounded context that belongs to a parent but is operationally independent.
- **"Team" (GitHub-like) is too small.** Teams imply people grouped by function (frontend team, design team). Google is not a "team" within Alphabet -- it is an entire business unit. Team also conflates organizational grouping with access control grouping.
- **"Project" (GCP-like) is too ephemeral.** Projects imply a finite scope of work. A knowledge base platform's sub-units are long-lived organizational divisions, not time-bound deliverables.
- **"Tenant -> Organization -> Project" (three tiers) is over-engineered.** Adding an explicit tenant layer above organization creates confusion. The organization already IS the tenant boundary for billing, auth, and data isolation. A third tier adds API complexity and mental overhead with no practical benefit at this stage.
- **Workspace aligns with established SaaS conventions.** Slack, Notion, and Vercel all use this pattern. Users intuitively understand that a workspace is "a place where a group works together" scoped under a larger organizational umbrella.

Final hierarchy:

```
Organization (tenant boundary -- billing, auth, data isolation)
  └── Workspace (sub-unit -- e.g., Google, Chrome, Android)
       └── Knowledge Base (collection of documents for RAG retrieval)
            ├── Document (uploaded file)
            ├── Source (web URL / sitemap / RSS)
            └── Chunks -> Embeddings
```

---

## 2. Entity List

All IDs are UUIDs. All timestamps are `TIMESTAMPTZ`. Every tenant-scoped table carries `org_id` for RLS.

### 2.1 Organizations

The top-level tenant. Maps to a Keycloak realm or client.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `name` | VARCHAR(255) | |
| `slug` | VARCHAR(100) UNIQUE | URL-friendly identifier |
| `status` | ENUM | `active`, `suspended`, `deactivated` |
| `settings` | JSONB | Rate limits, feature flags |
| `keycloak_realm` | VARCHAR(255) | Keycloak realm or client ID |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

### 2.2 Workspaces

Sub-units within an organization. Managed via Strapi.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK -> organizations | Tenant boundary |
| `name` | VARCHAR(255) | |
| `slug` | VARCHAR(100) | Unique within org |
| `settings` | JSONB | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

### 2.3 Users

Mirror of Keycloak user data. Primary identity lives in Keycloak; this table caches profile information for queries and foreign keys.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | Same as Keycloak user ID |
| `org_id` | UUID FK | |
| `email` | VARCHAR(255) | Unique within org |
| `display_name` | VARCHAR(255) | |
| `keycloak_sub` | VARCHAR(255) UNIQUE | Keycloak subject identifier |
| `status` | ENUM | `active`, `disabled` |
| `last_login_at` | TIMESTAMPTZ | |

### 2.4 Workspace Members (join table)

Many-to-many between users and workspaces with role assignment.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `workspace_id` | UUID FK | |
| `user_id` | UUID FK | |
| `role` | ENUM | `owner`, `admin`, `member`, `viewer` |
| `created_at` | TIMESTAMPTZ | |
| | | UNIQUE(workspace_id, user_id) |

### 2.5 Knowledge Bases

A collection of documents and sources. Primary unit of RAG retrieval.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `workspace_id` | UUID FK | |
| `name` | VARCHAR(255) | |
| `slug` | VARCHAR(100) | Unique within workspace |
| `description` | TEXT | |
| `settings` | JSONB | Chunk size, overlap, embedding model config |
| `status` | ENUM | `active`, `archived` |

### 2.6 Documents

Uploaded files (PDF, DOCX, images, etc.) processed through the ingestion pipeline.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `file_name` | VARCHAR(500) | |
| `file_type` | VARCHAR(50) | `pdf`, `docx`, `xlsx`, etc. |
| `file_size_bytes` | BIGINT | |
| `file_hash` | VARCHAR(128) | SHA-256 for deduplication |
| `storage_path` | TEXT | S3/MinIO object path |
| `processing_status` | ENUM | See section 6 |
| `processing_error` | TEXT | Error details if failed |
| `title` | VARCHAR(500) | Extracted or user-provided |
| `page_count` | INTEGER | |
| `metadata` | JSONB | Arbitrary key-value pairs |
| `uploaded_by` | UUID FK -> users | |

### 2.7 Sources

Web URLs, sitemaps, or RSS feeds to be scraped and ingested.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `source_type` | ENUM | `url`, `sitemap`, `rss_feed` |
| `url` | TEXT | |
| `crawl_depth` | INTEGER | For sitemaps |
| `crawl_frequency` | ENUM | `manual`, `daily`, `weekly`, `monthly` |
| `processing_status` | ENUM | See section 6 |
| `processing_error` | TEXT | |
| `title` | VARCHAR(500) | |
| `pages_crawled` | INTEGER | |
| `metadata` | JSONB | |
| `created_by` | UUID FK -> users | |

### 2.8 Chunks

The fundamental unit of retrieval. A segment of text extracted from a document or source.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `knowledge_base_id` | UUID FK | |
| `document_id` | UUID FK (nullable) | Exactly one of document_id or source_id must be set |
| `source_id` | UUID FK (nullable) | |
| `content` | TEXT | The chunk text |
| `chunk_index` | INTEGER | Ordering within parent |
| `token_count` | INTEGER | |
| `page_number` | INTEGER | For PDFs |
| `heading` | VARCHAR(500) | Nearest heading/section title |
| `chunk_type` | ENUM | `text`, `table`, `image_caption`, `code` |
| `metadata` | JSONB | |

ParadeDB BM25 index on `content` and `heading` for full-text keyword search.

### 2.9 Embeddings

Separated from chunks to support multiple embedding models and re-embedding without losing chunk data.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `chunk_id` | UUID FK | |
| `embedding` | vector(N) | pgvector column; dimension depends on model |
| `model_name` | VARCHAR(100) | e.g., `text-embedding-3-small` |
| `model_version` | VARCHAR(50) | |
| `dimensions` | INTEGER | e.g., 1536 |
| `created_at` | TIMESTAMPTZ | |
| | | UNIQUE(chunk_id, model_name) |

HNSW index on `embedding` using `vector_cosine_ops` for approximate nearest neighbor search.

### 2.10 LLM Provider Config

See section 5 for details.

### 2.11 Chat Sessions / Messages, Voice Sessions / Turns

High-throughput tables managed directly by PostgreSQL (not through Strapi). Sessions are scoped to a knowledge base and user. Messages store role, content, token count, retrieved chunk IDs, and model metadata.

---

## 3. Multi-Tenancy via RLS

**Strategy:** Shared schema with row-level security. All tenants share one database, one schema, one set of tables.

### How it works

1. **Every tenant-scoped table includes `org_id`** -- the organization UUID that serves as the tenant boundary.

2. **RLS policies are enabled on all tenant-scoped tables:**

```sql
ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON workspaces
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
```

3. **The API middleware sets the session variable on every request** by extracting `org_id` from the authenticated JWT:

```sql
SET app.current_org_id = '<uuid-from-jwt>';
```

4. **An admin bypass role** (`raven_admin`) exists for cross-tenant operations (migrations, analytics, support tooling):

```sql
CREATE POLICY admin_bypass ON workspaces
    FOR ALL TO raven_admin
    USING (true);
```

5. **Tables with RLS:** `workspaces`, `users`, `workspace_members`, `knowledge_bases`, `documents`, `sources`, `chunks`, `embeddings`, `llm_provider_configs`, `chat_sessions`, `chat_messages`, `voice_sessions`, `voice_turns`, `api_keys`.

### Why this approach

- Simplest to implement and operate -- single connection pool, atomic migrations
- Native Strapi compatibility (Strapi just adds an `org_id` relation field)
- Single pgvector HNSW index and single ParadeDB BM25 index (no per-tenant duplication)
- RLS is defense-in-depth: even if application code has a bug, cross-tenant reads are blocked at the database level
- Scales to 1,000+ tenants within PostgreSQL's comfortable range
- Can evolve to partitioned tables (by `org_id`) or schema-per-tenant if compliance demands arise later

---

## 4. LLM Provider Config

Organizations bring their own API keys (BYOK). Each organization can configure one or more LLM providers, and each workspace can select which provider to use.

### 4.1 `llm_provider_configs` table

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `org_id` | UUID FK | |
| `provider` | ENUM | `openai`, `anthropic`, `cohere`, `google`, `azure_openai`, `custom` |
| `display_name` | VARCHAR(255) | User-facing label, e.g., "Our OpenAI key" |
| `api_key_encrypted` | BYTEA | AES-256-GCM encrypted API key |
| `api_key_iv` | BYTEA | Initialization vector for decryption |
| `api_key_hint` | VARCHAR(20) | Last 4 chars for UI display, e.g., `...sk-abcd` |
| `base_url` | TEXT | Override for custom/Azure endpoints |
| `config` | JSONB | Provider-specific settings (deployment name, API version, org ID, etc.) |
| `is_default` | BOOLEAN | Default provider for this org |
| `status` | ENUM | `active`, `revoked`, `expired` |
| `created_by` | UUID FK -> users | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

### 4.2 Encryption approach

- **Encryption at rest:** API keys are encrypted with AES-256-GCM before storage. The encryption key is NOT stored in the database -- it lives in a secrets manager (AWS Secrets Manager, HashiCorp Vault, or environment variable as a fallback).
- **Key hierarchy:** A master key encrypts per-organization data encryption keys (DEKs). This allows key rotation per org without re-encrypting all keys globally.
- **Decryption happens only in the application layer** at the moment the key is needed for an LLM API call. Keys are never logged, never returned in API responses (only `api_key_hint` is exposed).
- **pgcrypto** can be used for database-level encryption if the application-layer approach is insufficient, but application-layer is preferred because it keeps the master key out of the database entirely.

### 4.3 Workspace-level model selection

Each workspace (or knowledge base) references which provider config to use via its `settings` JSONB:

```json
{
  "llm": {
    "provider_config_id": "uuid-of-llm-provider-config",
    "chat_model": "gpt-4o",
    "embedding_model": "text-embedding-3-small",
    "temperature": 0.7
  }
}
```

This allows different workspaces within the same org to use different providers or models while sharing the same encrypted API key store.

---

## 5. Embedding Storage Design

### 5.1 Chunks table

The chunks table stores the raw text segments. It carries a **ParadeDB BM25 index** for full-text keyword search:

```sql
-- BM25 index for full-text search (ParadeDB)
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

### 5.2 Embeddings table

Embeddings are separated from chunks to allow:
- **Multiple embedding models** per chunk (e.g., OpenAI + Cohere for A/B testing)
- **Re-embedding** when switching models without losing chunk data
- **Dimension flexibility** -- different models produce different vector dimensions

The table carries an **HNSW index** (pgvector) for approximate nearest neighbor search:

```sql
CREATE INDEX idx_embeddings_hnsw ON embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
```

### 5.3 Retrieval: Hybrid search via RRF

Since ParadeDB does not yet offer native hybrid search, retrieval combines two result sets using Reciprocal Rank Fusion (RRF):

1. **Semantic search** -- query embedding vs. stored embeddings via pgvector's `<=>` operator (cosine distance), filtered by `org_id` and `knowledge_base_id`
2. **Keyword search** -- query text vs. chunk content via ParadeDB's `@@@` BM25 operator, filtered the same way
3. **RRF fusion** -- `score = SUM(1 / (k + rank))` with k=60, combining both ranked lists into a single sorted result

### 5.4 Scaling strategy

- **Default:** Single HNSW index across all tenants, filtered by `org_id` at query time
- **At scale (>1M embeddings):** Partition the embeddings table by `org_id` using hash partitioning. Each partition gets its own HNSW index, keeping index size manageable
- **HNSW tuning:** `m=16`, `ef_construction=64` as defaults; increase `ef_search` at query time (default 40, up to 100-200) to trade latency for recall

---

## 6. Document Processing State Machine

### 6.1 States

```
queued --> parsing --> chunking --> embedding --> ready
  |          |           |             |
  |          v           v             v
  |       failed      failed        failed
  |          |           |             |
  +----------+-----------+-------------+---> reprocessing (manual retry)
                                                 |
                                                 v
                                              parsing (re-enters pipeline)
```

For **Sources**, an additional `crawling` state precedes `parsing` (web scraper fetches URLs first).

### 6.2 State definitions

| State | Description | Valid transitions |
|-------|-------------|-------------------|
| `queued` | Created, waiting in processing queue | `parsing`, `crawling` (sources only) |
| `crawling` | Web scraper fetching URL content (sources only) | `parsing`, `failed` |
| `parsing` | Extracting text from file (PDF->text, OCR, etc.) | `chunking`, `failed` |
| `chunking` | Splitting extracted text into chunks | `embedding`, `failed` |
| `embedding` | Generating vector embeddings for each chunk | `ready`, `failed` |
| `ready` | Complete -- chunks and embeddings available for retrieval | `reprocessing` |
| `failed` | Error occurred; `processing_error` column has details | `reprocessing` |
| `reprocessing` | Manual retry; clears old chunks/embeddings, re-enters pipeline | `parsing`, `crawling` |

### 6.3 Processing events (audit log)

A `processing_events` table records every state transition with `from_status`, `to_status`, `error_message`, and timing metadata. This provides a full audit trail for debugging and operational visibility.

---

## 7. Access Control Model

### 7.1 Role hierarchy

Four roles, ordered by privilege level:

| Role | Scope | Permissions |
|------|-------|-------------|
| **owner** | Workspace | Full control: CRUD workspace, knowledge bases, members. Can delete workspace. Can transfer ownership. |
| **admin** | Workspace | Manage knowledge bases, documents, sources. Manage members (except owners). Cannot delete workspace. |
| **member** | Workspace | Read knowledge bases. Upload documents. Create chat/voice sessions. Cannot manage members or settings. |
| **viewer** | Workspace | Read-only access to knowledge base content and chat history. No uploads, no chat creation. |

### 7.2 Four-layer enforcement

```
Layer 1: Keycloak (authentication)
    JWT contains: user ID, org_id, workspace_ids (custom claims via SPI)

Layer 2: API middleware (tenant scoping)
    Extracts org_id from JWT
    Sets SET app.current_org_id on DB connection
    Rejects requests where resource org_id != JWT org_id

Layer 3: Business logic (role-based authorization)
    Looks up workspace_members.role for the user + target workspace
    Enforces role hierarchy: owner > admin > member > viewer
    Each API endpoint declares its minimum required role

Layer 4: PostgreSQL RLS (defense-in-depth)
    Even if layers 2-3 have bugs, RLS prevents cross-tenant reads/writes
    Enforced at the database engine level -- cannot be bypassed by application code
```

### 7.3 Organization-level vs. workspace-level roles

- **Organization-level:** An org can have org-wide admins (stored as a role on the `users` table or a separate `org_admins` table). These users can manage all workspaces, billing, and org settings.
- **Workspace-level:** The `workspace_members` join table assigns per-workspace roles. A user can be an `admin` in one workspace and a `viewer` in another within the same org.

This two-tier role model keeps workspace access granular while allowing org-wide administrative authority.

---

## Entity Relationship Summary

```
Organization (tenant boundary)
  ├── User (Keycloak mirror)
  ├── LLMProviderConfig (encrypted BYOK keys)
  └── Workspace
       ├── WorkspaceMember (user + role)
       └── KnowledgeBase
            ├── Document --> Chunk --> Embedding
            ├── Source   --> Chunk --> Embedding
            ├── ChatSession --> ChatMessage
            └── VoiceSession --> VoiceTurn
```
