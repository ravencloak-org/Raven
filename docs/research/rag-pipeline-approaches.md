# RAG Pipeline Approaches for Raven

**Date:** 2026-03-27
**Status:** Research draft -- no code written

---

## Context

Raven is a multi-tenant knowledge-base platform. Documents are parsed by LiteParse (text/JSON with bounding boxes), web content is scraped as markdown, embeddings are stored in PostgreSQL + pgvector, and full-text search uses ParadeDB (BM25). Three interaction modes -- chatbot, voice agent, and WebRTC -- query the same retrieval pipeline.

This document presents three approaches to the RAG pipeline, covering chunking, embedding models, LLM selection, knowledge graph, retrieval strategy, and orchestration.

---

## Approach A: Pragmatic Hybrid (Recommended Starting Point)

> **Philosophy:** Ship fast with managed services, minimize operational complexity, and rely on proven hybrid retrieval without a knowledge graph.

### Components

| Layer | Choice | Rationale |
|-------|--------|-----------|
| **Chunking** | Recursive character splitting with document-structure awareness | LiteParse outputs bounding boxes and structural hints (headings, tables, lists). Use these to define chunk boundaries, falling back to recursive splitting (e.g., 512 tokens, 50-token overlap) within structurally flat sections. Preserve heading hierarchy as chunk metadata. |
| **Embedding model** | OpenAI `text-embedding-3-small` (1536d) | Best cost/performance ratio for a managed service. Supports `dimensions` parameter to reduce to 512d or 256d if needed for storage savings. Matryoshka-style truncation. No GPU infra required. |
| **LLM** | Multi-provider: Anthropic Claude (primary), OpenAI GPT-4o (fallback) | Claude Sonnet for chat and synthesis; Claude Haiku for low-latency voice agent responses. GPT-4o as a fallback provider for resilience. Provider abstraction via a thin routing layer. |
| **Knowledge graph** | **Skip KG** -- pure vector + BM25 hybrid | Adding a knowledge graph adds significant complexity (entity extraction, schema maintenance, graph queries). For the initial version, hybrid search with good chunking and metadata filtering covers the vast majority of knowledge-base queries. KG can be layered in later (see Approach C). |
| **Retrieval** | Hybrid vector + BM25 via Reciprocal Rank Fusion (RRF) | Two parallel queries: (1) pgvector cosine similarity, (2) ParadeDB BM25 `@@@` operator. Results merged via RRF (`k=60`). Top-N candidates passed to a reranker. |
| **Reranking** | Cohere Rerank v3 | Applied to the top 20-30 RRF results. Reranks to top 5-8 for the LLM context window. Significant relevance improvement for minimal latency cost (~100ms). |
| **Pipeline orchestration** | Custom (lightweight) | A thin Python service with clearly defined stages: chunk -> embed -> store -> retrieve -> rerank -> generate. No framework overhead. Each stage is a function with typed inputs/outputs. Easy to test, easy to swap components. |

### Pros
- **Fastest to production.** No graph DB, no self-hosted models, no framework lock-in.
- **Low operational burden.** Managed embedding and reranking APIs. Single Postgres instance for vectors + BM25 + relational data.
- **Good baseline quality.** Hybrid search + reranking consistently outperforms pure vector search in benchmarks (MTEB, BEIR).
- **Multi-tenant friendly.** Tenant isolation via Postgres row-level security or schema-per-tenant; no separate infra per tenant.

### Cons
- **API cost scales with volume.** Embedding and reranking API calls have per-token/per-query costs.
- **Vendor dependency.** Embedding model is OpenAI; reranking is Cohere; LLM is Anthropic/OpenAI. Three external dependencies.
- **No deep relational understanding.** Without a knowledge graph, the system cannot answer multi-hop relational queries well (e.g., "What products does the supplier of component X also provide?").
- **RRF fusion is manual.** Until ParadeDB ships native hybrid search, the fusion logic lives in application code.

### Chunking Detail

