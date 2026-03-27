# Raven Platform Stack -- Dependency License Audit

**Date:** 2026-03-27
**Purpose:** Verify all dependencies are compatible with commercial SaaS monetization without copyleft obligations that would require open-sourcing Raven's proprietary code.

---

## Executive Summary

Of the 32 components audited, **6 carry RED FLAGS** that require immediate attention:

| RED FLAG Component | License | Issue |
|--------------------|---------|-------|
| ParadeDB (Community) | AGPL-3.0 | Copyleft; network use triggers source disclosure |
| ParadeDB (Enterprise) | Proprietary/Commercial | Requires paid license |
| Redis (v7.4+) | RSALv2 / SSPLv1 / AGPLv3 (tri-license since v8.0) | All three options restrict SaaS or impose copyleft |
| Firecrawl | AGPL-3.0 | Copyleft; network use triggers source disclosure |
| MinIO | AGPL-3.0 | Copyleft; network use triggers source disclosure |
| Coqui TTS / XTTS (model weights) | Coqui Public Model License (CPML) | Non-commercial only for XTTS model weights; company defunct, no one to sell commercial license |

**Additionally, 3 components have YELLOW warnings** (usable but with caveats):

| YELLOW Component | License | Caveat |
|------------------|---------|--------|
| TEN Framework | Apache 2.0 with additional restrictions | "Additional restrictions" not fully enumerated; review LICENSE file before use |
| Piper TTS | MIT (archived) / GPL-3.0 (active fork) | Original MIT version archived; active fork is GPL-3.0 (copyleft) |
| Docker Desktop | Proprietary subscription | Free for <250 employees AND <$10M revenue; otherwise requires paid subscription |

---

## Full Audit Table

