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
