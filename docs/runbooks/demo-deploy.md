# Demo Deploy Runbook

## Current image-publish state (audited 2026-05-13)

| Workflow | Trigger | Images | Platforms | Tag scheme |
|---|---|---|---|---|
| `docker.yml` | `push: branches: [main]` + PRs touching Dockerfiles | go-api, python-worker, frontend | linux/amd64 + linux/arm64 (split across native runners on PRs; one job on main) | `type=ref,event=branch`, `type=semver`, `type=sha,prefix=sha-`, `latest` on main |
| `release.yml` | `push: tags: v*` | go-api, python-worker, frontend | linux/amd64 + linux/arm64 | semver via `docker/metadata-action` |
| `go.yml` | push/PR | none — CI tests only | n/a | n/a |
| `python.yml` | push/PR | none — CI tests only | n/a | n/a |
| `frontend.yml` | push/PR | none — CI tests only | n/a | n/a |

**Consequence for plan #2:** the plan's Task 2 (build-and-push composite action) and Task 6 (add publish jobs to per-language workflows) are not needed — `release.yml` already does multi-arch GHCR publishing on tag push, including SBOM, attestation, and image signing.

## Required GitHub repo configuration

- Actions variable `AWS_ACCOUNT_ID`: 12-digit AWS account ID hosting the demo.
- Actions secret `RESEND_API_KEY`: same value as SSM `/raven/demo/resend_api_key` (used by the notify job).

## What plan #2 still needs
- Task 3 — GitHub OIDC IAM role for SSM access (deploy/terraform/demo/oidc.tf)
- Task 4 — `update.sh` accepts `--tag`
- Task 5 — wire `FRONTEND_IMAGE` through `docker-compose.server.yml`
- Task 7 — `.github/workflows/demo-deploy.yml` (the SSM deploy bit, triggered after release.yml succeeds)
- Task 9 — deploy notification email via Resend
- Task 10 — deploy recovery runbook (this file, extended)
