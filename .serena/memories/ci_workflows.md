# Raven — CI/CD Workflows

## GitHub Actions Workflows
| File | Trigger | Purpose |
|------|---------|---------|
| `go.yml` | push/PR to main (cmd/, internal/, pkg/, go.mod) | Build, test (race), vet, golangci-lint |
| `frontend.yml` | push/PR to main (frontend/) | npm ci, build, Playwright E2E |
| `python.yml` | push/PR to main (ai-worker/) | ruff, pytest |
| `docker.yml` | push/PR to main (Dockerfile, docker-compose.yml) | Build + push to ghcr.io (amd64+arm64) |
| `pages.yml` | push/PR to main (frontend/) | Build + deploy to Cloudflare Pages |
| `security.yml` | scheduled | Security scanning (Trivy) |
| `ci-required.yml` | all | Gate job for branch protection (Mergify) |
| `claude.yml` | issue_comment, PR | Claude Code auto-review + fix |
| `opencode.yml` | issue_comment, PR review | opencode AI assistant |

## Docker Images (ghcr.io)
- `go-api:latest` / `go-api:<sha>` — multi-arch (amd64 + arm64)
- `python-worker:latest` / `python-worker:<sha>` — multi-arch

## Mergify
- Gates on `#check-failure = 0` across all workflows
- Config: `.mergify.yml`

## Dependabot
- Weekly updates: gomod, pip (ai-worker/), npm (frontend/), docker (root + ai-worker/), github-actions
