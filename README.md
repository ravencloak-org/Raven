<!-- Logo placeholder: Raven logo will go here -->
<p align="center">
  <img src="docs/assets/logo-placeholder.png" alt="Raven" width="200" />
</p>

<h1 align="center">Raven</h1>
<p align="center">Open-source multi-tenant knowledge base platform with AI-powered chat, voice, and WhatsApp</p>

<p align="center">
  <img src="https://img.shields.io/badge/build-passing-brightgreen" alt="Build Status" />
  <img src="https://img.shields.io/badge/license-TBD-lightgrey" alt="License" />
  <img src="https://img.shields.io/badge/PRs-welcome-blue" alt="PRs Welcome" />
</p>

---

## What is Raven?

Raven is a self-hostable, multi-tenant knowledge base platform that lets organizations ingest documents and web content, then query them through AI-powered channels -- an embeddable chatbot, a real-time voice agent, and WhatsApp. It combines hybrid retrieval (vector search + BM25), BYOK LLM support, and a modular architecture designed for both cloud and edge deployment.

The platform is organized around a clear hierarchy: **Organizations** (tenant boundaries) contain **Workspaces** (operational sub-units), which contain **Knowledge Bases** (collections of documents and web sources). Each layer enforces data isolation through PostgreSQL Row-Level Security, Keycloak-based authentication, and API middleware.

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

PostgreSQL serves as the single source of truth -- storing relational data, vector embeddings (pgvector), and full-text search indexes. A Vue.js SPA provides the admin dashboard, and Keycloak handles identity management.

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **API Server** | Go + Gin | REST API, JWT validation, tenant routing, SSE streaming |
| **AI Worker** | Python + gRPC | RAG queries, embeddings, document parsing, web scraping |
| **Database** | PostgreSQL 18 + pgvector | Relational data, vector search, BM25 full-text |
| **Frontend** | Vue.js 3 + Tailwind CSS | Admin dashboard (SPA, mobile-first, PWA-capable) |
| **Chatbot Widget** | Web Component | Embeddable `<raven-chat>` element |
| **Auth** | Keycloak | OIDC/OAuth2, user management, multi-tenant realms |
| **Job Queue** | Valkey (Redis fork) | Async document processing, caching, rate limiting |
| **Object Storage** | SeaweedFS | S3-compatible file storage (Apache 2.0) |
| **Voice** | LiveKit Server + Agents | WebRTC SFU, STT/LLM/TTS voice pipeline |
| **Reverse Proxy** | Traefik | Auto-TLS, routing, security headers |

## Quick Start / Deployment

> **Coming Soon** -- The Docker Compose deployment guide is currently in progress. Check back for updates or watch this repository for releases.

```bash
# Planned usage (not yet available):
git clone https://github.com/user/raven.git
cd raven
cp .env.example .env
docker compose up -d
```

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

- Browse [open issues](../../issues) for tasks and bug reports
- See the [design specification](docs/superpowers/specs/2026-03-27-raven-platform-design-final.md) for architectural context

## License

License TBD -- to be determined before public release.
