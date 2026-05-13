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
- **Every commit MUST be signed off** — append `Signed-off-by: Jobin Lawrance <jobinlawrance@gmail.com>` (use `git commit -s`). The DCO CI check enforces this; missing trailers block the PR.

## Rules

- **Never push directly to `main`** — always use a PR
- **Squash merge only** — never regular merge or rebase-merge
- **Never use `--no-verify`** — all hooks must pass
- **Never amend published commits** — create new commits instead. Exception: rewriting a PR branch's own history to add missing `Signed-off-by` trailers is acceptable (force-push with `--force-with-lease`).

## Local hooks

The repo ships two hooks under `.githooks/` that mirror CI checks. Install both at once with:

```bash
git config core.hooksPath .githooks
```

| Hook | Runs | Checks |
|------|------|--------|
| `pre-commit` | every `git commit` | `golangci-lint run --new-from-rev=HEAD ./...` when staged changes include `.go` files. Only NEW violations introduced by this commit fail the hook; pre-existing repo-wide debt is tracked separately. Matches CI's `only-new-issues: true`. |
| `pre-push` | every `git push` | every commit being sent carries a `Signed-off-by:` trailer |

`pre-commit` requires **golangci-lint v2.11.4** (pinned to match CI). Install the exact version:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
```

The hook will error if a different version is detected, to prevent local/CI drift.

To retroactively sign off a branch that was made before the hook existed:

```bash
git rebase origin/main --exec 'git commit --amend --no-edit -s'
git push --force-with-lease
```

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

## Agent skills

### Issue tracker

Issues live in GitHub Issues at `ravencloak-org/Raven` via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: `CONTEXT.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

# context-mode — MANDATORY routing rules

You have context-mode MCP tools available. These rules are NOT optional — they protect your context window from flooding. A single unrouted command can dump 56 KB into context and waste the entire session.

## BLOCKED commands — do NOT attempt these

### curl / wget — BLOCKED
Any Bash command containing `curl` or `wget` is intercepted and replaced with an error message. Do NOT retry.
Instead use:
- `ctx_fetch_and_index(url, source)` to fetch and index web pages
- `ctx_execute(language: "javascript", code: "const r = await fetch(...)")` to run HTTP calls in sandbox

### Inline HTTP — BLOCKED
Any Bash command containing `fetch('http`, `requests.get(`, `requests.post(`, `http.get(`, or `http.request(` is intercepted and replaced with an error message. Do NOT retry with Bash.
Instead use:
- `ctx_execute(language, code)` to run HTTP calls in sandbox — only stdout enters context

### WebFetch — BLOCKED
WebFetch calls are denied entirely. The URL is extracted and you are told to use `ctx_fetch_and_index` instead.
Instead use:
- `ctx_fetch_and_index(url, source)` then `ctx_search(queries)` to query the indexed content

## REDIRECTED tools — use sandbox equivalents

### Bash (>20 lines output)
Bash is ONLY for: `git`, `mkdir`, `rm`, `mv`, `cd`, `ls`, `npm install`, `pip install`, and other short-output commands.
For everything else, use:
- `ctx_batch_execute(commands, queries)` — run multiple commands + search in ONE call
- `ctx_execute(language: "shell", code: "...")` — run in sandbox, only stdout enters context

### Read (for analysis)
If you are reading a file to **Edit** it → Read is correct (Edit needs content in context).
If you are reading to **analyze, explore, or summarize** → use `ctx_execute_file(path, language, code)` instead. Only your printed summary enters context. The raw file content stays in the sandbox.

### Grep (large results)
Grep results can flood context. Use `ctx_execute(language: "shell", code: "grep ...")` to run searches in sandbox. Only your printed summary enters context.

## Tool selection hierarchy

1. **GATHER**: `ctx_batch_execute(commands, queries)` — Primary tool. Runs all commands, auto-indexes output, returns search results. ONE call replaces 30+ individual calls.
2. **FOLLOW-UP**: `ctx_search(queries: ["q1", "q2", ...])` — Query indexed content. Pass ALL questions as array in ONE call.
3. **PROCESSING**: `ctx_execute(language, code)` | `ctx_execute_file(path, language, code)` — Sandbox execution. Only stdout enters context.
4. **WEB**: `ctx_fetch_and_index(url, source)` then `ctx_search(queries)` — Fetch, chunk, index, query. Raw HTML never enters context.
5. **INDEX**: `ctx_index(content, source)` — Store content in FTS5 knowledge base for later search.

## Subagent routing

When spawning subagents (Agent/Task tool), the routing block is automatically injected into their prompt. Bash-type subagents are upgraded to general-purpose so they have access to MCP tools. You do NOT need to manually instruct subagents about context-mode.

## Output constraints

- Keep responses under 500 words.
- Write artifacts (code, configs, PRDs) to FILES — never return them as inline text. Return only: file path + 1-line description.
- When indexing content, use descriptive source labels so others can `ctx_search(source: "label")` later.

## ctx commands

| Command | Action |
|---------|--------|
| `ctx stats` | Call the `ctx_stats` MCP tool and display the full output verbatim |
| `ctx doctor` | Call the `ctx_doctor` MCP tool, run the returned shell command, display as checklist |
| `ctx upgrade` | Call the `ctx_upgrade` MCP tool, run the returned shell command, display as checklist |