```
Document (LiteParse output)
  |
  +-- Extract structural elements (headings, tables, lists, paragraphs)
  |
  +-- For each section (heading-bounded):
  |     |
  |     +-- If section fits in one chunk (< 512 tokens): keep as-is
  |     |
  |     +-- If section is too large: recursive split on paragraph -> sentence boundaries
  |           with 50-token overlap
  |
  +-- Tables: each table becomes its own chunk with the table caption/heading as prefix
  |
  +-- Metadata per chunk:
        - document_id, organization_id (tenant), source_type (pdf, web, etc.)
        - heading_hierarchy (e.g., ["Chapter 3", "Section 3.2", "Subsection 3.2.1"])
        - page_number (from bounding boxes), chunk_index
        - created_at, updated_at
```

### Retrieval Flow

```
User Query
    |
    v
+-------------------+
| Query Processing  |  (expand abbreviations, detect intent)
+-------------------+
    |
    +-------+-------+
    |               |
    v               v
 pgvector        ParadeDB
 (cosine sim)    (BM25 @@@)
    |               |
    +-------+-------+
            |
            v
    +---------------+
    |   RRF Merge   |  (k=60, top 20-30 candidates)
    +---------------+
            |
            v
    +---------------+
    | Cohere Rerank |  (top 5-8 results)
    +---------------+
            |
            v
    +---------------+
    |   LLM Gen     |  (Claude Sonnet / Haiku)
    +---------------+
            |
            v
       Response
```

---

## Approach B: Self-Hosted & Cost-Optimized

> **Philosophy:** Minimize external API dependencies and per-query costs by self-hosting embedding, reranking, and (optionally) the LLM. Best for high-volume workloads where API costs would dominate.

### Components

| Layer | Choice | Rationale |
|-------|--------|-----------|
| **Chunking** | Semantic chunking (embedding-based boundary detection) | Use a lightweight embedding model (e.g., `all-MiniLM-L6-v2`) to compute sentence-level embeddings, then split where cosine similarity between consecutive sentences drops below a threshold. This produces semantically coherent chunks of variable length. Still falls back to max-token limits (512) and uses LiteParse structure as hard boundaries. |
| **Embedding model** | Self-hosted: `nomic-embed-text-v1.5` (768d) or `BAAI/bge-large-en-v1.5` (1024d) | Top-tier open-source embedding models. Nomic offers Matryoshka support (can truncate to 256d/512d), Apache 2.0 license, and long context (8192 tokens). BGE-large is the MTEB benchmark leader among open-source models. Host via ONNX Runtime, vLLM, or TEI (Text Embeddings Inference by HuggingFace). |
| **LLM** | Self-hosted primary: Llama 3.1 70B (via vLLM or TGI) + Claude API fallback | Llama 3.1 70B is competitive with GPT-4o on many benchmarks. Run on 2xA100 or 4xA10G via vLLM for high throughput. Fall back to Claude API for complex queries or when self-hosted capacity is saturated. |
| **Knowledge graph** | **Skip KG** -- same rationale as Approach A | Same reasoning. Self-hosted LLMs are less reliable at entity extraction than frontier models, making KG construction even harder. |
| **Retrieval** | Hybrid vector + BM25 via RRF, with cross-encoder reranking | Same hybrid strategy as Approach A, but reranking uses a self-hosted cross-encoder (`cross-encoder/ms-marco-MiniLM-L-12-v2` or `BAAI/bge-reranker-v2-m3`) instead of Cohere API. |
| **Reranking** | Self-hosted cross-encoder: `BAAI/bge-reranker-v2-m3` | Open-source, multilingual, competitive with Cohere Rerank on BEIR benchmarks. Runs on a single GPU. Batch inference for efficiency. |
| **Pipeline orchestration** | Haystack 2.x | Haystack's pipeline abstraction is well-suited for self-hosted components. Built-in support for pgvector, custom retrievers, and local models. Good for teams that want a framework but not the complexity of LangChain. Alternatively, use a custom pipeline if the team prefers. |

### Pros
- **Predictable costs.** No per-query API charges for embeddings, reranking, or (if using Llama) generation. Cost is infrastructure (GPU instances).
- **Data sovereignty.** No document content leaves your infrastructure. Important for regulated industries or sensitive knowledge bases.
- **No rate limits.** Self-hosted models can be scaled horizontally without worrying about API rate limits.
- **Semantic chunking produces higher-quality chunks** than fixed-size or recursive splitting, at the cost of more compute during ingestion.

