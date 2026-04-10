# Raven — Task Completion Checklist

When completing a task, run the following checks before pushing/creating a PR.

## Go Backend Changes

1. **Lint**: `golangci-lint run` — must pass with zero issues
2. **Test**: `go test -race ./...` — all tests must pass
3. **Build**: `go build ./cmd/api` — must compile cleanly
4. **Swagger** (if handler annotations changed): `swag init -g cmd/api/main.go --output docs/swagger --parseDependency --parseInternal`
5. **Tidy**: `go mod tidy` (if dependencies changed)

## Python AI Worker Changes (cd ai-worker/)

1. **Lint**: `ruff check .` — must pass
2. **Format check**: `ruff format --check` — must pass
3. **Type check**: `mypy raven_worker` — must pass
4. **Test**: `pytest -v` — all tests must pass

## Frontend Changes (cd frontend/)

1. **Lint**: `npm run lint` — must pass
2. **Type check**: `npx vue-tsc --noEmit` — must pass
3. **Unit tests**: `npm run test:unit` — must pass
4. **E2E tests**: `npm run test:e2e` — must pass (when applicable)
5. **Build**: `npm run build` — must succeed

## All Changes

1. **No `--no-verify`**: All git hooks must pass
2. **No amending published commits**: Create new commits
3. **No AI attribution**: Do not add Co-Authored-By trailers
4. **Branch naming**: Use `type/descriptor` format
5. **PR**: Use `gh pr create`, then immediately `gh pr merge <N> --auto --squash`
