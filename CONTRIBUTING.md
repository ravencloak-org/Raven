# Contributing to Raven

## Before You Start

Open an issue first for anything beyond a trivial fix. This prevents duplicate effort and lets us align on approach before you invest time writing code.

## Branch Naming

| Prefix | When |
|--------|------|
| `feat/` | New feature |
| `fix/` | Bug fix |
| `refactor/` | Code restructure, no behaviour change |
| `ci/` | CI/CD changes |
| `chore/` | Tooling, deps, config |
| `deps/` | Dependency bumps |

Example: `feat/semantic-cache`, `fix/voice-session-timeout`

## Commit Style

- Use [Conventional Commits](https://www.conventionalcommits.org/) — `feat:`, `fix:`, `chore:`, etc.
- Keep the subject line under 72 characters
- No AI attribution trailers (`Co-Authored-By:` etc.)
- **Every commit must carry a `Signed-off-by:` trailer (DCO)** — see below

## Developer Certificate of Origin (DCO)

This project requires every commit to be signed off under the [Developer Certificate of Origin 1.1](https://developercertificate.org/). Signing off certifies that you wrote the patch (or otherwise have the right to submit it under the project's open-source license).

Add the trailer automatically with `-s`:

```bash
git commit -s -m "feat: your message here"
```

This appends:

```
Signed-off-by: Your Name <your.email@example.com>
```

Rules:

- The sign-off email must match the email on your commit
- Every commit on a PR must be signed — the `DCO` check blocks merge otherwise
- To back-fill sign-offs on an existing branch: `git rebase --signoff main` then force-push
- If you're committing on behalf of an employer, make sure your employer allows you to contribute under Apache 2.0 (the OSS license)

The DCO is enforced by a required CI check. See `.github/workflows/dco.yml`.

## Pull Request Workflow

1. Branch off `main`
2. Make your changes with passing tests and lint
3. Open a PR against `main`
4. Immediately queue auto-merge after creation:

```bash
gh pr create --title "..." --body "..." --base main
gh pr merge <PR_NUMBER> --auto --squash
```

PRs squash-merge only — no regular merge, no rebase-merge.

## Quality Gates

All of these must pass **locally** before pushing. CI will reject if they don't.

### Go (backend)

```bash
# Build
go build ./...

# Tests (including integration tests — needs Docker)
go test -race -timeout 30m ./...

# Lint
golangci-lint run
```

### Python (AI worker)

```bash
cd ai-worker
ruff check .
ruff format --check .
pytest
```

### Frontend

```bash
cd frontend
npm run lint          # ESLint
npx tsc --noEmit      # TypeScript check
npm run test:unit     # Vitest unit tests
npm run build         # Production build (catches type errors vite misses)
```

### End-to-End (Playwright)

E2E tests run against a live stack. See [DEVELOPMENT.md](DEVELOPMENT.md) for setup.

```bash
cd frontend
npm run test:e2e
```

## Code Style

### Go

- Follow standard Go conventions (`gofmt`, `golangci-lint`)
- Handler → Service → Repository layering — no direct DB access from handlers
- Return `*apierror.AppError` at service boundaries; let `apierror.ErrorHandler()` middleware render the response
- Use `pgx.Tx` with `db.WithOrgID` for all tenant-scoped DB operations (RLS enforcement)
- Wrap errors: `fmt.Errorf("ServiceName.MethodName: %w", err)`

### Python

- `ruff` for formatting and linting (configured in `pyproject.toml`)
- Type hints on all public functions
- `structlog` for structured logging — no bare `print()`

### Vue / TypeScript

- Composition API with `<script setup>` — no Options API
- Pinia for all shared state
- Tailwind CSS only — no additional CSS frameworks
- `useAuthStore()` for auth; `useFeatureFlag()` for PostHog feature flags

## Testing Expectations

- **Unit tests**: every new function with logic gets a test
- **Integration tests** (Go): use `testutil.NewTestDB()` — it spins up a real Postgres via testcontainers
- **No mocking the database** in integration tests — we got burned by mock/prod divergence before
- **Playwright E2E**: cover the happy path for any new user-facing flow

## Database Migrations

Migrations live in `migrations/` and use [goose](https://github.com/pressly/goose).

```bash
# Apply all pending migrations
make migrate-up

# Roll back one migration
make migrate-down
```

Rules:
- Never edit an existing migration file after it has been merged to `main`
- Every new table needs RLS policies — see existing migrations for the pattern
- Column renames = new migration with `ALTER TABLE ... RENAME COLUMN`

## Never

- Push directly to `main`
- Use `--no-verify` to skip hooks
- Amend published commits
- Force-push to `main`
