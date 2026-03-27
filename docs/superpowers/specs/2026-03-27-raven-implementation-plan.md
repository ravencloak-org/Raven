# Raven Platform -- Implementation Plan

**Status:** Draft
**Date:** 2026-03-27
**Source:** [Final Design Specification](./2026-03-27-raven-platform-design-final.md)
**Purpose:** GitHub milestones and issue creation

> Each task maps to one GitHub issue. Milestones map to GitHub milestones. Dependencies are expressed as task IDs. Labels correspond to GitHub issue labels.

---

## Label Definitions

| Label | Description |
|-------|-------------|
| `backend` | Go API server (Echo, gRPC client, middleware) |
| `ai-worker` | Python AI worker (gRPC server, ML pipelines) |
| `frontend` | Vue.js + Tailwind Plus admin dashboard and web component |
| `infra` | Docker Compose, Traefik, database, Valkey, SeaweedFS |
| `auth` | Keycloak, JWT, API keys, RBAC |
| `docs` | Documentation, API specs, developer guides |
| `ci-cd` | GitHub Actions, linting, testing, builds |
| `billing` | Stripe, subscriptions, usage tracking |
| `security` | Encryption, RLS, input validation, CORS, rate limiting |
| `observability` | OpenTelemetry, OpenObserve, PostHog |

---

## Milestone 1: Project Scaffolding (Week 1-2)

> Foundation: project structure, service skeletons, local development environment, CI/CD, and infrastructure containers. No business logic yet -- just the plumbing that everything else builds on.

---

#### Task 1.1: Initialize Go Module with Echo and Project Structure

- **Milestone:** Project Scaffolding
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** None
- **Description:** Initialize the Go module (`github.com/raven-platform/raven`) with Echo v4 web framework. Set up the canonical Go project layout:
  ```
  cmd/
    api/          -- main.go entry point for the Go API server
  internal/
    config/       -- viper-based configuration loading
    handler/      -- Echo HTTP handlers (organized by domain)
    middleware/    -- JWT, RBAC, rate limiting, tenant resolution
    model/        -- domain types and database models
    repository/   -- database access layer (pgx + sqlc generated)
    service/      -- business logic layer
    grpc/         -- gRPC client for Python AI worker
  pkg/
    apierror/     -- shared error types and HTTP error responses
    validator/    -- input validation helpers
  migrations/     -- goose SQL migration files
  proto/          -- protobuf definitions (shared with Python worker)
  ```
  Install core dependencies: Echo v4, pgx v5, go-redis v9, viper, goose v3, grpc-go. Configure `air` for hot-reload during development. Create a health check endpoint (`GET /healthz`) that returns 200.
- **Acceptance criteria:**
  - `go build ./cmd/api` produces a working binary
  - `air` hot-reload works in development
  - `GET /healthz` returns `{"status": "ok"}`
  - All core dependencies are in `go.mod`
  - Project structure matches the layout above
- **Labels:** `backend`

---

#### Task 1.2: Initialize Python AI Worker with gRPC Server Skeleton

- **Milestone:** Project Scaffolding
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** None
- **Description:** Create the `ai-worker/` directory with a Python 3.12 project structure:
  ```
  ai-worker/
    raven_worker/
      __init__.py
      server.py         -- gRPC server entry point
      services/
        __init__.py
        embedding.py    -- embedding service stub
        rag.py          -- RAG query service stub
      processors/
        __init__.py
        parser.py       -- LiteParse integration stub
        chunker.py      -- text chunking stub
        scraper.py      -- Crawl4AI integration stub
      providers/
        __init__.py
        base.py         -- EmbeddingProvider protocol
    proto/              -- symlink or copy from root proto/
    tests/
    pyproject.toml      -- project metadata and dependencies
    requirements.txt    -- pinned dependencies
    Dockerfile
  ```
  Define the protobuf service in `proto/ai_worker.proto` matching the design spec (`AIWorker` service with `ParseAndEmbed`, `QueryRAG`, `GetEmbedding` RPCs). Generate Python gRPC stubs. Implement a gRPC health check. Set up `ruff` for linting and `pytest` for testing.
- **Acceptance criteria:**
  - `python -m raven_worker` starts the gRPC server on port 50051
  - gRPC health check responds correctly
  - Protobuf definitions compile for both Python and Go
  - `ruff check` passes with zero errors
  - `pytest` runs (even if no tests yet)
- **Labels:** `ai-worker`

---

#### Task 1.3: Initialize Vue.js + Tailwind Plus Frontend (Vite)

- **Milestone:** Project Scaffolding
- **Priority:** P1 (important)
- **Estimate:** 2 days
- **Dependencies:** None
- **Description:** Scaffold the `frontend/` directory using Vite with Vue.js 3.5 and TypeScript:
  ```
  frontend/
    src/
      assets/
      components/
      composables/
      layouts/
      pages/
      router/
      stores/         -- Pinia state management
      api/            -- API client (typed, auto-generated from OpenAPI later)
      types/
      App.vue
      main.ts
    public/
    index.html
    vite.config.ts
    tailwind.config.ts
    tsconfig.json
    package.json
  ```
  Install and configure Tailwind CSS v4 and Tailwind Plus. Set up Vue Router with a basic layout (sidebar + content area). Configure Pinia for state management. Set up ESLint + Prettier. Create placeholder pages: Login, Dashboard, 404.
- **Acceptance criteria:**
  - `npm run dev` starts the Vite dev server
  - Tailwind Plus components render correctly
  - Vue Router navigates between placeholder pages
  - `npm run lint` passes
  - `npm run build` produces a production bundle
- **Labels:** `frontend`

---

#### Task 1.4: Docker Compose with All Services

- **Milestone:** Project Scaffolding
- **Priority:** P0 (blocker)
- **Estimate:** 3 days
- **Dependencies:** 1.1, 1.2, 1.3
- **Description:** Create `docker-compose.yml` at the project root with all infrastructure services:
  - **PostgreSQL 18** with pgvector extension (`pgvector/pgvector:pg18`). Include an init script to enable `pgvector`, `pg_trgm`, and optionally `pg_search` (ParadeDB) extensions.
  - **Valkey 8.1** (`valkey/valkey:8.1-alpine`) with persistence enabled.
  - **Keycloak 26** (`quay.io/keycloak/keycloak:26`) with dev mode for local development, mounting `kc-config/` volume for realm exports and SPI JARs.
  - **Traefik 3.3** (`traefik:v3.3`) configured as reverse proxy routing `/api/*` to Go API, `/auth/*` to Keycloak, `/cms/*` to Strapi (if present). Dashboard enabled in dev mode.
  - **SeaweedFS** (`chrislusf/seaweedfs`) with master + volume + filer configured for S3-compatible access.
  - **go-api** and **python-worker** as custom build services with Dockerfiles.

  Create a `raven-internal` bridge network. Define named volumes: `pg-data`, `kc-config`, `valkey-data`, `seaweedfs-data`. Create `.env.example` with all required environment variables. Create `.env.secrets.example` (git-ignored template) for credentials. All services should start with a single `docker compose up -d`.
- **Acceptance criteria:**
  - `docker compose up -d` starts all services without errors
  - PostgreSQL is reachable from Go API container with pgvector enabled
  - Valkey accepts connections from Go API and Python worker containers
  - Keycloak admin console is accessible at `http://localhost:8443`
  - SeaweedFS S3-compatible endpoint is accessible from Go API
  - Traefik dashboard shows all routes
  - `docker compose down -v` cleanly removes everything
- **Labels:** `infra`

---

#### Task 1.5: Database Migrations Setup + Initial Schema

- **Milestone:** Project Scaffolding
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.1, 1.4
- **Description:** Set up goose v3 for database migrations. Create the initial migration files:
  - `001_create_extensions.sql` -- enable `uuid-ossp`, `pgvector`, `pg_trgm`
  - `002_create_organizations.sql` -- organizations table per design spec (id, name, slug, status, settings JSONB, keycloak_realm, billing_customer_id, billing_subscription_id, billing_plan, timestamps)
  - `003_create_users.sql` -- users table (id, org_id FK, email, display_name, keycloak_sub, status, last_login_at, timestamps)
  - `004_create_workspaces.sql` -- workspaces table (id, org_id FK, name, slug, settings JSONB, timestamps) with UNIQUE(org_id, slug)
  - `005_create_workspace_members.sql` -- workspace_members join table (workspace_id, user_id, role ENUM, timestamps) with UNIQUE(workspace_id, user_id)
  - `006_enable_rls.sql` -- enable RLS on all tables, create tenant_isolation policies using `current_setting('app.current_org_id')`, create admin bypass policies for `raven_admin` role

  Create a Makefile target `make migrate-up` and `make migrate-down`. Integrate migration into the Go API startup (auto-migrate option via config flag).
- **Acceptance criteria:**
  - `goose -dir migrations postgres "$DATABASE_URL" up` runs all migrations successfully
  - `goose -dir migrations postgres "$DATABASE_URL" down` rolls back cleanly
  - All tables exist with correct columns, types, and constraints
  - RLS policies are active on all tenant-scoped tables
  - `make migrate-up` works as a shortcut
  - Migrations are idempotent (running `up` twice is safe)
- **Labels:** `backend`, `infra`

---

#### Task 1.6: Keycloak Realm Configuration + reavencloak SPI Integration