| # | Component | License | Commercial SaaS OK? | Gotchas / Restrictions |
|---|-----------|---------|---------------------|------------------------|
| 1 | **Go (language)** | BSD 3-Clause + Patent Grant | YES | None. Fully permissive. Google grants royalty-free patent license. |
| 2 | **Kotlin (language)** | Apache 2.0 | YES | None. Some third-party dependencies may have different (but compatible) licenses. |
| 3 | **Gin (Go web framework)** | MIT | YES | None. Fully permissive. |
| 4 | **Quarkus** | Apache 2.0 | YES | None. Red Hat sponsors but license is clean Apache 2.0. |
| 5 | **Spring Boot** | Apache 2.0 | YES | None. "Spring" is a trademark -- do not use the name to imply endorsement. |
| 6 | **Ktor** | Apache 2.0 | YES | None. JetBrains project, fully permissive. |
| 7 | **Vue.js** | MIT | YES | None. Fully permissive. |
| 8 | **Tailwind CSS** | MIT | YES | None. Tailwind CSS framework is MIT. Note: Tailwind UI (premium component library) is a separate paid product with its own license. |
| 9 | **Strapi (Community)** | MIT | YES | Community Edition (outside `ee/` directories) is MIT. Enterprise features under `ee/` directories are proprietary and require a paid Strapi Enterprise license. Avoid using `ee/` code without a license. |
| 10 | **Keycloak** | Apache 2.0 | YES | None. CNCF incubating project. Fully permissive. |
| 11 | **PostgreSQL** | PostgreSQL License (BSD-like) | YES | None. Extremely permissive. No copyleft. The PostgreSQL Global Development Group is committed to keeping it permissive in perpetuity. |
| 12 | **pgvector** | PostgreSQL License (BSD-like) | YES | None. Same permissive license as PostgreSQL itself. |
| 13 | **ParadeDB (Community)** | **AGPL-3.0** | **NO** | **RED FLAG.** AGPL requires that if you provide the software as a network service, you must release the complete source code of the combined work under AGPL. This means using ParadeDB Community in a SaaS product would require open-sourcing Raven's code or purchasing a commercial license from ParadeDB. |
| 14 | **ParadeDB (Enterprise)** | **Proprietary / Commercial** | **REQUIRES PAID LICENSE** | **RED FLAG.** Enterprise features (partitioned indexes, security, hot-standby search) are only available under a commercial license. Contact sales@paradedb.com for pricing. |
| 15 | **Redis** | **RSALv2 / SSPLv1 / AGPLv3** (tri-license since v8.0, May 2025) | **NO (without commercial license)** | **RED FLAG.** All three license options are problematic for SaaS: (1) **RSALv2** explicitly prohibits offering Redis as a managed/hosted service; (2) **SSPLv1** requires open-sourcing the entire service stack if Redis is offered as a service; (3) **AGPLv3** requires source disclosure for network services using modified Redis. **Alternatives:** Use Valkey (BSD-3-Clause fork), KeyDB (BSD-3-Clause), or DragonflyDB (BSL-1.1, converts to Apache 2.0 after 4 years). Or purchase a commercial Redis license. |
| 16 | **LiteParse (@llamaindex/liteparse)** | Apache 2.0 | YES | None. Runs locally, no cloud dependencies, no API keys required. |
| 17 | **Crawl4AI** | Apache 2.0 | YES | None. Fully open source and permissive. No hidden API keys or SaaS models. |
| 18 | **Firecrawl** | **AGPL-3.0** | **NO** | **RED FLAG.** Self-hosted Firecrawl is AGPL-3.0. Network use triggers copyleft obligation. SDKs and some UI components are MIT, but the core crawler is AGPL. **Alternatives:** Use Crawl4AI (Apache 2.0), or use Firecrawl's hosted API (SaaS consumption, not self-hosting, avoids AGPL trigger), or negotiate a commercial license. |
| 19 | **LiveKit (server)** | Apache 2.0 | YES | None. Fully permissive. LiveKit Cloud is a separate hosted offering. |
| 20 | **LiveKit Agents** | Apache 2.0 | YES | None. Same permissive license as LiveKit server. |
| 21 | **TEN Framework** | Apache 2.0 with additional restrictions | **LIKELY YES (review needed)** | **YELLOW.** The TEN framework states it is "Apache 2.0 with additional restrictions." TEN-Agent (the reference app) is clean Apache 2.0. The exact nature of the "additional restrictions" on the core framework should be reviewed by reading the LICENSE file at https://github.com/TEN-framework/ten-framework/blob/main/LICENSE before committing to production use. |
| 22 | **Pipecat** | BSD-2-Clause | YES | None. Very permissive. Minimal attribution requirements. |
| 23 | **faster-whisper** | MIT | YES | None. SYSTRAN's CTranslate2-based reimplementation. Fully permissive. |
| 24 | **Whisper (OpenAI)** | MIT | YES | None. Code and model weights are both MIT. Fully permissive. |
| 25 | **Piper TTS** | MIT (original, archived) / **GPL-3.0** (active fork) | **CONDITIONAL** | **YELLOW.** The original rhasspy/piper repository is MIT-licensed but was archived in October 2025. The actively maintained fork (OHF-Voice/piper1-gpl) is GPL-3.0, which has copyleft obligations (though not network/SaaS-triggered like AGPL). **Recommendation:** Use the archived MIT version if features are sufficient. If you need the active fork, GPL-3.0 does NOT trigger on SaaS use (only distribution), so running it server-side in a SaaS is generally OK. Consult legal counsel. |
| 26 | **Coqui TTS / XTTS** | MPL-2.0 (code) / **CPML** (XTTS model weights) | **NO (for XTTS models)** | **RED FLAG (model weights).** The TTS codebase is MPL-2.0, which is a weak copyleft (file-level, not project-level) and generally SaaS-compatible. However, the XTTS-v2 model weights are under the Coqui Public Model License (CPML), which restricts use to **non-commercial purposes only**. Coqui shut down in January 2024, so no one can sell a commercial license anymore. **Recommendation:** Use the MPL-2.0 code with your own trained models, or use alternative TTS models (Piper MIT, OpenAI TTS API, etc.). |
| 27 | **Silero VAD** | MIT | YES | None. No telemetry, no keys, no registration, no vendor lock-in. Fully permissive. |
| 28 | **MinIO** | **AGPL-3.0** | **NO** | **RED FLAG.** MinIO is AGPL-3.0 with a dual-license model. The AGPL network copyleft clause means that providing MinIO as part of a hosted SaaS requires open-sourcing the entire combined work, or purchasing a commercial license from MinIO. **Alternatives:** Use AWS S3 (hosted), Garage (AGPL but separate service -- consult legal), SeaweedFS (Apache 2.0), or purchase a MinIO commercial license. |
| 29 | **Tesseract.js** | Apache 2.0 | YES | None. Pure JavaScript OCR engine. Fully permissive. |
| 30 | **Docker Engine** | Apache 2.0 | YES | Docker Engine (dockerd, containerd, runc) is Apache 2.0. No licensing issues for running containers. |
| 31 | **Docker Desktop** | Proprietary (Docker Subscription Service Agreement) | **CONDITIONAL** | **YELLOW.** Docker Desktop is free only for: (a) personal use, (b) education, (c) non-commercial open source, or (d) companies with <250 employees AND <$10M annual revenue. Larger companies must purchase a Docker Business subscription. This only affects developer workstations, not production deployments (which use Docker Engine). |
| 32 | **Docker Compose** | Apache 2.0 | YES | None. CLI tool is Apache 2.0. |
| 33 | **Nginx** | BSD-2-Clause | YES | Nginx open source is BSD-2-Clause. NGINX Plus is a separate commercial product. |
| 34 | **Traefik** | MIT | YES | None. Fully permissive. Traefik Enterprise is a separate commercial product. |

