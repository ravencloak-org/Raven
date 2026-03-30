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

### 1. Branch Protection on `main`

Configure via `gh api PATCH /repos/ravencloak-org/Raven/branches/main/protection`:

- **Required status checks**: Go CI, Python CI, Frontend CI, Docker CI jobs (exact job names resolved at implementation)
- **Require branch to be up to date** before merging — ensures Mergify's rebase + CI re-run is the gate
- **No required approvals** — CodeRabbit "request changes" reviews do not block the merge; only CI does
- **Admin bypass** allowed for emergencies

### 2. Mergify (`.mergify.yml`)

Three rules:

**Rule 1 — Auto-merge (all PRs)**
Conditions: all required CI checks pass + no merge conflicts
Action: squash merge
Applies to: all PRs

**Rule 2 — Auto-update (conflict prevention)**
Conditions: PR is behind `main` + no merge conflicts
Action: rebase update
Applies to: all PRs
Effect: triggers fresh CI run; when CI passes, Rule 1 fires

**Rule 3 — Dependabot safety gate**
Conditions: author is `dependabot[bot]` + update type is `version-update:semver-patch` or `version-update:semver-minor`
Action: same as Rule 1 (squash merge)
Major version bumps: labelled `needs-review`, excluded from auto-merge

### 3. CLAUDE.md (repo root)

Loaded automatically by all Claude Code agents (local, worktree, GitHub Action).

Instructions:
- After `gh pr create`, always run `gh pr merge --auto --squash` on the same PR
- Branch naming: `type/descriptor` (feat, fix, refactor, ci, chore, deps)
- Never push directly to `main`
- Never use regular merge or rebase-merge — squash only

### 4. GitHub Repo Settings

Enabled via `gh api PATCH /repos/ravencloak-org/Raven`:
- `allow_auto_merge: true` — required for `gh pr merge --auto` to work
- `delete_branch_on_merge: true` — auto-deletes feature branches after squash merge

## Components Affected

| File | Change |
|------|--------|
| `CLAUDE.md` | New — PR creation instructions for all agents |
| `.mergify.yml` | New — auto-merge + auto-update rules |
| GitHub branch protection | New — required CI checks on `main` |
| GitHub repo settings | Updated — auto-merge + delete-branch-on-merge |

## Out of Scope

- CodeRabbit auto-response (CodeRabbit comments are not a merge gate)
- Major version Dependabot PRs (require manual review)
- PR creation scripts (agents already use `gh pr create`; CLAUDE.md adds the `--auto` step)
