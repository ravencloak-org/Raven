# Development Guide

Everything you need to run Raven locally, understand the codebase, and work with the AI-assisted development toolchain.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Local Setup (Docker Compose)](#local-setup-docker-compose)
3. [Local Setup (Without Docker)](#local-setup-without-docker)
4. [Go API Server](#go-api-server)
5. [Python AI Worker](#python-ai-worker)
6. [Vue.js Frontend](#vuejs-frontend)
7. [Database Migrations](#database-migrations)
8. [Running Tests](#running-tests)
9. [Make Targets](#make-targets)
10. [AI-Assisted Development with MCP Tools](#ai-assisted-development-with-mcp-tools)

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Docker + Docker Compose | Latest | [docs.docker.com](https://docs.docker.com/get-docker/) |
| Go | 1.26+ | [go.dev/dl](https://go.dev/dl/) |
| Python | 3.12+ | [python.org](https://www.python.org/downloads/) |
| Node.js | LTS (22+) | [nodejs.org](https://nodejs.org/) |
| `golangci-lint` | Latest | `brew install golangci-lint` |
| `goose` | Latest | `go install github.com/pressly/goose/v3/cmd/goose@latest` |
| `air` | Latest | `go install github.com/air-verse/air@latest` (hot reload) |
| `ruff` | Latest | `pip install ruff` |
| Playwright | — | `npx playwright install` (after `npm install`) |

---

## Local Setup (Docker Compose)

The fastest path — everything runs in containers.

```bash
git clone https://github.com/ravencloak-org/Raven.git
cd Raven

# Copy and configure environment
cp .env.example .env
# Edit .env — at minimum set:
#   ANTHROPIC_API_KEY or OPENAI_API_KEY (for LLM calls)
#   RAVEN_KEYCLOAK_ISSUERURL (default: http://localhost:8080/realms/raven)

# Start all services
docker compose up -d

# Watch logs
docker compose logs -f go-api python-worker
```

Services and their ports:

| Service | URL | Notes |
|---------|-----|-------|
| Admin dashboard | http://localhost:3000 | Vue.js frontend (served by Vite in dev mode) |
| Go API | http://localhost:8080 | REST API + SSE |
| Keycloak | http://localhost:8081 | Identity provider |
| PostgreSQL | localhost:5432 | pgvector + ParadeDB |
| Valkey | localhost:6379 | Redis-compatible cache/queue |
| SeaweedFS Filer | http://localhost:8888 | Object storage |
| OpenObserve | http://localhost:5080 | Logs, metrics, traces |
| Traefik dashboard | http://localhost:8090 | Reverse proxy (dev only) |

**First-time Keycloak setup:**

```bash
# Provision the raven realm automatically
curl -X POST http://localhost:8080/api/v1/internal/provision-realm \
  -H "Content-Type: application/json" \
  -H "X-Internal-Key: $(grep RAVEN_INTERNAL_API_KEY .env | cut -d= -f2)" \
  -d '{"realm": "raven"}'

# Then log in to Keycloak admin at http://localhost:8081
# Default admin creds are in your .env (KEYCLOAK_ADMIN / KEYCLOAK_ADMIN_PASSWORD)
# Create a user in the raven realm with role org_admin
```

---

## Local Setup (Without Docker)

Run each service natively for faster iteration.

### 1. Start infrastructure only

```bash
# Start only the backing services (Postgres, Valkey, Keycloak, SeaweedFS)
docker compose up -d postgres valkey keycloak seaweedfs-master seaweedfs-volume seaweedfs-filer
```

### 2. Apply database migrations

```bash
export DATABASE_URL="postgresql://raven:changeme@localhost:5432/raven?sslmode=disable"
make migrate-up
```

### 3. Go API (with hot reload)

```bash
export $(cat .env | grep -v '#' | xargs)
make dev       # uses `air` for hot reload
# OR
make run       # no hot reload
```

### 4. Python AI Worker

```bash
cd ai-worker
python -m venv .venv
source .venv/bin/activate
pip install -e ".[dev]"

# Start the gRPC worker
python -m raven_worker.server
```

### 5. Frontend

```bash
cd frontend
npm install
npm run dev    # Vite dev server at http://localhost:3000
```

---

## Go API Server

**Entry point:** `cmd/api/main.go`

**Package layout:**

```
internal/
├── config/       — Viper config, env bindings
├── handler/      — Gin HTTP handlers (one file per resource)
├── service/      — Business logic (handler → service → repository)
├── repository/   — pgx database queries (SQL constants + scan functions)
├── middleware/   — JWT auth, RLS org context, rate limiting, tier enforcement
├── model/        — Shared types (structs for DB rows, API requests/responses)
├── integration/  — External service clients (Keycloak admin, etc.)
├── jobs/         — Asynq background task handlers
└── ee/           — Enterprise Edition feature stubs
pkg/
├── apierror/     — AppError, QuotaError, error middleware
└── db/           — pgx helpers (WithOrgID for RLS transactions)
```

**Adding a new endpoint:**

1. Add SQL constants + scan function to `internal/repository/`
2. Add business logic method to `internal/service/`
3. Add Gin handler to `internal/handler/`
4. Wire route in `cmd/api/main.go`

**Error handling convention:**

```go
// In service layer — return typed apierror
return nil, apierror.NewNotFound("knowledge base not found")

// In handler — use c.Error() + c.Abort(), let middleware render
if err != nil {
    _ = c.Error(err)
    c.Abort()
    return
}
```

---

## Python AI Worker

**Entry point:** `ai-worker/raven_worker/server.py`

**gRPC service definitions:** `ai-worker/proto/`

**Key modules:**

```
raven_worker/
├── server.py          — gRPC server startup
├── servicer.py        — gRPC method implementations
├── rag/               — Retrieval-augmented generation pipeline
│   ├── retrieval.py   — pgvector + BM25 hybrid search with RRF
│   ├── reranker.py    — Cohere reranking
│   └── generator.py   — LLM call + streaming
├── processors/        — Document parsing (PDF, HTML, markdown)
├── embedding/         — Embedding provider abstraction (OpenAI, Cohere)
└── voice/             — LiveKit voice agent integration
```

**Running tests:**

```bash
cd ai-worker
pytest                        # all tests
pytest tests/test_rag_service.py -v    # specific module
pytest tests/smoke/ -v -m smoke        # smoke tests (need live API keys)
```

---

## Vue.js Frontend

**Entry point:** `frontend/src/main.ts`

**Directory layout:**

```
frontend/src/
├── api/          — API modules (one per backend resource, typed fetch wrappers)
├── components/   — Reusable Vue components
├── composables/  — Composition API utilities (useFeatureFlag, useOnboarding, etc.)
├── layouts/      — Page layouts (AuthLayout, DefaultLayout)
├── pages/        — Route views (one directory per feature area)
├── router/       — Vue Router configuration
├── stores/       — Pinia stores (auth, billing, etc.)
└── types/        — Shared TypeScript type definitions
```

**Adding a new page:**

1. Create `frontend/src/api/<resource>.ts` — typed `authFetch` wrappers
2. Create `frontend/src/stores/<resource>.ts` — Pinia store
3. Create `frontend/src/pages/<feature>/YourPage.vue`
4. Add route to `frontend/src/router/index.ts`

**Feature flags** (PostHog):

```typescript
import { useFeatureFlag } from '../composables/useFeatureFlag'
const { isEnabled } = useFeatureFlag('your_flag_name')
```

**402 / quota errors** — handled globally via the shared `authFetch` in `src/api/utils.ts`, which calls `useBillingStore().flagQuotaExceeded()` automatically. No per-page handling needed.

---

## Database Migrations

Migrations use [goose](https://github.com/pressly/goose) and live in `migrations/`.

```bash
make migrate-up      # apply all pending
make migrate-down    # roll back one step
```

**Writing a migration:**

```sql
-- migrations/00042_add_query_cache.sql
-- +goose Up
CREATE TABLE query_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    ...
);
-- RLS is required on every new tenant-scoped table
ALTER TABLE query_cache ENABLE ROW LEVEL SECURITY;
CREATE POLICY query_cache_isolation ON query_cache
    USING (org_id = current_setting('app.org_id')::uuid);

-- +goose Down
DROP TABLE query_cache;
```

Rules:
- Never edit a migration that has been merged to `main`
- Every tenant-scoped table needs an RLS policy
- Use `gen_random_uuid()` for primary keys

---

## Running Tests

### Go

```bash
go test ./...                          # all tests
go test -race -timeout 30m ./...       # with race detector (same as CI)
go test ./internal/service/... -v      # specific package
go test -run TestQuotaChecker ./...    # specific test
```

Integration tests spin up a real Postgres via [testcontainers](https://testcontainers.com/). Docker must be running.

```bash
# Skip integration tests if no Docker
go test -short ./...
```

### Python

```bash
cd ai-worker
pytest                   # all
pytest -x                # stop on first failure
pytest -k "test_rag"     # filter by name
ruff check .             # lint
ruff format --check .    # format check
```

### Frontend

```bash
cd frontend
npm run test:unit        # Vitest (fast, no browser)
npm run test:e2e         # Playwright (needs running stack)
npx tsc --noEmit         # TypeScript check without building
npm run build            # full production build (catches more type errors)
```

---

## Make Targets

```bash
make build        # go build -o bin/api ./cmd/api
make run          # go run ./cmd/api
make dev          # air (hot reload)
make test         # go test ./...
make lint         # golangci-lint run
make migrate-up   # goose up
make migrate-down # goose down
make swagger      # regenerate docs/swagger from source
make proto        # regenerate gRPC stubs from proto files
```

---

## AI-Assisted Development with MCP Tools

This project is developed with Claude Code, which has a suite of MCP (Model Context Protocol) tools wired in. If you're contributing using Claude Code, here's how to use them effectively.

### Local Semantic Search — `context-mode`

The most important tool for exploring the codebase. Instead of grepping for strings, index everything and query semantically.

**Index the codebase and search in one call:**

```
ctx_batch_execute(
  commands=[
    {"label": "Handler files", "command": "ls internal/handler/"},
    {"label": "Service layer", "command": "cat internal/service/quota.go"},
    {"label": "Config struct", "command": "grep -A 20 'type Config struct' internal/config/config.go"},
  ],
  queries=[
    "quota checker service interface methods",
    "how RLS org_id is set in transactions",
    "billing payment intent handler",
  ]
)
```

**Follow-up searches** (after an initial index):

```
ctx_search(queries=[
  "pgx transaction WithOrgID pattern",
  "apierror QuotaError 402 response",
])
```

**When to use it:**
- Before writing any new code — understand the existing pattern first
- When you don't know which file contains something
- When `grep` would return too many results to read

### Symbolic Code Navigation — `serena`

Navigate by symbol, not by file. Great for understanding how things connect.

```
# Find a symbol and its usages
serena.find_symbol("QuotaChecker", include_body=true)
serena.find_referencing_symbols("NewQuotaChecker")

# Get an overview of all symbols in a file
serena.get_symbols_overview("internal/service/quota.go")

# Safe rename across the codebase
serena.rename_symbol("OldName", "NewName", relative_path="internal/service/quota.go")
```

### Library Documentation — `context7`

Fetch accurate, up-to-date docs for any library instead of relying on training data.

```
# Always resolve the library ID first
context7.resolve-library-id("pgx v5 postgres go")
# → /jackc/pgx

context7.query-docs(
  library_id="/jackc/pgx",
  query="how to use pgx.Tx QueryRow and scan results"
)
```

Use this whenever you're working with: pgx, gin, pinia, vue-router, keycloak-js, livekit, asynq, goose, testcontainers, vitest, playwright.

### GitHub Operations — `mcp__github`

```
# Read an issue with full spec
mcp__github.issue_read(method="get", owner="ravencloak-org", repo="Raven", issue_number=256)

# Read a PR's diff
mcp__github.pull_request_read(method="get_diff", ...)

# Add a review comment
mcp__github.pull_request_review_write(...)
```

### Browser Automation — `claude-in-chrome`

For verifying frontend behaviour without manual clicking.

```
tabs_context_mcp()          # always start here — see what's open
navigate(url="http://localhost:3000/billing")
find(description="Upgrade button")
javascript_tool(code="document.querySelector('.usage-bar').style.width")
read_console_messages(pattern="\\[billing\\]")
```

### Git Worktrees — standard pattern for this repo

Every feature is developed in an isolated worktree, not on the main working tree.

```bash
# Create a worktree for a new feature
git worktree add .worktrees/feat-my-feature -b feat/my-feature

# Work inside it
cd .worktrees/feat-my-feature

# Clean up after merge
git worktree remove .worktrees/feat-my-feature
```

Worktrees are gitignored. Never commit the `.worktrees/` directory.

### Parallel Agents for Independent Tasks

When multiple files need to change independently (e.g. backend model + frontend component), dispatch subagents in parallel rather than working sequentially. Each agent gets a fresh context and works in the same worktree.

See the [superpowers skills](docs/superpowers/) for the full workflow: `writing-plans` → `subagent-driven-development` → `finishing-a-development-branch`.
