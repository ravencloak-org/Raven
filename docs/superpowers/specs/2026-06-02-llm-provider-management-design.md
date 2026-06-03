# M13 — LLM Provider Management UI

**Status:** Draft (review pending)
**Date:** 2026-06-02
**Milestone:** M13 — AI Provider Management UI

## Problem

Today the `/llm-providers` route lets a user **Create** and **Delete** providers but offers no way to:

- **Switch** the default provider between existing rows. The backend has `PUT /llm-providers/:id/default` and the store wraps it, but the list page only auto-defaults on the *first* create. A user with two providers cannot flip between them without deleting one.
- **Edit** existing providers. Rotating an API key, fixing a stale Ollama tunnel URL, or renaming a provider all require delete-and-recreate today.
- **See the current default at a glance.** The card layout does not surface `is_default`.
- **Reach the page from the user-profile menu.** `AppHeader.vue` dropdown contains only "Sign out". Discovery today is via Settings → button or the mobile tab bar.

## Goals

1. Make the default provider switchable from the list page with one click and an obvious visual indicator.
2. Allow editing of existing providers in-place where safe, in a dialog where credentials are involved.
3. Wire entry points so the page is reachable from the user-profile dropdown (and the desktop sidebar).
4. Preserve every existing safety net: Test Connection gate, write-only API keys, encrypted-at-rest storage, RLS scoping.

## Non-Goals

- Per-purpose default split (chat-default vs embed-default). See ADR-0009. Single default per Org stays.
- Per-User override of the Org default. The model is "Org owns its config" per CONTEXT.md.
- Quick-switch dropdown in the global header. YAGNI for now; one click to the list page is fast enough.
- RBAC tightening (admin-only mutations). Current behavior — any Org member can mutate — is preserved; tracked as a follow-up.
- Persisting last-tested-at / last-test-status on the provider row. Surface them in the in-flight UI only; schema work deferred.

## Decisions (grilled, all locked)

| # | Question | Decision | Reason |
|---|---|---|---|
| Q1 | Default scope | **Single org-wide default** | Schema + code already assume it. Chat/embed leak bug is a model-picker problem, not a provider-scope problem. ADR-0009. |
| Q2 | Switch action UX | **Instant toggle, optimistic update, rollback toast** | Cheap, reversible, no data loss; existing Create / Delete confirms already saturate the page's modal budget. |
| Q3 | Edit-in-place shape | **Hybrid — inline for safe fields, modal for credentials** | Honors "edit-in-place" spirit (rename, swap model in one click) while preserving the Test-Connection gate for `api_key` / `base_url`. |
| Q4 | API key edit semantics | **Explicit "Rotate API key" disclosure** | Zero ambiguity vs masked-placeholder; no secret-shaped noise on routine edits; aligns with secret-rotation mental model. |
| Q5 | Default visual indicator | **Amber "Default" pill on the card** + "Make default" button on non-default cards | Distinct from green `status=active` badge; pill rather than star/radio for legibility. |
| Q6 | Status & health display | **Keep existing status badge as-is** | Persisting `last_tested_at` is schema work; defer. |
| Q7 | Menu placement | **AppHeader dropdown (above Sign out) + AppSidebar (desktop)** | Two entry points for two contexts. Settings page button stays as the third path. |
| Q8 | RBAC | **No change (any Org member can mutate)** | Track admin-only gate as a separate follow-up issue. |
| Q9 | Header quick-switch dropdown | **No** | YAGNI. |

## Architecture

### Backend (Go, `internal/handler/llm_provider.go` + `internal/service/llm_provider.go`)

Two changes:

1. **`PUT /api/v1/orgs/:org_id/llm-providers/:provider_id`** already exists per `llm_provider_test.go:196`. Audit the handler to confirm it accepts a partial body (only mutates fields present) and never overwrites `api_key` when omitted. Add a regression test that omits `api_key` and asserts the stored ciphertext is unchanged.
2. **`POST /api/v1/orgs/:org_id/llm-providers/test`** extends to accept `{provider_id}` *instead of* credentials. When `provider_id` is set, the handler MUST: (a) verify the caller is a member of `:org_id` (existing session middleware), (b) load the provider row through an org-scoped query (RLS-enforced via `db.WithOrgID`, or an explicit `WHERE org_id = :org_id AND id = :provider_id`), (c) return 404 if not found and 403 if the membership check fails, (d) **only then** decrypt the stored key and run the probe. Without this order, the endpoint becomes a cross-tenant probing/decryption oracle if an IDOR slips into the handler. This is what the Edit dialog uses when the user hasn't clicked "Rotate API key".

### Frontend (Vue 3, `frontend/src/pages/llm-providers/LlmProviderListPage.vue`)

The page is rewritten around three pieces:

1. **Provider card** (one per row, same component for desktop and mobile, responsive Tailwind):
   - Header: `display_name` (inline-editable), provider type icon, status badge, **Default pill** (if `is_default`).
   - Body: provider type label, `base_url` (when set), model (inline `<select>` from `PROVIDER_MODELS[type]`), API key hint ("configured" / "not set").
   - Action row: `Make default` button (hidden on the default card), `Edit credentials` button (opens dialog), `Delete` button.
   - Inline edits (display_name, model) autosave on blur via `updateLlmProvider` and show a brief "Saved" tick.

