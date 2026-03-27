# Raven Backend Architecture Approaches

> Research draft -- 2026-03-27

---

## Context & Constraints

| Concern | Decision / Constraint |
|---|---|
| Frontend | Vue.js + Tailwind Plus |
| CMS | Strapi (headless, manages orgs/users) |
| Auth | Keycloak + custom SPI "reavencloak" |
| Doc Parsing | LiteParse (TypeScript/Node.js, Apache 2.0, local) |
| Vector Store | PostgreSQL + pgvector + ParadeDB (full-text) |
| Web Scraping | Separate tool needed (LiteParse cannot handle HTML) |
| Hierarchy | Application -> Organization -> Knowledge Base |
| Interaction modes | Embeddable chatbot, voice agent, WebRTC/WhatsApp voice |

---

## Approach 1: Node.js Modular Monolith

### Architecture Style
Modular monolith -- single deployable, internal modules with clear boundaries.

### Language(s)
TypeScript (Node.js) throughout.

### Key Components

| Module | Responsibility |
|---|---|
| `@raven/api` | REST/tRPC gateway, route definitions, auth middleware (Keycloak JWT validation) |
| `@raven/cms-bridge` | Thin adapter over Strapi REST API for org/user/KB CRUD |
| `@raven/ingest` | Orchestrates doc upload, web scraping, calls LiteParse |
| `@raven/embeddings` | Calls embedding model (OpenAI / local), writes to pgvector |
| `@raven/search` | Hybrid retrieval: pgvector ANN + ParadeDB FTS, re-ranking |
| `@raven/chat` | LLM orchestration (RAG pipeline), streaming responses |
| `@raven/voice` | WebRTC signaling, WhatsApp Business API bridge, TTS/STT |
| `@raven/jobs` | BullMQ workers (Redis-backed) for async parsing, embedding, scraping |

```
[ Vue.js SPA ] ---> [ API Gateway (tRPC or REST) ]
                          |
            +-------------+-------------+
            |             |             |
      cms-bridge      ingest        chat/voice
      (Strapi)     (LiteParse)    (LLM + RAG)
            |             |             |
            +------+------+------+------+
                   |             |
              PostgreSQL    Redis (BullMQ)
            (pgvector/PDB)
```

### Job Queue Architecture
- **BullMQ** on Redis.
- Queues: `doc-parse`, `web-scrape`, `embed`, `reindex`.
- Flow: upload -> `doc-parse` job -> on success fan-out to `embed` job(s).
- Web scrape: `web-scrape` job (using Crawlee/Playwright) -> `embed`.
- Dashboard: Bull Board for monitoring.

### API Design
- **tRPC** between Vue frontend and backend (end-to-end type safety, zero codegen).
- Strapi keeps its own REST API for CMS admin panel.
- Embeddable chatbot widget talks via REST or WebSocket for streaming.

### Pros
- Single language = simpler hiring, shared types, shared tooling.
- LiteParse is TypeScript -- direct in-process calls, no IPC overhead.
- tRPC gives end-to-end type safety Vue <-> Backend with minimal boilerplate.
- Modular monolith is easy to deploy early, can extract services later.
- BullMQ is battle-tested for Node.js job queues.

### Cons
- Node.js is weaker for CPU-heavy embedding work (mitigated if using external API).
- Python AI/ML ecosystem (LangChain, LlamaIndex, etc.) unavailable without a sidecar.
- Single process risk: a bad module can crash everything (mitigated by worker threads + separate BullMQ workers).
- tRPC ties you to TypeScript clients; if you later need mobile native, you need a REST/GraphQL layer too.

---

## Approach 2: Hybrid -- Node.js API + Python AI Workers

### Architecture Style
Service-oriented: two primary services + shared PostgreSQL.

### Language(s)
- **TypeScript (Node.js)**: API server, CMS bridge, ingest orchestration.
- **Python**: AI pipeline (embedding, RAG, LLM orchestration, voice).

### Key Components

