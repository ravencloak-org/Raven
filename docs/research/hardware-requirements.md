# Raven Platform -- Hardware Requirements Analysis

**Date:** 2026-03-27
**Status:** Research document -- no code written
**Scope:** Three deployment scenarios for the full Raven service stack

---

## Table of Contents

1. [Service Stack Overview](#1-service-stack-overview)
2. [Per-Service Resource Profiles](#2-per-service-resource-profiles)
3. [Scenario 1: Edge / Raspberry Pi (Go API Only)](#3-scenario-1-edge--raspberry-pi-go-api-only)
4. [Scenario 2: Self-Hosted Single Server](#4-scenario-2-self-hosted-single-server)
5. [Scenario 3: Production Cloud (Multi-Tenant SaaS)](#5-scenario-3-production-cloud-multi-tenant-saas)
6. [Deep Dives: Critical Resource Consumers](#6-deep-dives-critical-resource-consumers)
7. [Summary Comparison Table](#7-summary-comparison-table)

---

## 1. Service Stack Overview

| Service | Language/Runtime | Role | Deployment Phase |
|---------|-----------------|------|-----------------|
| Go API Server | Go (Gin) | REST API gateway, JWT validation, routing | Phase 1 |
| Python AI Worker | Python 3.11+ | Embedding generation, RAG queries, doc processing | Phase 1 |
| PostgreSQL 18 | C | Primary database + pgvector + ParadeDB (BM25) | Phase 1 |
| Valkey | C (Redis-compatible) | Cache, job queue (BullMQ-compatible) | Phase 1 |
| Keycloak | Java (Quarkus) | Authentication, SSO, realm management | Phase 1 |
| Strapi | Node.js | CMS for org/user/workspace management | Phase 1 |
| SeaweedFS | Go | S3-compatible object storage for documents | Phase 1 |
| Traefik | Go | Reverse proxy, TLS termination, routing | Phase 1 |
| LiveKit Server | Go | WebRTC SFU for voice agent sessions | Phase 2 |
| OpenObserve | Rust | Logs, metrics, traces (observability) | Optional |
| PostHog | Cloud SaaS | Product analytics -- no self-hosted resources | All phases |

---

## 2. Per-Service Resource Profiles

### 2.1 Go API Server (Gin)

| State | CPU | RAM | Disk | Notes |
|-------|-----|-----|------|-------|
| Idle | <0.01 cores | 10-15 MB | ~20 MB binary | Compiled Go binary, negligible idle footprint |
| Light load (10 req/s) | 0.05-0.1 cores | 20-40 MB | -- | Goroutine-per-request, minimal allocation |
| Moderate load (100 req/s) | 0.2-0.5 cores | 50-100 MB | -- | Connection pooling to Postgres/Valkey |
| Heavy load (1000 req/s) | 1-2 cores | 100-200 MB | -- | GC pauses negligible with Go 1.22+ |

**Key characteristics:** Go's compiled binary and goroutine model make this the most efficient service in the stack. Memory growth is linear with concurrent connections. Even under heavy load, a single instance rarely exceeds 200 MB. The binary itself is ~15-20 MB including all dependencies.

### 2.2 Python AI Worker

| State | CPU | RAM | Disk | Notes |
|-------|-----|-----|------|-------|
| Idle (worker listening) | <0.01 cores | 80-150 MB | ~500 MB (venv + deps) | Python runtime + imported libraries baseline |
| LiteParse document parsing | 0.5-1 core | 200-500 MB | -- | Depends on document size; PDFs with images spike higher |
| Large PDF processing (100+ pages) | 1-2 cores | 500 MB-1.5 GB | -- | In-memory page parsing, bounding box extraction |
| Embedding batch (OpenAI API) | 0.1-0.3 cores | 200-400 MB | -- | CPU-light (API call); RAM for chunk batches in memory |
| Embedding batch (local model, e.g., sentence-transformers) | 2-4 cores (or 1 GPU) | 1-3 GB | 1-2 GB model files | ONNX/PyTorch model loaded into memory |
| RAG query (retrieve + rerank) | 0.2-0.5 cores | 200-400 MB | -- | DB query + Cohere rerank API call + LLM streaming |
| Crawl4AI web scraping (headless Chromium) | 1-2 cores | 300 MB-1 GB per tab | ~400 MB (Chromium install) | Playwright + Chromium is the memory hog; each tab is ~150-300 MB |
| Peak concurrent (3 Crawl4AI tabs + PDF processing) | 3-6 cores | 2-4 GB | -- | Worst case during bulk ingestion |

**Key characteristics:** The Python worker is the most variable resource consumer. Idle footprint is modest (~100 MB), but spikes dramatically during document processing and web scraping. Crawl4AI's embedded Playwright/Chromium is the single largest memory consumer -- headless Chromium allocates 150-300 MB per browser tab. Large PDF processing with LiteParse can temporarily hold entire documents in memory. Plan for peak, not idle.

### 2.3 PostgreSQL 18 + pgvector + ParadeDB

| Configuration | CPU | RAM | Disk | Notes |
|---------------|-----|-----|------|-------|
| Baseline (empty, tuned) | 0.1 cores | 256 MB (shared_buffers) | 1 GB | Minimum viable; `shared_buffers=128MB`, `work_mem=4MB` |
| 100K embeddings (1536d, float32) | 0.2-0.5 cores | 512 MB-1 GB | 1-2 GB | ~900 MB raw vector data + HNSW index overhead |
| 1M embeddings (1536d, float32) | 0.5-1 core | 2-4 GB | 10-15 GB | HNSW index alone is ~6-8 GB; `shared_buffers=1GB` |
| 10M embeddings (1536d, float32) | 2-4 cores | 8-16 GB | 100-150 GB | HNSW graph is ~60-80 GB; `shared_buffers=4GB`, NVMe essential |
| BM25 index (ParadeDB, 1M docs) | 0.2-0.5 cores | 512 MB-1 GB | 2-5 GB | LSM tree with inverted + columnar segments |
| BM25 index (ParadeDB, 10M docs) | 0.5-1 core | 1-2 GB | 20-40 GB | Background merge operations need CPU headroom |

**Embedding storage math (1536-dimensional, float32):**
- Per vector: 1536 dims x 4 bytes = 6,144 bytes (~6 KB)
- 100K vectors raw: ~600 MB
- 1M vectors raw: ~6 GB
- 10M vectors raw: ~60 GB
- HNSW index overhead: approximately 1.0-1.3x raw vector size (graph edges, metadata)
- With quantization (halfvec, float16): halve the above numbers
- With scalar quantization (int8): quarter the above numbers

**Recommended PostgreSQL tuning by scale:**

| Parameter | 100K vectors | 1M vectors | 10M vectors |
|-----------|-------------|------------|-------------|
| `shared_buffers` | 256 MB | 1 GB | 4 GB |
| `effective_cache_size` | 1 GB | 4 GB | 16 GB |
| `work_mem` | 8 MB | 16 MB | 32 MB |
| `maintenance_work_mem` | 256 MB | 1 GB | 2 GB |
| `max_parallel_workers_per_gather` | 2 | 4 | 4 |
| `hnsw.ef_search` | 40 | 40-100 | 100-200 |
| `hnsw.m` (build-time) | 16 | 16-32 | 32-48 |

### 2.4 Valkey (Redis-Compatible)

| State | CPU | RAM | Disk | Notes |
|-------|-----|-----|------|-------|
| Idle | <0.01 cores | 5-10 MB | -- | Negligible baseline |
| Light caching (1000 keys) | <0.05 cores | 20-50 MB | -- | Session tokens, rate limit counters |
| Job queue (100 pending jobs) | <0.05 cores | 30-80 MB | -- | BullMQ-style sorted sets + job payloads |
| Active queue (1000 pending + 50 processing) | 0.1-0.2 cores | 100-300 MB | -- | Job metadata, retry state, delayed queues |
| Heavy cache (100K keys, avg 1KB) | 0.1-0.3 cores | 200-500 MB | -- | Hot document metadata, embedding cache |

**Key characteristics:** Valkey is memory-efficient for typical use cases. The job queue workload for Raven (document processing, scraping tasks) is bursty but individual job payloads are small (URLs, document IDs, metadata). Real memory usage depends heavily on what you cache. For a knowledge-base platform, session caching + job queues rarely exceed 500 MB even at moderate scale.

### 2.5 Keycloak (Java/Quarkus)

| State | CPU | RAM | Disk | Notes |
|-------|-----|-----|------|-------|
| Cold start | 0.5-1 core | 400-600 MB | ~200 MB (container) | JVM startup, class loading |
| Idle (warmed) | 0.05-0.1 cores | 512-768 MB | -- | JVM heap + metaspace baseline; cannot go much lower |
| Active auth (10 logins/min) | 0.1-0.3 cores | 512-768 MB | -- | Token issuance, JWT signing |
| Active auth (100 logins/min) | 0.3-0.5 cores | 768 MB-1 GB | -- | Connection pooling to Postgres |
| Heavy load (1000 logins/min) | 1-2 cores | 1-1.5 GB | -- | Multiple realms, custom SPIs |

**Key characteristics:** Keycloak is the heaviest "infrastructure" service due to the JVM. Since Keycloak 17+, it runs on Quarkus (not WildFly), which improved startup time and baseline memory. However, the JVM floor is ~512 MB even when idle. For a multi-tenant SaaS, a single Keycloak instance handles dozens of realms comfortably. The custom `reavencloak` SPI adds negligible overhead. Keycloak uses its own PostgreSQL schema (can share the same PostgreSQL instance).

**JVM tuning for constrained environments:**
```
JAVA_OPTS="-Xms256m -Xmx512m -XX:MaxMetaspaceSize=128m -XX:+UseG1GC"
```
This caps heap at 512 MB. Going below 256 MB heap risks frequent GC pauses and OOM under modest load.

### 2.6 Strapi (Node.js CMS)

| State | CPU | RAM | Disk | Notes |
|-------|-----|-----|------|-------|
| Idle | <0.05 cores | 100-200 MB | ~300 MB (node_modules) | Node.js event loop + loaded plugins |
| Active (10 req/s) | 0.1-0.3 cores | 200-350 MB | -- | Content CRUD, media uploads |
| Admin panel active | 0.2-0.5 cores | 250-400 MB | -- | Admin UI compilation, content type builder |
| Peak (50 req/s) | 0.5-1 core | 300-500 MB | -- | Rare -- Strapi mostly handles admin operations |

**Key characteristics:** Strapi is used for org/user/workspace CRUD and is not on the hot path for chatbot/voice queries. Its Node.js process idles at 100-200 MB. The admin panel React build adds a one-time disk cost. Strapi uses PostgreSQL as its backing store (can share the Raven PostgreSQL instance). For production, Strapi's API is primarily internal (Go API calls Strapi for content management), so it sees relatively low traffic.

### 2.7 SeaweedFS (Object Storage)

| State | CPU | RAM | Disk | Notes |
|-------|-----|-----|------|-------|
| Master (idle) | <0.05 cores | 30-50 MB | ~50 MB | Metadata management, volume assignments |
| Volume server (idle) | <0.05 cores | 50-100 MB | -- | Go binary, memory-mapped file access |
| Filer (S3 API gateway) | <0.1 cores | 50-100 MB | -- | Optional; provides S3-compatible API |
| Active upload (10 concurrent) | 0.2-0.5 cores | 100-200 MB | -- | Buffering incoming file chunks |
| Storage growth | -- | -- | Varies | ~1.1x raw file size (minimal overhead) |

**Storage estimation for documents:**
- Average PDF: 500 KB-5 MB
- 1000 documents per tenant: 500 MB-5 GB per tenant
- 100 tenants: 50-500 GB total
- 1000 tenants: 500 GB-5 TB total

**Key characteristics:** SeaweedFS (Apache 2.0 licensed, chosen over MinIO's AGPL) is extremely lightweight. The Go binary for master + volume server together consume under 200 MB RAM. Storage overhead is minimal compared to raw file sizes. SeaweedFS supports erasure coding for durability at scale, which adds ~1.5x storage overhead but provides fault tolerance.

### 2.8 Traefik (Reverse Proxy)

| State | CPU | RAM | Disk | Notes |
|-------|-----|-----|------|-------|
| Idle | <0.01 cores | 30-50 MB | ~50 MB (binary) | Go binary, loaded configuration |
| Active routing (100 req/s) | 0.05-0.1 cores | 50-80 MB | -- | Header inspection, routing rules |
| Active routing (1000 req/s) | 0.1-0.3 cores | 80-150 MB | -- | TLS termination is the main CPU cost |
| With Let's Encrypt ACME | <0.05 cores (periodic) | +10-20 MB | -- | Certificate renewal every 60-90 days |

**Key characteristics:** Traefik is one of the lightest services. Its Go binary is efficient, and it handles TLS termination, path-based routing, and middleware chains with minimal overhead. Memory scales linearly with the number of active connections, not with request rate. For Docker Compose deployments, Traefik auto-discovers services via Docker labels.

### 2.9 LiveKit Server (Phase 2)

| State | CPU | RAM | Disk | Notes |
|-------|-----|-----|------|-------|
| Idle (no rooms) | <0.05 cores | 30-50 MB | ~30 MB (binary) | Go binary, TURN/STUN listener |
| 1 active voice room (2 participants) | 0.1-0.2 cores | 50-80 MB | -- | SFU forwarding, no transcoding |
| 10 active voice rooms | 0.5-1 core | 200-400 MB | -- | Audio-only rooms are lightweight |
| 1 active video room (4 participants, 720p) | 0.5-1 core | 100-200 MB | -- | Simulcast reduces CPU; SFU does not transcode |
| 10 active video rooms (4 participants each) | 2-4 cores | 500 MB-1 GB | -- | Bandwidth is the bottleneck, not CPU |

**Bandwidth per voice session:**
- Opus audio codec: 32-64 kbps per stream
- 2-participant voice call: ~64-128 kbps total
- Voice agent session (user + AI agent): ~64-128 kbps
- 10 concurrent voice sessions: ~640 kbps-1.3 Mbps
- 100 concurrent voice sessions: ~6.4-12.8 Mbps

**Bandwidth per video session (optional, future):**
- 720p video: ~1.5-2.5 Mbps per stream
- 4-participant video room: ~6-10 Mbps total per room
- 10 concurrent video rooms: ~60-100 Mbps

**Key characteristics:** LiveKit is an SFU (Selective Forwarding Unit) -- it forwards media packets without transcoding, making it CPU-efficient. For Raven's voice agent use case (user speaks to AI), each session has exactly 2 audio streams (user in, AI out). The primary bottleneck is network bandwidth, not CPU or RAM. LiveKit's Go implementation is memory-efficient.

### 2.10 OpenObserve (Optional Observability)

| State | CPU | RAM | Disk | Notes |
|-------|-----|-----|------|-------|
| Idle | 0.05-0.1 cores | 100-200 MB | ~100 MB (binary) | Rust binary, embedded search |
| Light ingestion (100 events/s) | 0.1-0.3 cores | 200-500 MB | 1-5 GB/month | Compressed log storage |
| Moderate ingestion (1000 events/s) | 0.5-1 core | 500 MB-1 GB | 10-50 GB/month | All services logging + metrics |
| Heavy ingestion (10K events/s) | 1-2 cores | 1-2 GB | 50-200 GB/month | Traces, structured logs, metrics |

**Key characteristics:** OpenObserve (Rust-based) is dramatically lighter than Elasticsearch-based alternatives. It achieves ~140x lower storage cost through columnar compression. For a self-hosted deployment, it provides logs, metrics, and traces in a single binary. Disk usage depends entirely on retention period and log verbosity. A 7-day retention with moderate logging keeps storage under 10 GB.

---

## 3. Scenario 1: Edge / Raspberry Pi (Go API Only)

### Architecture

Only the Go API server and Traefik run on the edge device. All heavy services (Python AI Worker, PostgreSQL, Valkey, Keycloak, Strapi, SeaweedFS) run on a remote server. The Go API connects to the remote services over the network (VPN, Tailscale, or public endpoints with mTLS).

```
[Edge Device]                    [Remote Server]
+------------------+             +---------------------------+
| Traefik          |  ---------> | PostgreSQL + pgvector     |
| Go API Server    |  (network)  | Python AI Worker          |
+------------------+             | Valkey                    |
                                 | Keycloak                  |
                                 | Strapi                    |
                                 | SeaweedFS                 |
                                 +---------------------------+
```

### Use Cases for Edge Deployment

- **On-premise API gateway** for enterprises that want data to transit through their own network before reaching the cloud backend.
- **Low-latency API access** in locations with intermittent connectivity (caches responses in Traefik/local layer).
- **Branch office deployment** where a Raspberry Pi serves as a local access point to a centralized Raven instance.
- **Developer/testing** deployment for API-level development and integration testing.

### Hardware Requirements

#### Minimum: Raspberry Pi 4 (2 GB RAM)

| Resource | Requirement | Notes |
|----------|-------------|-------|
| CPU | 4 cores ARM Cortex-A72 (BCM2711) | Pi 4 provides this; adequate for Go API |
| RAM | 2 GB total | Go API (~40 MB) + Traefik (~50 MB) + OS (~300 MB) = ~400 MB used; ~1.6 GB free for OS buffers |
| Disk | 16 GB microSD (Class 10 / A1+) | OS (~4 GB) + Go binary (~20 MB) + Traefik (~50 MB) + logs |
| Network | 1 Gbps Ethernet (Pi 4 native) | Latency to remote server is the bottleneck, not bandwidth |
| Power | 5V/3A USB-C | Standard Pi 4 power supply |

#### Recommended: Raspberry Pi 4 (4 GB RAM) or Small ARM64 VPS

| Resource | Requirement | Notes |
|----------|-------------|-------|
| CPU | 4 cores ARM64 | Pi 4 (4 GB) or equivalent (e.g., Oracle Cloud ARM Ampere free tier) |
| RAM | 4 GB | Leaves ~3.5 GB for OS caching and headroom during traffic spikes |
| Disk | 32 GB microSD / NVMe (USB-attached for Pi, SSD for VPS) | Room for logs, local response caching |
| Network | 1 Gbps | -- |

#### Estimated Monthly Cost

| Option | Cost | Notes |
|--------|------|-------|
| Raspberry Pi 4 (2 GB) | $0/month (hardware: ~$35 one-time) | Power cost: ~$0.50-1/month |
| Raspberry Pi 4 (4 GB) | $0/month (hardware: ~$55 one-time) | Power cost: ~$0.50-1/month |
| Oracle Cloud ARM VPS (free tier) | $0/month | 4 OCPU, 24 GB RAM Ampere A1 (always free) |
| Hetzner Cloud CAX11 (ARM64) | ~$4/month | 2 vCPU, 4 GB RAM, 40 GB NVMe |
| AWS Graviton t4g.micro | ~$6/month | 2 vCPU, 1 GB RAM (free tier eligible first 12 months) |

### Edge-Specific Considerations

- **Cross-compilation:** Build the Go API binary for `linux/arm64` with `GOOS=linux GOARCH=arm64 go build`. The Traefik official Docker image supports `linux/arm64`.
- **Latency:** Every request from the Go API to PostgreSQL/Valkey on the remote server incurs network round-trip time. With a well-connected VPS in the same region, expect 5-20 ms added latency per database query. With cross-continent connections, 100-200 ms.
- **Connection pooling:** The Go API should use connection pooling (pgxpool for PostgreSQL, go-redis pool for Valkey) to avoid repeated connection setup overhead.
- **Offline resilience:** Consider implementing request queuing or local SQLite fallback for brief network outages if required.
- **Security:** Use Tailscale, WireGuard, or mTLS for the edge-to-remote connection. Never expose PostgreSQL/Valkey ports to the public internet.

---

## 4. Scenario 2: Self-Hosted Single Server

### Architecture

All services run on a single machine via Docker Compose. This is the recommended deployment for small teams, self-hosted enterprise customers, or development/staging environments.

```
+-----------------------------------------------------------+
|                    Single Server                           |
|                                                           |
|  +----------+  +----------+  +----------+  +----------+  |
|  | Traefik  |  | Go API   |  | Python   |  | Keycloak |  |
|  | (proxy)  |  | Server   |  | AI Worker|  | (auth)   |  |
|  +----------+  +----------+  +----------+  +----------+  |
|                                                           |
|  +----------+  +----------+  +----------+  +----------+  |
|  | Postgres |  | Valkey   |  | Strapi   |  | SeaweedFS|  |
|  | +pgvector|  | (cache)  |  | (CMS)    |  | (storage)|  |
|  +----------+  +----------+  +----------+  +----------+  |
|                                                           |
|  +----------+  +----------+                               |
|  | LiveKit  |  | OpenObs  |  (Phase 2 / Optional)        |
|  | (WebRTC) |  | (logs)   |                               |
|  +----------+  +----------+                               |
+-----------------------------------------------------------+
```

### RAM Budget (Phase 1 Core Services)

| Service | Idle/Baseline | Light Load | Heavy Load | Docker Overhead |
|---------|--------------|------------|------------|-----------------|
| Go API Server | 15 MB | 50 MB | 150 MB | +10 MB |
| Python AI Worker | 120 MB | 400 MB | 2 GB (peak) | +10 MB |
| PostgreSQL 18 + pgvector | 300 MB | 800 MB | 2 GB | +10 MB |
| Valkey | 10 MB | 50 MB | 300 MB | +5 MB |
| Keycloak | 600 MB | 700 MB | 1 GB | +10 MB |
| Strapi | 150 MB | 300 MB | 400 MB | +10 MB |
| SeaweedFS (master+volume+filer) | 150 MB | 200 MB | 300 MB | +10 MB |
| Traefik | 40 MB | 60 MB | 120 MB | +5 MB |
| **Phase 1 Total** | **~1.4 GB** | **~2.6 GB** | **~6.3 GB** | **+70 MB** |

### RAM Budget (Phase 2+ Additions)

| Service | Idle/Baseline | Light Load | Heavy Load |
|---------|--------------|------------|------------|
| LiveKit Server | 40 MB | 200 MB | 500 MB |
| OpenObserve (optional) | 150 MB | 400 MB | 1 GB |
| **Phase 2 Additional** | **+190 MB** | **+600 MB** | **+1.5 GB** |

### Full Stack RAM Summary

| Scenario | RAM Used | Notes |
|----------|---------|-------|
| All services idle | ~1.6 GB | Baseline after cold start and JVM warmup |
| All services, light load | ~3.2 GB | Normal day-to-day usage, few concurrent users |
| All services, moderate load | ~5 GB | Active document processing, 20-50 users |
| All services, heavy load (peak) | ~7.8 GB | Bulk ingestion + Crawl4AI + concurrent queries |
| OS + Docker Engine overhead | 500 MB-1 GB | Linux kernel, systemd, Docker daemon |

### MINIMUM Specifications (Barely Runs)

For a functional deployment that can handle 1-5 concurrent users, small document uploads, and infrequent scraping jobs. Expect slow performance during document processing.

| Resource | Specification | Notes |
|----------|--------------|-------|
| CPU | 4 cores (x86_64 or ARM64) | Keycloak JVM + Python worker contend for CPU during spikes |
| RAM | 4 GB | Extremely tight; requires aggressive JVM limits (`-Xmx384m`), no OpenObserve, no LiveKit; Python worker will OOM on large PDFs |
| Disk | 40 GB SSD | 10 GB OS, 5 GB Docker images, 5 GB PostgreSQL, 10 GB SeaweedFS, 10 GB headroom |
| Network | 10 Mbps | Sufficient for API traffic; insufficient for concurrent voice sessions |

**4 GB RAM allocation (minimum, Phase 1 only):**
```
OS + Docker Engine:          500 MB
PostgreSQL (shared_buffers=128MB): 350 MB
Keycloak (-Xmx384m):        500 MB
Python AI Worker:            400 MB  (will OOM on large docs)
Strapi:                      200 MB
Go API:                       50 MB
SeaweedFS:                   150 MB
Valkey:                       50 MB
Traefik:                      50 MB
Remaining headroom:          750 MB  (consumed by OS cache / spikes)
---
Total:                     ~3,250 MB allocated + 750 MB buffer
```

**Warning:** 4 GB is functionally the floor. Below this, Keycloak's JVM alone consumes a disproportionate share, and the Python worker cannot process documents larger than ~20 pages without risking OOM. Web scraping with Crawl4AI (headless Chromium) will likely fail due to insufficient memory for the browser process.

### RECOMMENDED Specifications

For a smooth self-hosted deployment supporting 10-50 concurrent users, regular document ingestion, web scraping, and room for Phase 2 services.

| Resource | Specification | Notes |
|----------|--------------|-------|
| CPU | 8 cores (x86_64) | 4 minimum + headroom for PostgreSQL parallel workers, Python processing, JVM |
| RAM | 16 GB | Comfortable for all Phase 1 services + LiveKit + OpenObserve with headroom |
| Disk | 200 GB NVMe SSD | PostgreSQL data (20-50 GB), SeaweedFS (50-100 GB), Docker images (10 GB), logs (10 GB), headroom |
| Network | 100 Mbps (1 Gbps preferred) | Required for LiveKit voice sessions at scale |

**16 GB RAM allocation (recommended, all services):**
```
OS + Docker Engine:              1 GB
PostgreSQL (shared_buffers=1GB): 2.5 GB  (comfortable for 1M embeddings)
Keycloak (-Xmx768m):            1 GB
Python AI Worker:                2 GB  (handles large PDFs + Crawl4AI)
Strapi:                        400 MB
Go API:                        150 MB
SeaweedFS:                     250 MB
Valkey:                        300 MB
Traefik:                       100 MB
LiveKit:                       200 MB  (Phase 2)
OpenObserve:                   500 MB  (optional)
Remaining headroom:            7.6 GB  (OS page cache, peak handling)
---
Total:                        ~8.4 GB allocated + 7.6 GB buffer
```

### CPU Allocation Guide

| Service | Reserved Cores | Peak Cores | Notes |
|---------|---------------|------------|-------|
| PostgreSQL | 2 | 4 | Parallel query workers, HNSW index builds |
| Python AI Worker | 1 | 4 | Crawl4AI + embedding processing |
| Keycloak | 0.5 | 1 | JVM GC + token generation |
| Go API | 0.25 | 1 | Goroutines are efficient |
| Strapi | 0.25 | 0.5 | Low-traffic admin operations |
| SeaweedFS | 0.25 | 0.5 | File I/O bound, not CPU |
| LiveKit | 0.25 | 2 | SFU packet forwarding |
| Traefik | 0.1 | 0.5 | TLS termination |
| OpenObserve | 0.25 | 1 | Log indexing |

### Disk Breakdown

| Component | Size Range | Type | Notes |
|-----------|-----------|------|-------|
| OS + Docker Engine | 5-10 GB | SSD | Base Ubuntu/Debian + Docker |
| Docker images (all services) | 8-12 GB | SSD | PostgreSQL (~400 MB), Keycloak (~400 MB), Strapi (~1 GB), Python worker (~1.5 GB with Chromium), others (~100-300 MB each) |
| PostgreSQL data (100K embeddings) | 2-5 GB | NVMe preferred | Relational data + vectors + HNSW index + BM25 index |
| PostgreSQL data (1M embeddings) | 15-25 GB | NVMe required | HNSW index performance degrades significantly on spinning disk |
| PostgreSQL WAL + temp | 2-5 GB | NVMe | Write-ahead log segments, temp sort files |
| SeaweedFS volumes | 10-500 GB | SSD/HDD | Document originals; grows linearly with uploads |
| Valkey persistence (RDB/AOF) | 100 MB-1 GB | SSD | Job queue state, cache snapshots |
| OpenObserve data | 5-50 GB | SSD | Compressed logs; depends on retention and verbosity |
| Application logs | 1-5 GB | Any | Docker container logs; configure log rotation |
| **Total (minimal, Phase 1)** | **40-60 GB** | **SSD** | -- |
| **Total (recommended, all services)** | **100-300 GB** | **NVMe** | Room for growth |

### Recommended Self-Hosted Hardware / VPS Options

| Provider | Instance | Specs | Monthly Cost | Notes |
|----------|----------|-------|-------------|-------|
| Hetzner Dedicated (AX42) | AX42 | 8-core Ryzen 5, 64 GB RAM, 2x512 GB NVMe | ~$50/month | Best value; Hetzner auction servers even cheaper |
| Hetzner Cloud (CPX41) | CPX41 | 8 vCPU, 16 GB RAM, 240 GB NVMe | ~$28/month | Minimum recommended cloud VPS |
| Hetzner Cloud (CCX33) | CCX33 | 8 dedicated vCPU, 32 GB RAM, 240 GB NVMe | ~$55/month | Dedicated CPU for consistent performance |
| DigitalOcean | Premium 8 vCPU | 8 vCPU, 16 GB RAM, 320 GB NVMe | ~$96/month | Higher cost, good UX and managed DB options |
| AWS EC2 (t3.xlarge) | t3.xlarge | 4 vCPU, 16 GB RAM | ~$120/month | Plus EBS storage costs; burstable CPU |
| AWS EC2 (m6i.xlarge) | m6i.xlarge | 4 vCPU, 16 GB RAM | ~$140/month | Fixed-performance; better for sustained workloads |
| AWS EC2 (m6i.2xlarge) | m6i.2xlarge | 8 vCPU, 32 GB RAM | ~$280/month | Comfortable for all services |

---

## 5. Scenario 3: Production Cloud (Multi-Tenant SaaS)

### Architecture Overview

Raven's own hosted SaaS offering. Services are decomposed for independent scaling. Managed services replace self-hosted infrastructure where available.

```
                        +-----------+
                        | CloudFront|
                        | / CDN     |
                        +-----+-----+
                              |
                        +-----+-----+
                        |  ALB/NLB  |
                        +-----+-----+
                              |
              +---------------+---------------+
              |               |               |
        +-----+-----+  +-----+-----+  +------+-----+
        | Go API    |  | Go API    |  | Go API     |
        | (ECS/K8s) |  | (replica) |  | (replica)  |
        +-----------+  +-----------+  +------------+
              |
    +---------+---------+---------+---------+
    |         |         |         |         |
+---+---+ +--+--+ +----+--+ +---+---+ +---+---+
|Postgres| |Valkey| |Keycloak| |Strapi | |SeaweedFS|
| (RDS)  | |(Elst)| | (ECS) | | (ECS) | | (ECS)  |
+--------+ +-----+ +-------+ +-------+ +---------+

              +------------------+
              | Python AI Worker |  (Auto-scaling ECS/K8s)
              | Worker pool      |
              +------------------+

              +------------------+
              | LiveKit Server   |  (Phase 2, dedicated instances)
              +------------------+
```

### Scaling Strategy

| Service | Scaling Model | Trigger |
|---------|--------------|---------|
| Go API | Horizontal (auto-scale replicas) | CPU > 60%, request latency > 200ms |
| Python AI Worker | Horizontal (queue-based auto-scale) | Queue depth > 10 jobs, processing time > 30s |
| PostgreSQL | Vertical (larger instance) + read replicas | Connection count, query latency, storage >80% |
| Valkey | Vertical (ElastiCache) or cluster mode | Memory > 70%, eviction rate > 0 |
| Keycloak | Horizontal (2-3 replicas behind LB) | Login latency > 500ms |
| Strapi | Horizontal (2 replicas for HA) | Rarely needs scaling; low traffic |
| SeaweedFS | Horizontal (add volume servers) | Storage > 80%, throughput |
| LiveKit | Horizontal (add SFU nodes) | Room count, bandwidth saturation |

### Tier 1: 10 Tenants, 50 Concurrent Users

**Scale assumptions:**
- 10 organizations, each with ~1,000 documents, ~500K chunks/embeddings total
- 50 concurrent users: ~10-20 chatbot queries/minute, 2-5 voice sessions
- Document ingestion: ~50-100 documents/day across all tenants

| Service | Instance Type | Count | RAM | CPU | Monthly Cost |
|---------|--------------|-------|-----|-----|-------------|
| Go API (ECS Fargate) | 0.5 vCPU, 1 GB | 2 | 2 GB | 1 vCPU | ~$30 |
| Python AI Worker (ECS Fargate) | 2 vCPU, 4 GB | 2 | 8 GB | 4 vCPU | ~$120 |
| PostgreSQL (RDS db.r6g.large) | 2 vCPU, 16 GB | 1 | 16 GB | 2 vCPU | ~$220 |
| Valkey (ElastiCache cache.t4g.small) | 2 vCPU, 1.55 GB | 1 | 1.55 GB | 2 vCPU | ~$25 |
| Keycloak (ECS Fargate) | 1 vCPU, 2 GB | 1 | 2 GB | 1 vCPU | ~$45 |
| Strapi (ECS Fargate) | 0.5 vCPU, 1 GB | 1 | 1 GB | 0.5 vCPU | ~$15 |
| SeaweedFS (EC2 t3.small + EBS) | 2 vCPU, 2 GB | 1 | 2 GB | 2 vCPU | ~$25 |
| Traefik / ALB | -- | 1 | -- | -- | ~$25 |
| LiveKit (EC2 c6i.large, Phase 2) | 2 vCPU, 4 GB | 1 | 4 GB | 2 vCPU | ~$65 |
| EBS Storage (500 GB gp3) | -- | -- | -- | -- | ~$40 |
| Data Transfer (500 GB/month) | -- | -- | -- | -- | ~$45 |
| OpenObserve (ECS Fargate) | 1 vCPU, 2 GB | 1 (optional) | 2 GB | 1 vCPU | ~$45 |
| **Total (Phase 1)** | | | **~33 GB** | **~13 vCPU** | **~$590/month** |
| **Total (Phase 1 + Phase 2 + OpenObserve)** | | | **~39 GB** | **~16 vCPU** | **~$700/month** |

**Database sizing (500K embeddings, 1536d float32):**
- Raw vector data: ~3 GB
- HNSW index: ~3.5-4 GB
- Relational data + BM25 index: ~2-3 GB
- Total PostgreSQL storage: ~10-15 GB
- RDS db.r6g.large (16 GB RAM) comfortably holds HNSW index in memory

### Tier 2: 100 Tenants, 500 Concurrent Users

**Scale assumptions:**
- 100 organizations, ~100K documents total, ~5M chunks/embeddings
- 500 concurrent users: ~100-200 queries/minute, 20-50 voice sessions
- Document ingestion: ~500-1000 documents/day

| Service | Instance Type | Count | RAM | CPU | Monthly Cost |
|---------|--------------|-------|-----|-----|-------------|
| Go API (ECS Fargate) | 1 vCPU, 2 GB | 3 | 6 GB | 3 vCPU | ~$130 |
| Python AI Worker (ECS Fargate) | 4 vCPU, 8 GB | 4 | 32 GB | 16 vCPU | ~$480 |
| PostgreSQL (RDS db.r6g.xlarge) | 4 vCPU, 32 GB | 1 primary + 1 read replica | 64 GB | 8 vCPU | ~$880 |
| Valkey (ElastiCache cache.r6g.large) | 2 vCPU, 13 GB | 1 | 13 GB | 2 vCPU | ~$195 |
| Keycloak (ECS Fargate) | 2 vCPU, 4 GB | 2 | 8 GB | 4 vCPU | ~$180 |
| Strapi (ECS Fargate) | 1 vCPU, 2 GB | 2 | 4 GB | 2 vCPU | ~$90 |
| SeaweedFS (EC2 t3.medium + EBS) | 2 vCPU, 4 GB | 2 | 8 GB | 4 vCPU | ~$70 |
| ALB | -- | 1 | -- | -- | ~$30 |
| LiveKit (EC2 c6i.xlarge, Phase 2) | 4 vCPU, 8 GB | 2 | 16 GB | 8 vCPU | ~$250 |
| EBS/S3 Storage (2 TB) | -- | -- | -- | -- | ~$160 |
| Data Transfer (2 TB/month) | -- | -- | -- | -- | ~$180 |
| OpenObserve (ECS Fargate) | 2 vCPU, 4 GB | 2 (optional) | 8 GB | 4 vCPU | ~$180 |
| **Total (Phase 1)** | | | **~135 GB** | **~39 vCPU** | **~$2,395/month** |
| **Total (all services)** | | | **~159 GB** | **~51 vCPU** | **~$2,825/month** |

**Database sizing (5M embeddings, 1536d float32):**
- Raw vector data: ~30 GB
- HNSW index: ~35-40 GB
- Relational data + BM25 index: ~10-15 GB
- Total PostgreSQL storage: ~80-100 GB
- RDS db.r6g.xlarge (32 GB RAM) cannot hold the full HNSW index in memory; consider:
  - Upgrading to db.r6g.2xlarge (64 GB RAM) for $1,760/month, OR
  - Using halfvec (float16) quantization to halve index size, OR
  - Partitioning embeddings by tenant/workspace with separate HNSW indexes

### Tier 3: 1000 Tenants, 5000 Concurrent Users

**Scale assumptions:**
- 1000 organizations, ~1M documents total, ~50M chunks/embeddings
- 5000 concurrent users: ~1000-2000 queries/minute, 200-500 voice sessions
- Document ingestion: ~5000-10000 documents/day

| Service | Instance Type | Count | RAM | CPU | Monthly Cost |
|---------|--------------|-------|-----|-----|-------------|
| Go API (ECS Fargate) | 2 vCPU, 4 GB | 8 | 32 GB | 16 vCPU | ~$580 |
| Python AI Worker (ECS Fargate) | 4 vCPU, 8 GB | 12 | 96 GB | 48 vCPU | ~$1,440 |
| PostgreSQL (RDS db.r6g.4xlarge) | 16 vCPU, 128 GB | 1 primary + 2 read replicas | 384 GB | 48 vCPU | ~$5,280 |
| Valkey (ElastiCache r6g.xlarge cluster) | 4 vCPU, 26 GB | 3-node cluster | 78 GB | 12 vCPU | ~$1,170 |
| Keycloak (ECS Fargate) | 2 vCPU, 4 GB | 3 | 12 GB | 6 vCPU | ~$270 |
| Strapi (ECS Fargate) | 1 vCPU, 2 GB | 2 | 4 GB | 2 vCPU | ~$90 |
| SeaweedFS (EC2 c6i.xlarge + EBS) | 4 vCPU, 8 GB | 4 | 32 GB | 16 vCPU | ~$260 |
| ALB + CloudFront | -- | -- | -- | -- | ~$150 |
| LiveKit (EC2 c6i.2xlarge, Phase 2) | 8 vCPU, 16 GB | 4 | 64 GB | 32 vCPU | ~$1,000 |
| EBS/S3 Storage (10 TB) | -- | -- | -- | -- | ~$250 |
| Data Transfer (10 TB/month) | -- | -- | -- | -- | ~$900 |
| OpenObserve (ECS Fargate) | 4 vCPU, 8 GB | 3 (optional) | 24 GB | 12 vCPU | ~$540 |
| **Total (Phase 1)** | | | **~638 GB** | **~148 vCPU** | **~$10,120/month** |
| **Total (all services)** | | | **~726 GB** | **~192 vCPU** | **~$11,930/month** |

**Database sizing (50M embeddings, 1536d float32):**
- Raw vector data: ~300 GB
- HNSW index: ~350-400 GB
- Relational data + BM25 index: ~50-80 GB
- Total PostgreSQL storage: ~700 GB-1 TB
- **Critical:** 50M embeddings at 1536d float32 will not fit in memory on any single instance. Required strategies:
  - **Mandatory: halfvec quantization** (float16) reduces storage to ~150 GB vectors + ~175-200 GB index
  - **Mandatory: Table partitioning** by org_id or workspace_id, with per-partition HNSW indexes
  - **Consider: Scalar quantization** (int8) further reduces to ~75 GB vectors
  - **Consider: Reduced dimensions** via OpenAI's `dimensions` parameter (768d or 512d instead of 1536d)
  - **Consider: Citus extension** for horizontal sharding across multiple PostgreSQL nodes
  - **Alternative: External vector DB** (Pinecone/Weaviate) if PostgreSQL vertical limits are exceeded

### Production Architecture Considerations

**Multi-region:**
- At 1000 tenants, deploy in 2+ AWS regions (e.g., us-east-1 + eu-west-1) for latency and compliance (GDPR).
- PostgreSQL: Use RDS cross-region read replicas or separate primary instances per region.
- SeaweedFS: S3 cross-region replication or CloudFront for static assets.

**Database connection management:**
- At 5000 concurrent users, PostgreSQL connection limits become critical.
- Use PgBouncer (connection pooler) in front of RDS.
- Target: 200-300 max connections to PostgreSQL, with PgBouncer managing 2000+ application connections.

**Worker auto-scaling:**
- Python AI Workers should auto-scale based on queue depth (Valkey sorted set length).
- Scale-to-zero during off-hours to reduce costs.
- Consider spot/preemptible instances for worker fleet (60-70% cost savings).

**Cost optimization opportunities:**
- Reserved Instances (1-year or 3-year) for RDS and base compute: 30-50% savings.
- Spot Instances for Python AI Workers: up to 70% savings.
- S3 Intelligent-Tiering for SeaweedFS-backed storage: automatic tier optimization.
- Graviton (ARM64) instances for Go API and SeaweedFS: 20% cheaper, comparable or better performance.

### Monthly Cost Summary by Tier

| Tier | Tenants | Concurrent Users | Embeddings | Phase 1 Cost | Full Stack Cost |
|------|---------|-----------------|------------|-------------|----------------|
| Tier 1 | 10 | 50 | 500K | ~$590/month | ~$700/month |
| Tier 2 | 100 | 500 | 5M | ~$2,395/month | ~$2,825/month |
| Tier 3 | 1000 | 5000 | 50M | ~$10,120/month | ~$11,930/month |
| Tier 3 (optimized*) | 1000 | 5000 | 50M | ~$6,500/month | ~$8,000/month |

*Optimized: Reserved Instances, Spot Workers, Graviton, halfvec quantization (smaller RDS instance), S3 instead of SeaweedFS at this scale.

---

## 6. Deep Dives: Critical Resource Consumers

### 6.1 PostgreSQL HNSW Index Memory Analysis

HNSW (Hierarchical Navigable Small World) indexes in pgvector are graph structures stored on disk but perform best when cached in memory. When the index exceeds available `shared_buffers` + OS page cache, query latency increases dramatically due to random disk I/O.

**HNSW index size formula (approximate):**
```
Index size = num_vectors * (dims * bytes_per_dim + M * 2 * 8 + overhead)

Where:
- dims = embedding dimensions (1536 for text-embedding-3-small)
- bytes_per_dim = 4 (float32), 2 (float16/halfvec), 1 (int8)
- M = max connections per layer (default 16, higher = better recall but larger index)
- 8 = bytes per edge (pointer)
- overhead = ~100 bytes per vector (metadata, level assignment)
```

**Concrete index sizes (M=16, ef_construction=64):**

| Vectors | Dimensions | Type | Raw Vectors | HNSW Index | Total |
|---------|-----------|------|-------------|------------|-------|
| 100K | 1536 | float32 | 600 MB | 800 MB | 1.4 GB |
| 100K | 1536 | halfvec | 300 MB | 500 MB | 800 MB |
| 500K | 1536 | float32 | 3 GB | 4 GB | 7 GB |
| 500K | 1536 | halfvec | 1.5 GB | 2.5 GB | 4 GB |
| 1M | 1536 | float32 | 6 GB | 8 GB | 14 GB |
| 1M | 1536 | halfvec | 3 GB | 4.5 GB | 7.5 GB |
| 5M | 1536 | float32 | 30 GB | 40 GB | 70 GB |
| 5M | 1536 | halfvec | 15 GB | 22 GB | 37 GB |
| 10M | 1536 | float32 | 60 GB | 80 GB | 140 GB |
| 10M | 1536 | halfvec | 30 GB | 42 GB | 72 GB |
| 50M | 1536 | float32 | 300 GB | 400 GB | 700 GB |
| 50M | 1536 | halfvec | 150 GB | 200 GB | 350 GB |
| 50M | 768 | halfvec | 37.5 GB | 55 GB | 92.5 GB |

**Impact on query latency:**
- Index fully in memory: 1-5 ms per query (p99)
- Index partially in memory (>50% cached): 5-20 ms per query
- Index mostly on disk (<20% cached): 50-200 ms per query (unacceptable for chatbot)

**Recommendation:** Keep the active HNSW index (the portion being queried) fully in `shared_buffers` + OS page cache. For multi-tenant deployments, per-tenant partitioned HNSW indexes allow the "hot" tenants' indexes to stay in cache while "cold" tenants' indexes can be paged out without affecting overall performance.

### 6.2 Python AI Worker Peak Memory During Document Processing

The Python worker's memory usage is highly variable. Here is a breakdown of peak scenarios:

**Scenario A: Large PDF processing (100-page technical document)**
```
Python runtime baseline:           80 MB
Imported libraries (numpy, etc.):  50 MB
LiteParse subprocess:             100-300 MB  (parsing, bounding box extraction)
Document in memory (raw bytes):    10-50 MB   (depends on PDF size)
Parsed pages in memory:           100-400 MB  (text + bounding boxes for all pages)
Chunking output:                   20-50 MB   (chunk text + metadata)
Embedding batch (API response):    10-30 MB   (1536d * batch_size * 4 bytes)
---
Total peak:                       370 MB - 1.16 GB
```

**Scenario B: Crawl4AI web scraping session**
```
Python runtime baseline:           80 MB
Imported libraries:                50 MB
Playwright controller:             30 MB
Headless Chromium process:        150-300 MB  (per tab, depends on page complexity)
  - Simple text page:             150 MB
  - JS-heavy SPA:                 250-400 MB
  - Media-rich page:              300-500 MB
Page content in memory:            10-30 MB   (markdown output)
---
Total peak (1 tab):               320 MB - 960 MB
Total peak (3 concurrent tabs):   620 MB - 1.96 GB
```

**Scenario C: Simultaneous PDF + scraping (worst case)**
```
PDF processing:                    500 MB - 1 GB
Crawl4AI (2 tabs):                400 MB - 1.2 GB
Embedding API calls:               30 MB
---
Total peak:                       930 MB - 2.23 GB
```

**Mitigation strategies:**
- Limit concurrent Crawl4AI tabs (max 2-3 per worker instance).
- Stream large PDF pages through LiteParse rather than loading the entire document.
- Set Python worker container memory limit to 2-4 GB with OOM restart policy.
- Use horizontal scaling (multiple worker instances) instead of one large worker.

### 6.3 LiveKit Bandwidth During Active Voice Sessions

LiveKit operates as an SFU, forwarding media without transcoding. For Raven's voice agent use case, each session consists of:
- User's audio stream (user -> LiveKit): Opus codec, 32-64 kbps
- AI agent's audio stream (LiveKit -> user): Opus codec, 32-64 kbps
- Signaling overhead: negligible (~1-5 kbps)

**Bandwidth calculations:**

| Concurrent Sessions | Per-Session (both directions) | Total Bandwidth | Monthly Transfer (sustained) |
|---------------------|------------------------------|-----------------|------------------------------|
| 1 | 64-128 kbps | 128 kbps | ~40 GB |
| 10 | 64-128 kbps | 1.28 Mbps | ~400 GB |
| 50 | 64-128 kbps | 6.4 Mbps | ~2 TB |
| 100 | 64-128 kbps | 12.8 Mbps | ~4 TB |
| 500 | 64-128 kbps | 64 Mbps | ~20 TB |

**Notes:**
- These are sustained bandwidth numbers. Actual usage depends on session duration (average voice session: 2-10 minutes).
- At 500 concurrent sessions, a single LiveKit instance on a 1 Gbps NIC is well within bandwidth limits. CPU (packet forwarding) becomes the bottleneck before bandwidth.
- TURN relay (for users behind restrictive NATs) doubles bandwidth consumption at the server. Estimate 20-30% of users require TURN relay.
- Monthly transfer costs at AWS: $0.09/GB outbound. 2 TB/month = ~$180 in data transfer alone.

### 6.4 Crawl4AI with Playwright: Headless Chromium Resource Profile

Crawl4AI uses Playwright to launch a headless Chromium browser. This is the single highest per-process memory consumer in the Raven stack.

**Chromium process tree (per browser launch):**
```
chromium (main browser process):     50-80 MB
  ├── GPU process:                   30-50 MB  (even headless allocates this)
  ├── Network service:               20-30 MB
  ├── Renderer (per tab):           100-300 MB (highly variable)
  └── Utility processes:             10-30 MB
---
Total per browser instance:         210-490 MB (1 tab)
Each additional tab adds:           100-300 MB
```

**Practical measurements for web scraping workloads:**
- Simple blog/docs page: 150-200 MB total per tab
- JavaScript SPA (React/Vue rendered): 250-400 MB per tab
- Media-heavy page (images, video embeds): 300-500 MB per tab
- Page with memory leaks (common in SPAs): can grow to 500+ MB over time

**Mitigation:**
- Reuse browser instances but create fresh contexts per page to limit memory leaks.
- Set `--max-old-space-size` for the Chromium V8 heap.
- Kill and restart the browser process after every N pages (e.g., 50) to reclaim leaked memory.
- Never run more than 3-5 concurrent tabs per worker instance.
- On a 4 GB worker, limit to 2 concurrent Crawl4AI tabs maximum.

---

## 7. Summary Comparison Table

| Dimension | Scenario 1: Edge/Pi | Scenario 2: Self-Hosted | Scenario 3a: 10 Tenants | Scenario 3b: 100 Tenants | Scenario 3c: 1000 Tenants |
|-----------|--------------------|-----------------------|------------------------|-------------------------|--------------------------|
| **Services on this machine** | Go API + Traefik | All services | All services (distributed) | All services (distributed) | All services (distributed) |
| **CPU (cores)** | 4 (ARM64) | 4 min / 8 rec | ~13 vCPU | ~39 vCPU | ~148 vCPU |
| **RAM (GB)** | 2 min / 4 rec | 4 min / 16 rec | ~33 GB | ~135 GB | ~638 GB |
| **Disk (GB)** | 16-32 GB (SD/SSD) | 40 min / 200 rec NVMe | ~500 GB SSD | ~2 TB SSD | ~10 TB SSD |
| **Network** | 1 Gbps | 100 Mbps-1 Gbps | 1 Gbps | 1-5 Gbps | 5-10 Gbps |
| **Monthly cost** | $0-6 | $28-55 (VPS) | ~$700 | ~$2,825 | ~$11,930 |
| **Optimized monthly cost** | -- | -- | ~$500 | ~$1,800 | ~$8,000 |
| **Embedding capacity** | N/A (remote) | 100K-1M | 500K | 5M | 50M |
| **Concurrent users** | API passthrough | 5-50 | 50 | 500 | 5000 |

### Key Takeaways

1. **The JVM tax is real.** Keycloak's 512-768 MB idle footprint is the single largest fixed cost on constrained deployments. On a 4 GB server, Keycloak alone consumes 15-20% of total RAM. There is no way around this without replacing Keycloak with a non-JVM auth server.

2. **PostgreSQL with HNSW is the scaling bottleneck.** Vector indexes must fit in memory for acceptable latency. At 5M+ embeddings (float32, 1536d), the index exceeds 40 GB. Use halfvec quantization, reduced dimensions, or table partitioning to manage this. This is the primary driver of database cost at scale.

3. **Crawl4AI/Chromium is the memory spike risk.** A single headless Chromium tab can consume 300+ MB. If a user triggers a bulk web scrape (crawl 100 pages from a site), the worker must process sequentially or with very limited concurrency (2-3 tabs max). Set hard container memory limits and OOM restart policies.

4. **Go services are negligible.** The Go API, Traefik, SeaweedFS, and LiveKit together consume less RAM idle (~120 MB total) than Keycloak alone. Go's compiled binary model makes it ideal for edge and resource-constrained deployments.

5. **LiveKit scales on bandwidth, not compute.** Voice-only sessions are ~64-128 kbps each. Even 100 concurrent voice sessions need only ~13 Mbps. The cost at scale is data transfer pricing, not instance sizing.

6. **The Python worker should scale horizontally.** Rather than giving one worker 8 GB RAM, run multiple 2-4 GB workers. This provides better isolation (one OOM crash does not affect other jobs) and allows queue-based auto-scaling.

7. **Self-hosted on Hetzner is 3-5x cheaper than AWS** for equivalent specs. For budget-conscious deployments, Hetzner dedicated servers ($50-80/month for 64 GB RAM, 8 cores, NVMe) outperform AWS instances costing $300+/month. The trade-off is less managed infrastructure and manual operational burden.

8. **At 1000 tenants, consider splitting PostgreSQL.** A single PostgreSQL instance handling 50M embeddings, BM25 indexes, relational data, and Keycloak schemas approaches vertical limits. Consider: dedicated PostgreSQL for vectors (pgvector) vs. dedicated PostgreSQL for application data + BM25, or migration to Citus for distributed queries.