2. **Edit credentials dialog** — same component shape as the Create dialog, two modes:
   - Default mode: `base_url` editable; **"Rotate API key"** button is the only way to reveal an api_key input.
   - When `base_url` changes OR the rotate input has a value, the dialog **re-runs Test Connection** before allowing Save (mirrors Create's gate, line 215+ in current file).
   - When neither changed, Save is unblocked and `api_key` is omitted from the PUT body — backend leaves the stored key untouched.

3. **Make-default action**:
   - Optimistic store mutation: flip `is_default` locally, call `setDefaultLlmProvider`, on failure roll back + toast.
   - The previously-default card's pill disappears with a 200ms fade; the new default card's pill fades in.
   - A non-blocking warning banner appears inside the card if the user clicks Make-default on a provider whose last in-session test was `fail` (we only know in-session state until Q6 schema work lands).

### Menu wiring

- **`AppHeader.vue`** (`components/AppHeader.vue`): add a `<RouterLink to="/llm-providers">` above the Sign-out button inside the menu. Icon: small chip glyph (Heroicons `cpu-chip` or equivalent inline SVG to match the rest of the file's stroked SVGs).
- **`AppSidebar.vue`** (`components/AppSidebar.vue`): add an "AI Providers" `<RouterLink>` between the existing main-nav items and the Account section. Title `AI Providers`, active-class same as siblings.

## Data flow

```text
                                ┌───────────────────────┐
                                │   /llm-providers      │
                                │   LlmProviderListPage │
                                └────────┬──────────────┘
                                         │
                                         ▼
                            useLlmProvidersStore (Pinia)
                                         │
            ┌────────────────────────────┼─────────────────────────────┐
            ▼                            ▼                             ▼
   GET  /llm-providers       PUT /llm-providers/:id        PUT /llm-providers/:id/default
   (list, on mount)          (inline + dialog edits)       (Make default, optimistic)

                            POST /llm-providers/test
                            { provider_id }  ← new shape, used by Edit dialog
                            { provider, base_url, api_key } ← existing shape (Create + Rotate)
```

## Glossary additions (in this PR)

CONTEXT.md gains:

- **LLM Provider**: A configured connection (provider type + `base_url` + credentials + a default model) that an Org uses to serve chat and embedding requests. An Org has zero or more.
- **Default Provider**: The single LLM Provider with `is_default = true`. All chat, embedding, and RAG calls resolve through it unless overridden by an explicit provider slug in the call.
- **Provider Type**: The vendor family — one of `openai`, `anthropic`, `ollama`, `custom`. Editable only by deleting and recreating.

## Out-of-scope follow-ups

- Persist `last_tested_at` + `last_test_status` on the provider row (schema work).
- Admin-only RBAC gate on mutations.
- Per-purpose default split (chat-default vs embed-default). Add a `default_for ENUM` only if a real product need surfaces.

## Issue breakdown (tracer-bullet vertical slices)

Six issues, each independently grabbable and behind-a-flag-free.

### A — Backend: provider Update audit + Test-with-stored-key

- Audit `PUT /llm-providers/:id` to confirm it preserves `api_key` when omitted; add regression test.
- Extend `POST /llm-providers/test` to accept `{provider_id}`; when set, decrypt stored key and probe.
- Unit tests + handler test.

### B — Frontend: Default badge + Make-default action

- "Default" amber pill component on `LlmProviderListPage` cards.
- "Make default" button on non-default cards.
- Optimistic store mutation; rollback toast on failure.
- Warning inline-banner if target's in-session test was `fail`.

### C — Frontend: Edit credentials dialog (hybrid inline + modal)

- Reuse Create-dialog markup as the Edit dialog, pre-filled from card.
- "Rotate API key" disclosure (hidden input until clicked).
- Re-run Test Connection when `base_url` or rotate input changed; gate Save.
- Omit `api_key` from PUT body when not rotated.

### D — Frontend: Menu entries

- Add "AI Providers" link to `AppHeader.vue` user-profile dropdown (above Sign out).
- Add "AI Providers" link to `AppSidebar.vue` desktop nav.
- Icons match existing inline-SVG style.

### E — Frontend: Card redesign + inline edits

- Refactor desktop + mobile card variants into one responsive component.
- Inline-editable `display_name` (click-to-edit, autosave on blur).
- Inline-selectable `model` (`<select>` from `PROVIDER_MODELS[type]`, autosave on change).
- Show provider type icon, `base_url`, key hint, status, default pill.

### F — E2E Playwright coverage

- Switch default between two providers; assert pill moves; assert chat call resolves to the new default.
- Rename inline; reload; assert persisted.
- Change model inline; reload; assert persisted.
- Rotate key; assert Test-Connection re-gate; assert Save unlocked only after pass.
- Cancel-and-rollback toast on a deliberately failing default switch (mocked).

## Success criteria

- User can flip the default between providers in one click and see the pill move within 200ms (optimistic).
- User can rename a provider, change its model, and rotate its API key without leaving the list page.
- "AI Providers" appears in both the user-profile dropdown and the desktop sidebar.
- All existing functionality (Create flow with Test Connection gate, Delete with confirm) is untouched.
- Frontend lint + golangci-lint pass with `--new-from-rev=HEAD` (no new violations).
- New Playwright scenarios pass headless in CI.