- **Milestone:** Project Scaffolding
- **Priority:** P0 (blocker)
- **Estimate:** 3 days
- **Dependencies:** 1.4
- **Description:** Configure Keycloak for the Raven platform:
  1. **Realm setup:** Create a `raven` realm with:
     - OIDC client for the Vue.js admin dashboard (public client, Authorization Code + PKCE)
     - OIDC client for the Go API (confidential client, for service-to-service)
     - User registration enabled with email verification
     - Password policy: minimum 8 characters, at least one uppercase, one number
     - SMTP configuration for password reset and email verification (using Keycloak's built-in SMTP config, pointed at a local MailHog for dev)
  2. **reavencloak SPI:** Create a Keycloak SPI (Service Provider Interface) as a Java JAR that:
     - Adds custom JWT claims: `org_id`, `org_role`, `workspace_ids[]`, `kb_permissions[]`
     - Implements an event listener that calls Go API internal webhook on user lifecycle events (create, update, delete, login)
     - Reads org/workspace membership from PostgreSQL (direct JDBC or via Go API internal endpoint)
  3. **Realm export:** Export the configured realm as JSON and store in `kc-config/` for reproducible setup.
- **Acceptance criteria:**
  - Keycloak `raven` realm is auto-imported on container start
  - OIDC PKCE flow works from a test client (curl or browser)
  - JWT tokens include `org_id` and `org_role` custom claims
  - User registration and password reset emails are sent (MailHog in dev)
  - reavencloak SPI JAR is loaded by Keycloak without errors
  - Realm export JSON is committed to the repository
- **Labels:** `auth`, `infra`

---

#### Task 1.7: CI/CD Pipeline (GitHub Actions)

- **Milestone:** Project Scaffolding
- **Priority:** P1 (important)
- **Estimate:** 2 days
- **Dependencies:** 1.1, 1.2, 1.3
- **Description:** Create GitHub Actions workflows:
  1. **Go API** (`.github/workflows/go.yml`):
     - Trigger: push/PR to `main`
     - Steps: checkout, setup Go 1.23, `golangci-lint`, `go test ./...`, `go build` (linux/amd64 + linux/arm64), `govulncheck`
     - Artifact: multi-arch Docker image (build and push to GitHub Container Registry on main merge)
  2. **Python Worker** (`.github/workflows/python.yml`):
     - Trigger: push/PR to `main`
     - Steps: checkout, setup Python 3.12, `pip install`, `ruff check`, `ruff format --check`, `pytest`, Docker build
     - Artifact: Docker image to GHCR on main merge
  3. **Frontend** (`.github/workflows/frontend.yml`):
     - Trigger: push/PR to `main`
     - Steps: checkout, setup Node 22, `npm ci`, `eslint`, `prettier --check`, `vitest`, `vite build`
  4. **Docker Compose integration test** (`.github/workflows/integration.yml`):
     - Trigger: PR to `main`
     - Steps: `docker compose up -d`, wait for health checks, run smoke tests (health endpoints), `docker compose down`
  5. **Security scanning** via Trivy for container images
- **Acceptance criteria:**
  - All four workflows run on PR creation
  - Lint failures block merge
  - Test failures block merge
  - Docker images are built and pushed to GHCR on main merge
  - Multi-arch (amd64 + arm64) Go API images are produced
  - Workflow files are committed and visible in GitHub Actions tab
- **Labels:** `ci-cd`

---

#### Task 1.8: Dependabot + CodeRabbit Configuration

- **Milestone:** Project Scaffolding
- **Priority:** P2 (nice-to-have)
- **Estimate:** 0.5 days
- **Dependencies:** 1.1, 1.2, 1.3
- **Description:** Configure automated dependency management and AI code review:
  1. **Dependabot** (`.github/dependabot.yml`): weekly scans for `gomod` (/), `pip` (/ai-worker), `npm` (/frontend), `docker` (/).
  2. **CodeRabbit** (`.coderabbit.yaml`): enable AI-powered code review on all PRs. Configure review focus areas: security, performance, test coverage, Go idioms, Python type hints.
- **Acceptance criteria:**
  - Dependabot creates PRs for outdated dependencies
  - CodeRabbit posts review comments on new PRs
  - Configuration files are committed to the repository
- **Labels:** `ci-cd`, `docs`

---

#### Task 1.9: OpenTelemetry Instrumentation Setup (Go + Python)

- **Milestone:** Project Scaffolding
- **Priority:** P1 (important)
- **Estimate:** 2 days
- **Dependencies:** 1.1, 1.2, 1.4
- **Description:** Set up OpenTelemetry instrumentation as foundational plumbing (no OpenObserve deployment yet -- traces go to stdout/OTLP endpoint when configured):
  1. **Go API:**
     - Install `go.opentelemetry.io/otel` SDK
     - Create an OTel initialization function that configures a TracerProvider and MeterProvider
     - Add Echo middleware that auto-creates spans per request (method, path, status code)
     - Add trace context propagation to gRPC client calls
     - OTLP exporter configurable via environment variable (`OTEL_EXPORTER_OTLP_ENDPOINT`)
  2. **Python Worker:**
     - Install `opentelemetry-sdk`, `opentelemetry-exporter-otlp`, `opentelemetry-instrumentation-grpc`
     - Configure TracerProvider in gRPC server startup
     - Add gRPC server interceptor for automatic span creation
     - OTLP exporter configurable via environment variable
  3. Both services should gracefully degrade (no-op) if no OTLP endpoint is configured.
- **Acceptance criteria:**
  - Go API logs trace IDs in structured JSON output
  - Python worker logs trace IDs in structured output
  - Trace context propagates from Go API gRPC call to Python worker gRPC handler
  - Setting `OTEL_EXPORTER_OTLP_ENDPOINT` sends traces to the configured endpoint
  - Omitting the environment variable results in no errors (no-op mode)
- **Labels:** `backend`, `ai-worker`, `observability`

---

## Milestone 2: Core API + Auth (Week 3-4)

> Build the authenticated CRUD API layer. After this milestone, the Go API can create orgs, workspaces, users, and knowledge bases with proper JWT auth, role-based access control, and tenant isolation.

---

#### Task 2.1: JWT Middleware (Keycloak JWKS Validation)

- **Milestone:** Core API + Auth
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.1, 1.6
- **Description:** Implement Echo middleware that:
  1. Extracts the Bearer token from the `Authorization` header
  2. Validates the JWT signature against Keycloak's JWKS endpoint (`/auth/realms/raven/protocol/openid-connect/certs`), with JWKS response cached (TTL 1 hour, refresh on signature failure)
  3. Validates standard claims: `iss` (must match Keycloak issuer), `aud`, `exp`, `nbf`
  4. Extracts custom claims into request context: `org_id`, `org_role`, `workspace_ids[]`, `kb_permissions[]`, `sub` (user ID), `email`
  5. Sets `app.current_org_id` on the PostgreSQL connection for RLS enforcement
  6. Returns 401 for missing/invalid/expired tokens with structured error response
  7. Supports both Keycloak JWT (admin dashboard) and API key auth (chatbot widget) -- the middleware should detect which auth method is being used and route accordingly
- **Acceptance criteria:**
  - Valid Keycloak JWT grants access to protected endpoints
  - Expired JWT returns 401 with `{"error": "token_expired"}`
  - Invalid signature returns 401
  - `org_id` is available in request context for all downstream handlers
  - PostgreSQL RLS is enforced per-request
  - JWKS cache reduces Keycloak calls (verify with logs)
  - Unit tests cover all validation paths
- **Labels:** `backend`, `auth`, `security`

---

#### Task 2.2: Organization CRUD API

- **Milestone:** Core API + Auth
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.5, 2.1
- **Description:** Implement Organization CRUD endpoints:
  - `POST /api/v1/orgs` -- create organization (auto-creates Keycloak realm/client, sets creator as org_admin)
  - `GET /api/v1/orgs/:org_id` -- get organization details
  - `PUT /api/v1/orgs/:org_id` -- update organization (name, settings)
  - `DELETE /api/v1/orgs/:org_id` -- soft-delete organization (set status to `deactivated`)
  - `GET /api/v1/orgs/:org_id/members` -- list organization members

  Use sqlc for type-safe database queries. Implement the repository and service layers following the project structure. Slugs are auto-generated from name (URL-safe, unique). Input validation via Echo's binder + custom validators.
- **Acceptance criteria:**
  - All CRUD operations work with valid JWT
  - Slug is auto-generated and unique
  - Only `org_admin` can update or delete the org
  - Soft-delete sets status to `deactivated` (data preserved)
  - Input validation rejects invalid payloads with 422 status
  - sqlc-generated code compiles and queries work
  - Unit tests for service layer, integration tests for API endpoints
- **Labels:** `backend`

---

#### Task 2.3: Workspace CRUD API

- **Milestone:** Core API + Auth
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 2.2
- **Description:** Implement Workspace CRUD endpoints:
  - `POST /api/v1/orgs/:org_id/workspaces` -- create workspace (creator becomes owner)
  - `GET /api/v1/orgs/:org_id/workspaces` -- list workspaces (filtered by user membership)
  - `GET /api/v1/orgs/:org_id/workspaces/:ws_id` -- get workspace details
  - `PUT /api/v1/orgs/:org_id/workspaces/:ws_id` -- update workspace
  - `DELETE /api/v1/orgs/:org_id/workspaces/:ws_id` -- soft-delete workspace
  - `POST /api/v1/orgs/:org_id/workspaces/:ws_id/members` -- add member with role
  - `PUT /api/v1/orgs/:org_id/workspaces/:ws_id/members/:user_id` -- update member role
  - `DELETE /api/v1/orgs/:org_id/workspaces/:ws_id/members/:user_id` -- remove member

  Workspace slug is unique within the org. Users can only see workspaces they are members of (unless org_admin).
- **Acceptance criteria:**
  - Workspace CRUD operates within org tenant boundary (RLS enforced)
  - Slug uniqueness is enforced within org
  - Member management respects role hierarchy (owners > admins > members > viewers)
  - Non-members cannot see or access the workspace
  - org_admin can access all workspaces
  - List endpoint supports pagination (offset + limit)
  - Integration tests cover all endpoints
- **Labels:** `backend`

---

#### Task 2.4: User Management API (Keycloak Sync)

- **Milestone:** Core API + Auth
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.6, 2.1
- **Description:** Implement user management that mirrors Keycloak:
  - `GET /api/v1/orgs/:org_id/users` -- list users in org
  - `GET /api/v1/orgs/:org_id/users/:user_id` -- get user details
  - `PUT /api/v1/orgs/:org_id/users/:user_id` -- update user (display_name, status)
  - `DELETE /api/v1/orgs/:org_id/users/:user_id` -- disable user (GDPR: cascade delete option)
  - `GET /api/v1/me` -- get current authenticated user's profile + org context
  - `POST /api/v1/internal/keycloak-webhook` -- internal endpoint for reavencloak SPI events (user created, updated, deleted, login). Not exposed externally (Traefik blocks `/api/v1/internal/*`).

  User records are created/updated via the Keycloak webhook. The Go API never creates users directly -- Keycloak is the source of truth for authentication. The `users` table is a read-optimized mirror.
- **Acceptance criteria:**
  - `/api/v1/me` returns current user's profile with org context
  - Keycloak webhook creates/updates user records in PostgreSQL
  - User list is filtered by org (RLS)
  - User deletion cascades or soft-deletes based on GDPR flag
  - Webhook endpoint is not accessible from external network
  - Integration tests cover user lifecycle (create via webhook, read, update, delete)
- **Labels:** `backend`, `auth`

---

#### Task 2.5: Knowledge Base CRUD API

- **Milestone:** Core API + Auth
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 2.3
- **Description:** Implement Knowledge Base CRUD endpoints:
  - `POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases` -- create KB
  - `GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases` -- list KBs in workspace
  - `GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id` -- get KB details (include document count, source count, chunk count, processing status summary)
  - `PUT /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id` -- update KB (name, description, settings)
  - `DELETE /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id` -- archive KB (set status to `archived`, keep data for recovery)

  KB settings JSONB includes: chunk_size, chunk_overlap, embedding_model, embedding_dimensions. Slug is unique within workspace.
- **Acceptance criteria:**
  - KB CRUD works within workspace scope
  - KB detail endpoint includes aggregated stats (doc count, source count, chunk count)
  - Slug uniqueness is enforced within workspace
  - Archiving preserves data but hides KB from default list
  - Settings JSONB validates known keys
  - Only workspace members with `member` role or above can create KBs
  - Only workspace `admin`/`owner` can delete KBs
  - Integration tests cover all endpoints
- **Labels:** `backend`

---

#### Task 2.6: Role-Based Access Control Middleware

- **Milestone:** Core API + Auth
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 2.1
- **Description:** Implement RBAC middleware as a reusable Echo middleware that enforces the four-layer access model:
  1. **Org-level:** `org_admin` has full access to all resources within the org
  2. **Workspace-level:** `owner` > `admin` > `member` > `viewer`
  3. **Resource-level:** certain actions require minimum roles (e.g., delete KB requires `admin`)

  Create a middleware factory: `RequireRole(minRole string)` that checks the user's role for the current workspace (extracted from JWT claims or looked up from `workspace_members`). Create convenience wrappers: `RequireOrgAdmin()`, `RequireWorkspaceAdmin()`, `RequireWorkspaceMember()`, `RequireWorkspaceViewer()`.

  Permission matrix:
  | Action | Minimum Role |
  |--------|-------------|
  | View workspace | `viewer` |
  | Upload document | `member` |
  | Create KB | `member` |
  | Manage KB settings | `admin` |
  | Delete KB | `admin` |
  | Manage members | `admin` |
  | Delete workspace | `owner` |
  | Org settings | `org_admin` |
- **Acceptance criteria:**
  - Middleware correctly blocks unauthorized access with 403
  - `org_admin` bypasses workspace role checks
  - Role hierarchy is respected (admin can do everything member can)
  - Middleware is composable (can combine with other middleware)
  - Unit tests cover all role combinations
  - Error responses include which role is required
- **Labels:** `backend`, `auth`, `security`

---

#### Task 2.7: PostgreSQL RLS Policies

- **Milestone:** Core API + Auth
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.5, 2.1
- **Description:** Implement comprehensive Row-Level Security as the defense-in-depth layer:
  1. Create a dedicated PostgreSQL role `raven_app` (used by Go API connections) and `raven_admin` (for migrations and admin operations)
  2. Enable RLS on ALL tenant-scoped tables: `workspaces`, `users`, `workspace_members`, `knowledge_bases`, `documents`, `sources`, `chunks`, `embeddings`, `llm_provider_configs`, `chat_sessions`, `chat_messages`, `api_keys`
  3. For each table, create policies:
     - `tenant_isolation` policy: `USING (org_id = current_setting('app.current_org_id')::uuid)` for SELECT, INSERT, UPDATE, DELETE
     - `admin_bypass` policy: `FOR ALL TO raven_admin USING (true)`
  4. In the Go API database middleware, execute `SET LOCAL app.current_org_id = '<uuid>'` within each transaction (using `SET LOCAL` so it's transaction-scoped, not session-scoped)
  5. Write integration tests that verify cross-tenant data is inaccessible
- **Acceptance criteria:**
  - Queries from `raven_app` role only return rows matching the current org_id
  - INSERT with wrong org_id is rejected by RLS
  - `raven_admin` role can access all data (for migrations, admin scripts)
  - Cross-tenant access test: org A cannot see org B's data (integration test)
  - `SET LOCAL` is used (not `SET`) to prevent connection pool contamination
  - Migration to enable RLS is applied cleanly
- **Labels:** `backend`, `security`

---

#### Task 2.8: API Versioning + CORS + Security Headers

- **Milestone:** Core API + Auth
- **Priority:** P1 (important)
- **Estimate:** 1 day
- **Dependencies:** 1.1
- **Description:** Configure API infrastructure:
  1. **API versioning:** All routes under `/api/v1/` group in Echo. Document versioning policy (v1 supported for minimum 12 months after v2 release). Add `Sunset` header support for future deprecation.
  2. **CORS:** Echo CORS middleware configured with:
     - Default allowed origins: admin dashboard URL
     - Per-API-key allowed origins (from `api_keys.allowed_domains`)
     - Allowed methods: GET, POST, PUT, DELETE, OPTIONS
     - Allowed headers: Authorization, Content-Type, X-API-Key
     - Max age: 3600s
  3. **Security headers** via Traefik middleware or Echo middleware:
     - `Strict-Transport-Security: max-age=31536000; includeSubDomains`
     - `X-Content-Type-Options: nosniff`
     - `X-Frame-Options: DENY`
     - `Content-Security-Policy` (restrictive default)
     - `Referrer-Policy: strict-origin-when-cross-origin`
- **Acceptance criteria:**
  - All API routes are under `/api/v1/`
  - CORS preflight requests succeed for allowed origins
  - CORS requests from disallowed origins are rejected
  - All security headers are present in responses
  - Headers can be verified with `curl -I`
- **Labels:** `backend`, `security`

---

#### Task 2.9: Rate Limiting Middleware (Valkey Sliding Window)

- **Milestone:** Core API + Auth
- **Priority:** P1 (important)
- **Estimate:** 2 days
- **Dependencies:** 1.1, 1.4
- **Description:** Implement rate limiting using Valkey sliding window counters:
  1. **Per-API-key rate limiting:** Each API key has a `rate_limit` (requests per minute). Enforce using Valkey sorted sets with sliding window algorithm. Key pattern: `raven:rl:apikey:{key_hash}`.
  2. **Per-user rate limiting:** Authenticated users have default rate limits based on their org's plan. Key pattern: `raven:rl:user:{user_id}`.
  3. **Per-org rate limiting:** Global rate limit per organization. Key pattern: `raven:rl:org:{org_id}`.
  4. **Response headers:** Include `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` in all responses.
  5. **429 responses:** Return `Retry-After` header with seconds until reset.
  6. Also configure Traefik's built-in rate limiter as a global per-IP defense against abuse.
- **Acceptance criteria:**
  - Requests exceeding rate limit receive 429 status
  - Rate limit headers are present in all API responses
  - Sliding window is accurate (not a leaky bucket -- tests verify window boundaries)
  - Different rate limits per API key work correctly
  - Valkey failure degrades gracefully (allow requests, log warning)
  - Integration tests verify rate limiting behavior
- **Labels:** `backend`, `security`

---

#### Task 2.10: Swagger/OpenAPI Generation (swaggo)

- **Milestone:** Core API + Auth
- **Priority:** P2 (nice-to-have)
- **Estimate:** 1 day
- **Dependencies:** 2.2, 2.3, 2.5
- **Description:** Set up automatic OpenAPI spec generation from Go code:
  1. Install `swaggo/swag` and annotate all existing handler functions with Swagger comments (`@Summary`, `@Description`, `@Tags`, `@Accept`, `@Produce`, `@Param`, `@Success`, `@Failure`, `@Router`)
  2. Add `swag init` to the build process (Makefile target: `make swagger`)
  3. Serve the generated spec via Scalar UI at `/api/docs`
  4. Include authentication schemas (Bearer JWT + API Key)
  5. Group endpoints by tags: Organizations, Workspaces, Users, Knowledge Bases
- **Acceptance criteria:**
  - `/api/docs` serves an interactive API documentation page
  - All existing endpoints are documented with request/response schemas
  - `make swagger` regenerates the spec from code annotations
  - Authentication schemes are documented (JWT + API Key)
  - CI runs `swag init` and fails if spec is outdated
- **Labels:** `backend`, `docs`

---

## Milestone 3: Ingestion Pipeline (Week 5-7)

> Build the document and URL processing pipeline: upload, parse, chunk, embed, index. After this milestone, users can upload documents and scrape URLs, with the content becoming searchable via hybrid retrieval.

---

#### Task 3.1: File Upload Endpoint + SeaweedFS Storage

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.4, 2.5
- **Description:** Implement file upload:
  - `POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents` -- multipart file upload
  - Validate file type (PDF, DOCX, XLSX, PPTX, Markdown, PNG, JPG, TIFF)
  - Validate file size (configurable max, default 50 MB)
  - Compute SHA-256 hash for dedup checking
  - Stream file to SeaweedFS via S3-compatible API (using AWS SDK for Go with custom endpoint)
  - Create document record in PostgreSQL with status `queued`
  - Return 202 Accepted with `{doc_id, status: "queued"}`

  Implement a storage abstraction interface to support both SeaweedFS and local filesystem:
  ```go
  type ObjectStorage interface {
      Put(ctx context.Context, path string, reader io.Reader, size int64, contentType string) error
      Get(ctx context.Context, path string) (io.ReadCloser, error)
      Delete(ctx context.Context, path string) error
  }
  ```
- **Acceptance criteria:**
  - File upload stores file in SeaweedFS and creates DB record
  - Duplicate files (same SHA-256) are detected and rejected with 409
  - Invalid file types are rejected with 422
  - File size limit is enforced
  - Storage abstraction allows swapping SeaweedFS for local filesystem via config
  - Large files (50 MB) upload without timeout
  - Integration test uploads a PDF and verifies storage + DB record
- **Labels:** `backend`, `infra`

---

#### Task 3.2: Document CRUD API (Metadata, Status Tracking)

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 1.5 days
- **Dependencies:** 3.1
- **Description:** Implement Document management endpoints:
  - `GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents` -- list documents (with status filter, pagination)
  - `GET /api/v1/.../documents/:doc_id` -- get document details (include processing status, chunk count, error message if failed)
  - `PUT /api/v1/.../documents/:doc_id` -- update metadata (title, custom metadata JSONB)
  - `DELETE /api/v1/.../documents/:doc_id` -- delete document + associated chunks + embeddings + SeaweedFS file
  - `POST /api/v1/.../documents/:doc_id/reprocess` -- trigger reprocessing (sets status to `reprocessing`, clears old chunks, re-enqueues)

  Status tracking: return processing status with timestamps for each stage transition (from `processing_events` table).
- **Acceptance criteria:**
  - Document list supports filtering by status and pagination
  - Document detail includes full processing timeline
  - Delete cascades to chunks, embeddings, and SeaweedFS
  - Reprocess clears old data and re-enqueues
  - Only workspace `member` or above can upload; `admin` or above can delete
  - Integration tests cover full CRUD lifecycle
- **Labels:** `backend`

---

#### Task 3.3: Source (URL) CRUD API

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 1.5 days
- **Dependencies:** 2.5
- **Description:** Implement Source management endpoints:
  - `POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/sources` -- add URL/sitemap/RSS source
  - `GET /api/v1/.../sources` -- list sources (with status filter, pagination)
  - `GET /api/v1/.../sources/:source_id` -- get source details
  - `PUT /api/v1/.../sources/:source_id` -- update source (crawl_depth, crawl_frequency, metadata)
  - `DELETE /api/v1/.../sources/:source_id` -- delete source + associated chunks + embeddings
  - `POST /api/v1/.../sources/:source_id/recrawl` -- trigger re-crawl

  Validate URLs (must be reachable, HTTP/HTTPS only). Source types: `url` (single page), `sitemap` (discover URLs from sitemap.xml), `rss_feed` (discover URLs from RSS). Store crawl configuration: depth, frequency (manual/daily/weekly/monthly), page limit.
- **Acceptance criteria:**
  - Source CRUD works with URL validation
  - All three source types (url, sitemap, rss_feed) are accepted
  - Crawl frequency configuration is persisted
  - Delete cascades to all derived chunks and embeddings
  - Re-crawl re-enqueues the source for processing
  - Integration tests cover all source types
- **Labels:** `backend`

---

#### Task 3.4: Asynq Job Queue Setup (Valkey-Backed)

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.4
- **Description:** Set up the Asynq job queue for async document processing:
  1. **Go API (producer):** Create an Asynq client that enqueues jobs to Valkey:
     - `raven:jobs:document_process` -- task payload: `{document_id, org_id, kb_id, storage_path, file_type}`
     - `raven:jobs:web_scrape` -- task payload: `{source_id, org_id, kb_id, url, source_type, crawl_depth}`
     - `raven:jobs:reindex` -- task payload: `{kb_id, org_id, new_model}`
  2. **Python Worker (consumer):** Implement Asynq-compatible consumer using Valkey client (Python). Since Asynq is Go-native, the Python worker will consume jobs via direct Valkey BRPOP on the Asynq queue keys, following Asynq's serialization format. Alternatively, use a thin Go sidecar that dequeues and forwards to gRPC.
  3. **Job configuration:** retry policy (3 retries, exponential backoff), visibility timeout (300s), max TTL (30 minutes), dead-letter queue for failed jobs.
  4. **Monitoring:** Expose job queue metrics (pending, active, failed counts) via the Go API health endpoint.
- **Acceptance criteria:**
  - Go API enqueues jobs that appear in Valkey
  - Python worker dequeues and processes jobs
  - Failed jobs retry with exponential backoff
  - After max retries, jobs move to dead-letter queue
  - Job queue metrics are exposed in health endpoint
  - Integration test: enqueue a job, verify Python worker processes it
- **Labels:** `backend`, `ai-worker`, `infra`

---

#### Task 3.5: Python Worker -- LiteParse Integration

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.2, 3.4
- **Description:** Integrate LiteParse document parsing into the Python worker:
  1. Install LiteParse (Apache 2.0) as a Node.js subprocess dependency (the Python worker invokes it via `subprocess.run`)
  2. Implement `parser.py`:
     - Download file from SeaweedFS (or receive file path for local storage)
     - Call LiteParse CLI: `liteparse --input <file> --format json`
     - Parse JSON output: extract text content, structural elements (headings, tables, lists), page numbers, metadata
     - Handle supported formats: PDF, DOCX, XLSX, PPTX, Markdown, images (OCR via Tesseract.js)
     - Update document status: `queued` -> `parsing`
     - On success: pass extracted text to chunking stage
     - On failure: set status to `failed` with error message, log to `processing_events`
  3. Include Node.js runtime in the Python worker Docker image (multi-stage build)
- **Acceptance criteria:**
  - PDF documents are parsed to structured text with page numbers
  - DOCX documents are parsed with heading hierarchy preserved
  - Images are OCR'd to text
  - LiteParse errors are caught and document status is set to `failed`
  - Processing events are logged for each status transition
  - Unit tests with sample documents (PDF, DOCX, image)
- **Labels:** `ai-worker`

---

#### Task 3.6: Python Worker -- Crawl4AI Web Scraping

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.2, 3.4
- **Description:** Integrate Crawl4AI (Apache 2.0) for web scraping:
  1. Install Crawl4AI with Playwright browser dependency
  2. Implement `scraper.py`:
     - For `url` type: scrape single URL with Crawl4AI async API
     - For `sitemap` type: parse sitemap.xml, discover URLs, scrape each (respecting crawl_depth and page_limit)
     - For `rss_feed` type: parse RSS feed, extract URLs, scrape each
     - Crawl4AI configuration: content filtering (remove navbars/footers/ads), extract markdown, configurable depth/page limits
     - Update source status: `queued` -> `crawling` -> `parsing`
     - Handle errors: timeout, 4xx/5xx responses, JavaScript rendering failures
  3. Rate limiting: respect `robots.txt`, add configurable delay between requests (default 1s)
  4. Include Playwright browser in Docker image (headless Chromium)
- **Acceptance criteria:**
  - Single URL scraping extracts clean markdown content
  - Sitemap scraping discovers and processes multiple URLs
  - RSS feed scraping discovers and processes articles
  - JavaScript-rendered pages are handled (Playwright)
  - Content filtering removes navigation/footer/ads
  - `robots.txt` is respected
  - Errors are handled gracefully with status updates
  - Integration test with a live URL
- **Labels:** `ai-worker`

---

#### Task 3.7: Python Worker -- Text Chunking

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 3.5, 3.6
- **Description:** Implement document-structure-aware chunking in the Python worker:
  1. Implement `chunker.py` following the spec:
     - **Target:** ~512 tokens per chunk
     - **Overlap:** 50 tokens between consecutive chunks
     - **Split hierarchy:** LiteParse structural elements (headings, tables, lists) -> paragraph -> sentence -> word boundaries
     - **Tables:** Each table becomes its own chunk with caption/heading as prefix
     - **Metadata per chunk:** document_id/source_id, org_id, knowledge_base_id, chunk_index, token_count, page_number, heading (nearest section title), chunk_type (`text`, `table`, `image_caption`, `code`), character offsets
  2. Token counting: use `tiktoken` (MIT) for accurate token counts
  3. Update status: `parsing` -> `chunking`
  4. Store chunks in PostgreSQL `chunks` table
  5. Log chunk count and processing time to `processing_events`
- **Acceptance criteria:**
  - Chunks are approximately 512 tokens with 50-token overlap
  - Structural boundaries are respected (no mid-sentence splits)
  - Tables are chunked independently with heading context
  - Metadata is correctly assigned to each chunk
  - Token counts are accurate (verified with tiktoken)
  - A 100-page PDF produces correctly ordered, overlapping chunks
  - Unit tests with various document structures
- **Labels:** `ai-worker`

---

#### Task 3.8: Python Worker -- Multi-Provider Embedding (BYOK Adapter Interface)

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 3 days
- **Dependencies:** 3.7
- **Description:** Implement the BYOK embedding provider system:
  1. Define the `EmbeddingProvider` protocol:
     ```python
     class EmbeddingProvider(Protocol):
         def embed(self, texts: list[str]) -> list[list[float]]: ...
         @property
         def model_name(self) -> str: ...
         @property
         def dimensions(self) -> int: ...
     ```
  2. Implement providers:
     - `OpenAIEmbeddingProvider` -- text-embedding-3-small (1536d), text-embedding-3-large (3072d)
     - `CohereEmbeddingProvider` -- embed-english-v3.0 (1024d)
     - `GoogleEmbeddingProvider` -- text-embedding-005
     - `CustomEmbeddingProvider` -- configurable base_url for self-hosted models
  3. Provider factory that reads org's `llm_provider_configs` and returns the appropriate provider instance
  4. Decryption: receive decrypted API key from Go API (passed as job metadata, encrypted in transit via gRPC TLS) or read from Valkey cache (short TTL)
  5. Batch embedding: split texts into batches appropriate for each provider's rate limits
  6. Store embeddings in PostgreSQL `embeddings` table with model_name and dimensions
  7. Update status: `chunking` -> `embedding` -> `ready`
- **Acceptance criteria:**
  - All four providers produce embeddings of correct dimensions
  - Provider selection is determined by org's configuration
  - Batch processing handles rate limits gracefully
  - API key is never logged or persisted outside encrypted storage
  - Embedding API errors (rate limit, auth) are handled per spec (retry vs fail)
  - Different KBs in same org can use different embedding models
  - Unit tests with mocked providers, integration test with at least one real provider
- **Labels:** `ai-worker`

---

#### Task 3.9: pgvector Schema + HNSW Index Setup

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 1 day
- **Dependencies:** 1.5, 3.8
- **Description:** Create migrations for the vector storage layer:
  1. Create `embeddings` table migration per design spec (id, org_id, chunk_id FK, embedding vector(N), model_name, model_version, dimensions, created_at, UNIQUE(chunk_id, model_name))
  2. Create HNSW index: `CREATE INDEX ON embeddings USING hnsw (embedding vector_cosine_ops) WITH (m=16, ef_construction=64);`
  3. Enable RLS on embeddings table
  4. Create a helper function for cosine similarity search:
     ```sql
     -- Parameterized function for vector search within tenant + KB scope
     ```
  5. Test index performance with sample data (1000+ embeddings)
- **Acceptance criteria:**
  - Embeddings table created with correct schema
  - HNSW index is created and used by query planner (verify with EXPLAIN)
  - Vector cosine similarity search returns correct nearest neighbors
  - RLS enforces tenant isolation on embeddings
  - Index creation handles variable dimensions (1536, 1024, 3072)
  - Performance test: 1000-embedding search completes in <100ms
- **Labels:** `backend`, `infra`

---

#### Task 3.10: ParadeDB / tsvector BM25 Index Setup

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.5
- **Description:** Implement the full-text search layer with swappable backends:
  1. Create the Go `FullTextSearcher` interface:
     ```go
     type FullTextSearcher interface {
         IndexChunk(ctx context.Context, chunk Chunk) error
         Search(ctx context.Context, query string, orgID uuid.UUID, kbIDs []uuid.UUID, limit int) ([]SearchResult, error)
         DeleteByDocument(ctx context.Context, documentID uuid.UUID) error
     }
     ```
  2. **TsvectorSearcher** (default, no license risk):
     - Add `content_tsvector tsvector` generated column to `chunks` table
     - Create GIN index: `CREATE INDEX ON chunks USING gin (content_tsvector);`
     - Search using `ts_rank` with `plainto_tsquery`
  3. **ParadeDBSearcher** (optional, AGPL):
     - Use ParadeDB `@@@` BM25 operator on `chunks.content`
     - Create BM25 index via ParadeDB
  4. Configuration: select implementation via environment variable (`RAVEN_FTS_BACKEND=tsvector|paradedb`)
  5. Migration adds tsvector column and GIN index (always); ParadeDB index is conditional
- **Acceptance criteria:**
  - `TsvectorSearcher` returns ranked results using `ts_rank`
  - `ParadeDBSearcher` returns ranked results using BM25 (if ParadeDB is available)
  - Backend is selected via config, defaulting to tsvector
  - Both implementations satisfy the same interface
  - Search results include rank scores
  - GIN index is used by query planner (verify with EXPLAIN)
  - Unit tests for both implementations
- **Labels:** `backend`, `infra`

---

#### Task 3.11: Document Processing State Machine

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 1.5 days
- **Dependencies:** 3.4, 3.5, 3.6, 3.7, 3.8
- **Description:** Implement the document processing state machine that orchestrates the full pipeline:
  1. Define valid state transitions per spec:
     - `queued` -> `crawling` (sources only) | `parsing`
     - `crawling` -> `parsing` | `failed`
     - `parsing` -> `chunking` | `failed`
     - `chunking` -> `embedding` | `failed`
     - `embedding` -> `ready` | `failed`
     - `ready` -> `reprocessing`
     - `failed` -> `reprocessing`
     - `reprocessing` -> `parsing` | `crawling`
  2. Each transition is logged to `processing_events` table with: from_status, to_status, timestamp, duration_ms, error_message
  3. Invalid transitions are rejected (e.g., cannot go from `queued` directly to `ready`)
  4. The Python worker orchestrator function calls parse -> chunk -> embed in sequence, updating status at each step
  5. Error handling per spec: corrupt file (no retry), scrape timeout (retry 3x), embedding rate limit (retry 5x), auth error (fail immediately)
- **Acceptance criteria:**
  - State machine enforces valid transitions only
  - Invalid transitions raise errors
  - All transitions are logged to `processing_events`
  - Duration is tracked for each state
  - Error handling follows the spec (retry counts, backoff strategy)
  - End-to-end test: upload document -> queued -> parsing -> chunking -> embedding -> ready
  - Failure test: corrupt file -> queued -> parsing -> failed
- **Labels:** `backend`, `ai-worker`

---

#### Task 3.12: LLM Provider Config API (BYOK Encrypted Key Storage)

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 2.2
- **Description:** Implement LLM provider configuration management:
  - `POST /api/v1/orgs/:org_id/llm-providers` -- add provider config (provider type, API key, base_url, config JSONB)
  - `GET /api/v1/orgs/:org_id/llm-providers` -- list providers (API key is NEVER returned, only `key_hint` showing last 4 chars)
  - `PUT /api/v1/orgs/:org_id/llm-providers/:provider_id` -- update config (rotating API key)
  - `DELETE /api/v1/orgs/:org_id/llm-providers/:provider_id` -- revoke provider
  - `POST /api/v1/orgs/:org_id/llm-providers/:provider_id/test` -- test connectivity (make a minimal API call to verify the key works)

  **Encryption:**
  - AES-256-GCM encryption of API keys before storage
  - Master key loaded from environment variable (or secrets manager in production)
  - Per-org Data Encryption Keys (DEKs) derived from master key + org_id
  - Initialization vector (IV) stored alongside ciphertext
  - Key hint (last 4 chars) stored in plaintext for UI display

  Supported providers: `openai`, `anthropic`, `cohere`, `google`, `azure_openai`, `custom`
- **Acceptance criteria:**
  - API keys are encrypted at rest in PostgreSQL (verified by inspecting raw DB)
  - API keys are NEVER returned in API responses
  - Key hint (last 4 chars) is displayed in list endpoint
  - Test endpoint validates key by making a real API call
  - Key rotation (update) encrypts the new key and invalidates old one
  - Only `org_admin` or workspace `admin` can manage providers
  - Unit tests for encryption/decryption, integration tests for CRUD
- **Labels:** `backend`, `security`

---

#### Task 3.13: Hybrid Search Implementation (Vector + BM25 + RRF Fusion)

- **Milestone:** Ingestion Pipeline
- **Priority:** P0 (blocker)
- **Estimate:** 3 days
- **Dependencies:** 3.9, 3.10, 3.8
- **Description:** Implement the hybrid retrieval pipeline in the Python worker:
  1. **Semantic search:** Query embedding via org's embedding provider -> pgvector `<=>` cosine similarity search within org + KB scope. Retrieve top 30 results.
  2. **Keyword search:** Query text -> full-text search via `FullTextSearcher` interface (tsvector or ParadeDB). Retrieve top 30 results.
  3. **RRF fusion:** Reciprocal Rank Fusion: `score = SUM(1 / (k + rank))` with k=60. Merge semantic and keyword results.
  4. **Reranking:** Top 20-30 RRF results passed to reranker:
     - Cohere Rerank v3 (API, BYOK)
     - Self-hosted BGE reranker (fallback)
     - No reranking (budget option)
  5. Return top 5-8 results with: chunk content, source document/URL, page number, relevance score, chunk metadata

  Implement as a gRPC service method in the Python worker, callable from Go API.
- **Acceptance criteria:**
  - Hybrid search combines vector and BM25 results via RRF
  - Results are better than either vector-only or keyword-only (qualitative test)
  - Reranking improves result ordering (with Cohere or BGE)
  - Search is scoped to org + specified KB(s)
  - RRF fusion produces normalized scores
  - End-to-end test: ingest document, then query returns relevant chunks
  - Performance: search over 10K chunks completes in <500ms
- **Labels:** `ai-worker`, `backend`

---

## Milestone 4: Chatbot MVP (Week 8-10)

> Build the end-user chatbot experience: RAG-powered chat with SSE streaming, conversation history, embeddable web component, and admin configurator.

---

#### Task 4.1: RAG Query gRPC Service (Python Worker)

- **Milestone:** Chatbot MVP
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 3.13
- **Description:** Implement the `QueryRAG` gRPC server-streaming RPC in the Python worker:
  1. Accept `RAGRequest` (query text, org_id, kb_ids, filters, conversation history)
  2. Execute hybrid search pipeline (Task 3.13)
  3. Construct LLM prompt with:
     - System prompt (configurable per KB)
     - Retrieved context chunks with source attribution
     - Conversation history (last N turns, configurable)
     - User query
  4. Stream LLM completion tokens via the org's configured provider (BYOK):
     - Anthropic Claude (Sonnet for quality, Haiku for speed)
     - OpenAI GPT-4o / GPT-4o-mini
     - Custom endpoints
  5. Each `RAGChunk` response includes: token text, source citations (when referenced), is_final flag
  6. Implement the `LLMProvider` protocol for streaming completions:
     ```python
     class LLMProvider(Protocol):
         async def stream_completion(self, messages: list[dict], context: str) -> AsyncIterator[str]: ...
     ```
- **Acceptance criteria:**
  - gRPC streaming returns tokens in real-time
  - Source citations are included when the LLM references retrieved chunks
  - Conversation history is included in the prompt
  - Multiple LLM providers work (test with at least 2)
  - Token budget is respected (context window management)
  - Error handling: LLM API failure returns error chunk, not crash
  - Latency: first token in <1s for simple queries
- **Labels:** `ai-worker`

---

#### Task 4.2: Chat API Endpoint with SSE Streaming (Go)

- **Milestone:** Chatbot MVP
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 4.1
- **Description:** Implement the chat endpoint in the Go API:
  - `POST /api/v1/chat/:kb_id/completions` -- SSE streaming response
  - Request body: `{query: string, conversation_id?: string, metadata?: object}`
  - Response: `Content-Type: text/event-stream` with events:
    - `event: token\ndata: {"text": "..."}\n\n`
    - `event: source\ndata: {"title": "...", "url": "...", "page": N}\n\n`
    - `event: error\ndata: {"message": "..."}\n\n`
    - `event: done\ndata: {"conversation_id": "...", "message_id": "..."}\n\n`
  - Authentication: API key (`X-API-Key` header) or JWT Bearer token
  - Flow: validate auth -> load conversation history -> gRPC streaming call to Python worker -> forward tokens as SSE events -> save message to chat_messages table
  - Handle client disconnect: cancel gRPC stream, save partial response
- **Acceptance criteria:**
  - SSE streaming works in browser (EventSource API)
  - Tokens arrive in real-time (not buffered)
  - Conversation ID is returned on first message, reusable for follow-ups
  - Source citations are included as separate SSE events
  - Client disconnect cancels the gRPC stream
  - Both API key and JWT auth work
  - Integration test with SSE client verifies streaming
- **Labels:** `backend`

---

#### Task 4.3: Conversation History Management

- **Milestone:** Chatbot MVP
- **Priority:** P0 (blocker)
- **Estimate:** 1.5 days
- **Dependencies:** 4.2
- **Description:** Implement conversation session management:
  1. **Chat sessions table** (migration): id, org_id, knowledge_base_id, user_id (nullable for anonymous), session_token (for anonymous users), metadata JSONB, created_at, expires_at
  2. **Chat messages table** (migration): id, session_id FK, org_id, role (user/assistant/system), content, token_count, chunk_ids (UUID array of retrieved chunks), model_name, latency_ms, created_at
  3. **Session management:**
     - Anonymous sessions: 24-hour TTL, identified by session_token
     - Authenticated sessions: persistent, linked to user_id
     - New conversation: auto-create session on first message
     - Resume conversation: load last N turns (configurable, default 10) as context
     - Sliding window: track token budget, trim oldest messages when context exceeds limit
  4. **API endpoints:**
     - `GET /api/v1/chat/:kb_id/conversations` -- list conversations (for authenticated users)
     - `GET /api/v1/chat/:kb_id/conversations/:conv_id/messages` -- get conversation messages
- **Acceptance criteria:**
  - Conversations persist across multiple messages
  - Anonymous sessions expire after 24 hours
  - Authenticated sessions are persistent
  - Conversation history is included in RAG queries
  - Sliding window correctly trims old messages
  - Token count is tracked per message
  - List conversations endpoint supports pagination
- **Labels:** `backend`

---

#### Task 4.4: API Key Generation + Auth Middleware

- **Milestone:** Chatbot MVP
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 2.1
- **Description:** Implement API key management for the embeddable chatbot widget:
  - `POST /api/v1/orgs/:org_id/api-keys` -- generate API key (scoped to KB, with domain allowlist and rate limit)
  - `GET /api/v1/orgs/:org_id/api-keys` -- list API keys (show prefix + name, never full key)
  - `DELETE /api/v1/orgs/:org_id/api-keys/:key_id` -- revoke API key
  - `PUT /api/v1/orgs/:org_id/api-keys/:key_id` -- update (name, allowed_domains, rate_limit)

  **Key format:** `rk_live_<random32chars>` for production, `rk_test_<random32chars>` for testing
  **Storage:** SHA-256 hash stored in PostgreSQL, plaintext shown ONCE at creation
  **Auth middleware:**
  - Extract `X-API-Key` header
  - Hash and look up in `api_keys` table
  - Verify `Origin`/`Referer` against `allowed_domains`
  - Check status is `active` and not expired
  - Apply per-key rate limit
  - Set org_id and kb_id scope in request context
- **Acceptance criteria:**
  - API key is shown only once at creation (not retrievable later)
  - Hashed keys in DB cannot be reversed to plaintext
  - Domain allowlist is enforced (wrong domain gets 403)
  - Rate limiting works per-key
  - Revoked keys are immediately rejected
  - API key auth grants access only to chat endpoints for the scoped KB
  - Integration tests cover generation, authentication, revocation
- **Labels:** `backend`, `auth`, `security`

---

#### Task 4.5: `<raven-chat>` Web Component (Vue.js, Shadow DOM)

- **Milestone:** Chatbot MVP
- **Priority:** P0 (blocker)
- **Estimate:** 4 days
- **Dependencies:** 4.2
- **Description:** Build the embeddable chatbot web component:
  1. **Architecture:** Vue.js custom element compiled as a web component, using Shadow DOM for style isolation. Distributed as a single JS file (`chat.js`) loadable via `<script>` tag.
  2. **Usage:**
     ```html
     <script src="https://cdn.raven.dev/chat.js"></script>
     <raven-chat kb="kb_abc123" api-key="rk_live_..."></raven-chat>
     ```
  3. **Attributes:**
     - `kb` -- knowledge base ID
     - `api-key` -- publishable API key
     - `theme` -- `light` | `dark` | `auto` (system preference)
     - `position` -- `bottom-right` | `bottom-left`
     - `title` -- chatbot window title
     - `welcome-message` -- initial greeting
     - `avatar` -- URL for bot avatar
     - `primary-color` -- hex color for theming
  4. **Features:**
     - Floating action button (FAB) that opens/closes chat panel
     - Message input with send button and Enter key support
     - Streaming response display (tokens appear as they arrive via SSE)
     - Source citations displayed as clickable links
     - Conversation history within session
     - Typing indicator while waiting for response
     - Mobile-responsive (adapts to viewport)
     - Keyboard accessible (Tab, Enter, Escape)
  5. **Build:** Vite library mode outputting a single IIFE bundle
- **Acceptance criteria:**
  - `<raven-chat>` renders correctly when embedded in any HTML page
  - Shadow DOM isolates styles (host page CSS does not affect widget)
  - SSE streaming displays tokens in real-time
  - Source citations are displayed and clickable
  - Theme customization works (light/dark/custom color)
  - Mobile-responsive layout
  - Bundle size < 100 KB gzipped
  - Works in Chrome, Firefox, Safari, Edge (latest 2 versions)
  - Accessibility: keyboard navigable, screen reader labels
- **Labels:** `frontend`

---

#### Task 4.6: Chatbot Configurator (Admin Dashboard)

- **Milestone:** Chatbot MVP
- **Priority:** P1 (important)
- **Estimate:** 2 days
- **Dependencies:** 4.5, 5.1
- **Description:** Build the chatbot configuration page in the admin dashboard:
  1. **Configuration form:**
     - Theme (light/dark/auto)
     - Primary color picker
     - Bot name and avatar upload
     - Welcome message text
     - System prompt (instructions for the LLM)
     - Position (bottom-right/bottom-left)
     - Max conversation turns
     - Response language preference
  2. **Live preview:** Side-by-side preview of the `<raven-chat>` widget with current settings
  3. **Embed code generator:** Copy-paste snippet with the configured attributes
  4. **Per-KB configuration:** Settings are stored in KB's `settings` JSONB

  Store chatbot config in the knowledge_base settings JSONB. Go API endpoint to read/update chatbot config.
- **Acceptance criteria:**
  - All configuration options are editable in the form
  - Live preview updates in real-time as settings change
  - Embed code snippet is generated correctly
  - Configuration persists after save
  - Preview matches what the actual embedded widget looks like
- **Labels:** `frontend`, `backend`

---

#### Task 4.7: Test Sandbox in Admin Dashboard

- **Milestone:** Chatbot MVP
- **Priority:** P1 (important)
- **Estimate:** 1.5 days
- **Dependencies:** 4.5, 4.6
- **Description:** Build a test sandbox page in the admin dashboard where admins can interact with the chatbot using a test API key:
  1. Full-size chat interface (not the small widget, but a full-page chat view)
  2. Uses a test API key (`rk_test_...`) auto-generated per KB
  3. Shows debug information alongside responses:
     - Retrieved chunks with relevance scores
     - Source documents referenced
     - Token count (prompt + completion)
     - Latency breakdown (retrieval, LLM, total)
     - Model used
  4. Option to clear conversation and start fresh
  5. Option to test with different system prompts without saving
- **Acceptance criteria:**
  - Sandbox chat works with test API key
  - Debug panel shows retrieval details and latency
  - Token counts are displayed
  - Different system prompts can be tested without saving
  - Clear conversation works
  - Sandbox is accessible only to workspace admins
- **Labels:** `frontend`

---

#### Task 4.8: Simple Exact-Match Response Cache

- **Milestone:** Chatbot MVP
- **Priority:** P2 (nice-to-have)
- **Estimate:** 1.5 days
- **Dependencies:** 4.2
- **Description:** Implement a simple response cache as the foundation for Phase 2's semantic cache:
  1. **Cache key:** SHA-256 hash of `(kb_id + normalized_query_text)` -- normalize by lowercasing and trimming whitespace
  2. **Cache storage:** Valkey with configurable TTL (default 1 hour)
  3. **Cache value:** Full response text + source citations + model_name
  4. **Cache lookup:** Before calling the Python worker, check Valkey for exact match
  5. **Cache invalidation:** Invalidate all cache entries for a KB when:
     - New documents are added or reprocessed
     - KB settings change (embedding model, system prompt)
  6. **Cache bypass:** `X-No-Cache: true` header to skip cache
  7. **Metrics:** Track cache hit/miss ratio in health endpoint
- **Acceptance criteria:**
  - Identical queries return cached response instantly
  - Cache miss falls through to full RAG pipeline
  - KB changes invalidate the cache
  - Cache bypass header works
  - Hit/miss metrics are tracked
  - TTL expiration works correctly
  - Cached responses include source citations
- **Labels:** `backend`

---

## Milestone 5: Admin Dashboard (Week 8-10, Parallel with M4)

> Build the full admin dashboard for managing the platform. Runs in parallel with the chatbot MVP.

---

#### Task 5.1: Vue.js App Scaffold with Tailwind Plus

- **Milestone:** Admin Dashboard
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.3
- **Description:** Build the foundational admin dashboard layout using Tailwind Plus components:
  1. **Layout system:**
     - Sidebar navigation (collapsible on mobile)
     - Top bar with user avatar, org switcher, notifications
     - Main content area with breadcrumb navigation
     - Footer with version and support links
  2. **Navigation structure:**
     - Dashboard (overview/analytics)
     - Knowledge Bases (list, detail, documents, sources)
     - Chatbot (configurator, sandbox)
     - Settings (organization, workspace, members, API keys, LLM providers)
  3. **Component library setup:** Configure Tailwind Plus component imports, establish design tokens (colors, spacing, typography). Create reusable composite components: DataTable, StatusBadge, ConfirmDialog, EmptyState, LoadingSpinner.
  4. **API client:** Typed API client using `fetch` with interceptors for JWT auth, error handling, and automatic token refresh.
- **Acceptance criteria:**
  - Dashboard layout renders with sidebar, topbar, and content area
  - Navigation works between all sections
  - Tailwind Plus components are styled correctly
  - API client handles authentication and token refresh
  - Mobile-responsive layout (sidebar collapses to hamburger menu)
  - Dark mode support
- **Labels:** `frontend`

---

#### Task 5.2: Auth Flow (Keycloak OIDC PKCE)

- **Milestone:** Admin Dashboard
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.6, 5.1
- **Description:** Implement the authentication flow in the Vue.js app:
  1. **Login:** Redirect to Keycloak OIDC Authorization Code flow with PKCE
  2. **Callback:** Handle the redirect callback, exchange code for tokens
  3. **Token storage:** Store access token in memory (not localStorage), refresh token in httpOnly cookie or secure memory
  4. **Token refresh:** Automatically refresh access token before expiry using refresh token
  5. **Logout:** Clear tokens, redirect to Keycloak logout endpoint
  6. **Route guards:** Vue Router guards that redirect to login for unauthenticated users
  7. **User context:** Pinia store with current user profile, org_id, roles
  8. Use a lightweight OIDC client library (e.g., `oidc-client-ts`)
- **Acceptance criteria:**
  - Login redirects to Keycloak and back
  - PKCE flow works (no client secret in browser)
  - Token refresh happens automatically before expiry
  - Logout clears all auth state
  - Unauthenticated routes redirect to login
  - User profile and org context are available after login
  - Works across page refreshes (session persists)
- **Labels:** `frontend`, `auth`

---

#### Task 5.3: Organization Management Pages

- **Milestone:** Admin Dashboard
- **Priority:** P1 (important)
- **Estimate:** 2 days
- **Dependencies:** 2.2, 5.1, 5.2
- **Description:** Build organization management pages:
  1. **Org settings page:**
     - Organization name and slug (editable by org_admin)
     - Organization status display
     - Billing plan display (links to billing page in M6)
     - Feature flags / settings JSONB editor
  2. **Member management:**
     - List org members with role, email, last login
     - Invite new member (triggers Keycloak invitation)
     - Change member role
     - Remove member (with confirmation)
  3. **Org switcher** in the top bar (for users who belong to multiple orgs)
- **Acceptance criteria:**
  - Org settings are editable and saved
  - Member list shows all org members with roles
  - Invite flow sends email via Keycloak
  - Role changes are reflected immediately
  - Member removal requires confirmation
  - Org switcher works for multi-org users
  - Only org_admin can access settings and manage members
- **Labels:** `frontend`

---

#### Task 5.4: Workspace Management Pages

- **Milestone:** Admin Dashboard
- **Priority:** P1 (important)
- **Estimate:** 2 days
- **Dependencies:** 2.3, 5.1, 5.2
- **Description:** Build workspace management pages:
  1. **Workspace list:** Card or table view of workspaces user belongs to, with member count and KB count
  2. **Workspace detail:** Name, slug, settings, member list
  3. **Workspace creation:** Form with name (auto-slug), initial settings
  4. **Member management:** Same as org member management but workspace-scoped
  5. **Workspace settings:** LLM provider selection, default chunk settings, custom config JSONB
- **Acceptance criteria:**
  - Workspace list shows only workspaces user is a member of
  - Workspace creation works with auto-generated slug
  - Member management within workspace works (add, change role, remove)
  - Workspace settings are editable and saved
  - Only workspace admin/owner can manage settings and members
  - Member/viewer roles see read-only views
- **Labels:** `frontend`

---

#### Task 5.5: Knowledge Base Management (Documents, Sources, Status)

- **Milestone:** Admin Dashboard
- **Priority:** P0 (blocker)
- **Estimate:** 3 days
- **Dependencies:** 2.5, 3.1, 3.2, 3.3, 5.4
- **Description:** Build the knowledge base management pages -- the core admin experience:
  1. **KB list page:** Table/card view within workspace, showing name, document count, source count, status
  2. **KB detail page:**
     - Overview tab: stats (total docs, sources, chunks, embeddings), processing status summary
     - Documents tab: file upload (drag-and-drop), document list with status badges (queued, parsing, chunking, embedding, ready, failed), reprocess/delete actions
     - Sources tab: add URL/sitemap/RSS form, source list with status, recrawl/delete actions
     - Settings tab: chunk size, overlap, embedding model, system prompt
  3. **Document upload:** Drag-and-drop zone with progress indicator, multi-file upload support
  4. **Real-time status:** Poll for processing status updates (or WebSocket in future)
  5. **Error display:** Failed documents/sources show error message with retry option
- **Acceptance criteria:**
  - KB list shows all KBs in workspace with stats
  - File upload works with drag-and-drop and file picker
  - Multi-file upload with individual progress indicators
  - Document list shows processing status with visual indicators
  - Source list shows crawl status and page count
  - Reprocess and delete actions work
  - Status polling updates the UI without page refresh
  - Failed items show error message with retry button
- **Labels:** `frontend`

---

#### Task 5.6: Analytics Dashboard (Conversation Volume, Top Queries)

- **Milestone:** Admin Dashboard
- **Priority:** P2 (nice-to-have)
- **Estimate:** 2 days
- **Dependencies:** 4.3, 5.1
- **Description:** Build an analytics page showing chatbot usage:
  1. **Conversation volume:** Line chart showing conversations per day/week/month
  2. **Top queries:** Table of most frequent queries with count
  3. **Source hit frequency:** Which documents/sources are cited most often
  4. **Response quality signals:** Average latency, token usage, conversation length
  5. **Date range selector:** Filter analytics by date range
  6. Use a lightweight charting library (Chart.js or similar)

  Go API analytics endpoints:
  - `GET /api/v1/orgs/:org_id/analytics/conversations` -- conversation volume over time
  - `GET /api/v1/orgs/:org_id/analytics/top-queries` -- most frequent queries
  - `GET /api/v1/orgs/:org_id/analytics/sources` -- source citation frequency
- **Acceptance criteria:**
  - Charts render with real data
  - Date range filter works
  - Top queries are ranked correctly
  - Source frequency shows which docs are most useful
  - Analytics are scoped to org (RLS)
  - Empty state when no data is available
- **Labels:** `frontend`, `backend`

---

#### Task 5.7: API Key Management Page

- **Milestone:** Admin Dashboard
- **Priority:** P1 (important)
- **Estimate:** 1.5 days
- **Dependencies:** 4.4, 5.1
- **Description:** Build the API key management page:
  1. **Key list:** Table showing key prefix, name, KB scope, allowed domains, rate limit, status, created date
  2. **Create key:** Form with name, KB selection, domain allowlist (comma-separated), rate limit. After creation, show the full key ONCE in a copyable dialog with warning that it cannot be retrieved again.
  3. **Edit key:** Update name, allowed domains, rate limit
  4. **Revoke key:** With confirmation dialog
  5. **Embed code:** After creating a key, show the `<raven-chat>` embed snippet with the key pre-filled
- **Acceptance criteria:**
  - Key creation shows full key exactly once
  - Key list never shows full key (only prefix)
  - Copy-to-clipboard works
  - Domain allowlist editing works
  - Revocation requires confirmation
  - Embed code snippet is correct and copyable
  - Only org_admin or workspace admin can manage API keys
- **Labels:** `frontend`

---

#### Task 5.8: LLM Provider Configuration Page

- **Milestone:** Admin Dashboard
- **Priority:** P1 (important)
- **Estimate:** 1.5 days
- **Dependencies:** 3.12, 5.1
- **Description:** Build the LLM provider management page:
  1. **Provider list:** Table showing provider type, display name, key hint (last 4 chars), status, is_default badge
  2. **Add provider:** Form with:
     - Provider type dropdown (OpenAI, Anthropic, Cohere, Google, Azure OpenAI, Custom)
     - Display name
     - API key (password field, shown as dots)
     - Base URL (for custom endpoints, optional for standard providers)
     - Config JSONB editor (advanced settings)
  3. **Test connectivity:** Button that calls the test endpoint and shows success/failure
  4. **Set default:** Mark a provider as default for the org
  5. **Rotate key:** Update API key with confirmation
  6. **Delete:** Revoke provider with confirmation
- **Acceptance criteria:**
  - All supported providers can be added
  - API key is entered as password (masked) and never displayed after save
  - Test connectivity shows clear success/failure feedback
  - Default provider badge is displayed
  - Key rotation works without downtime
  - Only key hint is displayed in the list
- **Labels:** `frontend`

---

#### Task 5.9: Mobile-Responsive Layout (PWA-Capable)

- **Milestone:** Admin Dashboard
- **Priority:** P2 (nice-to-have)
- **Estimate:** 1.5 days
- **Dependencies:** 5.1
- **Description:** Ensure the admin dashboard is fully responsive and PWA-capable:
  1. **Responsive design:**
     - Sidebar collapses to bottom navigation or hamburger menu on mobile
     - Tables switch to card layout on small screens
     - Forms are single-column on mobile
     - Touch-friendly tap targets (min 44px)
  2. **PWA setup:**
     - `manifest.json` with app name, icons, theme color
     - Service worker for offline shell (app shell caching)
     - Install prompt handling
     - Note: data still requires network (no offline data sync in MVP)
  3. **Viewport testing:** Test at 320px, 768px, 1024px, 1440px breakpoints
- **Acceptance criteria:**
  - Dashboard is usable on mobile phones (320px width)
  - Sidebar navigation works on mobile
  - Tables are readable on small screens
  - PWA can be installed on mobile devices
  - App launches from home screen with splash screen
  - No horizontal scrolling at any breakpoint
- **Labels:** `frontend`

---

## Milestone 6: SaaS Infrastructure (Week 10-12)

> Production readiness: billing, backups, scheduled jobs, legal compliance, edge deployment, and analytics.

---

#### Task 6.1: Stripe Integration (Subscriptions, Checkout, Webhooks)

- **Milestone:** SaaS Infrastructure
- **Priority:** P0 (blocker)
- **Estimate:** 5 days
- **Dependencies:** 2.2
- **Description:** Implement Stripe billing integration:
  1. **Stripe setup:**
     - Create Stripe products and price IDs for plans (Free, Pro, Enterprise)
     - Configure Stripe webhook endpoint
  2. **Go API endpoints:**
     - `POST /api/v1/orgs/:org_id/billing/checkout` -- create Stripe Checkout session for plan upgrade
     - `GET /api/v1/orgs/:org_id/billing` -- get current billing status (plan, subscription status, next invoice date, usage)
     - `POST /api/v1/orgs/:org_id/billing/portal` -- create Stripe Customer Portal session (manage payment methods, cancel subscription)
     - `POST /api/v1/webhooks/stripe` -- webhook receiver (no auth, Stripe signature verification)
  3. **Webhook handling:**
     - `checkout.session.completed` -- activate subscription, update org billing columns
     - `customer.subscription.updated` -- plan changes
     - `customer.subscription.deleted` -- downgrade to free
     - `invoice.payment_failed` -- set org status to `suspended` after grace period
     - `invoice.paid` -- clear suspension
  4. **Database columns:** Add to organizations table: `billing_customer_id`, `billing_subscription_id`, `billing_plan` (free/pro/enterprise), `billing_status` (active/past_due/canceled)
  5. **Plan enforcement:** Middleware that checks org's plan for feature gating (document limits, API rate limits, KB count limits)
  6. **Frontend:** Billing page in admin dashboard with plan display, upgrade button, portal link
- **Acceptance criteria:**
  - Checkout flow redirects to Stripe and returns to dashboard
  - Webhook correctly updates org billing status
  - Plan changes are reflected immediately
  - Failed payment triggers suspension after grace period
  - Feature gating enforces plan limits (e.g., free plan: 1 KB, 100 documents)
  - Stripe webhook signature verification prevents spoofing
  - Customer portal allows payment method management
  - Integration tests with Stripe test mode
- **Labels:** `backend`, `billing`, `frontend`

---

#### Task 6.2: SSL/TLS via Traefik ACME

- **Milestone:** SaaS Infrastructure
- **Priority:** P0 (blocker)
- **Estimate:** 1 day
- **Dependencies:** 1.4
- **Description:** Configure Traefik for automatic TLS certificate management:
  1. **ACME resolver:** Let's Encrypt with DNS-01 challenge (supports wildcard certificates)
  2. **Traefik configuration:**
     - TLS entrypoint on port 443
     - HTTP-to-HTTPS redirect on port 80
     - Certificate storage in a named volume (`traefik-certs`)
     - TLS minimum version 1.2, prefer 1.3
     - HSTS header with `max-age=31536000; includeSubDomains; preload`
  3. **DNS-01 challenge provider:** Configure for the domain registrar (Cloudflare, Route53, etc.) via environment variables
  4. **Development mode:** Self-signed certificates for local development (no ACME in Docker Compose dev profile)
- **Acceptance criteria:**
  - Production deployment automatically obtains Let's Encrypt certificates
  - HTTP requests redirect to HTTPS
  - TLS 1.3 is negotiated (verify with `openssl s_client` or `ssllabs.com`)
  - Certificate auto-renewal works (test with staging ACME)
  - HSTS header is present
  - Local dev uses self-signed certs without ACME
- **Labels:** `infra`, `security`

---

#### Task 6.3: pgBackRest Backup Configuration

- **Milestone:** SaaS Infrastructure
- **Priority:** P0 (blocker)
- **Estimate:** 2 days
- **Dependencies:** 1.4
- **Description:** Configure PostgreSQL backup strategy:
  1. **pgBackRest setup:**
     - Daily full backups
     - Continuous WAL archiving (point-in-time recovery)
     - 30-day retention policy
     - Backup storage: local volume (dev), S3-compatible (production)
     - Verify backup integrity with `pgbackrest check`
  2. **Restic for SeaweedFS:**
     - Daily backup of SeaweedFS data directory
     - Encrypted, deduplicated backups
     - 30-day retention
     - Backup to S3-compatible storage
  3. **Restore procedure:** Document and test the restore process:
     - Full database restore from backup
     - Point-in-time recovery to specific timestamp
     - SeaweedFS data restore
  4. **Docker Compose integration:** pgBackRest runs as a sidecar or cron job within the PostgreSQL container
- **Acceptance criteria:**
  - Daily full backup executes automatically
  - WAL archiving enables point-in-time recovery
  - Backup verification (`pgbackrest check`) passes
  - Full restore works to a new PostgreSQL instance
  - Point-in-time restore works to a specific timestamp
  - SeaweedFS backup and restore works
  - 30-day retention policy enforced (older backups pruned)
  - Restore procedure is documented
- **Labels:** `infra`

---

#### Task 6.4: Legal Pages (Privacy Policy, ToS, Cookie Consent)

- **Milestone:** SaaS Infrastructure
- **Priority:** P0 (blocker)
- **Estimate:** 3 days
- **Dependencies:** 5.1
- **Description:** Implement legal compliance:
  1. **Privacy Policy page:** Template privacy policy covering data collection, processing, BYOK data handling, third-party processors (LLM providers), data retention, GDPR rights. Served as a Vue.js page in the admin dashboard and a static page for the public site.
  2. **Terms of Service page:** Template ToS covering acceptable use, service level, liability, data ownership. Note: these should be reviewed by a lawyer before launch.
  3. **Cookie consent banner:** Integrate `cookieconsent` (MIT) library:
     - Banner in Vue.js admin dashboard
     - Configurable banner for `<raven-chat>` widget (opt-in for persistent cookies)
     - Consent categories: necessary, analytics (PostHog), preferences
     - Consent records stored in PostgreSQL (`consent_records` table)
  4. **GDPR API endpoints:**
     - `GET /api/v1/orgs/:org_id/export` -- export all org data as JSON archive
     - `DELETE /api/v1/orgs/:org_id/users/:user_id/data` -- GDPR right to erasure (cascade delete all user data)
  5. **Consent records migration:** Create `consent_records` table (user_id, consent_type, granted, timestamp, ip_address)
- **Acceptance criteria:**
  - Privacy Policy and ToS pages render correctly
  - Cookie consent banner appears on first visit
  - Consent choices are persisted
  - PostHog analytics only loads after consent is granted
  - GDPR export endpoint generates a downloadable JSON archive
  - GDPR erasure endpoint cascades deletion through all user data
  - Consent records are stored in the database
- **Labels:** `frontend`, `backend`, `security`, `docs`

---

#### Task 6.5: Scheduled Jobs via Asynq Cron

- **Milestone:** SaaS Infrastructure
- **Priority:** P1 (important)
- **Estimate:** 2 days
- **Dependencies:** 3.4
- **Description:** Implement scheduled background jobs using Asynq's cron scheduler:
  1. **Source re-crawling:** Periodically re-crawl sources based on their `crawl_frequency` setting (daily, weekly, monthly). Create new chunks/embeddings, mark old ones for cleanup.
  2. **Session cleanup:** Delete expired anonymous chat sessions (24h TTL) and their messages.
  3. **API key expiration:** Check for expired API keys, update status to `expired`.
  4. **Usage aggregation:** Aggregate daily usage stats per org (conversation count, token usage, document count) into a `usage_stats` table for billing and analytics.
  5. **Backup verification:** Periodic check that pgBackRest backups are current.

  Cron schedules:
  | Job | Schedule |
  |-----|----------|
  | Source re-crawl check | Every hour |
  | Session cleanup | Every 6 hours |
  | API key expiration | Daily at midnight |
  | Usage aggregation | Daily at 1 AM |
  | Backup verification | Daily at 3 AM |
- **Acceptance criteria:**
  - Cron jobs run at scheduled times
  - Source re-crawling respects each source's frequency setting
  - Expired sessions are cleaned up
  - Expired API keys are deactivated
  - Usage stats are aggregated daily
  - Jobs are idempotent (safe to run multiple times)
  - Job execution is logged
- **Labels:** `backend`, `infra`

---

#### Task 6.6: Edge Deployment Mode

- **Milestone:** SaaS Infrastructure
- **Priority:** P1 (important)
- **Estimate:** 3 days
- **Dependencies:** 1.4, 1.1
- **Description:** Implement the edge/split deployment mode for ARM64 devices:
  1. **Docker Compose edge profile:** `docker-compose.edge.yml` with minimal services:
     - Go API (ARM64 image, ~25 MB)
     - PostgreSQL 18 + pgvector (ARM64, no ParadeDB)
     - Valkey (ARM64, `--maxmemory 64mb`)
     - Traefik (ARM64)
     - No Python worker, Keycloak, SeaweedFS, or Strapi
  2. **Remote worker configuration:** Environment variable `RAVEN_AI_WORKER_GRPC_ADDR` points to cloud-hosted Python worker. gRPC connection with TLS + optional mTLS.
  3. **Go API ARM64 build:**
     - CI produces `linux/arm64` Docker image
     - Cross-compilation: `GOOS=linux GOARCH=arm64 go build -ldflags="-s -w"`
     - Target: <25 MB binary, <50ms startup, <10 MB RAM at idle
  4. **Full-text search fallback:** When `RAVEN_FTS_BACKEND` is not set and ParadeDB is unavailable, auto-detect and use tsvector
  5. **Storage fallback:** When SeaweedFS is unavailable, use local filesystem storage
  6. **Documentation:** Edge deployment guide with Raspberry Pi 4/5 setup instructions
- **Acceptance criteria:**
  - `docker compose -f docker-compose.edge.yml up -d` starts on Raspberry Pi 4
  - Go API binary is <25 MB and starts in <50ms
  - Go API connects to remote Python worker via gRPC
  - Full RAG pipeline works (upload -> process on cloud -> query -> response)
  - Total RAM usage on edge device is <500 MB
  - Local filesystem storage works as SeaweedFS fallback
  - tsvector is auto-selected when ParadeDB is unavailable
- **Labels:** `infra`, `backend`

---

#### Task 6.7: PostHog Cloud Integration (Analytics Events)

- **Milestone:** SaaS Infrastructure
- **Priority:** P2 (nice-to-have)
- **Estimate:** 2 days
- **Dependencies:** 5.1
- **Description:** Integrate PostHog Cloud for product analytics:
  1. **Vue.js admin dashboard:**
     - PostHog JavaScript SDK initialization (after cookie consent)
     - Track: page views, feature usage (KB create, document upload, chatbot config), user identification
     - Session replay (opt-in via cookie consent)
  2. **`<raven-chat>` widget:**
     - Minimal PostHog tracking: widget opened, conversation started, message sent, satisfaction signal
     - Only after consent (respect host page's consent state)
  3. **Go API (server-side):**
     - `posthog-go` SDK for server-side events
     - Track: API key creation, org creation, billing events
     - Feature flag evaluation for plan-gated features
  4. **Python worker:**
     - `posthog-python` SDK for processing events
     - Track: document processing time, embedding costs, RAG query performance
  5. Use `org_id` as group key for per-tenant analytics
- **Acceptance criteria:**
  - PostHog receives events from all four surfaces
  - User identification links sessions across dashboard and API
  - Feature flags can be evaluated per-org
  - Analytics only track after consent
  - PostHog dashboard shows meaningful data (page views, feature usage)
  - Events are batched (not per-request API calls)
- **Labels:** `frontend`, `backend`, `ai-worker`, `observability`

---

## Milestone 7: Phase 2 -- Voice Agent (Post-MVP)

> High-level outline. Detailed task breakdowns will be created when Phase 2 begins.

---

#### Task 7.1: LiveKit Server Deployment

- **Milestone:** Voice Agent
- **Priority:** P1
- **Estimate:** 3 days
- **Dependencies:** M1-M6 complete
- **Description:** Deploy LiveKit Server (Apache 2.0) as a Docker Compose service. Configure TURN/STUN. Generate API keys. Set up room management.
- **Labels:** `infra`

---

#### Task 7.2: LiveKit Agents Integration (Python Worker)

- **Milestone:** Voice Agent
- **Priority:** P0
- **Estimate:** 5 days
- **Dependencies:** 7.1
- **Description:** Integrate LiveKit Agents framework into the Python worker. Agent joins LiveKit rooms as a participant, receives audio frames, processes through STT -> RAG -> TTS pipeline, returns audio.
- **Labels:** `ai-worker`

---

#### Task 7.3: STT Integration (Deepgram / faster-whisper)

- **Milestone:** Voice Agent
- **Priority:** P0
- **Estimate:** 3 days
- **Dependencies:** 7.2
- **Description:** Implement speech-to-text: Deepgram Nova-3 API for initial deployment, faster-whisper for self-hosted fallback. Provider abstraction for swapping.
- **Labels:** `ai-worker`

---

#### Task 7.4: TTS Integration (Cartesia / Piper)

- **Milestone:** Voice Agent
- **Priority:** P0
- **Estimate:** 3 days
- **Dependencies:** 7.2
- **Description:** Implement text-to-speech: Cartesia Sonic API initially, Piper TTS (MIT) for self-hosted. Sentence-boundary dispatch for reduced latency.
- **Labels:** `ai-worker`

---

#### Task 7.5: Voice Session Management

- **Milestone:** Voice Agent
- **Priority:** P1
- **Estimate:** 2 days
- **Dependencies:** 7.2
- **Description:** Voice session CRUD (Go API), LiveKit room lifecycle, session-to-conversation linking, voice turn storage with audio metadata.
- **Labels:** `backend`

---

#### Task 7.6: Email Notifications (Transactional)

- **Milestone:** Voice Agent
- **Priority:** P1
- **Estimate:** 3 days
- **Dependencies:** M6
- **Description:** AWS SES / Resend integration via `go-mail`. Post-conversation summaries, admin digest notifications, document processing notifications.
- **Labels:** `backend`

---

#### Task 7.7: Smart Caching Layer (Semantic Cache)

- **Milestone:** Voice Agent
- **Priority:** P1
- **Estimate:** 5 days
- **Dependencies:** 4.8
- **Description:** Upgrade from exact-match cache to semantic similarity cache. Query embedding -> pgvector search on `response_cache` table with >0.95 cosine threshold. Cache learning, invalidation on KB update. Optional in-DB model adaptation.
- **Labels:** `ai-worker`, `backend`

---

#### Task 7.8: PostHog User Tracking (Cross-Channel Identity)

- **Milestone:** Voice Agent
- **Priority:** P2
- **Estimate:** 2 days
- **Dependencies:** 6.7, 7.5
- **Description:** Link user identity across chat and voice sessions in PostHog. Track conversation history per user. Analytics: most active users, preferred channels, session patterns.
- **Labels:** `observability`

---

## Milestone 8: Phase 3 -- WebRTC / WhatsApp (Post-MVP)

> High-level outline. Detailed task breakdowns will be created when Phase 3 begins.

---

#### Task 8.1: WhatsApp Business Calling API Integration

- **Milestone:** WebRTC / WhatsApp
- **Priority:** P0
- **Estimate:** 5 days
- **Dependencies:** M7 complete
- **Description:** Meta Graph API webhook receiver, SDP offer/answer exchange, WebRTC media bridge into LiveKit rooms.
- **Labels:** `backend`, `infra`

---

#### Task 8.2: WhatsApp-to-LiveKit Room Bridge

- **Milestone:** WebRTC / WhatsApp
- **Priority:** P0
- **Estimate:** 4 days
- **Dependencies:** 8.1
- **Description:** Go service that creates RTCPeerConnection from Meta's SDP offer, bridges media streams into LiveKit rooms. Voice agent handles WhatsApp calls identically to browser calls.
- **Labels:** `backend`

---

#### Task 8.3: Browser WebRTC ("Call the Assistant" Button)

- **Milestone:** WebRTC / WhatsApp
- **Priority:** P1
- **Estimate:** 3 days
- **Dependencies:** 7.1
- **Description:** LiveKit room token endpoint in Go API. "Call" button in `<raven-chat>` widget that connects via livekit-client-sdk-js to a voice agent room.
- **Labels:** `frontend`, `backend`

---

#### Task 8.4: WebRTC Session Management

- **Milestone:** WebRTC / WhatsApp
- **Priority:** P1
- **Estimate:** 2 days
- **Dependencies:** 8.1, 8.3
- **Description:** Unified session management for WhatsApp and browser WebRTC calls. Call history, duration tracking, quality metrics.
- **Labels:** `backend`

---

#### Task 8.5: WhatsApp Message Handling (Text + Voice)

- **Milestone:** WebRTC / WhatsApp
- **Priority:** P1
- **Estimate:** 3 days
- **Dependencies:** 8.1
- **Description:** Handle both text messages and voice calls from WhatsApp. Route text to RAG chat pipeline, voice to STT -> RAG -> TTS pipeline.
- **Labels:** `backend`, `ai-worker`

---

## Summary

### Effort Estimates by Milestone

| Milestone | Tasks | Total Days | Weeks |
|-----------|-------|------------|-------|
| M1: Project Scaffolding | 9 | 18.5 | ~2 |
| M2: Core API + Auth | 10 | 18 | ~2 |
| M3: Ingestion Pipeline | 13 | 25 | ~3 |
| M4: Chatbot MVP | 8 | 16.5 | ~2 |
| M5: Admin Dashboard | 9 | 18.5 | ~2 (parallel with M4) |
| M6: SaaS Infrastructure | 7 | 18 | ~2 |
| M7: Voice Agent (outline) | 8 | 26 | ~3 |
| M8: WebRTC/WhatsApp (outline) | 5 | 17 | ~2 |

### Critical Path (P0 Blockers)

```
M1 (Scaffolding)
  --> M2 (Core API + Auth)
    --> M3 (Ingestion Pipeline)
      --> M4 (Chatbot MVP)  [parallel with M5]
        --> M6 (SaaS Infrastructure)
          --> MVP Launch
```

### Parallelization Opportunities

- **M4 + M5:** Chatbot MVP and Admin Dashboard can be built in parallel by frontend and backend developers
- **Tasks 1.1, 1.2, 1.3:** All three service scaffolds can be done in parallel
- **Tasks 3.5, 3.6:** LiteParse and Crawl4AI integration can be done in parallel
- **Tasks 2.2, 2.3, 2.5:** CRUD APIs can be parallelized after 2.1 (JWT middleware) is done

### Risk Register

| Risk | Impact | Mitigation |
|------|--------|------------|
| ParadeDB AGPL license | High | tsvector fallback is always available (Task 3.10) |
| Keycloak SPI complexity | Medium | Start with basic JWT claims, iterate on SPI (Task 1.6) |
| Crawl4AI Chromium resource usage | Medium | Configure memory limits, test on constrained environments (Task 3.6) |
| LLM API costs during development | Low | Use test/mock providers, cache responses (Task 4.8) |
| Edge deployment ARM64 compatibility | Medium | Test ARM64 builds early in CI (Task 1.7) |

---

*This plan derives from the [Final Design Specification](./2026-03-27-raven-platform-design-final.md). All task descriptions reference the spec as the source of truth for technical decisions.*