### Cons
- **Significant GPU infrastructure required.** Llama 3.1 70B needs 2xA100 80GB or 4xA10G 24GB. Embedding model needs at least 1 GPU for reasonable throughput. Reranker needs another GPU.
- **Operational complexity.** Model serving (vLLM/TGI), GPU monitoring, model updates, and scaling are non-trivial.
- **Embedding quality gap.** Open-source embeddings are competitive but still trail `text-embedding-3-large` on some benchmarks, especially for domain-specific content.
- **LLM quality gap.** Llama 3.1 70B is strong but not at Claude Sonnet or GPT-4o level for nuanced synthesis and instruction-following.
- **Slower iteration.** Changing models requires redeployment and potentially re-embedding the entire corpus (if switching embedding models).

### Embedding Model Comparison

| Model | Dimensions | MTEB Avg | Context | License | Matryoshka | Notes |
|-------|-----------|----------|---------|---------|------------|-------|
| `nomic-embed-text-v1.5` | 768 | 62.28 | 8192 | Apache 2.0 | Yes | Long context, flexible dims |
| `BAAI/bge-large-en-v1.5` | 1024 | 64.23 | 512 | MIT | No | Higher quality, shorter context |
| `BAAI/bge-m3` | 1024 | 63.55 | 8192 | MIT | No | Multilingual, multi-granularity |
| `sentence-transformers/all-MiniLM-L6-v2` | 384 | 56.26 | 256 | Apache 2.0 | No | Lightweight, good for semantic chunking |
| OpenAI `text-embedding-3-small` | 1536 | 62.26 | 8191 | Proprietary | Yes | Managed API, no GPU needed |
| OpenAI `text-embedding-3-large` | 3072 | 64.59 | 8191 | Proprietary | Yes | Best quality, highest cost |
| Cohere `embed-v3` | 1024 | 64.47 | 512 | Proprietary | No | Strong multilingual |

### Self-Hosted Reranker Comparison

| Model | BEIR nDCG@10 | Multilingual | Speed (queries/sec on A10G) | License |
|-------|-------------|-------------|---------------------------|---------|
| `BAAI/bge-reranker-v2-m3` | ~52.5 | Yes (100+ languages) | ~50-80 | MIT |
| `cross-encoder/ms-marco-MiniLM-L-12-v2` | ~49.2 | English only | ~150-200 | Apache 2.0 |
| `BAAI/bge-reranker-large` | ~51.8 | English primarily | ~60-100 | MIT |
| Cohere Rerank v3 (API) | ~53.1 | Yes | N/A (API) | Proprietary |

---

## Approach C: Knowledge-Graph-Enhanced (Advanced)

> **Philosophy:** Layer a knowledge graph on top of hybrid search to enable multi-hop reasoning, entity-centric retrieval, and richer context for the LLM. Higher complexity, higher ceiling for certain query types.

### Components

| Layer | Choice | Rationale |
|-------|--------|-----------|
| **Chunking** | Document-structure-aware + entity-annotated chunks | Same as Approach A's structural chunking, but with an additional entity extraction pass. Each chunk is annotated with extracted entities (people, organizations, products, concepts) and their relationships. |
| **Embedding model** | OpenAI `text-embedding-3-small` (or Cohere `embed-v3` for multilingual) | Managed API for simplicity. The complexity budget is spent on the knowledge graph, not on self-hosting embeddings. |
| **LLM** | Claude Sonnet (primary) for both chat and entity extraction | Claude's long context window (200K tokens) and strong instruction-following make it well-suited for entity extraction prompts and multi-hop synthesis. |
| **Knowledge graph** | LlamaIndex `PropertyGraphIndex` backed by Neo4j | LlamaIndex provides a high-level API for KG construction from documents: automatic entity/relation extraction via LLM, storage in Neo4j, and graph-aware retrieval. Neo4j is the most mature graph database with excellent Cypher query language and visualization tools. |
| **Retrieval** | Three-pronged: vector + BM25 + graph traversal | (1) Hybrid vector+BM25 via RRF (same as Approach A). (2) Graph retrieval: extract entities from the query, traverse the knowledge graph for related entities and subgraphs. (3) Merge all results, prioritizing graph-retrieved context for relational queries. |
| **Reranking** | Cohere Rerank v3 | Applied after merging vector+BM25+graph results. |
| **Pipeline orchestration** | LlamaIndex | LlamaIndex has first-class support for `PropertyGraphIndex`, hybrid retrieval, and composable query engines. It provides the most integrated path for KG-enhanced RAG. |

