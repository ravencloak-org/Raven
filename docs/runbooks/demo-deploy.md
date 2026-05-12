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

## Recovering from a failed deploy

### If `wait-for-images` failed

One of `release.yml`'s publish jobs didn't push to GHCR.

1. Open the `Release` workflow run for the tag, identify the failure.
2. Fix the underlying issue on `main`.
3. Delete the tag and recreate it:
   ```bash
   git tag -d v0.x.y && git push --delete origin v0.x.y
   git tag v0.x.y && git push origin v0.x.y
   ```

### If `deploy` failed at SSM send-command

OIDC role couldn't be assumed or the instance isn't tagged correctly.

1. Verify the role ARN matches `terraform output github_deploy_role_arn`.
2. Verify the instance has `Name=raven-demo-app` and tag `Environment=demo`.
3. Re-run the `Demo deploy` workflow via `workflow_dispatch`.

### If `deploy` failed during `update.sh`

The new image is bad or compose failed to come up.

1. SSM into the box:
   ```bash
   aws ssm start-session --target $(cd deploy/terraform/demo && terraform output -raw instance_id) --region ap-south-1
   ```
2. Tail container logs:
   ```bash
   cd /opt/raven
   docker compose -f deploy/ec2/docker-compose.server.yml --env-file .env.server logs --tail=200
   ```
3. Roll back: re-run `update.sh --tag <previous-good-tag>`.
4. Open an issue with the failure logs.

### If `/healthz` smoke-test failed

App is up but unhealthy.

1. SSM into the box. Tail go-api logs.
2. Check `https://observability.demo.raven.ravencloak.org` for traces from the deploy window.
3. If unfixable in 15 minutes, roll back per the previous section.

## First successful deploy

Record here once Task 8 (manual smoke) runs successfully:

- Tag: (pending)
- Date: (pending)
- Workflow run URL: (pending)
- Operator: (pending)
