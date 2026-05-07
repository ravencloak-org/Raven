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

## Commit Style

- **No AI attribution** — do not add `Co-Authored-By:` trailers to any commit message
- Commits should appear as authored by the repo owner only

## Rules

- **Never push directly to `main`** — always use a PR
- **Squash merge only** — never regular merge or rebase-merge
- **Never use `--no-verify`** — all hooks must pass
- **Never amend published commits** — create new commits instead

## Memory: Stash MCP

Raven uses the Stash MCP server (`mcp__stash__*`) for persistent, cross-session memory. Treat it as the source of truth for project history, working rules, and decisions.

### Session-start protocol

1. `mcp__stash__init` — idempotent; ensures `/self` scaffold exists.
2. `mcp__stash__get_context` on `/projects/raven` — pick up any in-progress focus.
3. `mcp__stash__recall` with a query relevant to the user's first message, scoped to `/projects/raven` (recursive — covers all sub-namespaces). If the user's message is generic, also recall from `/users/jobinlawrance`.
4. If recall returns nothing on a topic, say so explicitly — do NOT answer from training data and pretend to remember.

### Namespace map

- `/users/jobinlawrance` — Jobin's profile, hardware, environment, payment/region constraints.
- `/projects/raven` — tech stack, edge-deployment requirements, compliance posture, Phase 2 features.
- `/projects/raven/auth` — Keycloak current state, Zitadel post-mortem, pluggable-auth direction.
- `/projects/raven/ai-worker` — Python gRPC service (port 50051), parsing, embeddings, RAG, voice.
- `/projects/raven/infra` — Compose variants, deployment, observability (OpenObserve + Beszel).
- `/projects/raven/milestones` — delivered + active queue (currently M9: #256, #257, #258).
- `/projects/raven/feedback` — standing rules: squash merge, no AI attribution, lint-before-push, testing gates, open-PR dependency, milestone protocol, respect tech choices.

### When to write

Call `mcp__stash__remember` (with the matching namespace) any time:

- A decision is finalised — technical, product, or process.
- The user states a preference, constraint, or correction.
- A milestone or issue changes state (started, blocked, shipped).
- You discover a non-obvious fact about the codebase, infra, or a third-party tool that future sessions would have to rediscover.
- You complete a session — write a one-line summary of what was done.

Use `mcp__stash__create_goal` for milestone-level intent and `mcp__stash__create_failure` for things that didn't work (e.g. the Zitadel decision). Use `mcp__stash__set_context` on `/projects/raven` when starting focused work, so the next session can pick up via `get_context`.

### Discipline

- Before writing to a namespace, verify it exists with `list_namespaces`. If missing, `create_namespace` first.
- Write complete self-contained sentences in `remember` — readable with zero prior context.
- Don't double-write the same fact to both `MEMORY.md` (built-in auto-memory) and Stash. Built-in memory holds terse rules loaded every turn; Stash holds project history, episodes, and goal/failure tracking queried on demand.
