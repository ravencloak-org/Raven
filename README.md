<!-- Logo placeholder: Raven logo will go here -->
<p align="center">
  <img src="docs/assets/logo-placeholder.png" alt="Raven" width="200" />
</p>

<h1 align="center">Raven</h1>
<p align="center">Open-source multi-tenant knowledge base platform with AI-powered chat, voice, and WhatsApp</p>

<p align="center">
  <a href="https://github.com/ravencloak-org/Raven/actions/workflows/go.yml"><img src="https://github.com/ravencloak-org/Raven/actions/workflows/go.yml/badge.svg" alt="Go CI" /></a>
  <a href="https://github.com/ravencloak-org/Raven/actions/workflows/frontend.yml"><img src="https://github.com/ravencloak-org/Raven/actions/workflows/frontend.yml/badge.svg" alt="Frontend CI" /></a>
  <a href="https://github.com/ravencloak-org/Raven/actions/workflows/python.yml"><img src="https://github.com/ravencloak-org/Raven/actions/workflows/python.yml/badge.svg" alt="Python CI" /></a>
  <a href="https://github.com/ravencloak-org/Raven/actions/workflows/docker.yml"><img src="https://github.com/ravencloak-org/Raven/actions/workflows/docker.yml/badge.svg" alt="Docker Build" /></a>
  <a href="https://github.com/ravencloak-org/Raven/actions/workflows/security.yml"><img src="https://github.com/ravencloak-org/Raven/actions/workflows/security.yml/badge.svg" alt="Security" /></a>
  <a href="https://codecov.io/gh/ravencloak-org/Raven"><img src="https://codecov.io/gh/ravencloak-org/Raven/branch/main/graph/badge.svg" alt="Coverage" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="License: Apache 2.0" /></a>
  <a href="https://baseline.openssf.org/versions/2026-02-19"><img src="https://img.shields.io/badge/OpenSSF%20Baseline-L2%20target-blue" alt="OpenSSF Baseline L2" /></a>
  <img src="https://img.shields.io/badge/PRs-welcome-blue" alt="PRs Welcome" />
</p>
<p><a href="https://www.bestpractices.dev/projects/12590"><img src="https://www.bestpractices.dev/projects/12590/badge"></a>
</p>
---

## What is Raven?

Raven is a self-hostable, multi-tenant knowledge base platform that lets organizations ingest documents and web content, then query them through AI-powered channels -- an embeddable chatbot, a real-time voice agent, and WhatsApp. It combines hybrid retrieval (vector search + BM25), BYOK LLM support, and a modular architecture designed for both cloud and edge deployment.

The platform is organized around a clear hierarchy: **Organizations** (tenant boundaries) contain **Workspaces** (operational sub-units), which contain **Knowledge Bases** (collections of documents and web sources). Each layer enforces data isolation through PostgreSQL Row-Level Security, SuperTokens-based authentication, and API middleware.

Raven is built for teams that need a production-grade RAG platform without vendor lock-in. Bring your own LLM keys (Anthropic, OpenAI, Cohere), deploy on a cloud VM or a Raspberry Pi, and own your data end to end.

## Key Features

- **Multi-tenant by design** -- organization-level data isolation with RLS, per-tenant billing, and role-based access control
- **Embeddable chatbot** -- drop-in `<raven-chat>` web component with SSE streaming and domain-scoped API keys
- **Voice agent** -- real-time voice interface via LiveKit Agents with STT/TTS pipeline
- **WebRTC and WhatsApp** -- browser-based and WhatsApp Business Calling API integration
- **BYOK LLM** -- bring your own API keys for Anthropic Claude, OpenAI, Cohere, or self-hosted models
- **Hybrid search** -- pgvector cosine similarity + BM25 full-text search with Reciprocal Rank Fusion
- **Self-hostable** -- single Docker Compose deployment with no external SaaS dependencies required
- **Edge-deployable** -- Go API compiles to a ~25 MB ARM64 binary; run the API on a Raspberry Pi with a remote AI worker

## Architecture Overview

Raven uses a **two-process architecture**: a Go API server handles HTTP routing, authentication, and orchestration, while a Python AI worker handles all ML/AI workloads (embedding, RAG queries, document parsing, web scraping). The two communicate over gRPC, with Valkey (Redis-compatible) as the async job queue.

