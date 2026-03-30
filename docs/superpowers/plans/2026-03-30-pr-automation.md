# PR Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every PR (from Claude agents or Dependabot) auto-merges once CI passes; stale PRs are automatically brought up to date with `main`.

**Architecture:** Four CI workflows are refactored to always run (with skip jobs for unchanged paths), enabling GitHub branch protection to gate on them. Mergify drives the update+merge queue. CLAUDE.md instructs all agents to enqueue auto-merge at PR creation time.

**Tech Stack:** GitHub Actions (`dorny/paths-filter@v4`), Mergify GitHub App, `gh` CLI, CLAUDE.md

> **Note on action versions:** The replacement workflows use `actions/checkout@v6`, `actions/setup-go@v6`, `actions/setup-node@v6`, `actions/setup-python@v6` — matching the versions already in the existing workflows (which are passing CI). The `changes` detector job uses `actions/checkout@v4` (a known-stable version for that lightweight step). Do not change these versions unless CI fails at the "Download action repository" step.

---

## Task 1: Refactor `go.yml` — path-filter bypass

**Files:**
- Modify: `.github/workflows/go.yml`

The workflow currently uses workflow-level `paths:` which prevents it running on frontend-only PRs, deadlocking branch protection. Replace with a `changes` detector job and per-job gates.

- [ ] **Step 1: Read the current file**

  Run: `cat .github/workflows/go.yml`

- [ ] **Step 2: Replace the workflow**

  Replace the entire contents of `.github/workflows/go.yml` with:

```yaml
name: Go CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      src: ${{ steps.filter.outputs.src }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v4
        id: filter
        with:
          filters: |
            src:
              - 'cmd/**'
              - 'internal/**'
              - 'pkg/**'
              - 'go.mod'
              - 'go.sum'
              - '.github/workflows/go.yml'

  build-and-test:
    name: Build & Test
    needs: changes
    if: needs.changes.outputs.src == 'true'
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v6

      - name: Setup Go
        uses: actions/setup-go@v6
        with:
          go-version: '1.26'
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Build
        run: go build ./cmd/api

      - name: Test
        run: go test -race -coverprofile=coverage.txt ./...

      - name: Test (migrations integration)
        run: go test -race -run '^TestMigrationsUpAndDown$' ./migrations/...

      - name: Upload coverage to Codecov
        uses: codecov/codecov-action@v5
        with:
          files: coverage.txt
          fail_ci_if_error: false

      - name: Vet
        run: go vet ./...

  build-and-test-skip:
    name: Build & Test
    needs: changes
    if: needs.changes.outputs.src != 'true'
    runs-on: ubuntu-latest
    steps:
      - run: echo "No Go changes, skipping"

  lint:
    name: Lint
    needs: changes
    if: needs.changes.outputs.src == 'true'
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v6

      - name: Setup Go
        uses: actions/setup-go@v6
        with:
          go-version: '1.26'
          cache: true

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.11.4

  lint-skip:
    name: Lint
    needs: changes
    if: needs.changes.outputs.src != 'true'
    runs-on: ubuntu-latest
    steps:
      - run: echo "No Go changes, skipping"
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/go.yml
git commit -m "ci: refactor go.yml — always-run with paths-filter bypass"
```

---

## Task 2: Refactor `python.yml` — path-filter bypass

**Files:**
- Modify: `.github/workflows/python.yml`

- [ ] **Step 1: Replace the workflow**

  Replace the entire contents of `.github/workflows/python.yml` with:

```yaml
name: Python CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      src: ${{ steps.filter.outputs.src }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v4
        id: filter
        with:
          filters: |
            src:
              - 'ai-worker/**'
              - 'proto/**'
              - '.github/workflows/python.yml'

  test:
    name: Lint, Type-check & Test
    needs: changes
    if: needs.changes.outputs.src == 'true'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ai-worker
    steps:
      - name: Checkout code
        uses: actions/checkout@v6

      - name: Setup Python
        uses: actions/setup-python@v6
        with:
          python-version: '3.12'
          cache: pip
          cache-dependency-path: ai-worker/pyproject.toml

      - name: Install dependencies
        run: pip install -e ".[dev]"

      - name: Ruff lint
        run: ruff check

      - name: Ruff format check
        run: ruff format --check

      - name: Type check (mypy)
        run: mypy raven_worker

      - name: Run tests with coverage
        run: pytest -v --cov=raven_worker --cov-report=xml --cov-report=term-missing

      - name: Upload coverage report
        uses: actions/upload-artifact@v4
        with:
          name: coverage-report
          path: ai-worker/coverage.xml
          retention-days: 7

  test-skip:
    name: Lint, Type-check & Test
    needs: changes
    if: needs.changes.outputs.src != 'true'
    runs-on: ubuntu-latest
    steps:
      - run: echo "No Python changes, skipping"
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/python.yml
git commit -m "ci: refactor python.yml — always-run with paths-filter bypass"
```

