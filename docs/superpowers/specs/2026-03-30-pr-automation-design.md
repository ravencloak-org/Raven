# PR Automation Design

**Date:** 2026-03-30
**Status:** Approved

## Problem

PRs created by Claude Code agents and Dependabot sit open indefinitely after CI passes because:
- No auto-merge is configured
- No branch protection gates the merge
- Stale PRs accumulate conflicts with `main` and require manual rebase

## Goal

Every PR — from Claude agents or Dependabot — merges automatically once CI passes, without human intervention. Conflicts are resolved automatically.

## Design

### 1. GitHub Repo Settings

Enable via `gh api PATCH /repos/ravencloak-org/Raven`:
- `allow_auto_merge: true` — required for `gh pr merge --auto` to work
- `delete_branch_on_merge: true` — auto-deletes feature branches after squash merge

### 2. CI Workflow Path-Filter Refactor

**Problem:** All four CI workflows use workflow-level `paths:` triggers. When a PR doesn't touch a workflow's paths, the workflow never runs and GitHub shows no status. Branch protection's required checks then wait forever for a check that will never arrive, deadlocking the merge.

**Fix:** Convert all four workflows from workflow-level `paths:` triggers to always-triggered workflows with a `changes` detector job using `dorny/paths-filter`. Each real job gates itself on the detector output. A matching skip job reports instant success under the same name when paths didn't change.

Pattern (applied to `go.yml`, `python.yml`, `frontend.yml`, `docker.yml`):

```yaml
# 1. Remove paths: from the on: trigger — workflow always runs on PRs to main

# 2. Add a changes detector job (first job):
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
              - 'cmd/**'      # adjust per workflow
              - 'internal/**'
              - 'go.mod'
              - 'go.sum'
              - '.github/workflows/go.yml'

  # 3. Real job — only runs when changes detected:
  build-and-test:
    name: Build & Test
    needs: changes
    if: needs.changes.outputs.src == 'true'
    ...

  # 4. Skip job — same name as real job, runs when no changes:
  build-and-test-skip:
    name: Build & Test      # must match real job name exactly
    needs: changes
    if: needs.changes.outputs.src != 'true'
    runs-on: ubuntu-latest
    steps:
      - run: echo "No relevant changes, skipping"
```

Apply the same pattern to every named job in each workflow.

### 3. Branch Protection on `main`

Configure via `gh api PUT /repos/ravencloak-org/Raven/branches/main/protection`.

**Required status checks** (exact GitHub check strings — `{workflow name} / {job name}`):

| Workflow | Check string |
|---|---|
| Go CI | `Go CI / Build & Test` |
| Go CI | `Go CI / Lint` |
| Python CI | `Python CI / Lint, Type-check & Test` |
| Frontend CI | `Frontend CI / Lint & Build` |
| Frontend CI | `Frontend CI / Playwright E2E` |
| Docker Build & Push | `Docker Build & Push / Build go-api` |
| Docker Build & Push | `Docker Build & Push / Build python-worker` |

> **Bootstrap note:** The Docker CI job name is a matrix expansion (`Build ${{ matrix.image }}`). GitHub only allows adding a check string to branch protection after at least one workflow run has produced it. Before configuring branch protection, trigger one Docker CI run (e.g. push a trivial Dockerfile comment change) so GitHub registers `Build go-api` and `Build python-worker` as known check strings.

Other settings:
- **Require branch to be up to date** before merging
- **No required approvals** — CodeRabbit "request changes" does not block the merge; only CI does
- **Admin bypass** allowed for emergencies

### 4. Mergify (`.mergify.yml`)

Rules are evaluated in order — first match wins.

**Rule 1 — Auto-update (conflict prevention, runs first)**
```yaml
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
```

**Rule 2 — Auto-merge (fires after Rule 1 keeps PR up to date)**
```yaml
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
```

**Rule 3 — Dependabot major-version safety gate**
```yaml
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

Dependabot sets `version-update:semver-major` on major bump PRs. Rule 3 catches these and adds `needs-review`, which Rule 2's `label != "needs-review"` condition then blocks. Minor/patch Dependabot PRs fall through to Rule 2 normally.

### 5. `claude.yml` Permission Fix

The Claude Code GitHub Action workflow grants `pull-requests: read` (line 23). Change to `pull-requests: write` so the agent can call `gh pr merge --auto` from within a workflow run. `opencode.yml` only comments and does not create or merge PRs — no change needed there.

### 6. CLAUDE.md (repo root)

Loaded automatically by all Claude Code agents (local sessions, worktree agents, GitHub Action).

Key instructions:
- After `gh pr create`, always immediately run `gh pr merge --auto --squash` on the same PR number
- Branch naming: `type/descriptor` (feat, fix, refactor, ci, chore, deps)
- Never push directly to `main`
- Squash merge only — never regular merge or rebase-merge

## Components Changed

| File | Change |
|------|--------|
| `CLAUDE.md` | New — PR creation instructions for all agents |
| `.mergify.yml` | New — auto-update + auto-merge rules |
| `.github/workflows/go.yml` | Refactor to `changes` detector + skip jobs |
| `.github/workflows/python.yml` | Refactor to `changes` detector + skip jobs |
| `.github/workflows/frontend.yml` | Refactor to `changes` detector + skip jobs |
| `.github/workflows/docker.yml` | Refactor to `changes` detector + skip jobs |
| `.github/workflows/claude.yml` | `pull-requests: read` → `write` |
| GitHub branch protection | New — required CI checks on `main` |
| GitHub repo settings | Enable auto-merge + delete-branch-on-merge |

## Implementation Order

1. Refactor CI workflows (path-filter bypass) and push — this must land on `main` first
2. Trigger a Docker CI run to register matrix check strings with GitHub
3. Enable repo settings (auto-merge, delete-branch-on-merge) via `gh api`
4. Configure branch protection via `gh api`
5. Create `.mergify.yml` and install Mergify GitHub App
6. Fix `claude.yml` permissions
7. Create `CLAUDE.md`

## Out of Scope

- CodeRabbit auto-response (CodeRabbit comments are not a merge gate)
- Security workflow checks (runs on schedule, not intended as a merge gate)