| Service | Language | Responsibility |
|---|---|---|
| `raven-api` | TypeScript | REST API, auth (Keycloak), Strapi bridge, file upload, WebSocket for chat streaming |
| `raven-ai` | Python | Embedding generation, RAG retrieval, LLM calls, re-ranking, voice STT/TTS |
| `raven-workers` | TypeScript | BullMQ workers: doc parsing (LiteParse), web scraping (Crawlee) |
| Strapi | JS (stock) | CMS admin for orgs/users/KBs -- used as-is |
| Redis | -- | Job queue (BullMQ) + pub/sub for real-time |
| PostgreSQL | -- | pgvector + ParadeDB, application data |

```
[ Vue.js SPA ] ---> [ raven-api (Node.js) ]
                          |          \
                    [ Strapi ]    [ Redis/BullMQ ]
                          |          \
                    [ PostgreSQL ]  [ raven-workers (Node.js) ]
                                        |
                                   [ LiteParse | Crawlee ]
                                        |
                                   [ raven-ai (Python) ]
                                        |
                                   [ LLM API | pgvector ]
```

### Job Queue Architecture
- **BullMQ** (Node.js side) for doc parsing and scraping.
- **Celery** or **Dramatiq** (Python side) for embedding and LLM tasks -- OR keep BullMQ as the single queue and have Python workers consume via a thin bridge (e.g., HTTP callback or Redis direct).
- Simpler option: Node.js BullMQ worker calls Python `raven-ai` via internal HTTP/gRPC after parsing completes. Avoids two queue systems.

### API Design
- **REST** (OpenAPI 3.1 spec) for the main API -- broadest client compatibility.
- Internal: gRPC between `raven-api` and `raven-ai` for low-latency embedding/RAG calls.
- WebSocket endpoint on `raven-api` for streaming chat responses.

### Pros
- Best of both worlds: Node.js for I/O-heavy API work, Python for AI/ML.
- Full access to Python ecosystem: LangChain, LlamaIndex, sentence-transformers, whisper, etc.
- LiteParse stays in its native Node.js runtime.
- REST API is universally consumable (mobile, embeddable widget, third-party).
- Easier to scale AI workers independently.

### Cons
- Two languages = two build systems, two dependency managers, more DevOps complexity.
- Inter-service communication adds latency and failure modes.
- Need to keep shared types/schemas in sync (mitigate with OpenAPI/protobuf).
- Potentially two queue systems (BullMQ + Celery) unless you standardize on one.

---

## Approach 3: Python-First with Node.js Sidecar

### Architecture Style
Monolith (Python) + thin Node.js sidecar for TypeScript-specific tasks.

### Language(s)
- **Python**: Primary backend (FastAPI).
- **TypeScript (Node.js)**: Sidecar for LiteParse + web scraping only.

### Key Components

| Component | Language | Responsibility |
|---|---|---|
| `raven-core` | Python (FastAPI) | REST API, auth middleware, Strapi bridge, RAG pipeline, LLM orchestration, embeddings, voice |
| `raven-parser` | TypeScript | Thin HTTP service wrapping LiteParse + Crawlee for scraping |
| Strapi | JS (stock) | CMS admin |
| Celery + Redis | Python | Job queue for all async work |
| PostgreSQL | -- | pgvector + ParadeDB |

```
[ Vue.js SPA ] ---> [ raven-core (FastAPI) ]
                          |           \
                    [ Strapi ]    [ Celery + Redis ]
                          |           \
                    [ PostgreSQL ]  [ raven-parser (Node.js sidecar) ]
                                        |
                                   [ LiteParse | Crawlee ]
```

### Job Queue Architecture
- **Celery** with Redis broker for all async tasks.
- Tasks: `parse_document` (calls `raven-parser` sidecar via HTTP), `generate_embeddings`, `scrape_url`, `reindex_kb`.
- Celery Canvas for chaining: `parse_document | generate_embeddings`.

### API Design
- **REST** (FastAPI auto-generates OpenAPI docs).
- WebSocket via FastAPI for streaming chat.
- Internal HTTP between `raven-core` and `raven-parser` sidecar.

