---
title: "Dev Setup"
---

# Dev Setup

Zero-to-running on a fresh checkout in 5–10 minutes. This page tracks the
canonical developer flow — every command is verified against `main`. For
deeper architectural detail, see
[Architecture Decisions](/contributing/architecture-decisions). For release
mechanics, see [Release Process](/contributing/release-process). For the
exhaustive environment variable list, see
[Configuration](/reference/configuration).

## Prerequisites

| Tool | Version | Install hint |
|------|---------|--------------|
| Go | 1.25+ (toolchain pins `go1.26.2`) | `brew install go` · `apt install golang-go` · [go.dev/dl](https://go.dev/dl/) |
| Python | 3.12+ | `brew install python@3.12` · `apt install python3.12` · [python.org](https://www.python.org/downloads/) |
| Node.js | 22 LTS+ | `brew install node@22` · `apt install nodejs npm` · [nodejs.org](https://nodejs.org/) |
| Docker + Docker Compose | Latest | `brew install --cask docker` · [docs.docker.com](https://docs.docker.com/get-docker/) — required for testcontainers integration tests |
| `uv` (recommended) | Latest | `curl -LsSf https://astral.sh/uv/install.sh \| sh` · `brew install uv` |
| `golangci-lint` | v2 | `brew install golangci-lint` · `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` |
| `make` | Any | Preinstalled on macOS/Linux; on Windows use WSL2 |

Optional but useful:

| Tool | Use |
|------|-----|
| `dotenvx` | Wraps Make targets to load `.env`; `npm i -g @dotenvx/dotenvx` |
| `air` | Hot reload for the Go API; `go install github.com/air-verse/air@latest` |
| `goose` | Database migrations; `go install github.com/pressly/goose/v3/cmd/goose@latest` |
| `ruff` | Python lint/format; bundled in `ai-worker` dev deps, also `pipx install ruff` |

::: tip Windows users
Develop inside WSL2 (Ubuntu 22.04+). Native Windows is not a supported dev
target — pre-commit shell hooks and Docker bind mounts both assume POSIX.
:::

## Clone and bootstrap

```bash
git clone https://github.com/ravencloak-org/Raven.git
cd Raven

# Seed local config from the committed template
cp .env.example .env
# Required overrides at minimum:
#   POSTGRES_PASSWORD       — any non-empty string for local dev
#   RAVEN_ENCRYPTION_AES_KEY — 32-byte hex (e.g. `openssl rand -hex 32`)
#   ZO_ROOT_USER_PASSWORD   — OpenObserve admin password
# BYOK secrets (set whichever providers you'll exercise):
#   ANTHROPIC_API_KEY / OPENAI_API_KEY / COHERE_API_KEY

# Wire up the repo's hooks (pre-commit lint + pre-push DCO)
git config core.hooksPath .githooks
```

The two hooks under `.githooks/` mirror CI:

| Hook | Triggers | Checks |
|------|----------|--------|
| `pre-commit` | `git commit` | `golangci-lint run ./...` when staged changes touch `.go` files |
| `pre-push` | `git push` | every commit being pushed has a `Signed-off-by:` trailer (DCO) |

`pre-commit` skips silently if `golangci-lint` is not on `PATH`. Always
commit with `git commit -s` so the DCO check passes.

## Bring up dependencies

For most workflows you only need the data layer running in containers. Run
the application services natively for fast iteration.

```bash
docker compose up -d postgres valkey supertokens
```

This starts:

| Service | Image | Port |
|---------|-------|------|
| `postgres` | `pgvector/pgvector:pg18` | `5432` |
| `valkey` | `valkey/valkey:8.1-alpine` | `6379` |
| `supertokens` | `registry.supertokens.io/supertokens/supertokens-postgresql` | internal `3567` |

Add `openobserve` if you want to inspect traces/metrics locally:

```bash
docker compose up -d postgres valkey supertokens openobserve
# UI at http://localhost:5080
```

To run **everything** in containers (the slow path) use `make compose`,
which wraps `docker compose up --build` with `dotenvx`.

## Run the Go API

The API binary lives under `cmd/api`. The Makefile targets load `.env`
through `dotenvx` automatically.

```bash
# Apply migrations (first run only, or after pulling new ones)
make migrate-up

# Start the server
make run            # plain `go run ./cmd/api`
# OR
make dev            # hot-reload via `air`
# OR, without dotenvx:
go run ./cmd/api
```

Default listen port is **`8081`** (configurable via `RAVEN_SERVER_PORT`).
Smoke-test:

```bash
curl http://localhost:8081/healthz
# → {"status":"ok"}
```

OpenAPI / Swagger UI is regenerated with `make swagger` and served at
`/swagger/index.html`.

## Run the AI worker

The Python gRPC service lives in `ai-worker/`. `uv` is the recommended
dependency manager — it's faster than pip and respects the lockfile.

```bash
cd ai-worker

# Create the venv and install runtime + dev deps
uv sync --extra dev

# Generate gRPC stubs from .proto files (one-time, or when proto changes)
uv run ./scripts/generate_proto.sh

# Start the worker
uv run python -m raven_worker
```

The worker exposes:

| Endpoint | Port | Purpose |
|----------|------|---------|
| gRPC | `50051` | Embeddings, RAG, ingestion (called by Go API) |
| HTTP internal | `8090` | `POST /internal/summarize` and other intra-cluster RPCs |

gRPC reflection is enabled in dev — point `grpcurl` at it:

```bash
grpcurl -plaintext localhost:50051 list
```

If you don't have `uv`, the legacy flow still works:

```bash
cd ai-worker
python -m venv .venv
source .venv/bin/activate
pip install -e ".[dev]"
python -m raven_worker
```

## Run the frontend

```bash
cd frontend
npm install
npm run dev
```

Vite dev server listens on **`http://localhost:3000`** (overridden in
`vite.config.ts`); it proxies `/api/*` to `http://localhost:8080`. If you
run the Go API on the default `8081`, either update the Vite proxy target
or set `RAVEN_SERVER_PORT=8080` when starting the API.

The first time you run Playwright e2e tests, install the browser binaries:

```bash
npx playwright install
```

## Run tests

### Go

```bash
# Unit tests
go test ./...

# With race detector (matches CI)
go test -race -timeout 30m ./...

# Integration tests — requires Docker (testcontainers)
go test -tags=integration ./internal/integration/... -v -timeout 5m -count=1

# Or via Make
make test                 # unit
make test-integration     # integration
make coverage             # merged unit + integration coverage report
```

### Python

```bash
cd ai-worker
uv run pytest                        # all
uv run pytest -k "test_rag"          # filter by name
uv run pytest -x                     # stop on first failure
```

### Frontend

```bash
cd frontend
npm run test:unit         # Vitest, headless, no live stack needed
npm run test:e2e          # Playwright; auto-spawns Vite via webServer
npm run build             # vue-tsc + vite build — catches TS errors Vite skips
```

Playwright runs against `http://localhost:5173` in CI (`vite preview`) and
spawns the dev server otherwise. The `webServer` block in
`playwright.config.ts` handles startup and reuse.

## Linting

| Linter | Scope | Command | Gated by hook? |
|--------|-------|---------|----------------|
| `golangci-lint` | All Go code | `golangci-lint run ./...` (or `make lint`) | ✅ pre-commit |
| `ruff` (lint) | `ai-worker/` | `cd ai-worker && uv run ruff check .` | ❌ run before push |
| `ruff` (format) | `ai-worker/` | `cd ai-worker && uv run ruff format --check .` | ❌ run before push |
| `eslint` | `frontend/` | `cd frontend && npm run lint` | ❌ run before push |
| `vue-tsc` | `frontend/` | `cd frontend && npx vue-tsc --noEmit` | ❌ run before push |
| DCO sign-off | every commit | enforced by pre-push hook | ✅ pre-push |

The pre-commit hook only runs `golangci-lint`. Python and frontend linters
are not hooked — run them manually before pushing. The shortest pre-push
checklist:

```bash
# From repo root
make lint && \
( cd ai-worker && uv run ruff check . && uv run ruff format --check . ) && \
( cd frontend && npm run lint && npx vue-tsc --noEmit )
```

CI rejects any push that fails these. Never use `--no-verify`.

## Editor setup

VS Code is the reference editor; the suggestions below also apply to
JetBrains GoLand / PyCharm with equivalent plugins.

**Extensions**

- `golang.go` — Go language server (`gopls`), test runner, debug
- `charliermarsh.ruff` — Ruff lint + format on save for Python
- `vue.volar` — Vue 3 Single File Components, TypeScript-aware
- `dbaeumer.vscode-eslint` — ESLint for the frontend
- `ms-azuretools.vscode-docker` — Compose file support
- `tamasfe.even-better-toml` — `pyproject.toml` editing
- `redhat.vscode-yaml` — schema-aware YAML

**Recommended `.vscode/settings.json`** (workspace-local, not committed):

```jsonc
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast-only"],
  "go.formatTool": "goimports",
  "[python]": {
    "editor.defaultFormatter": "charliermarsh.ruff",
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": { "source.organizeImports": "explicit" }
  },
  "[vue]": { "editor.defaultFormatter": "Vue.volar" },
  "eslint.workingDirectories": [{ "directory": "frontend", "changeProcessCWD": true }],
  "files.eol": "\n"
}
```

Make sure `gopls` picks up the toolchain pin — run
`go env GOTOOLCHAIN` and confirm it returns `go1.26.2` or higher.

## Tip: parallel-worktree workflow

Raven is developed almost entirely inside `git worktree` checkouts. Each
feature gets its own worktree under `.worktrees/`, which is gitignored.

```bash
# Spin up a worktree for a new feature
git worktree add .worktrees/feat-my-feature -b feat/my-feature

# Work inside it
cd .worktrees/feat-my-feature

# Tear it down once the PR has merged
git worktree remove .worktrees/feat-my-feature
```

This isolates each branch's `bin/`, `node_modules/`, and `.venv/` from
your main checkout — important when running multiple agents in parallel,
or just hopping between long-lived features. Never commit the
`.worktrees/` directory, and never let two worktrees run the same dev
servers on the same ports simultaneously.

---

**Next steps**

- [Configuration reference](/reference/configuration) — every env var
  the API and worker read
- [Architecture Decisions](/contributing/architecture-decisions) — why
  Raven is built the way it is
- [Release Process](/contributing/release-process) — milestone cadence,
  squash-merge rules, tagging
