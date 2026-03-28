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
| **Raven Pro: Enterprise Connectors** | Airbyte-powered data connectors, on-prem/hybrid deployment, data classification, ClickHouse + QBit vectors at scale | Future |
| **Edge Optimization** | eBPF-based XDP pre-filtering, kernel-level observability, security audit trail | Future (post-Phase 2) |

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

## Raven Pro: Enterprise Connectors (Future)

On-premise and hybrid deployment model with data connectors for enterprise clients.

**Deployment Models:**
- Cloud-with-agent: Raven cloud + secure agent on client network
- On-premise: Full Raven stack inside client infrastructure

**Data Connectors (Airbyte-powered):**
- 700+ source connectors via embedded Airbyte (MIT core)
- Databases: PostgreSQL, MySQL, MongoDB, ClickHouse, Oracle, MSSQL, Snowflake, BigQuery
- SaaS: Salesforce, HubSpot, Confluence, Notion, Jira, Google Drive, SharePoint
- Files: S3, GCS, Azure Blob, local filesystem, Parquet
- CDC for real-time incremental sync; pipeline must be idempotent (dedup on source_id + chunk_hash)

**Data Classification & Knowledge Base Routing:**
- Tier 1: Auto-pull metadata from existing data catalogs (dbt, DataHub, Apache Atlas, Glue, Snowflake tags)
- Tier 2: LLM-assisted schema inference for raw databases (sample → classify → admin approves)
- Tier 3: Column-based routing rules for multi-tenant tables (e.g., WHERE org = 'harvard' → Harvard KB)
- Admin configures via UI (cloud) or YAML (self-hosted)

**Knowledge Base Vector Storage at Scale (replacing pgvector with ClickHouse + QBit):**
- Phase 1 (MVP): pgvector + ParadeDB for RAG embeddings (ceiling ~5–10M chunks)
- Enterprise scale: Migrate embeddings to ClickHouse + QBit, partitioned by org_id/kb_id. PostgreSQL stays for relational data + ParadeDB BM25. Trigger: any tenant exceeds ~5M chunks.
- Extreme scale (single KB > 10M chunks): Evaluate Qdrant with scalar quantization

**Observability (Two-Tier):**
- Cloud/enterprise: SigNoz (ClickHouse-backed, all three signals)
- Edge/self-hosted: OpenObserve (single binary, user-swappable via OTel endpoint env var)

## Edge Optimization (Future)

Low-level Linux kernel optimizations using eBPF. See `docs/research/ebpf-edge-optimization.md`.

**1. XDP Pre-filtering (Rate Limiting Offload)** — Drop/throttle traffic at the NIC before TCP stack.
**2. Kernel-level Observability (Zero-agent Metrics)** — CPU, memory, syscall metrics via kprobes/tracepoints.
**3. Security Audit Trail (Process + Syscall Monitoring)** — Trace sys_execve and socket calls for GDPR/SOC2.

## Milestones

See [GitHub Milestones](https://github.com/ravencloak-org/Raven/milestones) for detailed task tracking.
