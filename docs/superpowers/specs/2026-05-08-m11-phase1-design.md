# Raven Local — Phase 1 Design (M11: Ollama + Wizard + Installer)

**Date:** 2026-05-08
**Status:** Approved (awaiting implementation plan)
**Owner:** Jobin Lawrance
**Milestone:** [M11 — Raven Local (Desktop)](https://github.com/ravencloak-org/Raven/milestone/14)
**Issues covered:** [#421](https://github.com/ravencloak-org/Raven/issues/421), [#422](https://github.com/ravencloak-org/Raven/issues/422), [#423](https://github.com/ravencloak-org/Raven/issues/423)
**Phase 0 spec:** none (Phase 0 issues #417–#420 shipped without a unified spec)
**Wiki page:** [Raven-Local](../../wiki/Raven-Local.md)

## Context

Phase 0 (issues #417–#420, all merged to `main`) shipped the foundation: Tauri 2 shell, full Docker compose lifecycle (detect → up → healthz polling → log streaming → clean shutdown), system-requirements precheck with bypass env, and single-user mode (`RAVEN_SINGLE_USER=true`) on the API side.

Phase 1 closes the loop on **first-run user experience**: the desktop installer launches, runs precheck, brings compose up, and walks the user through picking a local LLM (Ollama-served), optionally adding BYOK keys, and naming a workspace. After Phase 1, a user can install Raven Local from a `.dmg`/`.msi`/`.AppImage`, launch it, and have a working chat-with-documents experience without ever creating an account or using a cloud provider.

## Goals

1. Bundle Ollama as a sidecar in Raven Local's compose stack, accessible to both chat (via the existing BYOK pipeline) and embeddings (via a new Python provider).
2. First-run wizard tailored for single-user desktop: model picker (with RAM-tier recommendations), optional BYOK, default workspace.
3. Tag-triggered installer pipeline producing universal-macOS, Windows, and Linux artefacts (unsigned).

## Non-Goals (explicitly deferred)

- Code signing / notarization for any platform — tracked by #426.
- Settings panel for changing model / BYOK keys post-onboarding — tracked by #424.
- Tray / menubar status app — tracked by #425.
- Auto-update channel — tracked by #426.
- Cross-platform native CI runners (separate macOS/Windows/Linux jobs already exist; consolidating into a release pipeline is part of #427, but is also covered partially in #423 below).
- Migrating the existing cloud onboarding wizard to a refactored composable structure — defer until a third onboarding flow needs it.

## Key locked decisions (from brainstorming)

| Decision | Choice | Why |
|----------|--------|-----|
| Embeddings in single-user mode | Bundled local Ollama embeddings (`nomic-embed-text`) | Fully offline RAG; +280 MB initial download is acceptable. Alternative BYOK-required would break the privacy-first story. |
| Default chat model | RAM-tier auto-pick at first-run | 8 GB → `llama3.2:3b`, 12–16 GB → `llama3.1:8b`, 16 GB+ → user-choice between `8b` and `13b`. Single global default would be wrong on the 8 GB floor. |
| Wizard structure | Extend existing `OnboardingWizard.vue` with conditional steps | Smaller diff than building a parallel desktop wizard; risk that the file grows is mitigated by extracting the new steps into single-purpose child components in the same module. |
| macOS bundle | Universal binary (arm64 + x86_64) | One `.dmg` for all Macs; standard macOS-app expectation. Slower CI but acceptable. |
| Spec scope | One unified Phase 1 spec | Issues #421/#422/#423 share the user-facing flow. Implementation plan can still produce three PRs (one per issue) for clean review. |

## Architecture

### Component diagram (additions on top of Phase 0)

```
                    ┌──────────────────────────┐
                    │  Tauri shell (Rust)      │
                    │   ├─ ollama.rs (NEW)     │   pull progress events
                    │   │   ├─ pull_model      │ ────────────────────────►
                    │   │   ├─ list_models     │
                    │   │   └─ event stream    │
                    │   └─ existing modules    │
                    │       (compose,          │
                    │        precheck)         │
                    └────────────┬─────────────┘
                                 │  HTTP (127.0.0.1:11434)
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│  Docker compose (desktop variant)                               │
│  ├─ Go API (Gin) ─── single-user, reads llm_provider_configs    │
│  │       │                                                      │
│  │       ▼ HTTP (OpenAI-compatible) for chat                    │
│  │       └─────────────────────────────────►┐                   │
│  │                                          ▼                   │
│  ├─ Python AI worker (gRPC) ─── OllamaProvider for embeddings   │
│  │       │                                  │                   │
│  │       ▼ HTTP (/api/embeddings)           │                   │
│  │       └─────────────────────────────────►│                   │
│  │                                          ▼                   │
│  ├─ Ollama sidecar (NEW)        ◄───── chat + embeddings        │
│  │   port 11434, models in named volume                         │
│  │                                                              │
│  └─ existing services (Postgres, Valkey, SuperTokens, etc.)     │
└─────────────────────────────────────────────────────────────────┘
                                 ▲
                                 │  /api/v1/* (single-user mode)
                                 │
                    ┌────────────┴─────────────┐
                    │  Vue frontend            │
                    │   └─ OnboardingWizard    │
                    │       (extended with     │
                    │        ModelPickerStep)  │
                    └──────────────────────────┘
```

### Service additions

**`desktop/docker-compose.local.yml`** replaces the placeholder include from Phase 0 with a real override that adds:

```yaml
services:
  ollama:
    image: ollama/ollama:latest
    pull_policy: missing
    volumes:
      - ollama-models:/root/.ollama
    networks:
      - raven-internal
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "ollama", "list"]
      interval: 10s
      timeout: 5s
      retries: 5
    # Port exposed only to the Tauri host so the shell can pull models;
    # other compose services reach it via the internal network.
    ports:
      - "127.0.0.1:11434:11434"

  go-api:
    environment:
      RAVEN_SINGLE_USER: "true"

volumes:
  ollama-models:

include:
  - ../docker-compose.yml
```

The host-bound port (`127.0.0.1:11434`) lets the Tauri Rust process reach Ollama for pulls without going through the Go API. Other services use the internal `raven-internal` network.

### `llm_provider_configs` seed

Migration `migrations/00039_seed_ollama_local_provider.sql`:

```sql
-- +migrate Up
-- Seed an "ollama" provider row for the local org (created in 00038 by #419)
-- pointing at the desktop Ollama sidecar's OpenAI-compatible endpoint.
-- Idempotent — safe to run on multi-user deployments where the local org
-- doesn't exist (the WHERE clause filters it out).

INSERT INTO llm_provider_configs (
    id, org_id, provider, display_name, api_key_encrypted, api_key_iv,
    api_key_hint, base_url, config, is_default, status, created_by
)
SELECT
    gen_random_uuid(),
    '00000000-0000-0000-0000-000000000001',  -- the local org from 00038
    'ollama',
    'Local Ollama',
    decode('', 'hex'),                        -- no key required
    decode('', 'hex'),
    'local',
    'http://ollama:11434/v1',
    '{"is_local": true}'::jsonb,
    true,
    'active',
    '00000000-0000-0000-0000-000000000002'   -- the local user from 00038
WHERE EXISTS (
    SELECT 1 FROM organizations WHERE id = '00000000-0000-0000-0000-000000000001'
)
ON CONFLICT DO NOTHING;

-- +migrate Down
-- Intentionally empty.
```

Verify the actual constraint set on `llm_provider_configs` during implementation (the `ON CONFLICT DO NOTHING` requires either a `UNIQUE` constraint covering `(org_id, provider)` or no conflict at all — the migration may need a `unique` index added if one doesn't exist).

### Python `OllamaProvider`

`ai-worker/raven_worker/providers/ollama_provider.py` implements the existing `EmbeddingProvider` base. Calls `POST http://ollama:11434/api/embeddings` with `{ "model": "nomic-embed-text", "prompt": "..." }` and returns the `embedding` array. Registry mapping in `registry.py` adds `"ollama"` → `OllamaProvider`. No API key required.

### Tauri Rust additions

`desktop/src-tauri/src/ollama.rs` — new module exposing two Tauri commands:

- `ollama_pull_model(model: String)` — POST to `http://127.0.0.1:11434/api/pull` with streaming response; emits `ollama:pull-progress` events `{ model, status, completed, total, percent }` for the wizard's progress bar.
- `ollama_list_models()` — GET `/api/tags`, returns the list of locally available models.

Wired into `main.rs`'s `invoke_handler`. Unit tests in the same file use a recorded fixture stream of Ollama's pull JSONL responses.

### Frontend wizard extension

`frontend/src/pages/onboarding/OnboardingWizard.vue` (existing component) gets driven by a new computed flag `singleUser` from `useServerConfigStore()`. Two new child components extracted into the same file:

- `<ModelPickerStep />` — listens to the Tauri `precheck:result` event from #420 and stores the host's RAM tier in a new `frontend/src/stores/precheck.ts` Pinia store; renders a 1-of-N model picker pre-selected per tier, with a "Pull model" button that invokes `ollama_pull_model` and listens for `ollama:pull-progress` to render the progress bar.
- `<OptionalBYOKStep />` — wraps the existing `LlmProviderListPage`'s "Add provider" form, makes it skippable.

When `singleUser=true`:
1. Skip the existing org-creation step.
2. Insert `<ModelPickerStep />` as step 1.
3. `<OptionalBYOKStep />` as step 2.
4. Workspace name step, default to "Personal".
5. Mark `onboarding` Pinia store complete; subsequent launches route directly to `/dashboard`.

When `singleUser=false` (cloud), the wizard's existing flow is unchanged.

### Installer packaging

`desktop/src-tauri/tauri.conf.json`:

```json
{
  "productName": "Raven Local",
  "identifier": "io.ravencloak.local",
  "version": "0.1.0",
  "bundle": {
    "active": true,
    "targets": ["dmg", "msi", "appimage", "deb"],
    "category": "Productivity",
    "shortDescription": "Self-contained Raven for local use",
    "longDescription": "Privacy-first desktop edition of Raven that runs the entire stack locally with Ollama as a bundled LLM provider. No cloud, no telemetry by default.",
    "copyright": "© 2026 Raven Contributors",
    "icon": [...]
  }
}
```

New workflow `.github/workflows/desktop-release.yml`:

- Trigger: tag `raven-local-v*` push, plus `workflow_dispatch` for testing.
- Three native-runner jobs in a matrix:
  - `macos-latest`: build with `--target universal-apple-darwin` (requires both arm64 and x86_64 Rust targets installed).
  - `windows-latest`: standard `cargo tauri build`.
  - `ubuntu-latest`: standard `cargo tauri build`.
- Each job uploads its bundle to a draft GitHub Release named after the tag.
- SLSA v3 provenance attestation per artefact (using the existing `actions/attest-build-provenance` pattern from `release.yml`).
- Build time per job: target ≤ 15 min with cargo cache.

The pre-existing `.github/workflows/desktop-build.yml` (PR-time, macOS-only) stays as fast PR feedback.

## Data flow — first launch

1. User installs Raven Local from the platform installer; launches the app.
2. Tauri's `setup` runs: precheck (#420) → if OK, `compose up -d` (#418) → poll healthz.
3. While compose comes up, frontend (loaded from compose'd Vue) fetches `/api/v1/config`; `singleUser=true` is detected; router redirects to `/onboarding`.
4. `<ModelPickerStep />` renders, pre-selecting `llama3.2:3b` for an 8 GB host. User accepts.
5. Frontend invokes `ollama_pull_model("llama3.2:3b")`; Rust streams pull-progress events; progress bar updates in the wizard.
6. After chat-model pull, the Python AI worker lazily pulls `nomic-embed-text` on first embedding request (or the wizard pulls it eagerly — see Open question below).
7. Optional BYOK step (skipped for purely local).
8. Workspace name (defaults to "Personal") → POST to `/api/v1/workspaces` → `onboarding` store marks done → redirect to `/dashboard`.
9. Subsequent launches: `singleUser=true` + onboarding done → straight to `/dashboard`.

## Error handling

| Failure | Behaviour |
|---------|-----------|
| Ollama unreachable on pull | Wizard shows "Cannot reach local model server. Retry?" with a Retry button. Tauri command returns a typed error; wizard maps it to a friendly message. |
| Pull fails midway (network/disk) | `ollama:pull-progress` event includes `status: "error"` with the underlying message; wizard shows it and offers Retry. Partial bytes are not deleted (Ollama resumes on retry). |
| Embedding model missing on first chat | `OllamaProvider` returns a typed `ModelNotPulledError`. Go API maps to HTTP 503 with `Retry-After: 30`. Frontend's chat panel offers an inline "Pull `nomic-embed-text`" button. |
| Compose service down (Ollama exited) | Existing `compose:status` event surfaces it; user sees the error in the splash/dashboard banner. |

## Configuration knobs (env, with defaults)

| Variable | Default | Used by |
|----------|---------|---------|
| `RAVEN_OLLAMA_BASE_URL` | `http://ollama:11434` | Python AI worker provider, Go API base_url override |
| `RAVEN_OLLAMA_DEFAULT_CHAT_MODEL` | `llama3.2:3b` | Wizard pre-selection fallback when RAM tier is unknown |
| `RAVEN_OLLAMA_DEFAULT_EMBEDDING_MODEL` | `nomic-embed-text` | Python provider; lazy-pull on first embedding |

## Testing strategy

### Per-issue test additions

**#421 (Ollama):**
- Python: `ai-worker/tests/test_ollama_provider.py` — mock `httpx` against `/api/embeddings`, assert request shape and response parsing.
- Go: `internal/repository/llm_provider_test.go` extended for `provider="ollama"` + `base_url` round-trip.
- Migration: `migrations/migrations_test.go` runs `00039` against a fresh DB and a single-user-seeded DB; asserts idempotency (running it twice doesn't error).
- Rust: `desktop/src-tauri/src/ollama.rs` — fixture-stream tests for the pull-progress parser; one for happy path, one for `error` line, one for partial JSON line.

**#422 (Wizard):**
- Frontend: Playwright e2e (`frontend/e2e/onboarding.local.spec.ts`) — boots compose with `RAVEN_SINGLE_USER=true`, mocks Tauri commands via the test bridge, walks through ModelPickerStep → optional BYOK → workspace, asserts dashboard reached.
- Component: existing OnboardingWizard tests extended for the `singleUser=true` branch; assert org-creation step is skipped.

**#423 (Installer):**
- CI: the new `desktop-release.yml` runs on every PR via `workflow_dispatch` (no real release artefacts uploaded — just builds for sanity). Tagged release exercises the upload path.
- Smoke: a final `gh release download` step verifies the artefacts are downloadable post-publish.

### Cross-cutting

- All commits signed off (`-s`); pre-commit + pre-push hooks active per the new `.githooks/`.
- `golangci-lint`, `ruff`, and `cargo clippy` all clean (the `cargo clippy` step gets added to `desktop-build.yml` as part of #423).

## Files added (Phase 1)

| Path | Issue |
|------|-------|
| `desktop/docker-compose.local.yml` (rewrite) | #421 |
| `ai-worker/raven_worker/providers/ollama_provider.py` | #421 |
| `ai-worker/tests/test_ollama_provider.py` | #421 |
| `migrations/00039_seed_ollama_local_provider.sql` | #421 |
| `desktop/src-tauri/src/ollama.rs` | #421 |
| `frontend/src/pages/onboarding/ModelPickerStep.vue` (or inline child) | #422 |
| `frontend/src/pages/onboarding/OptionalBYOKStep.vue` (or inline child) | #422 |
| `frontend/e2e/onboarding.local.spec.ts` | #422 |
| `.github/workflows/desktop-release.yml` | #423 |

## Files modified (Phase 1)

| Path | Issue | Change |
|------|-------|--------|
| `ai-worker/raven_worker/providers/registry.py` | #421 | Register `"ollama"` provider |
| `internal/repository/llm_provider.go` | #421 | Verify `"ollama"` provider with empty key is accepted |
| `desktop/src-tauri/src/lib.rs` | #421 | Re-export `ollama` module |
| `desktop/src-tauri/src/main.rs` | #421 | Register Tauri commands |
| `desktop/src-tauri/Cargo.toml` | #421 | Add deps if needed (likely none — reqwest already present) |
| `frontend/src/pages/onboarding/OnboardingWizard.vue` | #422 | Conditional branch on `singleUser` |
| `frontend/src/stores/server-config.ts` | #422 | Expose `singleUser` already; verify shape |
| `frontend/src/stores/onboarding.ts` | #422 | Track desktop-flow completion |
| `desktop/src-tauri/tauri.conf.json` | #423 | Bundle metadata polish |
| `.github/workflows/desktop-build.yml` | #423 | Add `cargo clippy` step |

## Rollout

1. Issue #421 first (Ollama backend pieces — Python provider, migration, Tauri commands, compose service). Independently mergeable.
2. Issue #423 next or in parallel (installer packaging — touches mostly Tauri config + new workflow). Independent of #421/#422.
3. Issue #422 last (wizard) — depends on #421's Tauri commands existing.
4. Each issue lands as its own PR with auto-merge enabled per the repo's CLAUDE.md policy. Squash merge only. No AI attribution. Every commit signed off (the active pre-push hook enforces this).

## Open questions resolved during brainstorming

- ~~Embeddings strategy~~ → Bundled local Ollama embeddings.
- ~~Default chat model~~ → RAM-tier auto-pick.
- ~~Wizard structure~~ → Extend existing.
- ~~macOS bundle~~ → Universal binary.
- ~~Spec scope~~ → One unified Phase 1 spec.

## Open questions deferred to implementation plan

- Whether to eagerly pull `nomic-embed-text` in the wizard (better UX, +280 MB extra time) or lazily on first chat-with-documents (faster wizard, possibly confusing first-chat error). Recommendation: **eager pull**, but the plan can revisit.
- Exact `unique` constraint shape on `llm_provider_configs` for the seed migration's `ON CONFLICT DO NOTHING`. Implementation plan surveys the existing constraints; if there's no suitable unique index, the migration adds one.
- macOS universal-binary build needs both `aarch64-apple-darwin` and `x86_64-apple-darwin` Rust targets installed on the runner. Plan confirms the right `rustup target add` commands and whether the existing macos-latest runner image preinstalls them.