---

## Task 3: Refactor `frontend.yml` — path-filter bypass

**Files:**
- Modify: `.github/workflows/frontend.yml`

- [ ] **Step 1: Replace the workflow**

  Replace the entire contents of `.github/workflows/frontend.yml` with:

```yaml
name: Frontend CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      src: ${{ steps.filter.outputs.src }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v4
        id: filter
        with:
          filters: |
            src:
              - 'frontend/**'
              - '.github/workflows/frontend.yml'

  build:
    name: Lint & Build
    needs: changes
    if: needs.changes.outputs.src == 'true'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
    steps:
      - name: Checkout code
        uses: actions/checkout@v6

      - name: Setup Node.js
        uses: actions/setup-node@v6
        with:
          node-version: '22'
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Install dependencies
        run: npm ci

      - name: Lint
        run: npm run lint

      - name: Type check
        run: npx vue-tsc --noEmit

      - name: Build
        run: npm run build

  build-skip:
    name: Lint & Build
    needs: changes
    if: needs.changes.outputs.src != 'true'
    runs-on: ubuntu-latest
    steps:
      - run: echo "No frontend changes, skipping"

  e2e:
    name: Playwright E2E
    needs: changes
    if: needs.changes.outputs.src == 'true'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
    steps:
      - name: Checkout code
        uses: actions/checkout@v6

      - name: Setup Node.js
        uses: actions/setup-node@v6
        with:
          node-version: '22'
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Install dependencies
        run: npm ci

      - name: Build
        run: npm run build

      - name: Install Playwright browsers
        run: npx playwright install --with-deps chromium

      - name: Run Playwright tests
        run: npm run test:e2e

  e2e-skip:
    name: Playwright E2E
    needs: changes
    if: needs.changes.outputs.src != 'true'
    runs-on: ubuntu-latest
    steps:
      - run: echo "No frontend changes, skipping"
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/frontend.yml
git commit -m "ci: refactor frontend.yml — always-run with paths-filter bypass"
```

---

## Task 4: Refactor `docker.yml` — path-filter bypass

**Files:**
- Modify: `.github/workflows/docker.yml`

Docker uses a matrix job (`Build ${{ matrix.image }}`). Skip jobs must have explicit names matching the expanded check strings (`Build go-api`, `Build python-worker`).

- [ ] **Step 1: Replace the workflow**

  Replace the entire contents of `.github/workflows/docker.yml` with:

```yaml
name: Docker Build & Push

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read
  packages: write

jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      src: ${{ steps.filter.outputs.src }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v4
        id: filter
        with:
          filters: |
            src:
              - 'Dockerfile'
              - 'ai-worker/Dockerfile'
              - 'docker-compose.yml'
              - '.github/workflows/docker.yml'

  build:
    name: Build ${{ matrix.image }}
    needs: changes
    if: needs.changes.outputs.src == 'true'
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        include:
          - image: go-api
            context: .
            dockerfile: Dockerfile
          - image: python-worker
            context: ai-worker
            dockerfile: ai-worker/Dockerfile
    steps:
      - name: Checkout code
        uses: actions/checkout@v6

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v4

      - name: Login to GHCR
        if: github.event_name == 'push' && github.ref == 'refs/heads/main'
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build ${{ matrix.image }}
        uses: docker/build-push-action@v7
        with:
          context: ${{ matrix.context }}
          file: ${{ matrix.dockerfile }}
          push: ${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}
          platforms: linux/amd64,linux/arm64
          cache-from: type=gha
          cache-to: type=gha,mode=max
          tags: ghcr.io/${{ github.repository_owner }}/${{ matrix.image }}:${{ github.sha }}

  build-skip-go-api:
    name: Build go-api
    needs: changes
    if: needs.changes.outputs.src != 'true'
    runs-on: ubuntu-latest
    steps:
      - run: echo "No Docker changes, skipping"

  build-skip-python-worker:
    name: Build python-worker
    needs: changes
    if: needs.changes.outputs.src != 'true'
    runs-on: ubuntu-latest
    steps:
      - run: echo "No Docker changes, skipping"
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/docker.yml
git commit -m "ci: refactor docker.yml — always-run with paths-filter bypass"
```

---

## Task 5: Push CI workflow changes and bootstrap Docker check strings

The four CI workflow changes must land on `main` first so branch protection can reference the check strings. The Docker matrix check strings only appear in GitHub's registry after a real workflow run.

- [ ] **Step 1: Push the branch and open a PR**

