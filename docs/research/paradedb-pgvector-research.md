# ParadeDB + pgvector Research Summary

**Date:** 2026-03-27
**Status:** Research only - no code written

---

## 1. What is ParadeDB?

ParadeDB is a **PostgreSQL extension** (`pg_search`) that brings Elastic-quality full-text search and analytics directly inside Postgres. It is NOT a fork; it installs as an extension on any Postgres 15+ instance.

**Core extension:** `pg_search` -- introduces a custom **BM25 index** type built on an LSM tree architecture, with each segment containing both an inverted index (for text search) and a columnar index (for fast aggregates).

**Key features already shipping (v0.22.3):**
- Full-text search with BM25 scoring
- 12+ tokenizers, 20+ language support
- Top-K retrieval, highlighting, faceted search
- Filtering, aggregates (bucket, metrics, facets) with columnar storage
- JOIN pushdown (beta) for INNER/SEMI/ANTI joins
- Custom operators (`@@@`) and custom scan nodes with parallelization

**What is NOT yet shipping:**
- Vector search (coming soon -- on roadmap)
- Hybrid search combining BM25 + vector similarity (coming soon -- on roadmap)

### How it differs from plain PostgreSQL + pgvector

ParadeDB and pgvector solve **different problems**:

| Aspect | pgvector | ParadeDB (pg_search) |
|--------|----------|---------------------|
| Primary purpose | Vector similarity search (embeddings) | Full-text search (BM25), analytics |
| Index type | HNSW, IVFFlat | BM25 (LSM tree with inverted + columnar) |
| Query type | Nearest-neighbor on embeddings | Text search, filtering, aggregates |
| Scoring | Cosine/L2/inner product distance | BM25 relevance scoring |
| Hybrid search | Not natively (manual RRF needed) | Planned: combine BM25 + vector in single query |

**They are complementary, not competing.** For a knowledge base, you would use:
- **pgvector** for semantic similarity via embeddings
- **ParadeDB** for keyword/full-text search with BM25

ParadeDB's roadmap explicitly states they plan to improve vector search by addressing pgvector's limitations around filtered queries -- specifically, queries that combine vector similarity with metadata filters or full-text search predicates.

---

## 2. Deployment Options

### Community (Open Source, AGPL-3.0)
- **Docker:** `paradedb/paradedb:latest` (bundles PostgreSQL + pg_search)
- **Kubernetes:** Helm chart based on CloudNativePG
- **Extension install:** `curl -fsSL https://paradedb.com/install.sh | sh` into existing self-hosted Postgres
- **One-click cloud:** Railway, Render, DigitalOcean (for dev/staging only)
- **Citus compatible:** For distributed/sharded workloads

### Enterprise (Closed Source)
- **ParadeDB Enterprise:** Adds WAL support (critical for production durability)
- **ParadeDB BYOC:** Managed deployment inside your own AWS or GCP account
- **Managed Cloud:** Fully managed offering is in progress (on roadmap)

### Critical Production Warning
> ParadeDB Community does **NOT** have WAL support. Without WALs, data can be lost or corrupted on crash/restart, requiring reindex and causing downtime. **Enterprise license required for production.**

---

## 3. Vector Similarity Search

### Current State (as of v0.22.3)
ParadeDB does **NOT** natively handle vector embeddings today. Vector search and hybrid search are listed as "coming soon" on both the README checklist and roadmap.

### How to get vector search today with ParadeDB
Since ParadeDB is a standard Postgres extension, you can **install pgvector alongside it**:
```sql
CREATE EXTENSION vector;       -- pgvector for embeddings
CREATE EXTENSION pg_search;    -- ParadeDB for full-text search
```

For hybrid search (combining BM25 + vector), you would currently need to:
1. Run a BM25 query via ParadeDB's `@@@` operator
2. Run a vector similarity query via pgvector's `<=>` operator
3. Combine results manually using Reciprocal Rank Fusion (RRF) or similar

### Roadmap for native vector/hybrid search
ParadeDB plans to:
- Improve vector search performance in Postgres by addressing pgvector's limitations around filtered queries
- Support queries combining vector similarity with metadata filters or full-text search predicates natively

---

## 4. Performance Characteristics for Knowledge Base Use Cases

### Architecture advantages
- **LSM tree structure:** Optimized for high-ingest workloads; background merging and pending lists buffer writes
- **Columnar index per segment:** Fast aggregates, GROUP BY, COUNT operations
- **Custom scan node:** Bypasses standard Postgres execution for search queries; supports parallelization
- **Real-time indexing:** BM25 index updates within the transaction (no eventual consistency)
- **MVCC compliant:** Full concurrent read/write support

### Scale
- Production deployments comfortably operate in the **1-10 TB range**
- Largest known single ParadeDB database in production: **10 TB**
- For larger datasets: partitioned tables + Citus for distributed sharding

### Tuning levers
- Read throughput: `max_parallel_worker` pool, `shared_buffers` size
- Write throughput: per-statement memory (`maintenance_work_mem`)
- Background merging and pending list (already completed features)