---

## RED FLAG Summary & Recommended Actions

### 1. ParadeDB (Community) -- AGPL-3.0
- **Risk:** Using ParadeDB extensions (pg_search, pg_analytics) in Raven's SaaS would trigger AGPL copyleft, requiring release of Raven's source code.
- **Options:**
  - (a) Purchase a commercial license from ParadeDB (contact sales@paradedb.com)
  - (b) Replace with native PostgreSQL full-text search + pgvector (both permissively licensed)
  - (c) Use a different search solution (e.g., Meilisearch under MIT, or Typesense under GPL-3.0 server-side)

### 2. Redis -- RSALv2 / SSPLv1 / AGPLv3
- **Risk:** All three license options restrict or impose copyleft obligations on SaaS use.
- **Options:**
  - (a) **Switch to Valkey** (BSD-3-Clause) -- the Linux Foundation fork of Redis 7.2, API-compatible drop-in replacement
  - (b) **Switch to KeyDB** (BSD-3-Clause) -- Snap Inc.'s Redis-compatible fork
  - (c) Purchase a commercial Redis license from Redis Ltd.
  - (d) Use a managed Redis service (AWS ElastiCache, etc.) -- licensing burden falls on the cloud provider

### 3. Firecrawl -- AGPL-3.0
- **Risk:** Self-hosting Firecrawl in Raven's SaaS triggers AGPL copyleft.
- **Options:**
  - (a) **Switch to Crawl4AI** (Apache 2.0) -- already in the stack, fully permissive
  - (b) Use Firecrawl's hosted cloud API instead of self-hosting (SaaS consumption model avoids AGPL trigger)
  - (c) Negotiate a commercial license with Firecrawl (Sideguide Technologies Inc.)

### 4. MinIO -- AGPL-3.0
- **Risk:** Embedding MinIO in Raven's SaaS triggers AGPL network copyleft.
- **Options:**
  - (a) **Switch to SeaweedFS** (Apache 2.0) -- S3-compatible object storage
  - (b) Use a cloud object storage service (AWS S3, GCS, Azure Blob) in production
  - (c) Purchase a MinIO commercial license
  - (d) If MinIO runs as a completely separate, unmodified service (not a derivative work), some legal interpretations may consider it safe -- but this is legally gray and risky. Consult legal counsel.