```bash
git push origin HEAD
gh pr create --title "ci: always-run workflows with paths-filter bypass" \
  --body "Refactor all CI workflows to always run on PRs, using dorny/paths-filter@v4 for skip jobs. Required before configuring branch protection." \
  --base main
```

- [ ] **Step 2: Wait for CI to pass, then squash-merge**

```bash
# Get PR number from previous step output, e.g. 133
gh pr merge <PR_NUMBER> --squash --delete-branch
```

- [ ] **Step 3: Bootstrap Docker check strings**

  After the merge, push a trivial Dockerfile change to trigger a Docker CI run so GitHub registers `Build go-api` and `Build python-worker` as known check strings:

```bash
git checkout -b ci/bootstrap-docker-checks main
git pull origin main
echo "# trigger" >> Dockerfile
git add Dockerfile
git commit -m "ci: trigger Docker CI to register check strings for branch protection"
git push origin ci/bootstrap-docker-checks
gh pr create --title "ci: bootstrap Docker check strings" \
  --body "Trivial change to trigger Docker CI run. Required to register matrix check strings before configuring branch protection. Delete after merge." \
  --base main
```

- [ ] **Step 4: Wait for Docker CI to complete, then merge**

```bash
gh pr merge <PR_NUMBER> --squash --delete-branch
```

- [ ] **Step 5: Revert the Dockerfile change**

```bash
git checkout main && git pull origin main
git checkout -b ci/revert-dockerfile-trigger
git revert HEAD --no-edit
git push origin ci/revert-dockerfile-trigger
gh pr create --title "ci: revert Dockerfile bootstrap trigger" --base main
gh pr merge <PR_NUMBER> --squash --delete-branch
```

---

## Task 6: Enable GitHub repo settings

- [ ] **Step 1: Enable auto-merge and delete-branch-on-merge**

```bash
gh api PATCH /repos/ravencloak-org/Raven \
  --field allow_auto_merge=true \
  --field delete_branch_on_merge=true
```

  Expected output includes `"allow_auto_merge": true` and `"delete_branch_on_merge": true`.

- [ ] **Step 2: Verify**

```bash
gh api /repos/ravencloak-org/Raven --jq '{"allow_auto_merge": .allow_auto_merge, "delete_branch_on_merge": .delete_branch_on_merge}'
```

  Expected: `{"allow_auto_merge": true, "delete_branch_on_merge": true}`

---

## Task 7: Configure branch protection on `main`

- [ ] **Step 1: Apply branch protection**

```bash
gh api PUT /repos/ravencloak-org/Raven/branches/main/protection \
  --input - <<'EOF'
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "Go CI / Build & Test",
      "Go CI / Lint",
      "Python CI / Lint, Type-check & Test",
      "Frontend CI / Lint & Build",
      "Frontend CI / Playwright E2E",
      "Docker Build & Push / Build go-api",
      "Docker Build & Push / Build python-worker"
    ]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false
}
EOF
```

- [ ] **Step 2: Verify**

```bash
gh api /repos/ravencloak-org/Raven/branches/main/protection \
  --jq '.required_status_checks.contexts'
```

  Expected: array containing all 7 check strings listed above.

---

## Task 8: Create `.mergify.yml`

**Files:**
- Create: `.mergify.yml`

- [ ] **Step 1: Install the Mergify GitHub App**

  Visit https://github.com/apps/mergify and install on `ravencloak-org/Raven`. Grant access to this repository only.

- [ ] **Step 2: Create `.mergify.yml` at repo root**

```yaml
pull_request_rules:
  - name: Auto-update PRs behind main
    conditions:
      - base = main
      - -draft
      - -title ~= "^WIP"
      - -conflict
      - -merged
      - -closed
      - "#commits-behind > 0"
    actions:
      update: {}

  - name: Auto squash-merge when CI passes
    conditions:
      - base = main
      - -draft
      - -title ~= "^WIP"
      - -conflict
      - check-success = "Go CI / Build & Test"
      - check-success = "Go CI / Lint"
      - check-success = "Python CI / Lint, Type-check & Test"
      - check-success = "Frontend CI / Lint & Build"
      - check-success = "Frontend CI / Playwright E2E"
      - check-success = "Docker Build & Push / Build go-api"
      - check-success = "Docker Build & Push / Build python-worker"
      - label != "needs-review"
    actions:
      merge:
        method: squash
        commit_message_template: "{{ title }} (#{{ number }})"

  - name: Flag Dependabot major bumps for manual review
    conditions:
      - author = dependabot[bot]
      - "label = version-update:semver-major"
    actions:
      label:
        add:
          - needs-review
      comment:
        message: "Major version bump — needs manual review before merging."
```

- [ ] **Step 3: Commit and push as a PR**

