# Roadmap

## Phase Overview

| Phase | Scope | Timeline |
|-------|-------|----------|
| **Phase 1: MVP (Chatbot)** | Core API, ingestion pipeline, embeddable chatbot, admin dashboard, SaaS infra | 8-12 weeks |
| **Phase 2: Voice Agent** | LiveKit, STT/TTS, voice sessions, email notifications, smart caching | 4-6 weeks after Phase 1 |
| **v1.0 GA Readiness** | Error tracking, API docs, CDN, status page, virus scanning, secrets mgmt | Alongside Phase 2/3 |
| **Phase 3: WebRTC/WhatsApp** | WhatsApp Business Calling API, LiveKit bridging, browser WebRTC | 4-6 weeks after Phase 2 |
| **Phase 4: Knowledge Graph** | Neo4j, entity extraction, multi-hop queries | Future |
| **Phase 5: Cloud Managed** | AWS Terraform, hosted offering, pricing, i18n, a11y | Future |

## Phase 1 -- MVP (Chatbot)

**Core:** Organization + Workspace + Knowledge Base CRUD, Keycloak auth, PostgreSQL 18 + pgvector

**Ingestion:** File upload (PDF, DOCX, images), URL scraping (Crawl4AI), chunking, multi-provider embeddings (BYOK)

**Retrieval:** Hybrid search (pgvector + BM25 + RRF fusion), reranking

**Interaction:** Embeddable `<raven-chat>` web component with SSE streaming, API key auth

**Dashboard:** Vue.js + Tailwind Plus admin UI, chatbot configurator, test sandbox, analytics

**SaaS Infra:** Stripe billing, SSL/TLS, backups, legal pages, rate limiting, scheduled jobs

## Phase 2 -- Voice Agent + Smart Caching

**Voice:** LiveKit Agents, Deepgram/faster-whisper STT, Cartesia/Piper TTS, Silero VAD

**Email:** Conversation summaries via AWS SES, "resume conversation" links

**Smart Cache:** Semantic response cache (pgvector similarity) to minimize LLM API costs, in-database LLM adaptation for personalized cached responses

## Phase 3 -- WebRTC / WhatsApp

WhatsApp Business Calling API (WebRTC native), LiveKit room bridging, browser "call the assistant" button

## Milestones

See [GitHub Milestones](https://github.com/ravencloak-org/Raven/milestones) for detailed task tracking.
