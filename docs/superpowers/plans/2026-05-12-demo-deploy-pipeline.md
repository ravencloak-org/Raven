# Demo Deploy Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `git tag v0.x.y && git push --tags` → multi-arch GHCR images → demo box updated via GitHub OIDC → AWS SSM Run Command. No SSH from CI. Full audit trail in CloudTrail.

**Architecture:** GitHub Actions runs on `push: tags: 'v*'`. A build job per service (go-api, python-worker, frontend) sets up Docker Buildx, builds linux/amd64 + linux/arm64, pushes to `ghcr.io/ravencloak-org/<service>:<tag>`. A deploy job assumes an IAM role via GitHub OIDC, then calls `aws ssm send-command` to run `deploy/ec2/update.sh --tag v0.x.y` on the demo instance. The workflow polls SSM until the command completes and surfaces stdout/stderr in the workflow log.

**Tech Stack:** GitHub Actions, Docker Buildx, GHCR, AWS OIDC, AWS Systems Manager Run Command, existing `deploy/ec2/update.sh`.

**Spec reference:** `docs/superpowers/specs/2026-05-12-public-demo-deployment-design.md` §5
**Prerequisite plan:** `2026-05-12-demo-infrastructure.md` (the EC2 instance and SSM agent must exist)

---

## File Structure

**Create:**
- `deploy/terraform/demo/oidc.tf` — IAM role assumed by GitHub OIDC, scoped to the `ravencloak-org/Raven` repo
- `.github/workflows/demo-deploy.yml` — orchestrates build → push → deploy on tag
- `.github/actions/build-and-push/action.yml` — reusable composite action for multi-arch buildx + GHCR push
- `docs/runbooks/demo-deploy.md` — how to cut a release tag, how to recover from a failed deploy

**Modify:**
- `deploy/ec2/update.sh` — add `--tag` flag (existing accepts `--sha`)
- `deploy/ec2/docker-compose.server.yml` — read `GO_API_IMAGE`, `PYTHON_WORKER_IMAGE`, `FRONTEND_IMAGE` from env (verify; the current script already patches the first two)
- `deploy/ec2/.env.server.example` — add `FRONTEND_IMAGE`
- `.github/workflows/go.yml`, `python.yml`, `frontend.yml` — confirm they push to GHCR (or move push to the new demo-deploy workflow if not)

---

## Tasks

### Task 1: Inspect what existing workflows already publish

**Files:** none (read-only)

- [ ] **Step 1: Open the three image-building workflows and document, in `docs/runbooks/demo-deploy.md`, exactly which images they push and on which triggers**

```bash
grep -nE "ghcr.io|docker/build-push-action|push:" .github/workflows/go.yml .github/workflows/python.yml .github/workflows/frontend.yml .github/workflows/docker.yml
```

Capture per workflow:
- Trigger (push to main? tag? PR?)
- Image name(s) pushed
- Platforms (single-arch? multi-arch?)
- Whether the image is tagged with `latest`, the commit SHA, the git tag, or all three

- [ ] **Step 2: Write the findings into `docs/runbooks/demo-deploy.md` as a "Current image-publish state" table**

```markdown
# Demo Deploy Runbook

## Current image-publish state (recorded YYYY-MM-DD)

| Workflow | Trigger | Image | Platforms | Tags |
|---|---|---|---|---|
| go.yml | … | … | … | … |
| python.yml | … | … | … | … |
| frontend.yml | … | … | … | … |
| docker.yml | … | … | … | … |
```

- [ ] **Step 3: Commit the runbook skeleton**

```bash
git add docs/runbooks/demo-deploy.md
git commit -m "docs(demo): deploy runbook skeleton with current publish-state audit"
```

---

### Task 2: Reusable composite action for multi-arch build + GHCR push

**Files:**
- Create: `.github/actions/build-and-push/action.yml`

- [ ] **Step 1: Write the composite action**