### Pros
- Python is the richest ecosystem for AI/ML/LLM work -- first-class LangChain, LlamaIndex, sentence-transformers, whisper, pydantic, etc.
- FastAPI is high-performance, async, and auto-generates OpenAPI docs.
- Celery is the most mature Python job queue with excellent monitoring (Flower).
- Node.js sidecar is minimal -- just wraps LiteParse, easy to maintain.
- Simplest path if AI/RAG complexity is the primary technical challenge.

### Cons
- Python is slower for raw I/O compared to Node.js (mitigated by FastAPI's async).
- Strapi bridge is less natural from Python (HTTP calls to a JS CMS).
- No type safety between Vue frontend and Python backend (mitigate with OpenAPI codegen).
- Node.js sidecar is an extra process to deploy and monitor.
- Less code sharing with the Vue frontend.

---

## Comparison Matrix

| Criteria | Approach 1: Node.js Modular Monolith | Approach 2: Hybrid Node+Python | Approach 3: Python-First |
|---|---|---|---|
| **Deployment simplicity** | Best (single deploy) | Medium (2-3 services) | Medium (2 services) |
| **AI/ML ecosystem access** | Poor (need external APIs) | Excellent | Excellent |
| **LiteParse integration** | Native (in-process) | Native (in worker) | HTTP sidecar call |
| **Type safety (frontend<->backend)** | Excellent (tRPC) | Good (OpenAPI codegen) | Good (OpenAPI codegen) |
| **Scaling flexibility** | Limited (scale whole monolith) | Best (scale independently) | Good (scale workers) |
| **Team skill requirements** | TypeScript only | TypeScript + Python | Primarily Python |
| **Operational complexity** | Lowest | Highest | Medium |
| **Long-term extensibility** | Good (can extract services) | Best (already service-oriented) | Good (can extract) |
| **Voice/WebRTC support** | Decent (mediasoup/livekit) | Best (Python whisper + Node signaling) | Good (Python-native) |

---

## Recommendation

**Start with Approach 2 (Hybrid)** but structure it to begin as simple as Approach 1:

1. **Phase 1 (MVP):** Build `raven-api` as a Node.js modular monolith with BullMQ. Use LiteParse in-process. Call LLM/embedding APIs externally (OpenAI, etc.) from Node.js -- no Python needed yet. Use REST + WebSocket.

2. **Phase 2 (AI maturity):** When you need local models, advanced RAG (re-ranking, query decomposition, agentic retrieval), or voice (Whisper), introduce `raven-ai` as a Python service. Shift embedding and RAG orchestration to Python.

3. **Phase 3 (Scale):** Extract hot paths into independent services. Scale `raven-ai` workers horizontally. Consider gRPC for internal comms.

### Rationale
- You avoid premature complexity while keeping the door open for Python's AI ecosystem.
- LiteParse integration remains native and fast.
- Strapi stays as the CMS -- don't fight it, just bridge to it.
- The modular monolith structure in Phase 1 lets one small team move fast.
- BullMQ handles all job queue needs initially; only add Celery if/when Python workers justify it.

### Key Decisions to Lock In Early
- **Database schema**: Design multi-tenant schema (shared DB, schema-per-org, or row-level security) upfront -- this is hard to change later.
- **Embedding model**: Choose a model and dimensionality (e.g., OpenAI `text-embedding-3-small` at 1536d or a local model) -- affects pgvector index type (IVFFlat vs HNSW) and storage.
- **Web scraper**: Evaluate Crawlee (TypeScript, Apify) vs Scrapy (Python) -- decision depends on which approach you pick.
- **API contract**: Define OpenAPI spec early even if using tRPC initially -- needed for embeddable widget and future mobile clients.

### On Strapi's Role
Strapi should remain a **content/admin tool**, not the API backbone:
- Use it for: org management, user profiles, KB metadata, content modeling, admin UI.
- Don't use it for: real-time chat, document processing, vector search, or anything performance-critical.
- Bridge pattern: `raven-api` calls Strapi's REST API internally for CRUD, exposes its own API to the frontend.
