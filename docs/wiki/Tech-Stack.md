# Tech Stack

## Decision Rationale

**Go + Gin** was chosen over Node.js and Kotlin/JVM for the backend API based on:
- Native compilation to single static binary: 10-25 MB Docker images
- Startup time under 50ms, RAM usage 5-10 MB at idle
- Trivial ARM64 cross-compilation for Raspberry Pi / edge deployment
- gRPC is Go-native (grpc-go is the reference implementation)
- Goroutines handle thousands of concurrent SSE/WebSocket connections

**Python AI Worker** runs separately via gRPC for the full ML/AI ecosystem.

## Complete Dependency Table

| # | Component | Version | License | Purpose | SaaS-Safe? |
|---|-----------|---------|---------|---------|------------|
| 1 | **Go** | 1.23.x | BSD-3-Clause | Backend API language | YES |
| 2 | **Gin** | v1.10.x | MIT | HTTP framework | YES |
| 3 | **grpc-go** | 1.70.x | Apache 2.0 | Go <-> Python communication | YES |
| 4 | **pgx** | v5.7.x | MIT | PostgreSQL driver | YES |
| 5 | **sqlc** | 1.28.x | MIT | Type-safe SQL code generation | YES |
| 6 | **go-redis** | v9.7.x | BSD-2-Clause | Valkey client | YES |
| 7 | **goose** | v3.24.x | MIT | Database migrations | YES |
| 8 | **viper** | 1.20.x | MIT | Configuration management | YES |
| 9 | **Python** | 3.12.x | PSF License | AI worker language | YES |
| 10 | **grpcio** | 1.69.x | Apache 2.0 | gRPC for Python | YES |
| 11 | **Vue.js** | 3.5.x | MIT | Frontend SPA | YES |
| 12 | **Tailwind CSS** | 4.x | MIT | Utility-first CSS | YES |
| 13 | **PostgreSQL** | 18.x | PostgreSQL License | Primary database | YES |
| 14 | **pgvector** | 0.8.x | PostgreSQL License | Vector similarity search | YES |
| 15 | **ParadeDB** | 0.22.x | AGPL-3.0 | BM25 full-text search | CAUTION |
| 16 | **Valkey** | 8.1.x | BSD-3-Clause | Job queue, caching (Redis replacement) | YES |
| 17 | **SeaweedFS** | 3.82.x | Apache 2.0 | Object storage (MinIO replacement) | YES |
| 18 | **Keycloak** | 26.x | Apache 2.0 | Identity provider | YES |
| 19 | **Strapi** | 5.x | MIT | Headless CMS | YES |
| 20 | **LiteParse** | Latest | Apache 2.0 | Document parsing | YES |
| 21 | **Crawl4AI** | 0.6.x | Apache 2.0 | Web scraping | YES |
| 22 | **LiveKit Server** | 2.3.x | Apache 2.0 | WebRTC SFU | YES |
| 23 | **LiveKit Agents** | 1.1.x | Apache 2.0 | Voice agent framework | YES |
| 24 | **faster-whisper** | 1.1.x | MIT | Self-hosted STT | YES |
| 25 | **Piper TTS** | 1.2.x | MIT | Self-hosted TTS | YES |
| 26 | **Silero VAD** | 5.1.x | MIT | Voice Activity Detection | YES |
| 27 | **Traefik** | 3.3.x | MIT | Reverse proxy with auto-TLS | YES |
| 28 | **PostHog** | Cloud | MIT core | Product analytics | YES |
| 29 | **OpenObserve** | 0.70+ | AGPL-3.0 | Observability (logs, metrics, traces) | CAUTION |
| 30 | **Asynq** | Latest | MIT | Scheduled jobs (Valkey-backed) | YES |
| 31 | **Stripe** | API | Proprietary | Payment processing | YES |

## License Risk Notes

- **ParadeDB (AGPL-3.0):** Purchase commercial license or use PostgreSQL native `tsvector` as fallback
- **OpenObserve (AGPL-3.0):** Safe as unmodified infrastructure; AGPL triggers only if you modify and serve it
- **Valkey** replaces Redis (which changed to restrictive licensing in 2024)
- **SeaweedFS** replaces MinIO (AGPL-3.0)
- **Crawl4AI** replaces Firecrawl (AGPL-3.0)