```yaml
name: Build and push multi-arch image to GHCR
description: |
  Builds a service image for linux/amd64 and linux/arm64, pushes to
  ghcr.io/ravencloak-org/<service> tagged with the git ref (tag or sha-short)
  plus 'latest'.

inputs:
  service:
    description: Service name, e.g. go-api, python-worker, frontend
    required: true
  context:
    description: Build context directory
    required: true
  dockerfile:
    description: Path to the Dockerfile relative to the repo root
    required: true
  tag:
    description: Image tag (e.g. v0.3.0). If empty, uses sha-short.
    required: false
    default: ""

outputs:
  image:
    description: Full image reference that was pushed
    value: ${{ steps.set-image.outputs.image }}

runs:
  using: composite
  steps:
    - name: Set up QEMU
      uses: docker/setup-qemu-action@v3

    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3

    - name: Log in to GHCR
      uses: docker/login-action@v3
      with:
        registry: ghcr.io
        username: ${{ github.actor }}
        password: ${{ env.GITHUB_TOKEN }}

    - name: Resolve tag
      id: resolve-tag
      shell: bash
      run: |
        if [ -n "${{ inputs.tag }}" ]; then
          echo "tag=${{ inputs.tag }}" >> "$GITHUB_OUTPUT"
        else
          echo "tag=sha-${GITHUB_SHA::7}" >> "$GITHUB_OUTPUT"
        fi

    - name: Build and push
      uses: docker/build-push-action@v5
      with:
        context: ${{ inputs.context }}
        file: ${{ inputs.dockerfile }}
        platforms: linux/amd64,linux/arm64
        push: true
        tags: |
          ghcr.io/ravencloak-org/${{ inputs.service }}:${{ steps.resolve-tag.outputs.tag }}
          ghcr.io/ravencloak-org/${{ inputs.service }}:latest
        cache-from: type=gha,scope=${{ inputs.service }}
        cache-to: type=gha,mode=max,scope=${{ inputs.service }}

    - name: Export image reference
      id: set-image
      shell: bash
      run: echo "image=ghcr.io/ravencloak-org/${{ inputs.service }}:${{ steps.resolve-tag.outputs.tag }}" >> "$GITHUB_OUTPUT"
```

- [ ] **Step 2: Commit**

```bash
git add .github/actions/build-and-push/action.yml
git commit -m "feat(ci): reusable composite action for multi-arch GHCR push"
```

---

### Task 3: Add OIDC IAM role for GitHub Actions

**Files:**
- Create: `deploy/terraform/demo/oidc.tf`

- [ ] **Step 1: Write the OIDC provider + role**

```hcl
# GitHub Actions OIDC provider (one per AWS account; create if absent)
data "aws_iam_openid_connect_provider" "github" {
  url = "https://token.actions.githubusercontent.com"
}

# If the provider does not yet exist, comment the data source above and
# uncomment this resource:
# resource "aws_iam_openid_connect_provider" "github" {
#   url             = "https://token.actions.githubusercontent.com"
#   client_id_list  = ["sts.amazonaws.com"]
#   thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
# }

resource "aws_iam_role" "github_demo_deploy" {
  name = "${local.name_prefix}-github-deploy"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Federated = data.aws_iam_openid_connect_provider.github.arn
      }
      Action = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
        }
        StringLike = {
          "token.actions.githubusercontent.com:sub" = "repo:ravencloak-org/Raven:ref:refs/tags/v*"
        }
      }
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy" "github_demo_deploy" {
  name = "ssm-send-command-demo"
  role = aws_iam_role.github_demo_deploy.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ssm:SendCommand",
          "ssm:GetCommandInvocation",
          "ssm:ListCommandInvocations"
        ]
        Resource = "*"
        Condition = {
          StringEquals = {
            "aws:ResourceTag/Environment" = "demo"
          }
        }
      },
      {
        Effect = "Allow"
        Action = ["ssm:SendCommand"]
        Resource = "arn:aws:ssm:*::document/AWS-RunShellScript"
      },
      {
        Effect = "Allow"
        Action = ["ssm:GetCommandInvocation", "ssm:ListCommandInvocations"]
        Resource = "*"
      }
    ]
  })
}

output "github_deploy_role_arn" {
  description = "ARN of the role GitHub Actions assumes for demo deploys"
  value       = aws_iam_role.github_demo_deploy.arn
}
```

- [ ] **Step 2: Validate**

```bash
cd deploy/terraform/demo
terraform fmt && terraform validate
```

