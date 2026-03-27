# Data Model

## Hierarchy

```
Organization (tenant boundary -- billing, auth, data isolation)
  +-- Workspace (sub-unit -- e.g., Google, Chrome, Android)
       +-- Knowledge Base (collection of documents for RAG retrieval)
            +-- Document (uploaded file)
            +-- Source (web URL / sitemap / RSS)
            +-- Chunks -> Embeddings
```

## Core Entities

All IDs are UUIDs. All timestamps are TIMESTAMPTZ. Every tenant-scoped table carries `org_id` for RLS.

### Organizations
Top-level tenant. Maps to Keycloak realm.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | VARCHAR(255) | |
| slug | VARCHAR(100) UNIQUE | URL-friendly |
| status | ENUM | active, suspended, deactivated |
| settings | JSONB | Rate limits, feature flags |

### Workspaces
Sub-units within an organization.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| org_id | UUID FK | Tenant boundary |
| name | VARCHAR(255) | |
| slug | VARCHAR(100) | Unique within org |
| settings | JSONB | LLM provider selection |

### Knowledge Bases

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| org_id | UUID FK | |
| workspace_id | UUID FK | |
| name | VARCHAR(255) | |
| settings | JSONB | Chunk size, overlap, embedding model |

### Documents / Sources / Chunks / Embeddings
See the [full design spec](https://github.com/ravencloak-org/Raven/blob/main/docs/superpowers/specs/2026-03-27-raven-platform-design-final.md#4-data-model) for complete schemas.

## Multi-Tenancy via RLS

Shared schema with Row-Level Security. All tenants share one database.

```sql
ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON workspaces
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid);
```

Go API middleware sets `SET app.current_org_id = '<uuid>'` on every request from JWT claims.

## Document Processing State Machine

```
queued -> crawling* -> parsing -> chunking -> embedding -> ready
                                                           |
                                                        failed -> reprocessing
```
*crawling only for Sources (web scraper fetches URLs first)

## Access Control

| Role | Permissions |
|------|-------------|
| **owner** | Full control, delete workspace |
| **admin** | Manage KBs, documents, members |
| **member** | Read KBs, upload, create sessions |
| **viewer** | Read-only access |

Four-layer enforcement: Keycloak (authn) -> API middleware (tenant scoping) -> Business logic (role checks) -> PostgreSQL RLS (defense-in-depth).
