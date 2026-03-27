# Design Spec Review: Raven Platform Design

**Reviewer:** Claude Opus 4.6
**Date:** 2026-03-27
**Spec:** `/docs/superpowers/specs/2026-03-27-raven-platform-design.md`
**Verdict:** NOT APPROVED -- 2 critical, 4 important issues require resolution

---

## 1. Hierarchy Naming Mismatch Between Research and Spec

**[CRITICAL]** The research documents use a **three-level hierarchy**: `Application -> Organization -> Knowledge Base`, where `Application` is the true tenant boundary and `Organization` is a sub-division within it. The design spec **collapsed this to two levels**: `Organization -> Workspace -> Knowledge Base`, where `Organization` became the tenant boundary and `Workspace` replaced what was previously `Organization`.

Specific inconsistencies:

- **Data model research** (`data-model-approaches.md`, lines 24-35) defines `Application` as the top-level tenant with `Organization` beneath it. The spec drops `Application` entirely.
- **Research data model** has `application_id` on every table for RLS. The spec uses `org_id` instead, which maps to a different level of the hierarchy.
- **Research JWT claims** include `application_id` and `organization_ids[]` (plural). The spec's JWT has only `org_id` and `org_role`.
- **Research API keys** table has both `application_id` and `organization_id` scoping. The spec's API keys are scoped to a knowledge base only.
- **Backend architecture research** (`backend-architecture-approaches.md`, line 17) uses `Application -> Organization -> Knowledge Base`.

**Impact:** If the hierarchy change was intentional (collapsing Application into Organization, renaming Organization to Workspace), this is fine architecturally -- but it must be acknowledged explicitly. The current spec reads as if the hierarchy was always `Organization -> Workspace -> KB`, while the research documents consistently use a different hierarchy. A future implementer referencing both documents would be confused.

**Recommendation:** Add a "Design Decisions" or "Deviations from Research" section that explicitly states the hierarchy was simplified from 3 levels to 3 levels with different names, and why. Alternatively, if the Application tier is still intended, add it back.

---

## 2. Backend Language Changed Without Explanation

**[CRITICAL]** The backend architecture research recommends **Node.js (TypeScript) for the API + Python AI Workers** (Approach 2, Hybrid), with a phased plan to start Node.js-only and add Python later. The spec instead specifies **Go (Gin or FastHTTP) for the backend API**.

This is a major architectural departure:

- The research doc's recommendation (lines 216-229) explicitly says "Start with Approach 2 (Hybrid)" using Node.js + Python.
- LiteParse is a TypeScript/Node.js tool. The research notes the advantage of "native in-process calls" from Node.js. The spec has LiteParse invoked as a subprocess from the Python worker, requiring Node.js to be installed inside the Python Docker image (spec line 154).
- Strapi is a Node.js CMS. The research notes "Strapi bridge is less natural from Python (HTTP calls to a JS CMS)" as a con. Go has the same issue.
- The research identifies BullMQ (Node.js) as the job queue. The spec uses plain Redis queues instead.

**Impact:** The Go choice may be perfectly valid (better performance, team expertise, etc.), but since it contradicts the research recommendation, the rationale must be documented. Without it, the research documents become misleading to anyone reading them alongside the spec.

**Recommendation:** Document why Go was chosen over Node.js. If Go is the choice, also revisit the LiteParse integration strategy -- running Node.js as a subprocess inside a Python container adds operational complexity.

---

## 3. ParadeDB Community Edition WAL Limitation Not Addressed

**[IMPORTANT]** The ParadeDB research document (`paradedb-pgvector-research.md`, lines 60-61) includes a critical production warning:

> "ParadeDB Community does NOT have WAL support. Without WALs, data can be lost or corrupted on crash/restart, requiring reindex and causing downtime. Enterprise license required for production."

The design spec does not mention this limitation or how it will be addressed. The Docker compose setup (Section 6.1) references `pgvector/pgvector:pg16 + ParadeDB` but does not specify whether Community or Enterprise edition is being used.

**Recommendation:** Either (a) budget for ParadeDB Enterprise licensing for production, (b) document a fallback strategy using PostgreSQL's built-in tsvector/GIN for BM25 if ParadeDB Enterprise is not feasible, or (c) explicitly accept the risk for MVP with a plan to address before production.

---

## 4. Strapi Role Unclear and Potentially Redundant

**[IMPORTANT]** The research data model document (Section 4, lines 581-723) defines a detailed split between what Strapi manages vs. what PostgreSQL manages directly, with Strapi handling CRUD for Applications, Organizations, Users, KBs, Documents, and Sources via its admin UI and lifecycle hooks.

The design spec mentions Strapi only briefly (Section 2.2, line 111): "CMS for platform content, admin CRUD for orgs/workspaces/KBs." However, the spec also describes a full Vue.js Admin Dashboard (Section 5.1) with its own configurator, test sandbox, analytics, and API key management.

This creates ambiguity:
- Are admins using the Strapi admin panel, the Vue.js dashboard, or both?
- If the Vue.js dashboard handles all admin CRUD, what does Strapi add beyond being a REST API layer?
- The Go API consumes Strapi internally (line 111) -- but why not have the Go API manage the PostgreSQL tables directly, eliminating the Strapi dependency?

With Go as the API server (not Node.js), the argument for Strapi is weaker. Strapi's value was strongest in the Node.js monolith approach where it could serve as the admin backend directly.

**Recommendation:** Clarify Strapi's exact role. Either (a) Strapi is the admin backend and the Vue.js dashboard is a custom frontend consuming Strapi's API, or (b) drop Strapi and have the Go API handle all CRUD directly. Keeping both adds an unnecessary service to operate.

---