### Architecture

```
                    Ingestion Pipeline
                    ==================

Document (LiteParse / Web scrape)
    |
    +-- Structural chunking (same as Approach A)
    |
    +-- Entity extraction (Claude Sonnet prompt)
    |     |
    |     +-- Entities: Person, Org, Product, Concept, Location, Date
    |     +-- Relations: "works_at", "supplies", "related_to", "part_of", etc.
    |
    +-- Embed chunks (text-embedding-3-small)
    |
    +-- Store:
          +-- Chunks + embeddings -> PostgreSQL + pgvector
          +-- Chunks + BM25 index -> PostgreSQL + ParadeDB
          +-- Entities + relations -> Neo4j PropertyGraph
          +-- Entity-to-chunk mapping -> PostgreSQL (or Neo4j)


                    Retrieval Pipeline
                    ==================

User Query
    |
    +-- Query analysis (detect if relational/multi-hop)
    |
    +-------+-------+-------+
    |               |               |
    v               v               v
 pgvector        ParadeDB        Neo4j
 (vector sim)    (BM25)          (graph traversal)
    |               |               |
    +-------+-------+-------+
                    |
                    v
            +---------------+
            |   RRF Merge   |  (weighted: vector 0.4, BM25 0.3, graph 0.3)
            +---------------+
                    |
                    v
            +---------------+
            | Cohere Rerank |
            +---------------+
                    |
                    v
            +---------------+
            |   LLM Gen     |  (Claude Sonnet, with graph context)
            +---------------+
                    |
                    v
               Response
```

### Knowledge Graph Schema (Example)

```
Nodes:
  - Entity(id, name, type, description, source_document_id, organization_id)
    Types: Person, Organization, Product, Concept, Location, Event, Date

  - Chunk(id, text_preview, document_id, organization_id)

Relationships:
  - MENTIONS(Entity -> Chunk)           -- entity appears in this chunk
  - RELATED_TO(Entity -> Entity)        -- generic relation
  - WORKS_AT(Person -> Organization)
  - SUPPLIES(Organization -> Product)
  - PART_OF(Concept -> Concept)         -- hierarchical
  - OCCURRED_ON(Event -> Date)
  - LOCATED_IN(Entity -> Location)

Multi-tenancy:
  - All nodes carry organization_id
  - Neo4j queries always filter by organization_id
  - Alternatively: separate Neo4j database per tenant (better isolation, higher overhead)
```

### Pros
- **Multi-hop reasoning.** Can answer questions like "Who are the suppliers of components used in Product X, and what other products do they supply?" by traversing the graph.
- **Entity-centric retrieval.** When a query mentions a known entity, the system can retrieve all related context (chunks, related entities, subgraphs) even if the exact words don't appear in the query.
- **Richer LLM context.** The LLM receives not just relevant text chunks but also structured entity/relationship context, leading to more grounded and precise answers.
- **Disambiguation.** The graph helps distinguish between entities with the same name (e.g., "Apple" the company vs. "apple" the fruit) via their relationship context.
- **Progressive enhancement.** The KG improves over time as more documents are ingested and more entity/relation triples are extracted.

### Cons
- **Significant complexity.** Neo4j is another service to deploy, monitor, backup, and scale. Entity extraction adds LLM cost and latency to the ingestion pipeline.
- **Entity extraction quality.** LLM-based entity extraction is imperfect. Errors compound: bad entities lead to bad relations lead to bad retrieval. Requires ongoing quality monitoring.
- **Ingestion latency.** Each document now requires: chunking + embedding + BM25 indexing + entity extraction + graph insertion. This adds significant time to the ingestion pipeline.
- **Multi-tenancy in Neo4j.** Graph database multi-tenancy is less mature than in PostgreSQL. Options are property-based filtering (slower) or database-per-tenant (operational overhead).
- **Overkill for simple queries.** Most knowledge-base queries ("What is our refund policy?") are well-served by hybrid search alone. The graph adds value only for relational and multi-hop queries.
- **LlamaIndex coupling.** Using LlamaIndex's PropertyGraphIndex ties the pipeline to LlamaIndex's abstractions and versioning.

