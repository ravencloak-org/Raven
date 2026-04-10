# Raven — Codebase Structure

```
raven/
├── cmd/
│   ├── api/          # Go API server entrypoint (main.go)
│   └── worker/       # Go background worker entrypoint
├── internal/         # Go internal packages (not importable externally)
│   ├── cache/        # Valkey/Redis cache layer
│   ├── config/       # Viper-based configuration (Config struct)
│   ├── crypto/       # AES encryption utilities
│   ├── db/           # Database connection pool setup
│   ├── ebpf/         # eBPF programs (audit, observability, XDP)
│   ├── ee/           # Enterprise Edition features (analytics, audit, connectors, lead, licensing, security, sso, webhooks)
│   ├── grpc/         # gRPC client to Python AI worker
│   ├── handler/      # Gin HTTP handlers (one file per domain: chat, org, kb, document, search, voice, billing, etc.)
│   ├── hyperswitch/  # Payment gateway integration
│   ├── integration/  # Integration tests (gRPC fault, migration, SSE chat, webhook)
│   ├── jobs/         # Asynq background jobs (airbyte sync, recrawl, cleanup, email, webhook delivery, usage, voice usage)
│   ├── middleware/    # Gin middleware (auth/JWT, CORS, rate limit, RBAC, security rules, API key, tracking)
│   ├── model/        # Domain models and request/response DTOs
│   ├── posthog/      # PostHog analytics client
│   ├── queue/        # Asynq queue client/server/task definitions
│   ├── repository/   # PostgreSQL data access layer (one file per domain)
│   ├── service/      # Business logic layer (one file per domain)
│   ├── storage/      # SeaweedFS file storage
│   ├── stt/          # Speech-to-text provider abstraction
│   ├── telemetry/    # OpenTelemetry setup
│   ├── testutil/     # Test helpers (TestDB with testcontainers)
│   └── tts/          # Text-to-speech provider abstraction
├── pkg/              # Public Go packages
│   ├── apierror/     # Standard API error type
│   ├── livekit/      # LiveKit client helpers
│   ├── meta/         # Metadata utilities
│   └── validator/    # Request validation
├── ai-worker/        # Python gRPC AI worker
│   ├── raven_worker/ # Main Python package
│   ├── tests/        # Python tests
│   ├── pyproject.toml
│   └── Makefile
├── frontend/         # Vue.js 3 SPA
│   ├── src/
│   │   ├── api/         # API client modules (one per domain)
│   │   ├── components/  # Vue components
│   │   ├── composables/ # Vue composables (useAuth, useFeatureFlag, etc.)
│   │   ├── ee/          # Enterprise frontend features
│   │   ├── layouts/     # Page layouts
│   │   ├── pages/       # Route pages (analytics, chatbot, knowledge-bases, orgs, voice, whatsapp, etc.)
│   │   ├── plugins/     # Vue plugins
│   │   ├── router/      # Vue Router config
│   │   ├── stores/      # Pinia stores (one per domain, each with .spec.ts)
│   │   └── types/       # TypeScript type definitions
│   ├── e2e/          # Playwright E2E tests
│   └── package.json
├── proto/            # Protobuf definitions (ai_worker.proto, buf config)
├── contracts/        # OpenAPI stub (openapi-stub.yaml)
├── migrations/       # Goose SQL migrations (00001–00020+)
├── deploy/           # Deployment configs (Ansible, EC2, Cloudflare, edge, Keycloak, LiveKit, etc.)
├── scripts/          # Dev/CI scripts
├── tests/            # Cross-cutting eBPF tests
├── docs/             # Documentation and swagger output
├── docker-compose.yml        # Main compose (go-api, python-worker, python-agent, postgres, valkey, keycloak, etc.)
├── docker-compose.ebpf.yml   # eBPF overlay
├── docker-compose.edge.yml   # Edge deployment overlay
├── docker-compose.chartdb.yml # ChartDB overlay
├── Dockerfile        # Multi-stage Go API build
├── Makefile          # Go build/test/lint/migrate commands
├── Makefile.edge     # Edge-specific build
├── go.mod / go.sum   # Go module definition
└── CLAUDE.md         # Claude Code instructions
```

## Architecture Pattern
**Handler → Service → Repository** (3-layer) with dependency injection via interfaces.
- **Handler**: Defines a `*Servicer` interface, receives service via constructor, handles HTTP binding
- **Service**: Defines `*Repository` interface, contains business logic
- **Repository**: Direct pgx SQL queries with named query constants
- **Model**: Shared DTOs between layers