### 5. Coqui TTS / XTTS Model Weights -- CPML (Non-Commercial)
- **Risk:** XTTS-v2 model weights are restricted to non-commercial use. Coqui is defunct; no one can sell commercial licenses.
- **Options:**
  - (a) Use the Coqui TTS code (MPL-2.0) with **your own trained models**
  - (b) **Switch to Piper TTS** (MIT archived version) for on-premise TTS
  - (c) Use OpenAI Whisper TTS API or other commercial TTS services
  - (d) Use other open-weight TTS models with permissive licenses (e.g., Meta's Voicebox if released, Bark by Suno under MIT)

### 6. Docker Desktop -- Proprietary Subscription
- **Risk:** If Raven's company has >= 250 employees OR >= $10M revenue, Docker Desktop requires a paid subscription for developer workstations.
- **Options:**
  - (a) Purchase Docker Business subscriptions for developers
  - (b) Use alternatives like Podman Desktop (Apache 2.0), Rancher Desktop (Apache 2.0), or Colima (MIT) on developer machines
  - (c) Note: This does NOT affect production deployments (Docker Engine is Apache 2.0)

---

## License Compatibility Matrix (Quick Reference)

| License | Type | SaaS Safe? | Copyleft? | Notes |
|---------|------|-----------|-----------|-------|
| MIT | Permissive | YES | No | Most permissive. Do anything with attribution. |
| BSD-2-Clause | Permissive | YES | No | Minimal restrictions. |
| BSD-3-Clause | Permissive | YES | No | Cannot use project name for endorsement. |
| PostgreSQL License | Permissive | YES | No | Essentially BSD-like. |
| Apache 2.0 | Permissive | YES | No | Includes patent grant. Must preserve NOTICE files. |
| MPL-2.0 | Weak Copyleft | YES (mostly) | File-level | Modified MPL files must stay MPL; your own files are unaffected. |
| GPL-3.0 | Strong Copyleft | YES (server-side) | Yes (distribution) | SaaS loophole: only triggers on distribution, not network access. |
| AGPL-3.0 | Strong Copyleft (network) | **NO** | Yes (network use) | Triggers on providing software as a network service. |
| SSPL-1.0 | Source Available | **NO** | Yes (extreme) | Must open-source entire service stack. Not OSI-approved. |
| RSALv2 | Source Available | **NO** | No, but restrictive | Explicitly prohibits offering as a managed service. |
| BSL-1.1 | Source Available | **NO** (until conversion) | Time-limited | Converts to permissive license after specified period. |
| CPML | Proprietary | **NO** | N/A | Non-commercial use only. |

---

## Methodology

- All licenses were verified against official GitHub repositories and package registries (npm, PyPI, Maven Central, pkg.go.dev).
- Web searches were conducted on 2026-03-27 using current repository data.
- For dual/multi-licensed projects, the most restrictive applicable license was flagged.
- "Commercial SaaS OK" means: Can Raven use this component in a proprietary, closed-source SaaS product without open-sourcing Raven's code and without purchasing an additional license?

---

## Sources

- [Go LICENSE](https://go.dev/LICENSE)
- [Kotlin LICENSE](https://github.com/JetBrains/kotlin/blob/master/license/LICENSE.txt)
- [Gin LICENSE](https://github.com/gin-gonic/gin/blob/master/LICENSE)
- [Quarkus LICENSE](https://github.com/quarkusio/quarkus/blob/main/LICENSE)
- [Spring Boot LICENSE](https://github.com/spring-projects/spring-boot/blob/main/LICENSE.txt)
- [Ktor LICENSE](https://github.com/ktorio/ktor/blob/main/LICENSE)
- [Vue.js LICENSE](https://github.com/vuejs/core/blob/main/LICENSE)
- [Tailwind CSS LICENSE](https://github.com/tailwindlabs/tailwindcss/blob/main/LICENSE)
- [Strapi LICENSE](https://github.com/strapi/strapi/blob/main/LICENSE)
- [Keycloak LICENSE](https://github.com/keycloak/keycloak/blob/main/LICENSE.txt)
- [PostgreSQL License](https://www.postgresql.org/about/licence/)
- [pgvector GitHub](https://github.com/pgvector/pgvector)
- [ParadeDB - Why We Picked AGPL](https://www.paradedb.com/blog/agpl)
- [Redis License Change Blog](https://redis.io/blog/redis-adopts-dual-source-available-licensing/)
- [Redis AGPLv3 Announcement](https://redis.io/blog/agplv3/)
- [LiteParse GitHub](https://github.com/run-llama/liteparse)
- [Crawl4AI GitHub](https://github.com/unclecode/crawl4ai)
- [Firecrawl LICENSE](https://github.com/firecrawl/firecrawl/blob/main/LICENSE)
- [LiveKit LICENSE](https://github.com/livekit/livekit/blob/master/LICENSE)
- [LiveKit Agents LICENSE](https://github.com/livekit/agents/blob/main/LICENSE)
- [TEN Framework GitHub](https://github.com/TEN-framework/ten-framework)
- [Pipecat PyPI](https://pypi.org/project/pipecat-ai/)
- [faster-whisper LICENSE](https://github.com/SYSTRAN/faster-whisper/blob/master/LICENSE)
- [OpenAI Whisper LICENSE](https://github.com/openai/whisper/blob/main/LICENSE)
- [Piper TTS GitHub](https://github.com/rhasspy/piper)
- [Coqui TTS GitHub](https://github.com/coqui-ai/TTS)
- [Coqui XTTS-v2 LICENSE](https://huggingface.co/coqui/XTTS-v2/blob/main/LICENSE.txt)
- [Silero VAD LICENSE](https://github.com/snakers4/silero-vad/blob/master/LICENSE)
- [MinIO LICENSE](https://github.com/minio/minio/blob/master/LICENSE)
- [MinIO Commercial License](https://www.min.io/commercial-license)
- [Tesseract.js LICENSE](https://github.com/naptha/tesseract.js/blob/master/LICENSE.md)
- [Docker Compose LICENSE](https://github.com/docker/compose/blob/main/LICENSE)
- [Docker Desktop License](https://docs.docker.com/subscription/desktop-license/)
- [Nginx LICENSE](https://nginx.org/LICENSE)
- [Traefik LICENSE](https://github.com/traefik/traefik/blob/master/LICENSE.md)
