# Raven — Suggested Commands

## Go (API Server)

```bash
# Build
go build ./cmd/api

# Test (with race detector)
go test -race -coverprofile=coverage.txt ./...

# Migration tests
go test -race -run '^TestMigrationsUpAndDown$' ./migrations/...

# Vet
go vet ./...

# Lint (must pass before push)
golangci-lint run

# Hot reload dev
air  # uses .air.toml
```

## Python (AI Worker)

```bash
cd ai-worker

# Lint (must pass before push)
ruff check

# Test
pytest
```

## Frontend (Vue.js)

```bash
cd frontend

# Install deps
npm ci

# Dev server
npm run dev

# Build
npm run build

# Lint (must pass before push)
npm run lint

# Unit tests
npm run test:unit

# E2E tests (Playwright, requires Chromium)
npx playwright install --with-deps chromium
npm run test:e2e
```

## Docker / Deployment

```bash
# Main stack (dev)
docker compose up -d

# EC2 server deployment
DOCKER_HOST=unix:///run/raven/docker.sock \
docker compose -f deploy/ec2/docker-compose.server.yml --env-file .env.server up -d

# Admin/monitoring tools
docker compose -f docker-compose.admin.yml up -d

# Edge (Raspberry Pi)
docker compose -f docker-compose.edge.yml --env-file .env.edge up -d

# Hyperswitch (payment dev)
docker compose -f deploy/hyperswitch/docker-compose.hyperswitch.yml up -d
```

## Git / PR Workflow

```bash
# Create PR then immediately queue auto-merge
gh pr create ...
gh pr merge <PR_NUMBER> --auto --squash
```