### Knowledge base relevance
For a RAG/knowledge base system, the typical pattern would be:
- **Ingestion:** Documents parsed, chunked, embedded (pgvector), and text-indexed (ParadeDB)
- **Retrieval:** Hybrid search combining semantic (pgvector) + keyword (ParadeDB BM25)
- **Filtering:** Metadata filters on the BM25 index (category, date, source, etc.)
- **Aggregates:** Faceted search for browsing/filtering the knowledge base

### Limitations for knowledge base
- No native vector search yet (must use pgvector separately)
- No native hybrid search yet (manual RRF fusion required)
- JOIN pushdown only supports INNER/SEMI/ANTI (LEFT/RIGHT/OUTER coming)
- Community edition lacks WAL durability (enterprise required for production)
- DDL replication has caveats (BM25 indexes not replicated via logical replication)

---

## 5. Comparison with Alternatives

### vs. Pinecone / Weaviate / Qdrant (Purpose-built vector databases)

| Aspect | ParadeDB + pgvector | Pinecone/Weaviate/Qdrant |
|--------|-------------------|--------------------------|
| **Primary strength** | Full-text BM25 search + relational queries | Vector similarity search |
| **Vector search** | Via pgvector extension (separate) | Native, highly optimized |
| **Hybrid search** | Manual fusion (native coming soon) | Native hybrid search |
| **Metadata filtering** | Full SQL power, JOINs, aggregates | Limited filtering APIs |
| **Operational overhead** | Single Postgres instance | Separate service to manage |
| **Data consistency** | ACID transactions (with Enterprise WAL) | Eventually consistent typically |
| **Ecosystem** | Full Postgres ecosystem (ORMs, tools, backups) | Proprietary APIs |
| **Cost** | Open source (Enterprise for production) | SaaS pricing, can be expensive at scale |
| **Scale model** | Vertical (1-10TB), Citus for horizontal | Horizontally scalable natively |

**When to choose ParadeDB + pgvector over purpose-built vector DBs:**
- Your primary database is already Postgres
- You need strong full-text search (BM25) alongside vector search
- You want to avoid ETL pipelines syncing data to external services
- You need ACID transactions and complex SQL queries on your knowledge base
- Your dataset fits in the 1-10TB range on a single node

**When to choose a purpose-built vector DB:**
- Vector search is your primary workload and needs maximum performance
- You need horizontal scaling beyond what a single Postgres node provides
- You want turnkey hybrid search without manual fusion
- You do not need complex SQL, JOINs, or relational data alongside vectors

### vs. Plain pgvector (without ParadeDB)

| Aspect | Plain pgvector | pgvector + ParadeDB |
|--------|---------------|-------------------|
| **Vector search** | Yes (HNSW, IVFFlat) | Yes (same pgvector) |
| **Full-text search** | Postgres tsvector/GIN (limited) | BM25 index (Elastic-quality) |
| **BM25 scoring** | No | Yes |
| **Tokenization** | Basic | 12+ tokenizers, 20+ languages |
| **Faceted search** | No | Yes |
| **Highlighting** | Basic ts_headline | Advanced highlighting |
| **Aggregates** | Standard Postgres (slow on large sets) | Columnar index (fast) |
| **Hybrid search** | Manual tsvector + vector fusion | Better BM25 + vector fusion (native coming) |

**Bottom line:** If you only need vector similarity search, plain pgvector is sufficient. If you also need production-grade full-text search (BM25 scoring, facets, highlighting, advanced tokenization), ParadeDB adds significant value.

### vs. Elasticsearch/OpenSearch

ParadeDB positions itself as a direct Elasticsearch replacement for teams already on Postgres:
- Zero ETL (no data sync pipeline needed)
- Pure SQL interface (no separate query DSL)
- Can run as a logical replica of managed Postgres (RDS, Supabase, Neon, etc.)
- Lacks Elasticsearch's mature distributed clustering (Citus helps but is not equivalent)

---

## Key Takeaways for Knowledge Base Architecture

1. **ParadeDB is primarily a full-text search solution**, not a vector database. It excels at BM25 search, faceted navigation, and analytics inside Postgres.

2. **For vector embeddings, you still need pgvector.** ParadeDB and pgvector are complementary extensions that can coexist on the same Postgres instance.

3. **Native hybrid search (BM25 + vector) is on the roadmap but not yet available.** Today you must implement manual result fusion (e.g., RRF).

4. **The "single Postgres" advantage is compelling** -- one database for relational data, full-text search, and vector search eliminates ETL complexity and operational overhead.

5. **Production use requires Enterprise** due to the WAL limitation in the Community edition.

6. **For a knowledge base specifically:** The combination of pgvector (semantic search) + ParadeDB (keyword search, filtering, facets) on a single Postgres instance is a strong architectural choice if you want to avoid managing separate services, and your data fits within the 1-10TB vertical scaling range.