### When to Choose This Approach
- The knowledge base contains highly interconnected information (e.g., supply chain data, organizational hierarchies, product catalogs with complex relationships).
- Users frequently ask relational or multi-hop questions.
- The team has capacity to operate Neo4j and invest in entity extraction quality.

---

## Cross-Cutting Concerns

### Multi-Tenancy

All three approaches must enforce tenant isolation:

| Layer | Isolation Strategy |
|-------|-------------------|
| PostgreSQL (chunks, embeddings) | `organization_id` column + Row-Level Security (RLS) policies. Every query includes `WHERE organization_id = ?`. |
| ParadeDB (BM25 index) | BM25 index includes `organization_id` as a filterable field. Filter applied at query time. |
| Neo4j (if used) | Property-based filtering (`organization_id` on all nodes) or database-per-tenant for stronger isolation. |
| Embedding namespacing | Not needed if using Postgres (RLS handles it). If ever moving to Pinecone/Weaviate, use namespace-per-tenant. |

### Chunking Strategy Comparison

| Strategy | Quality | Ingestion Speed | Complexity | Best For |
|----------|---------|-----------------|-----------|----------|
| **Fixed-size** (e.g., 512 tokens) | Low (cuts mid-sentence) | Very fast | Trivial | Prototyping only |
| **Recursive character** (split on \n\n, \n, ., space) | Medium | Fast | Low | General purpose, good default |
| **Document-structure-aware** (LiteParse headings/tables) | High | Fast | Medium | Raven (we have structure from LiteParse) |
| **Semantic** (embedding-based boundary detection) | Highest | Slow (requires embedding each sentence) | High | When chunk coherence is critical |

**Recommendation:** Start with document-structure-aware chunking (Approach A). It leverages LiteParse output directly and produces good-quality chunks without the ingestion overhead of semantic chunking. Evaluate semantic chunking if retrieval quality is insufficient.

### Pipeline Orchestration Comparison

| Framework | Pros | Cons | Best For |
|-----------|------|------|----------|
| **Custom (plain Python)** | Full control, no abstraction overhead, easy to debug, no version conflicts | Must build everything: retries, streaming, component swapping | Teams that want simplicity and control |
| **LlamaIndex** | First-class KG support (`PropertyGraphIndex`), good pgvector integration, composable query engines | Heavy abstractions, frequent breaking changes between versions, can be opaque | KG-enhanced RAG (Approach C) |
| **LangChain** | Largest ecosystem, most integrations, good for prototyping | Overly complex abstractions, "framework tax", debugging is painful, rapid version churn | Prototyping and demos |
| **Haystack 2.x** | Clean pipeline API, good for production, built-in evaluation, strong typing | Smaller ecosystem than LangChain, fewer integrations | Self-hosted production pipelines (Approach B) |

**Recommendation:** Start custom (Approach A). Adopt LlamaIndex only if pursuing the KG path (Approach C). Avoid LangChain for production.

### LLM Provider Strategy

| Model | Strengths | Weaknesses | Cost (approx per 1M tokens) | Best Use in Raven |
|-------|-----------|------------|---------------------------|-------------------|
| **Claude 3.5 Sonnet** | Strong reasoning, 200K context, excellent instruction-following | Slightly slower than GPT-4o-mini for simple queries | $3 input / $15 output | Primary chat LLM, entity extraction |
| **Claude 3.5 Haiku** | Fast, cheap, good for straightforward tasks | Less capable for complex reasoning | $0.25 input / $1.25 output | Voice agent (latency-sensitive), simple queries |
| **GPT-4o** | Fast, multimodal, good at structured output (JSON mode) | Slightly weaker on long-form reasoning vs. Claude | $2.50 input / $10 output | Fallback provider, structured extraction |
| **GPT-4o-mini** | Very fast, very cheap | Less capable | $0.15 input / $0.60 output | High-volume, low-complexity queries |
| **Llama 3.1 70B** | Self-hosted, no API costs, good quality | Requires GPU infra, not as capable as frontier models | Infra cost only | Cost-optimized high-volume (Approach B) |
| **Mistral Large 2** | Strong European language support, self-hostable | Smaller community, fewer benchmarks | $2 input / $6 output (API) | Multilingual knowledge bases |

