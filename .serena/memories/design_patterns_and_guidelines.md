# Raven — Design Patterns & Guidelines

## Dependency Injection via Interfaces
- Each layer defines the interface it depends on (not the implementation)
- Handler defines `*Servicer` interface; Service defines `*Repository` interface
- Constructors accept interfaces, return concrete types
- This enables easy mocking in tests

## Multi-Tenancy
- All data is scoped by `org_id` (organization)
- PostgreSQL Row-Level Security (RLS) policies enforced at DB level (migration 00015)
- JWT claims carry `OrgID`, `OrgRole`, `WorkspaceIDs`, `KBPermissions`
- Middleware extracts identity from JWT/API-key and injects into context

## Configuration
- Viper-based config with `RAVEN_*` environment variable prefix
- Nested config struct (`Config.Server`, `Config.Database`, `Config.Valkey`, etc.)
- `.env` files for Docker Compose; env vars in CI

## Background Jobs
- Asynq task queue backed by Valkey
- Job definitions in `internal/jobs/` (airbyte sync, recrawl, cleanup, email, webhook delivery, usage tracking)
- Task types defined in `internal/queue/tasks.go`

## Enterprise Edition (EE)
- `internal/ee/` contains premium features: analytics, audit, connectors, lead gen, licensing, SSO, security, webhooks
- `frontend/src/ee/` for frontend EE components
- Separate `ee-LICENSE` and `ee-README.md`

## API Design
- RESTful JSON API at `/api/v1/`
- Swagger/OpenAPI documentation auto-generated
- SSE (Server-Sent Events) for streaming chat completions
- Standard error format via `pkg/apierror.AppError`

## Database
- PostgreSQL 18 with extensions: pgvector, pg_trgm, BM25
- goose for versioned SQL migrations (`migrations/` directory)
- Raw SQL queries (no ORM), pgx driver
- ClickHouse for analytics/embeddings (optional via overlay compose)

## Observability
- OpenTelemetry for traces, metrics, and logs
- eBPF programs for kernel-level audit and XDP network filtering
- PostHog for user analytics/tracking

## Security
- JWT validation via JWKS (Keycloak)
- API key authentication (middleware)
- Rate limiting (per-user, per-org, per-plan tier)
- RBAC middleware
- Security rules middleware
- eBPF audit logging
