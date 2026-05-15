# Demo App Features — Codebase Inventory

Audit date: 2026-05-13. Recorded before implementing the rest of plan #3.

## Insertion points

| Concept | File / path | Notes |
|---|---|---|
| API bootstrap (Gin engine, route registration) | `cmd/api/main.go` (line 506 `router := gin.Default()`) | All middlewares + routes assembled here. New DSAR / admin routes mount here. |
| SuperTokens recipe init | `internal/auth/supertokens_init.go` (function `InitSuperTokens`, line 24) | Currently builds `recipeList` with `thirdparty` + `session`. No user-create hook exists yet — to fire the sample-workspace seed on signup we need to override `RecipeImpl` and wrap `SignInUpPOST` (or equivalent) and call our seeder after a successful sign-up. |
| Auth proxy / SuperTokens HTTP catchall | `cmd/api/main.go:543` (`router.Any("/auth/*path", handler.SuperTokensMiddleware())`) | Turnstile gate has to wrap this — either add a Gin middleware in front of the same `/auth/*` group, or check `cf-turnstile-token` inside `handler.SuperTokensMiddleware()`. |
| Python worker LLM call sites | `ai-worker/raven_worker/agent.py`, `ai-worker/raven_worker/services/rag_service.py` (Anthropic `messages.stream`, OpenAI `chat.completions.create`) | LLM-fuse guard wraps each call site. |
| Frontend signup page | `frontend/src/pages/LoginPage.vue` | Combined login/signup page. Turnstile widget mounts here. |
| Frontend app shell | `frontend/src/App.vue` (verify exact path) | Cookie banner mounts here. |
| Voice surfaces in admin dashboard | `frontend/src/components/chat-widget/RavenChat.ce.ts` already has `voice-enabled` attr; full sweep needed across `frontend/src/pages/voice*`, `frontend/src/components/voice*` — see `rg voice frontend/src --type vue` |

## Open questions surfaced by the audit

1. **SuperTokens user-create hook.** No existing override in `supertokens_init.go`. Adding a `ThirdParty.Override.Functions` block with a wrapper around `SignInUp` is the canonical pattern, but it requires careful handling of the `recipeImpl` pattern. This is the main risk in plan #3 Task 14 (sample-workspace seed) and Task 13 (admin endpoint for retention).
2. **Turnstile placement.** The signup endpoint is `/auth/signup` (proxied to SuperTokens). The cleanest place to verify the token is inside `handler.SuperTokensMiddleware()` (per-request, before forwarding to SuperTokens) rather than as a separate Gin middleware that wouldn't see all signup paths.
3. **Mail bootstrap.** API DI pattern is constructor-injection through `main.go`. The `MailSender` follows the existing pattern of `apiKeyRepo`, `notifPrefsHandler`, etc.
4. **Frontend runtime config.** The frontend already accepts `VITE_*` env at build time. For runtime overrides (e.g. `RAVEN_TURNSTILE_SITE_KEY` injected via Docker), need to confirm whether a `/config.json` endpoint or build-arg pattern is used. Inspect the existing Dockerfile.

## Plan #3 task readiness

- **Tasks 2–4 (MailSender):** straightforward Go new-package work. No codebase coupling.
- **Tasks 5–7 (Turnstile):** need to decide on the SuperTokens placement (open question 2).
- **Tasks 8–9 (LLM fuse):** straightforward Python work, but each call site in `agent.py` / `rag_service.py` needs the same wrap. Cost-estimation function can be coarse for the demo.
- **Tasks 10–12 (DSAR):** plus `scheduled_deletes` migration. `account.SQLRepo` is the main effort.
- **Task 13 (retention purge):** depends on Task 12's repo to satisfy `RetentionRepo`. Admin endpoint authn pattern needs an `adminOnlyMiddleware` — verify whether one already exists.
- **Task 14 (sample-workspace seed):** blocked on open question 1 (SuperTokens hook).
- **Tasks 15–16 (legal pages + cookie banner):** docs / frontend only.
- **Task 17 (voice hide):** frontend sweep + runtime config (open question 4).
