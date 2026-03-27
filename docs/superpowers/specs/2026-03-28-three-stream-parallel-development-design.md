# Three-Stream Parallel Development — Design Spec

**Date:** 2026-03-28
**Status:** Approved
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
| 1 | #24 | Organization CRUD API | P0-blocker |
| 2 | #28 | Role-Based Access Control Middleware | P0-blocker |
| 3 | #29 | PostgreSQL RLS Policies | P0-blocker |
| 4 | #25 | Workspace CRUD API | P0-blocker |
| 5 | #26 | User Management API (Keycloak Sync) | P0-blocker |
| 6 | #27 | Knowledge Base CRUD API | P0-blocker |
| 7 | #32 | Swagger/OpenAPI Generation | P2-nice-to-have |

**Dependencies satisfied:** #23 (JWT), #30 (CORS/versioning), #31 (rate limiting) all merged into main.

### Stream 2 — Frontend (M5: Admin Dashboard)

| Order | Issue | Title | Priority |
|-------|-------|-------|----------|
| 1 | #41 | Vue.js App Scaffold with Tailwind Plus | P0-blocker |
| 2 | #42 | Auth Flow (Keycloak OIDC PKCE) | P0-blocker |
| 3 | #43 | Organization Management Pages | P0-blocker |

Builds against `contracts/openapi-stub.yaml` until backend PRs merge.

### Stream 3 — AI Worker (M3: Ingestion Pipeline)

| Order | Issue | Title | Priority |
|-------|-------|-------|----------|
| 1 | #14 | Python Worker: LiteParse Integration | P0-blocker |
| 2 | #16 | Python Worker: Text Chunking | P0-blocker |
| 3 | #17 | Python Worker: Multi-Provider Embedding (BYOK) | P0-blocker |

Builds against `contracts/raven_worker.proto` stub.

---

## 3. Interface Contracts

Committed to `main` before agents start, under `contracts/`:

- **`contracts/openapi-stub.yaml`** — REST stubs for Org, Workspace, KB, User endpoints (paths, request/response schemas, Bearer JWT auth). Frontend develops against this mock.
- **`contracts/raven_worker.proto`** — gRPC service stubs: `DocumentIngest`, `ChunkText`, `GenerateEmbedding`. AI worker implements these interfaces.

---

## 4. Quality Gates (all streams, no exceptions)

Before any `git push` or PR creation:

1. **All issue task checkboxes** satisfied (every `- [ ]` in the GitHub issue ticked)
2. **Tests pass locally:**
   - Backend: `go test ./...` (unit + integration)
   - Frontend: `vitest` (unit) + Playwright (E2E)
   - AI Worker: `pytest` (unit + integration)
3. **End-to-end validation:** `docker compose up` — feature works as intended locally
4. **PR format:** squash merge, linked to its issue with `Closes #N`

---

## 5. Agent Responsibilities

Each agent:
- Works only within its own worktree branch — never touches main directly
- Creates one PR per issue (not batched)
- Runs all tests locally before pushing
- Follows existing code patterns in the repo (reads codebase before writing)
- Does not start the next issue until the current one has a passing PR

---

## 6. Integration Strategy

- Each stream's PRs merge into `main` via squash merge as they complete
- Frontend and AI streams reconcile against real backend endpoints once backend PRs land
- Contract stubs are replaced by real implementations as backend PRs merge
- No cross-stream code sharing within worktrees — integration happens only through `main`