## 5. Chat Session Model Missing Anonymous/Widget User Support

**[IMPORTANT]** The spec describes a chatbot widget (`<raven-chat>`) that uses publishable API keys and explicitly states "No end-user login required" (Section 5.1, line 477). However, the data model research's `chat_sessions` table has `user_id UUID NOT NULL REFERENCES users(id)` -- a required foreign key to the users table.

The design spec's data model does not define the Chat Sessions table in detail (line 349 just says "high-throughput tables managed directly by PostgreSQL"), so this gap is inherited from the research.

**Impact:** Anonymous chatbot widget users will not have a `user_id`. The schema must support nullable `user_id` or introduce an anonymous/ephemeral user concept.

**Recommendation:** Make `user_id` nullable on `chat_sessions`, add an `api_key_id` or `widget_session_id` field for anonymous sessions, and define the TTL behavior for anonymous sessions (the spec mentions 24h TTL on line 501 but the data model does not encode this).

---

## 6. BYOK Embedding Dimension Constraint vs. pgvector Fixed-Width Columns

**[IMPORTANT]** Section 4.6 (line 440) states: "All documents within an org must use the same model/dimensions (pgvector columns are fixed-width). Changing providers requires re-indexing."

However, the data model separates embeddings from chunks (Section 3.2) and includes `model_name` and `dimensions` columns, plus a `UNIQUE(chunk_id, model_name)` constraint -- which was explicitly designed to support multiple models per chunk (research document line 429: "Separated from chunks to allow multiple embedding models and easy re-embedding").

The HNSW index is created with a fixed dimension: `CREATE INDEX ON embeddings USING hnsw (embedding vector_cosine_ops)`. This index cannot serve vectors of different dimensions simultaneously. The spec does not describe how to handle orgs using different embedding models (e.g., OpenAI 1536d vs. Cohere 1024d).

**Recommendation:** Clarify the strategy. Options: (a) one HNSW index per dimension size using partial indexes: `CREATE INDEX ... WHERE dimensions = 1536`, (b) standardize all orgs on one dimension, or (c) use the `vector(N)` column without a fixed N and accept that pgvector requires dimension-specific indexes.

---

## 7. Reranking Step Not Specified in the Spec

**[SUGGESTION]** The RAG research document (Approach A) recommends Cohere Rerank v3 as a critical quality step between RRF fusion and LLM generation. The design spec mentions "Rerank top-K" in the Chat/RAG Query flow (Section 2.5, line 196) but does not specify how reranking is performed, which provider/model is used, or whether it is a managed API call or self-hosted cross-encoder.

**Recommendation:** Add a brief mention of the reranking strategy in Section 4.7 (Hybrid Retrieval), consistent with the RAG research recommendation.

---

## 8. LLM Provider Configs Table Missing Embedding vs. Chat Distinction

**[SUGGESTION]** The `llm_provider_configs` table (Section 3.2) stores a single provider config per org. But the RAG research recommends different models for different purposes: embedding model (text-embedding-3-small), chat LLM (Claude Sonnet), voice LLM (Claude Haiku), and reranker (Cohere). A single `provider` + `is_default` boolean does not capture this.

**Recommendation:** Add a `purpose` or `model_type` enum: `embedding`, `chat_completion`, `reranking`, `voice_completion`. This allows an org to configure different providers for different pipeline stages.

---

## 9. Missing Sections for MVP Completeness

**[SUGGESTION]** The following subsystems are referenced but not detailed in the spec:

- **Rate limiting strategy**: Mentioned for API keys (Section 6.4) and Redis (Section 2.2), but no concrete design (per-key, per-org, sliding window vs. token bucket, limits for embedding API calls).
- **Monitoring and observability**: No mention of logging, tracing, metrics, or health checks.
- **Backup and disaster recovery**: No mention of PostgreSQL backup strategy, MinIO data durability, or Keycloak realm exports.
- **Migration strategy**: No mention of database migration tooling (golang-migrate, Atlas, etc.).

These are not blockers for a design spec but should be addressed before implementation begins.

---

## 10. Security: Admin Bypass Role Needs Guardrails

**[SUGGESTION]** Section 3.3 defines a `raven_admin` role that bypasses RLS for cross-tenant operations. The spec does not describe how this role is assigned or restricted. If the Go API's database connection uses this role by default (or can be escalated to it), a bug in the API could expose all tenant data.

**Recommendation:** Ensure the `raven_admin` role is used only by dedicated admin tools/scripts, never by the API server's connection pool. Document this constraint in the spec.

---

## Summary

| # | Category | Severity | Issue |
|---|----------|----------|-------|
| 1 | Consistency | CRITICAL | Hierarchy naming mismatch between research (App->Org->KB) and spec (Org->Workspace->KB) |
| 2 | Consistency | CRITICAL | Backend language changed from Node.js to Go without documented rationale |
| 3 | Feasibility | IMPORTANT | ParadeDB Community WAL limitation not addressed |
| 4 | YAGNI | IMPORTANT | Strapi role unclear when Go API + Vue dashboard exist |
| 5 | Completeness | IMPORTANT | Chat sessions require user_id but widget users are anonymous |
| 6 | Feasibility | IMPORTANT | BYOK multi-dimension embeddings vs. fixed-width HNSW index |
| 7 | Completeness | SUGGESTION | Reranking strategy not specified |
| 8 | Completeness | SUGGESTION | LLM config table missing model purpose/type distinction |
| 9 | Completeness | SUGGESTION | Rate limiting, observability, backup, migrations not covered |
| 10 | Security | SUGGESTION | Admin bypass role needs usage guardrails |

**Verdict: NOT APPROVED** -- Resolve critical issues #1 and #2, and address important issues #3-#6 before proceeding to implementation.
