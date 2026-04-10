# Raven — Code Style & Conventions

## Go
- Linter: golangci-lint (config: `.golangci.yml`), version v2.11.4
- Go version: 1.26
- Standard Go naming: camelCase for unexported, PascalCase for exported
- All lint + vet must pass before push

## Python (ai-worker/)
- Linter: ruff check
- Tests: pytest
- Both must pass before push

## TypeScript/Vue (frontend/)
- Linter: ESLint (config: `eslint.config.js`)
- Formatter: Prettier (`.prettierrc`)
- Node.js: 22 LTS
- `npm run lint` must pass before push
- E2E: Playwright (Chromium only)

## Database
- All migrations must include both UP and DOWN
- Migration test: `go test -race -run '^TestMigrationsUpAndDown$' ./migrations/...`

## API / OpenAPI
- Contracts in `contracts/openapi-stub.yaml` and `docs/swagger/`
- All API changes must be backward compatible

## Commits
- No Co-Authored-By trailers (no AI attribution)
- Conventional commits: feat/, fix/, refactor/, ci/, chore/, deps/
- Never force-push, never --no-verify, never amend published commits

## PR Rules
- Squash merge only
- Never push directly to main
- Always queue auto-merge after PR creation: `gh pr merge <N> --auto --squash`
- No AGPL-licensed dependencies
- No secrets/tokens committed