```bash
git checkout -b ci/add-mergify main
git pull origin main
# (write the file as above)
git add .mergify.yml
git commit -m "ci: add Mergify auto-update and auto-merge rules"
git push origin ci/add-mergify
gh pr create --title "ci: add Mergify auto-update and auto-merge rules" --base main
```

- [ ] **Step 4: Wait for CI, then let Mergify self-merge (or merge manually)**

  Since Mergify is now installed and `.mergify.yml` is in the PR, Mergify will evaluate this PR itself. Wait for all CI checks to pass:

```bash
gh pr checks <PR_NUMBER> --watch
gh pr merge <PR_NUMBER> --squash --delete-branch
```

---

## Task 9: Fix `claude.yml` permissions

**Files:**
- Modify: `.github/workflows/claude.yml:23`

- [ ] **Step 1: Change `pull-requests: read` to `pull-requests: write`**

  In `.github/workflows/claude.yml`, find the permissions block and change:

```yaml
      pull-requests: read
```

  to:

```yaml
      pull-requests: write
```

- [ ] **Step 2: Commit and push as a PR**

```bash
git checkout -b ci/claude-pr-write-permission main
git pull origin main
# (make the edit)
git add .github/workflows/claude.yml
git commit -m "ci: grant pull-requests write permission to Claude Code action"
git push origin ci/claude-pr-write-permission
gh pr create --title "ci: grant pull-requests write permission to Claude Code action" --base main
gh pr merge <PR_NUMBER> --squash --delete-branch
```

---

## Task 10: Create `CLAUDE.md`

**Files:**
- Create: `CLAUDE.md`

- [ ] **Step 1: Create `CLAUDE.md` at repo root**

```markdown
# Raven — Claude Code Instructions

## Pull Request Workflow

After creating a PR with `gh pr create`, **always** immediately queue it for auto-merge:

```bash
gh pr merge <PR_NUMBER> --auto --squash
```

This queues the PR to squash-merge automatically once all CI checks pass. Do not wait for CI — enqueue immediately after creation.

## Branch Naming

Use `type/descriptor` format:

| Type | When |
|------|------|
| `feat/` | New feature |
| `fix/` | Bug fix |
| `refactor/` | Code restructure, no behaviour change |
| `ci/` | CI/CD changes |
| `chore/` | Tooling, deps, config |
| `deps/` | Dependency bumps |

## Rules

- **Never push directly to `main`** — always use a PR
- **Squash merge only** — never regular merge or rebase-merge
- **Never use `--no-verify`** — all hooks must pass
```

- [ ] **Step 2: Commit and push as a PR**

```bash
git checkout -b ci/add-claude-md main
git pull origin main
# (write the file as above)
git add CLAUDE.md
git commit -m "docs: add CLAUDE.md with PR workflow instructions for agents"
git push origin ci/add-claude-md
gh pr create --title "docs: add CLAUDE.md with PR workflow instructions for agents" --base main
gh pr merge <PR_NUMBER> --squash --delete-branch
```

---

## Task 11: Smoke test

- [ ] **Step 1: Verify branch protection is active**

```bash
gh api /repos/ravencloak-org/Raven/branches/main/protection \
  --jq '{strict: .required_status_checks.strict, checks: .required_status_checks.contexts}'
```

  Expected: `strict: true`, all 7 check strings present.

- [ ] **Step 2: Open a test PR touching only Go files and verify all checks run**

```bash
git checkout -b test/pr-automation-smoke main
git pull origin main
echo "// smoke test" >> cmd/api/main.go
git add cmd/api/main.go
git commit -m "test: smoke test PR automation"
git push origin test/pr-automation-smoke
gh pr create --title "test: smoke test PR automation" --base main
gh pr merge <NUMBER> --auto --squash
```

  Verify:
  - Go CI runs (Build & Test, Lint)
  - Python CI / Lint, Type-check & Test shows ✅ (skip job)
  - Frontend CI / Lint & Build shows ✅ (skip job)
  - Frontend CI / Playwright E2E shows ✅ (skip job)
  - Docker Build & Push / Build go-api shows ✅ (skip job)
  - Docker Build & Push / Build python-worker shows ✅ (skip job)
  - Mergify auto-merges after all checks pass

- [ ] **Step 3: Close smoke-test branch**

  If Mergify merged it automatically, the branch was already deleted. Otherwise:

```bash
gh pr merge <NUMBER> --squash --delete-branch
```

- [ ] **Step 4: Revert the smoke test commit on main**

```bash
git checkout -b ci/revert-smoke-test main
git pull origin main
git revert HEAD --no-edit
git push origin ci/revert-smoke-test
gh pr create --title "ci: revert smoke test commit" --base main
gh pr merge <NUMBER> --squash --delete-branch
```
