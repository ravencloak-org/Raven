# Enterprise Connectors — Research Notes

> Status: **Parked / Future Milestone (Raven Pro)**
> Assessed: 2026-03-28
> Related issues: #111, #112, #113, #114, #115, #116

---

## 1. Why Airbyte, Not Native Connectors

### Options Evaluated

| Approach | Pros | Cons |
|----------|------|------|
| **Build native connectors** | Full control, lighter weight | Maintaining 20+ connectors long-term (schema changes, API versioning, auth flows per source) |
| **Embed Airbyte (MIT core)** | 700+ connectors day one, community maintained | Adds dependency, Airbyte platform overhead (~2–4 GB for scheduler/API) |
| **Hybrid** | Native for databases, Airbyte for SaaS | Two codepaths to maintain |

### Decision: Airbyte for everything

Even for databases (PostgreSQL, MySQL, ClickHouse), Airbyte handles CDC (WAL-based), incremental sync, schema discovery, connection pooling, SSH tunneling, and SSL cert management. Building native Go connectors would start at ~200 lines but grow to 2,000+ per connector. Not worth it.

Raven's value-add is everything **after** Airbyte — chunking, embedding, classification, knowledge graph routing. Not raw data movement.

---

## 2. Airbyte Data Integrity & Tradeoffs

### Initial Sync (Existing Data)

Airbyte does a **Full Refresh** — `SELECT *` from configured tables, streams to destination. Reliable, no corruption risk.

### Known Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| **Type coercion** — JSONB, NUMERIC(38,18) mapped to Airbyte internal types; mismatches logged in `_airbyte_meta.changes` per row | Low | Raven chunks text for embeddings, not doing financial arithmetic |
| **Duplicate rows on retry** — failed sync + retry, or CDC WAL gap → fallback to full refresh → temporary duplicates until dedup runs | Medium | Idempotent chunking pipeline: `ON CONFLICT (source_id, chunk_hash) DO NOTHING` |
| **Schema drift** — client alters source table (add/rename/remove columns) between syncs | Medium | Schema mapping in connector config ("which columns contain text to chunk") + validation on each sync |
| **Initial bulk speed** — Airbyte is not the fastest bulk loader vs raw `COPY` | None | Embedding is the bottleneck (orders of magnitude slower than transfer), not Airbyte |
| **CDC WAL gap** — WAL retention too short + sync interval too long → missed changes → fallback to full refresh | Low | Deployment docs must specify minimum WAL retention per source type |

### Key Requirement

**Raven's chunking/embedding pipeline must be idempotent.** Dedup on `(source_id, chunk_hash)`. This handles retry duplicates, full refresh fallbacks, and CDC gaps.

---

## 3. Deployment Models

### Option A: Cloud-with-agent (mid-market)

```
[Client's Postgres / S3 / Salesforce]
         ↓ (Airbyte agent on client network, secure tunnel)
[Raven Cloud — Airbyte destination]
         ↓
[Raven ingestion pipeline — chunking, embedding, classification]
         ↓
[Knowledge base (pgvector / ClickHouse + QBit)]
```

### Option B: On-premise (enterprise)

```
[Client's internal data sources]
         ↓ (Airbyte running inside client infra)
[Raven on-prem — full stack inside client network]
         ↓
[Knowledge base — data never leaves client network]
```

### Decision: Support both (Option C)

Enterprise gets on-prem. Mid-market gets cloud-with-agent. Same Raven codebase, different deployment config.

---

## 4. Data Classification & Knowledge Base Routing

### Industry Standard Research

Enterprise data systems have labeling, but at different granularities:

| System | Labels | Granularity | Row-level? |
|--------|--------|-------------|------------|
| **Snowflake** | Tags (key-value, inherited) | Database → schema → table → column | No |
| **BigQuery** | Labels (key-value) | Dataset → table → view | No |
| **AWS Glue** | Data Catalog (auto-crawled) | Database → table → column | No |
| **DataHub** | Tags + Business Glossary | Dataset → schema → column | No |
| **dbt** | Tags + meta + descriptions | Model → column | No |

**Key finding:** No system labels at the row level. They tell you "this table is owned by Marketing" but not "rows 1–50K belong to Institution A."

### The Two Classification Problems

**Problem 1: Which tables/sources feed which knowledge base?**
Structural mapping. Snowflake tags, dbt metadata, DataHub terms ARE useful here. If the enterprise already tagged their data, Raven reads tags and auto-routes.

**Problem 2: Within a single table, which rows go where?**
Multi-tenant table scenario. No external labeling system solves this — it's a filter condition. Admin tells Raven: "From `documents` table, rows where `org = 'harvard'` → Harvard KB."

### Decision: Don't Build a Labeling System

Raven consumes labels that already exist, or lets the admin point at what they want exposed.

**Three routing modes:**

```yaml
# Mode 1: Static — whole table → one KB
- name: support_tickets
  knowledge_base: "Support KB"
  text_columns: [subject, body, resolution]

# Mode 2: Column-based — route by discriminator column
- name: documents
  text_columns: [title, content]
  routing:
    column: institution
    rules:
      - value: "harvard"
        knowledge_base: "Harvard KB"
      - value: "mit"
        knowledge_base: "MIT KB"
      - default: "Unclassified KB"

# Mode 3: Auto — LLM classifies on ingest (expensive, use sparingly)
- name: product_catalog
  knowledge_base: auto
  text_columns: [name, description, specs]
```

**When external catalogs exist** (Snowflake tags, dbt, DataHub), Raven pre-populates this config by reading the tags. Admin confirms rather than building from scratch.

### UI vs Config

- **Cloud tier:** Mapping UI (tree view, filter rules, drag-drop tagging)
- **Self-hosted:** YAML config file (if they can self-host, they can write YAML)
- Both are paid features in the `ee/` directory

---

## 5. Architecture Flow

```
[Client Data Source]
         ↓
[Airbyte (sync)]
         ↓
[Raven Connector Service]
    ├── Schema validation (detect drift)
    ├── Apply routing rules (static / column-based / auto)
    ├── Extract text columns
    └── Dedup check (source_id + chunk_hash)
         ↓
[Raven Ingestion Pipeline (existing)]
    ├── Chunking
    ├── Embedding (BYOK LLM)
    └── Store in knowledge base
         ↓
[pgvector (MVP) / ClickHouse + QBit (enterprise scale)]
```