PostgreSQL serves as the single source of truth -- storing relational data, vector embeddings (pgvector), and full-text search indexes. A Vue.js SPA provides the admin dashboard, and SuperTokens handles identity management.

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **API Server** | Go + Gin | REST API, JWT validation, tenant routing, SSE streaming |
| **AI Worker** | Python + gRPC | RAG queries, embeddings, document parsing, web scraping |
| **Database** | PostgreSQL 18 + pgvector | Relational data, vector search, BM25 full-text |
| **Frontend** | Vue.js 3 + Tailwind CSS | Admin dashboard (SPA, mobile-first, PWA-capable) |
| **Chatbot Widget** | Web Component | Embeddable `<raven-chat>` element |
| **Auth** | SuperTokens | Email/password + OAuth (Google), session management, MFA |
| **Job Queue** | Valkey (Redis fork) | Async document processing, caching, rate limiting |
| **Object Storage** | SeaweedFS | S3-compatible file storage (Apache 2.0) |
| **Voice** | LiveKit Server + Agents | WebRTC SFU, STT/LLM/TTS voice pipeline |
| **Reverse Proxy** | Traefik | Auto-TLS, routing, security headers |

## Quick Start

```bash
git clone https://github.com/ravencloak-org/Raven.git
cd Raven
cp .env.example .env        # fill in required values (see comments inside)
docker compose up -d        # starts all services
```

The admin dashboard is available at `http://localhost:3000` once all containers are healthy. See [docs/quickstart.md](docs/quickstart.md) for a full walkthrough including first-user setup and SuperTokens configuration.

For local development without Docker, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Testing

```bash
# Unit tests
make test

# Integration tests (requires Docker — spins up pgvector via testcontainers)
make test-integration

# Benchmarks (BM25, hybrid search, cache, ingestion throughput)
make bench-integration

# Frontend E2E (Playwright)
cd frontend && npm run test:e2e
```

**Integration test coverage** (47 test cases + 7 benchmarks across 5 suites):

| Suite | Tests | What it covers |
|-------|-------|----------------|
| Ingestion | 20 | Document lifecycle (8-state machine), chunk/embedding storage, source creation, duplicates, concurrent ingestion, large docs (500 chunks), token accuracy |
| Search | 14 | BM25 keyword/phrase/filter/clamping, vector nearest-neighbor, hybrid RRF fusion/fallbacks, Unicode (CJK/emoji/RTL), duplicate embeddings |
| Cache | 10 | Valkey SHA256 exact-match, Postgres response_cache (hit_count, TTL, HNSW index, invalidation, Stats) |
| Benchmarks | 7 | BM25 p95 <100ms, hybrid p95 <200ms, HNSW p95 <50ms, ingestion <2s, token consistency |
| RLS | 8 | Document/chunk/embedding/cache/source tenant isolation, cross-org KB access, admin bypass |

All integration tests run against a real PostgreSQL (pgvector) instance via testcontainers-go. Build tag `integration` keeps them out of `go test ./...`.

## Roadmap

Development is organized into five phases. See the full [design specification](docs/superpowers/specs/2026-03-27-raven-platform-design-final.md) for details.

| Phase | Scope | Target |
|-------|-------|--------|
| **Phase 1** | MVP -- embeddable chatbot, document ingestion, hybrid search, admin dashboard | 8-12 weeks |
| **Phase 2** | Voice agent (LiveKit), smart response caching, email notifications | +4-6 weeks |
| **Phase 3** | WebRTC / WhatsApp Business Calling API integration | +4-6 weeks |
| **Phase 4** | Knowledge graph (Neo4j, multi-hop queries) | Future |
| **Phase 5** | Cloud managed offering, multi-region, SOC 2 | Future |

## Contributing

Contributions are welcome. Please open an issue to discuss proposed changes before submitting a pull request.

- Read [CONTRIBUTING.md](CONTRIBUTING.md) for branch naming, commit style, and PR workflow
- Read [DEVELOPMENT.md](DEVELOPMENT.md) to get your local environment running
- Browse [open issues](../../issues) for tasks and bug reports
- See the [architecture overview](docs/wiki/Architecture-Overview.md) and [data model](docs/wiki/Data-Model.md) for context

## Security

Vulnerability disclosure, supported versions, and response SLAs are in [SECURITY.md](SECURITY.md). Please do not open public issues for suspected security problems; use GitHub private advisories instead.

Current maintainers and their areas of ownership are in [MAINTAINERS.md](MAINTAINERS.md).

## Licensing

Raven is dual-licensed:

- **Open-source portion** — everything in this repository is licensed under the [Apache License 2.0](./LICENSE) **except** files and directories whose name begins with `ee-`.
- **Enterprise portion** — files prefixed with `ee-` (for example `ee-LICENSE`, `ee-README.md`) are covered by the [Raven Enterprise License](./ee-LICENSE) and are **not** open-source. They are not included in any Apache-2.0 obligations, nor in the release artifacts that target OpenSSF Baseline compliance.

The open-source portion targets [OpenSSF Baseline 2026-02-19](https://baseline.openssf.org/versions/2026-02-19) **Level 2** compliance. See [`docs/architecture.md`](docs/architecture.md) for a system-level overview and [`docs/dependency-policy.md`](docs/dependency-policy.md) for how we manage supply chain.
