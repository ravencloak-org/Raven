# Three-Stream Parallel Development — Design Spec

**Date:** 2026-03-28
**Status:** Active
**Milestone scope:** M2 (backend CRUD + security), M5 (frontend scaffold + auth), M3 (AI worker ingestion)

---

## 1. Stream Architecture

Three isolated git worktrees, each owned by one parallel agent running concurrently:

```
Raven/
├── (main — integration target)
└── .claude/worktrees/
    ├── stream-backend/    ← feat/stream-backend-m2-crud
    ├── stream-frontend/   ← feat/stream-frontend-m5-scaffold
    └── stream-ai/         ← feat/stream-ai-m3-ingestion
```

All open M2 PRs (#100 JWT, #101 CORS, #103 rate limiting, #104 CI fix) are **merged**. No blockers remain.

---

## 2. Scope per Stream

### Stream 1 — Backend (M2: Core API + Auth)

Sequential within one worktree. Each issue gets its own squash PR after full local validation.

| Order | Issue | Title | Priority |
|-------|-------|-------|----------|
| 1 | #29 | PostgreSQL RLS Policies | P0-blocker |
| 2 | #28 | Role-Based Access Control Middleware | P0-blocker |
| 3 | #24 | Organization CRUD API | P0-blocker |
| 4 | #25 | Workspace CRUD API | P0-blocker |
| 5 | #26 | User Management API (Keycloak Sync) | P0-blocker |
| 6 | #27 | Knowledge Base CRUD API | P0-blocker |
| 7 | #32 | Swagger/OpenAPI Generation | P2-nice-to-have |

**Dependencies satisfied (all confirmed closed):**
- #23 (JWT middleware) — merged via PR #100
- #30 (CORS/versioning) — merged via PR #101
- #31 (rate limiting) — merged via PR #103
- #5 (database migrations / M1 DB setup) — closed, M1 complete 2026-03-27
- #6 (Keycloak realm setup) — closed, M1 complete 2026-03-27

**Note on #26 (User Management):** Issue #26 requires a Keycloak webhook. The `docker-compose.yml` in this repo includes Keycloak with the reavencloak SPI pre-loaded. The internal webhook endpoint (`POST /api/v1/internal/keycloak-webhook`) must be verified reachable only from the compose network (not externally). If Keycloak SPI is not yet emitting events in the local environment, the webhook handler can be implemented and unit-tested with mocked payloads — a follow-up integration test can be added once the SPI is configured. Do not block the PR on a live Keycloak webhook firing.

### Stream 2 — Frontend (M5: Admin Dashboard)

> **Note:** #41 (Vue scaffold) is already implemented — the `frontend/` directory has a complete Vue 3 + TypeScript + Tailwind scaffold. Start directly at #42.

| Order | Issue | Title | Priority |
|-------|-------|-------|----------|
| 1 | #42 | Auth Flow (Keycloak OIDC PKCE) | P0-blocker |
| 2 | #43 | Organization Management Pages | P0-blocker |

Builds against `contracts/openapi-stub.yaml` (committed to `main`). Playwright E2E tests use `page.route()` to mock API responses — no live backend required during development.

### Stream 3 — AI Worker (M3: Ingestion Pipeline)

> **Note on #15 (Crawl4AI):** Issue #16 declares a dependency on #15 (Crawl4AI web scraping). This is a runtime pipeline dependency — the chunker processes text regardless of its source. The `TextChunker` implementation in #16 has no code dependency on #15. #15 is therefore excluded from this plan; it will be implemented separately. The AI stream's test suite mocks document content rather than calling the scraper.

| Order | Issue | Title | Priority |
|-------|-------|-------|----------|
| 1 | #14 | Python Worker: LiteParse Integration | P0-blocker |
| 2 | #16 | Python Worker: Text Chunking | P0-blocker |
| 3 | #17 | Python Worker: Multi-Provider Embedding (BYOK) | P0-blocker |

Builds against `proto/ai_worker.proto` (already exists in the repo). The proto file is the authoritative contract — do not modify it.

---

## 3. Interface Contracts

Both contracts committed to `main` and available at launch:

- **`contracts/openapi-stub.yaml`** — REST stubs for Org, Workspace, KB, User endpoints (paths, request/response schemas, Bearer JWT auth). Frontend mocks API calls against this during development.
- **`proto/ai_worker.proto`** — gRPC service definition: `ParseAndEmbed`, `QueryRAG`, `GetEmbedding`. Already exists. AI worker implements these interfaces. Do not modify during M3 work.

---

## 4. Quality Gates (all streams, no exceptions)

Before any `git push` or PR creation:

1. **All issue task checkboxes** satisfied (every `- [ ]` in the GitHub issue ticked)
2. **Tests pass locally:**
   - Backend: `go test -short ./...` (unit); integration tests require `docker compose up`
   - Frontend: `vitest` (unit) + Playwright E2E with `page.route()` mocks (no live backend needed)
   - AI Worker: `pytest -v` (unit + mocked integration)
3. **Build succeeds:**
   - Backend: `go build ./...` + `golangci-lint run`
   - Frontend: `npm run build` (TypeScript + Vite, zero errors)
   - AI Worker: `ruff check .` + `ruff format --check .`
4. **PR format:** squash merge, linked to its issue with `Closes #N`
5. **"Passing PR"** means GitHub Actions CI passes on the PR — not just local tests.

---

## 5. Agent Responsibilities

Each agent:
- Works only within its own worktree branch — never touches main directly
- Creates one PR per issue (not batched)
- Runs all tests locally before pushing
- Follows existing code patterns in the repo (reads codebase before writing)
- Does not start the next issue until the current PR has CI passing on GitHub

---

## 6. Integration Strategy

- Each stream's PRs merge into `main` via squash merge as they complete
- Frontend and AI streams reconcile against real backend endpoints once backend PRs land
- Contract stubs are replaced by real implementations as backend PRs merge
- No cross-stream code sharing within worktrees — integration happens only through `main`

---

## 7. Integration Handoff

**Trigger:** All Stream 1 (backend) PRs are squash-merged to `main`.

**Actions:**
1. Stream 2 (frontend) agent rebases `feat/stream-frontend-m5-scaffold` on updated `main`, replaces `page.route()` mocks with real API calls to the local backend, re-runs full Playwright suite.
2. Stream 3 (AI) agent rebases `feat/stream-ai-m3-ingestion` on updated `main`, runs an end-to-end gRPC call through the full `ParseAndEmbed` pipeline with a real document.

**Acceptance gate:** `docker compose up` with all three services — backend, frontend dev server, AI worker — produces correct responses for the full ingest → query flow. This gate is owned by the human (Jobin) after agents complete their streams.