**Recommendation:** Multi-provider with Claude as primary. Use Claude Haiku for the voice agent path (latency matters) and Claude Sonnet for chatbot (quality matters). GPT-4o as fallback. Abstract the LLM behind a provider interface from day one.

---

## Recommendation Summary

| Dimension | Recommended Starting Point | Rationale |
|-----------|---------------------------|-----------|
| **Chunking** | Document-structure-aware (LiteParse boundaries + recursive fallback) | Leverages existing LiteParse output; good quality without semantic chunking overhead |
| **Embedding** | OpenAI `text-embedding-3-small` (1536d, or truncated to 512d) | Best managed cost/performance; switch to `nomic-embed-text-v1.5` if self-hosting needed |
| **LLM** | Claude Sonnet (chat) + Claude Haiku (voice) + GPT-4o (fallback) | Multi-provider resilience; Claude for quality, Haiku for latency |
| **Knowledge graph** | Skip initially; add later if relational queries become a significant use case | Complexity vs. value; hybrid search covers 80-90% of queries well |
| **Retrieval** | Hybrid vector + BM25 via RRF, with Cohere Rerank v3 | Proven to outperform pure vector; reranking adds significant relevance lift |
| **Orchestration** | Custom lightweight pipeline | Minimal abstraction; adopt framework only when complexity demands it |

### Phased Roadmap

```
Phase 1 (MVP):       Approach A -- Pragmatic Hybrid
                      Custom pipeline, managed APIs, hybrid search + reranking
                      Target: 4-6 weeks to working RAG

Phase 2 (Optimize):  Evaluate retrieval quality with real user queries
                      Consider semantic chunking if quality is insufficient
                      Consider self-hosted embeddings if costs are too high
                      Add query routing (simple vs. complex queries)

Phase 3 (Enhance):   If relational queries are a significant use case:
                        Approach C -- Add knowledge graph layer
                      If cost is the primary concern:
                        Approach B -- Self-host embedding + reranking
                      Both can be layered incrementally
```

---

## Key Sources

### Chunking
- [Chunking Strategies for LLM Applications (Pinecone)](https://www.pinecone.io/learn/chunking-strategies/)
- [5 Levels of Text Splitting (Greg Kamradt)](https://github.com/FullStackRetrieval-com/RetrievalTutorials)
- [Evaluating Chunking Strategies for Retrieval (Arxiv 2024)](https://arxiv.org/abs/2406.14774)

### Embedding Models
- [MTEB Leaderboard (HuggingFace)](https://huggingface.co/spaces/mteb/leaderboard)
- [Nomic Embed Text v1.5](https://huggingface.co/nomic-ai/nomic-embed-text-v1.5)
- [BGE Models (BAAI)](https://huggingface.co/BAAI/bge-large-en-v1.5)
- [OpenAI Embeddings Guide](https://platform.openai.com/docs/guides/embeddings)

### Retrieval & Reranking
- [Hybrid Search Explained (Weaviate)](https://weaviate.io/blog/hybrid-search-explained)
- [Reciprocal Rank Fusion (RRF) (Cormack et al.)](https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf)
- [Cohere Rerank](https://docs.cohere.com/docs/reranking)
- [BGE Reranker (BAAI)](https://huggingface.co/BAAI/bge-reranker-v2-m3)

### Knowledge Graph RAG
- [LlamaIndex PropertyGraphIndex](https://docs.llamaindex.ai/en/stable/module_guides/indexing/property_graph_index/)
- [Graph RAG: Unlocking LLM Discovery (Microsoft Research)](https://www.microsoft.com/en-us/research/blog/graphrag-unlocking-llm-discovery-on-narrative-private-data/)
- [Neo4j + LlamaIndex Integration](https://neo4j.com/labs/genai-ecosystem/llamaindex/)

### Pipeline Orchestration
- [LlamaIndex Documentation](https://docs.llamaindex.ai/)
- [Haystack 2.x Documentation](https://docs.haystack.deepset.ai/docs/intro)
- [Why We Replaced LangChain (Octomind)](https://www.octomind.dev/blog/why-we-no-longer-use-langchain-for-building-our-ai-agents)

### ParadeDB + pgvector
- See companion document: [paradedb-pgvector-research.md](./paradedb-pgvector-research.md)