- [ ] **Step 3: Apply (if infra plan #1 already applied)**

```bash
terraform plan -out oidc.tfplan && terraform apply oidc.tfplan
```

- [ ] **Step 4: Capture the role ARN**

```bash
terraform output -raw github_deploy_role_arn
```

Add the value to `docs/runbooks/demo-deploy.md` under a new "Deploy role ARN" section.

- [ ] **Step 5: Commit**

```bash
git add deploy/terraform/demo/oidc.tf docs/runbooks/demo-deploy.md
git commit -m "feat(demo): github OIDC role for tag-triggered demo deploys"
```

---

### Task 4: Extend `update.sh` to accept a `--tag` argument

**Files:**
- Modify: `deploy/ec2/update.sh`

- [ ] **Step 1: Rewrite the argument parser to support both `--sha` and `--tag`**

Replace lines 16-28 (the current `SHA` parse block) with:

```bash
GO_API_IMAGE=""
PYTHON_WORKER_IMAGE=""
FRONTEND_IMAGE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sha)
      REF="sha-${2::7}"
      shift 2
      ;;
    --tag)
      REF="$2"
      shift 2
      ;;
    *)
      echo "Unknown arg: $1" >&2
      exit 2
      ;;
  esac
done

if [ -n "${REF:-}" ]; then
  GO_API_IMAGE="ghcr.io/ravencloak-org/go-api:${REF}"
  PYTHON_WORKER_IMAGE="ghcr.io/ravencloak-org/python-worker:${REF}"
  FRONTEND_IMAGE="ghcr.io/ravencloak-org/frontend:${REF}"

  sed -i "s|^GO_API_IMAGE=.*|GO_API_IMAGE=${GO_API_IMAGE}|" "$ENV_FILE"
  sed -i "s|^PYTHON_WORKER_IMAGE=.*|PYTHON_WORKER_IMAGE=${PYTHON_WORKER_IMAGE}|" "$ENV_FILE"
  if grep -q '^FRONTEND_IMAGE=' "$ENV_FILE"; then
    sed -i "s|^FRONTEND_IMAGE=.*|FRONTEND_IMAGE=${FRONTEND_IMAGE}|" "$ENV_FILE"
  else
    echo "FRONTEND_IMAGE=${FRONTEND_IMAGE}" >> "$ENV_FILE"
  fi
  echo "Pinned to ref: ${REF}"
fi
```

- [ ] **Step 2: Add `FRONTEND_IMAGE` to the example env file**

Open `deploy/ec2/.env.server.example` (create if missing) and ensure it contains:

```dotenv
GO_API_IMAGE=ghcr.io/ravencloak-org/go-api:latest
PYTHON_WORKER_IMAGE=ghcr.io/ravencloak-org/python-worker:latest
FRONTEND_IMAGE=ghcr.io/ravencloak-org/frontend:latest
```

- [ ] **Step 3: Syntax-check the script**

```bash
bash -n deploy/ec2/update.sh
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add deploy/ec2/update.sh deploy/ec2/.env.server.example
git commit -m "feat(deploy): update.sh accepts --tag and patches FRONTEND_IMAGE"
```

---

### Task 5: Wire `FRONTEND_IMAGE` through compose

**Files:**
- Modify: `deploy/ec2/docker-compose.server.yml`

- [ ] **Step 1: Inspect the compose file and add `image: ${FRONTEND_IMAGE}` to the frontend service if it currently builds from context**

```bash
grep -nE "frontend:|build:|image:" deploy/ec2/docker-compose.server.yml
```

Find the `frontend:` service definition. If it has a `build:` block, replace with:

```yaml
  frontend:
    image: ${FRONTEND_IMAGE}
```

(Leave the local `docker-compose.yml` building from source for dev — only the server overlay uses pre-built images.)

- [ ] **Step 2: Commit**

```bash
git add deploy/ec2/docker-compose.server.yml
git commit -m "feat(deploy): server overlay reads FRONTEND_IMAGE from env"
```

---

### Task 6: Build-only workflow update — push on tag

**Files:**
- Modify: `.github/workflows/go.yml`
- Modify: `.github/workflows/python.yml`
- Modify: `.github/workflows/frontend.yml`

Add a new job to each existing workflow that runs only on `push: tags: 'v*'` and uses the composite action from Task 2.

- [ ] **Step 1: Add the publish job to `.github/workflows/go.yml`**

Append to the workflow (top-level keys preserved; add a new job entry):

```yaml
  publish:
    if: startsWith(github.ref, 'refs/tags/v')
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4

      - name: Build and push
        uses: ./.github/actions/build-and-push
        with:
          service: go-api
          context: .
          dockerfile: Dockerfile
          tag: ${{ github.ref_name }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Also add `tags: ['v*']` to the workflow's `on.push` triggers if not already present.

- [ ] **Step 2: Repeat for `.github/workflows/python.yml`**

Same shape, but:
- `service: python-worker`
- `context: ./ai-worker`
- `dockerfile: ai-worker/Dockerfile`

- [ ] **Step 3: Repeat for `.github/workflows/frontend.yml`**

Same shape, but:
- `service: frontend`
- `context: ./frontend`
- `dockerfile: frontend/Dockerfile` (verify path)

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/go.yml .github/workflows/python.yml .github/workflows/frontend.yml
git commit -m "feat(ci): publish multi-arch images to GHCR on v* tags"
```

---

### Task 7: Demo deploy workflow

**Files:**
- Create: `.github/workflows/demo-deploy.yml`

- [ ] **Step 1: Write the workflow**

```yaml
name: Demo deploy

on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      tag:
        description: Existing v* tag to deploy
        required: true

permissions:
  id-token: write     # needed for OIDC
  contents: read

env:
  AWS_REGION: ap-south-1
  ROLE_ARN: arn:aws:iam::<ACCOUNT_ID>:role/raven-demo-github-deploy
  INSTANCE_TAG_KEY: Name
  INSTANCE_TAG_VALUE: raven-demo-app

jobs:
  wait-for-images:
    runs-on: ubuntu-latest
    steps:
      - name: Wait for service images to be available on GHCR
        env:
          TAG: ${{ github.event.inputs.tag || github.ref_name }}
        run: |
          set -euo pipefail
          for svc in go-api python-worker frontend; do
            for i in {1..30}; do
              if docker manifest inspect ghcr.io/ravencloak-org/$svc:$TAG > /dev/null 2>&1; then
                echo "Found $svc:$TAG"
                break
              fi
              echo "Waiting for $svc:$TAG (attempt $i/30)..."
              sleep 30
            done
          done

  deploy:
    needs: wait-for-images
    runs-on: ubuntu-latest
    steps:
      - name: Configure AWS credentials via OIDC
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ env.ROLE_ARN }}
          aws-region: ${{ env.AWS_REGION }}

      - name: Resolve demo instance ID
        id: instance
        run: |
          INSTANCE_ID=$(aws ec2 describe-instances \
            --filters "Name=tag:${INSTANCE_TAG_KEY},Values=${INSTANCE_TAG_VALUE}" \
                      "Name=instance-state-name,Values=running" \
            --query 'Reservations[0].Instances[0].InstanceId' \
            --output text)
          if [ "$INSTANCE_ID" = "None" ] || [ -z "$INSTANCE_ID" ]; then
            echo "No running demo instance found"
            exit 1
          fi
          echo "instance_id=$INSTANCE_ID" >> "$GITHUB_OUTPUT"

      - name: Send update.sh via SSM
        id: ssm
        env:
          TAG: ${{ github.event.inputs.tag || github.ref_name }}
          INSTANCE_ID: ${{ steps.instance.outputs.instance_id }}
        run: |
          set -euo pipefail
          CMD_ID=$(aws ssm send-command \
            --instance-ids "$INSTANCE_ID" \
            --document-name AWS-RunShellScript \
            --comment "demo deploy $TAG" \
            --parameters "commands=[\"cd /opt/raven && git fetch --tags && git checkout $TAG && bash deploy/ec2/update.sh --tag $TAG\"]" \
            --timeout-seconds 1800 \
            --query 'Command.CommandId' --output text)
          echo "command_id=$CMD_ID" >> "$GITHUB_OUTPUT"

      - name: Wait for SSM command to complete
        env:
          CMD_ID: ${{ steps.ssm.outputs.command_id }}
          INSTANCE_ID: ${{ steps.instance.outputs.instance_id }}
        run: |
          set -euo pipefail
          for i in {1..60}; do
            STATUS=$(aws ssm get-command-invocation \
              --command-id "$CMD_ID" --instance-id "$INSTANCE_ID" \
              --query Status --output text)
            echo "[$i] Status: $STATUS"
            case "$STATUS" in
              Success) exit 0 ;;
              Failed|Cancelled|TimedOut)
                aws ssm get-command-invocation \
                  --command-id "$CMD_ID" --instance-id "$INSTANCE_ID" \
                  --query '{stdout:StandardOutputContent,stderr:StandardErrorContent}'
                exit 1
                ;;
            esac
            sleep 30
          done
          echo "Timed out waiting for SSM command"
          exit 1

      - name: Smoke-test /healthz
        run: |
          for i in {1..20}; do
            if curl -fsS https://demo.raven.ravencloak.org/healthz; then
              echo "OK"; exit 0
            fi
            sleep 15
          done
          echo "Health check never went green"
          exit 1
```

Note: `<ACCOUNT_ID>` should be replaced with the actual AWS account ID. Track it as a repo-level Actions variable `AWS_ACCOUNT_ID` instead — update the env line:

```yaml
  ROLE_ARN: arn:aws:iam::${{ vars.AWS_ACCOUNT_ID }}:role/raven-demo-github-deploy
```

- [ ] **Step 2: Set the `AWS_ACCOUNT_ID` repo variable**

In GitHub repo Settings → Secrets and variables → Actions → Variables, add `AWS_ACCOUNT_ID` with the account number.

Document the requirement in `docs/runbooks/demo-deploy.md`:

```markdown
## Required GitHub repo configuration

- Actions variable `AWS_ACCOUNT_ID`: 12-digit AWS account ID hosting the demo.
```

- [ ] **Step 3: Validate workflow syntax with `actionlint`** (if installed locally)

```bash
actionlint .github/workflows/demo-deploy.yml
```

If not installed, skip — GitHub will surface syntax errors on push.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/demo-deploy.yml docs/runbooks/demo-deploy.md
git commit -m "feat(ci): demo-deploy workflow — OIDC + SSM Run Command"
```

---

### Task 8: First end-to-end deploy smoke

**Files:** none (operational)

Prerequisite: plan #1 (infrastructure) tasks 1-16 complete and the demo box is up.

- [ ] **Step 1: Tag a dry-run release**

```bash
git checkout main && git pull
git tag v0.0.1-demo
git push origin v0.0.1-demo
```

- [ ] **Step 2: Watch the workflow**

Open https://github.com/ravencloak-org/Raven/actions and find the run for tag `v0.0.1-demo`. Watch `wait-for-images`, then `deploy`.

Expected: green within ~15 minutes.

- [ ] **Step 3: Verify the images landed on GHCR**

```bash
for svc in go-api python-worker frontend; do
  docker manifest inspect ghcr.io/ravencloak-org/$svc:v0.0.1-demo | head -5
done
```

Expected: each prints a manifest with `linux/amd64` and `linux/arm64` entries.

- [ ] **Step 4: Verify the demo is responding with the new tag**

```bash
curl -fsS https://demo.raven.ravencloak.org/healthz
```

Expected: 200 OK. (If Cloudflare Access is still on the whole hostname per Phase 1, ensure the `/healthz` bypass policy from plan #1 Task 17 Step 3 is in place.)

- [ ] **Step 5: SSM into the box and confirm container images are at the new tag**

```bash
aws ssm start-session --target $(cd deploy/terraform/demo && terraform output -raw instance_id) --region ap-south-1
# inside:
sudo docker ps --format '{{.Image}}'
```

Expected: rows containing `:v0.0.1-demo`.

- [ ] **Step 6: Append outcome to the runbook**

Add to `docs/runbooks/demo-deploy.md`:

```markdown
## First successful deploy

- Tag: v0.0.1-demo
- Date: YYYY-MM-DD
- Workflow run: <URL>
- Operator: <initials>
```

- [ ] **Step 7: Commit runbook update**

```bash
git add docs/runbooks/demo-deploy.md
git commit -m "docs(demo): record first successful end-to-end deploy"
```

---

### Task 9: Deploy notification (post-success email)

**Files:**
- Modify: `.github/workflows/demo-deploy.yml`

- [ ] **Step 1: Add a final `notify` job that emails on success and failure**

Append to the workflow's `jobs:` block:

```yaml
  notify:
    needs: deploy
    if: always()
    runs-on: ubuntu-latest
    steps:
      - name: Send result email via Resend API
        env:
          RESEND_API_KEY: ${{ secrets.RESEND_API_KEY }}
          TAG: ${{ github.event.inputs.tag || github.ref_name }}
          OUTCOME: ${{ needs.deploy.result }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
        run: |
          curl -fsS -X POST https://api.resend.com/emails \
            -H "Authorization: Bearer $RESEND_API_KEY" \
            -H "Content-Type: application/json" \
            -d "{
              \"from\": \"noreply@ravencloak.org\",
              \"to\": [\"jobinlawrance@gmail.com\"],
              \"subject\": \"[demo] $TAG deploy $OUTCOME\",
              \"text\": \"Tag: $TAG\\nResult: $OUTCOME\\nRun: $RUN_URL\"
            }"
```

- [ ] **Step 2: Set the `RESEND_API_KEY` GitHub Actions secret**

In repo Settings → Secrets → Actions, add `RESEND_API_KEY` from the Resend dashboard (or reuse the value already stored in SSM under `/raven/demo/resend_api_key`).

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/demo-deploy.yml
git commit -m "feat(ci): email deploy outcome via Resend on success or failure"
```

---

### Task 10: Recovery procedure for a failed deploy

**Files:**
- Modify: `docs/runbooks/demo-deploy.md`

- [ ] **Step 1: Append a recovery section**

```markdown
## Recovering from a failed deploy

### If `wait-for-images` failed

One of the build workflows (go.yml, python.yml, frontend.yml) didn't push.
1. Open the per-service workflow run for the tag, identify the failure.
2. Fix the underlying issue on `main`.
3. Delete the tag and recreate it:
   \`\`\`bash
   git tag -d v0.x.y && git push --delete origin v0.x.y
   git tag v0.x.y && git push origin v0.x.y
   \`\`\`

### If `deploy` failed at SSM send-command

OIDC role couldn't be assumed or the instance isn't tagged correctly.
1. Verify the role ARN matches `terraform output github_deploy_role_arn`.
2. Verify the instance has `Name=raven-demo-app` and tag `Environment=demo`.
3. Re-run the `Demo deploy` workflow via `workflow_dispatch`.

### If `deploy` failed during `update.sh`

The new image is bad or compose failed to come up.
1. SSM into the box.
2. \`\`\`bash
   cd /opt/raven
   docker compose -f deploy/ec2/docker-compose.server.yml --env-file .env.server logs --tail=200
   \`\`\`
3. Roll back: re-run `update.sh --tag <previous-good-tag>`.
4. Open an issue with the failure logs.

### If `/healthz` smoke-test failed

App is up but unhealthy.
1. SSM into the box. Tail go-api logs.
2. Check `https://observability.demo.raven.ravencloak.org` for traces from
   the deploy window.
3. If unfixable in 15 minutes, roll back.
```

- [ ] **Step 2: Commit**

```bash
git add docs/runbooks/demo-deploy.md
git commit -m "docs(demo): deploy failure recovery procedures"
```

---

## Self-review

| Spec section | Plan task(s) |
|---|---|
| §5 CI/CD — multi-arch GHCR | Tasks 2, 6 |
| §5 CI/CD — OIDC IAM role | Task 3 |
| §5 CI/CD — SSM Run Command from workflow | Task 7 |
| §5 update.sh accepts a tag | Task 4 |
| §5 No SSH from CI | Task 7 (uses SSM exclusively) |
| §5 Audit trail in CloudTrail | implicit — SSM SendCommand is logged |
| Deploy notification (operational nicety not in spec but explicitly requested in §9 paging) | Task 9 |

No placeholders. Role ARN is parameterised via `vars.AWS_ACCOUNT_ID`. Image tag flows consistently as `github.ref_name` everywhere.

One known unresolved item: Task 1 audits current workflows but the plan assumes go.yml/python.yml/frontend.yml exist and are the canonical builders. If they don't currently push to GHCR at all, Task 6 is a from-scratch add rather than an extension. The audit in Task 1 surfaces this before Task 6, so the order is safe.

A second known follow-up: the `docker.yml` workflow may already publish a single multi-service image (the badge in README suggests so). If it does, decide during Task 1 whether to retire `docker.yml`'s publish path or have it coexist with the per-service workflows from Task 6. The plan does not prescribe either — flag it for an architectural decision in the audit step.
