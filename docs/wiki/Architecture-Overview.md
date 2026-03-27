# Architecture Overview

## High-Level Component Diagram

```
                         CLIENTS
              Vue.js + Tailwind Plus (SPA)
                         |
                       HTTPS
                         |
              REVERSE PROXY (Traefik)
    /api/* -> Go API   /cms/* -> Strapi   /auth/* -> Keycloak
         |                |                    |
         v                v                    v
    +----------+   +-----------+         +-----------+
    |  Go API  |   |  Strapi   |         | Keycloak  |
    |  (Gin)   |   |  CMS      |         | + reaven- |
    +----+-----+   +-----------+         |   cloak   |
         |                               +-----------+
         | gRPC
         v
    +-------------------+
    | Python AI Worker  |
    | - RAG Engine      |
    | - Embeddings      |     +----------+
    | - LiteParse (CLI) |---->| Valkey   |
    +--------+----------+     | (Queue)  |
             |                +----------+
             v
    +---------------------------+
    |       PostgreSQL 18       |
    | + pgvector (embeddings)   |
    | + ParadeDB (BM25 search)  |
    +---------------------------+
```

## Service Breakdown

| Service | Role | Exposed |
|---------|------|---------|
| **Go API (Gin)** | Primary API gateway. JWT validation, routing, tenant resolution, REST CRUD, enqueues async jobs, delegates AI to Python via gRPC. | Yes (:8080) |
| **Python AI Worker** | All AI/ML workloads. gRPC server for RAG queries, embedding generation. Consumes Valkey jobs for async document processing. | No (gRPC) |
| **LiteParse** | Document-to-text extraction. Invoked by Python worker as subprocess. | No (co-located) |
| **Strapi** | Headless CMS for marketing content and rapid admin tooling. Not in critical request path. | Yes (:1337) |
| **Keycloak** | Identity provider. OIDC/OAuth2, reavencloak SPI for custom claims. | Yes (:8443) |
| **PostgreSQL 18** | Primary datastore. pgvector for embeddings, ParadeDB for BM25, RLS for tenant isolation. | No |
| **Valkey** | Job queue, rate limiting, caching. BSD-3 Redis replacement. | No |
| **SeaweedFS** | S3-compatible object storage for uploaded files. | No |
| **Traefik** | Reverse proxy with auto-TLS via Let's Encrypt. | Yes (:80/:443) |

## Go <-> Python gRPC Interface

```protobuf
service AIWorker {
  rpc ParseAndEmbed (ParseRequest)      returns (ParseResponse);
  rpc QueryRAG      (RAGRequest)        returns (stream RAGChunk);
  rpc GetEmbedding  (EmbeddingRequest)  returns (EmbeddingResponse);
}
```

- **Synchronous** (`GetEmbedding`): Go blocks on call
- **Server-streaming** (`QueryRAG`): Python streams LLM tokens, Go forwards via SSE
- **Async via queue**: Document uploads go through Valkey, not gRPC

## Key Request Flows

### Upload Document
```
Client -> Go API (validate, store in SeaweedFS, enqueue Valkey job) -> 202 Accepted
Python Worker: dequeue -> LiteParse -> chunk -> embed (BYOK) -> store in pgvector
```

### Chat / RAG Query
```
Client -> Go API (auth, rate-limit) -> gRPC stream to Python Worker
Python Worker: embed query -> hybrid search (pgvector + BM25) -> RRF -> rerank -> LLM stream
Go API: forward tokens as SSE -> Client
```

## Edge Deployment Mode

For Raspberry Pi / ARM64 edge nodes:
- **On edge**: Go API only (~10MB binary, 5-10MB RAM)
- **Remote**: Python AI Worker, PostgreSQL, Valkey on a cloud server
- Connected via gRPC over network
