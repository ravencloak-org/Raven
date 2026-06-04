# M14 — Passkey Authentication (WebAuthn)

**Status:** Draft (review pending)
**Date:** 2026-06-04
**Milestone:** M14 — Passkey Authentication

## Problem

Raven's only sign-in method today is Google ThirdParty via SuperTokens. This forces every user through a Google account, which:

- Excludes users without a Google identity (or who refuse to use one for this product).
- Adds an external dependency on Google availability and OAuth-consent UX for every sign-in.
- Doesn't allow a user to bind their account to a strong hardware credential (phishing-resistant).

Passkeys (WebAuthn) solve all three: they are phishing-resistant by construction, work without an external identity provider, and run inside the user's own authenticator (Touch ID, Windows Hello, security key).

## Goals

1. A logged-in user can enrol one or more passkeys for the current device via **Settings → Authentication**.
2. A user with an enrolled passkey can sign in by clicking **"Sign in with Passkey"** on the login page — no email entry, browser picks any matching credential for the origin.
3. Google ThirdParty sign-in continues to work unchanged; passkey is purely additive.
4. Lost-device recovery is implicit: sign in with Google, then re-enrol new passkeys.
5. Credentials are stored by SuperTokens core (single source of truth for auth artifacts, matching the Google ThirdParty pattern).

## Non-Goals (v1)

- "Sudo mode" / time-bound re-auth before credential changes. The WebAuthn add ceremony itself requires biometric/PIN.
- Recovery codes — SuperTokens MFA recipe can add these later.
- Conditional-UI auto-prompt on login page (Q4 option C in brainstorming).
- Email-first sign-in flow (Q4 option B).
- Email/password sign-up — passkey replaces the need for one.
- Cross-tenant credential sharing or any per-Org passkey concepts; passkeys are per-User.

## Decisions (grilled, all locked)

| # | Question | Decision | Reason |
|---|---|---|---|
| Q1 | Scope | Enrolment **and** sign-in shipped together | User asked for both; splitting ships invisible value |
| Q2 | Implementation | SuperTokens **Webauthn recipe** via `supertokens-web-js@0.16.0`; Go SDK has no webauthn recipe but existing `SuperTokensMiddleware` proxies `/auth/webauthn/*` to the core | One source of truth; matches Google ThirdParty pattern; no new auth state outside SuperTokens |
| Q3 | Multi-passkey | Multiple, user-labeled, capped at 10 per user | Real users have multi-device; labels enable safe revocation |
| Q4 | Login UX | Two buttons (Google + Passkey) on login page, no email-first | Minimal page change; passkeys are origin-scoped so no email lookup needed |

## Safe defaults (flagged in design review, can revise)

- **RP ID:** `ravencloak.org` (covers all subdomains so a passkey enrolled on `demo.ravencloak.org` works on `app.ravencloak.org` too).
- **Authenticator types:** any — platform (Touch ID, Face ID, Windows Hello) and cross-platform (YubiKey, USB security keys) both accepted. `userVerification: "required"`.
- **Re-auth for changes:** the WebAuthn add ceremony's biometric/PIN is treated as implicit re-auth. Removal uses a confirm dialog (matches GitHub SSH-key UX).
- **Recovery:** Google sign-in is always the fallback. No recovery codes in v1.
- **Rate limiting:** SuperTokens core handles ceremony rate-limiting natively.
- **Default label** pre-fills from `navigator.userAgent` parsing ("MacBook · Chrome 142"); user can override.
- **Cap:** 10 passkeys per user — sanity guard, median user has ≤ 3.

## Architecture

```text
                              Vue 3 SPA (supertokens-web-js 0.16.0)
                                          │
                  ┌───────────────────────┼─────────────────────────┐
                  │                       │                         │
        Settings → Authentication      Login page             plugins/supertokens.ts
        (NEW tab)                      (NEW button)           Session + ThirdParty + NEW Webauthn
                  │                       │                         │
                  ▼                       ▼                         │
        stores/passkeys.ts (Pinia)   Webauthn.authenticate          │
        list, add, remove, relabel   CredentialWithSignIn()         │
                  │                       │                         │
                  └───────┬───────────────┘                         │
                          │                                         │
                          ▼                                         │
              api/passkeys.ts (Raven)         /auth/webauthn/* ◄────┘
              GET    /api/v1/me/passkeys      (proxied by existing
              PATCH  /api/v1/me/passkeys/:id   SuperTokensMiddleware)
              DELETE /api/v1/me/passkeys/:id            │
                          │                             ▼
                          ▼                  SuperTokens core (Postgres)
              Go: internal/handler/passkey.go   ← credential storage
                          │                             │
                          ▼                             │ post-signIn hook
              Raven users + new user_passkey_labels ◄───┘
              (label/created_at/last_used_at,           (syncs external_id ↔ users.id,
               keyed on SuperTokens credential ID;       same pattern as Google ThirdParty)
               RLS by user_id)
```

## Data flow

### Enrolment (Settings → Authentication → Add passkey)

1. User clicks **Add passkey**. Frontend pre-fills label from `navigator.userAgent`.
2. Frontend calls `Webauthn.getRegisterOptions()` → proxied to `/auth/webauthn/options/register` on core → returns challenge.
3. Frontend calls `Webauthn.createCredential(options)` → browser opens authenticator (biometric/PIN) → returns credential.
4. Frontend calls `Webauthn.registerCredential(response)` → proxied to `/auth/webauthn/register` on core → core persists credential against current user's SuperTokens ID.
5. Frontend calls Raven `PATCH /api/v1/me/passkeys/:credentialId { label }` → upserts row in `user_passkey_labels`.
6. Settings refreshes the list.

### Sign-in (Login page → Sign in with Passkey)

1. User clicks **Sign in with Passkey**. No email input.
2. Frontend calls `Webauthn.authenticateCredentialWithSignIn()` → browser picker (all matching credentials for `ravencloak.org`) → user picks one, completes biometric.
3. Frontend's response includes the SuperTokens session tokens. Raven's existing post-signin hook fires, mapping the SuperTokens user ID to `users.id`.
4. Redirect to dashboard.

### Removal (Settings → Authentication → Remove)

1. User clicks **Remove** on a passkey row → confirm dialog.
2. Frontend calls Raven `DELETE /api/v1/me/passkeys/:credentialId` → handler:
   - Authn check: caller must own this credential (verify via SuperTokens core lookup).
   - Calls SuperTokens core REST `DELETE /recipe/webauthn/user/credential` → core removes credential.
   - Deletes our `user_passkey_labels` row.
3. Settings refreshes.

## Components

### Frontend (`frontend/src/`)

| File | Change |
|---|---|
| `plugins/supertokens.ts` | Add `import Webauthn from 'supertokens-web-js/recipe/webauthn'` + `Webauthn.init()` next to Session and ThirdParty. |
| `api/passkeys.ts` (NEW) | Thin wrapper: `listPasskeys`, `relabelPasskey`, `removePasskey`. Uses existing `authFetch`. |
| `stores/passkeys.ts` (NEW) | Pinia store: `passkeys`, `fetchPasskeys`, `addPasskey`, `removePasskey`, `relabelPasskey`. Optimistic UI with rollback. |
| `pages/settings/SettingsPage.vue` | Add new `Authentication` tab + section. List with label/created/last-used columns; Add and Remove actions. |
| `pages/LoginPage.vue` | Add "Sign in with Passkey" button beside the Google button. Calls `Webauthn.authenticateCredentialWithSignIn()`. |
| `stores/passkeys.spec.ts` (NEW) | Vitest covering list, add (mock Webauthn), remove, relabel, rollback. |

### Backend (`internal/`)

| File | Change |
|---|---|
| `handler/passkey.go` (NEW) | `GET/PATCH/DELETE /api/v1/me/passkeys[/:id]`. List joins SuperTokens core (HTTP) with `user_passkey_labels`. PATCH upserts label. DELETE calls core's REST + removes our row. User-scoped via session middleware; verify caller owns each credential. |
| `handler/auth.go` | Extend the post-signin user-mapping path so the existing Google flow is mirrored when the recipe is `webauthn`. Same `external_id` mapping. |
| `migrations/00054_user_passkey_labels.sql` (NEW) | `(user_id uuid NOT NULL, credential_id text PRIMARY KEY, label text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), last_used_at timestamptz NULL)`. RLS by user_id. Goose no-transaction; concurrent index on user_id. |
| `service/passkey.go` (NEW) | Small service wrapping SuperTokens core REST calls (list credentials by user ID, delete credential). |
| `handler/passkey_test.go` (NEW) | Handler tests with mocked SuperTokens core HTTP. |

### Infra

| File | Change |
|---|---|
| `docker-compose.yml` | Pin `registry.supertokens.io/supertokens/supertokens-postgresql` to a version that ships WebAuthn (verify which CDI version first). Add `RAVEN_WEBAUTHN_RP_ID=ravencloak.org` and `RAVEN_WEBAUTHN_RP_NAME=Raven` env vars; backend reads them and forwards to SuperTokens recipe config. |
| `.env.example` | Document the two new env vars. |

### Docs

- `docs/superpowers/specs/2026-06-04-passkey-auth-design.md` (this file).
- CONTEXT.md glossary additions (when the parallel Marketplace docs land — for now, defer): **Passkey**, **WebAuthn**, **Authenticator**, **Relying Party (RP)**.

## Out-of-scope follow-ups

- Conditional-UI auto-prompt on login.
- Email-first sign-in.
- Sudo-mode / time-bound re-auth before sensitive changes.
- Recovery codes.
- Disabling Google sign-in entirely (passkey-only mode).

## Issue breakdown (tracer-bullet vertical slices)

Six issues. **A, B, E are independent and can be implemented in parallel.** C depends on B; D depends on B; F depends on all.

### A — Backend: passkey handler + label migration + service

- New migration `00054_user_passkey_labels.sql` (per Architecture section).
- New `internal/handler/passkey.go` + `internal/service/passkey.go`.
- Wire routes into `cmd/api/main.go` under existing auth middleware (user-scoped).
- Handler tests with mocked SuperTokens core HTTP.
- Extend `internal/handler/auth.go` post-signin mapping for `webauthn` recipe.
- Acceptance: GET returns merged credentials+labels; PATCH upserts label; DELETE removes from core + our row; tests pass; lint passes.

### B — Frontend foundation: Webauthn.init + store + api wrapper

- Add `Webauthn.init()` to `plugins/supertokens.ts`.
- New `api/passkeys.ts` thin wrapper (depends on A for endpoint URLs but URL contract is fixed in this spec).
- New `stores/passkeys.ts` Pinia store with optimistic updates + rollback.
- Vitest unit tests for the store.
- Acceptance: store mocks pass; `Webauthn.init` resolves; lint passes.

### C — Frontend: Settings → Authentication tab

- Add new "Authentication" tab to `pages/settings/SettingsPage.vue`.
- Section: list of passkeys (label, created_at, last_used_at, "current device" badge), Add button, Remove button per row, inline relabel.
- Default label from UA parsing.
- Empty state: "No passkeys yet. Add one to skip Google sign-in next time."
- Toast errors on add/remove failure.
- Depends on Issue B.

### D — Frontend: Login page "Sign in with Passkey" button

- Add button beside the existing Google button in `pages/LoginPage.vue`.
- On click: `Webauthn.authenticateCredentialWithSignIn()`, handle success → redirect; handle `INVALID_CREDENTIALS_ERROR` / `WEBAUTHN_NOT_SUPPORTED` with inline error.
- Disabled state with tooltip if `doesBrowserSupportWebAuthn()` returns false.
- Depends on Issue B.

### E — Infra: pin SuperTokens core image + RP config env vars

- Probe the current `:latest` SuperTokens core image for WebAuthn endpoint support. If absent, pin to a known-good version in `docker-compose.yml`.
- Add `RAVEN_WEBAUTHN_RP_ID` and `RAVEN_WEBAUTHN_RP_NAME` to `.env.example` and to the API container's env.
- Document in a README or runbook entry.
- Independent of A/B/C/D.

### F — E2E Playwright coverage

- **Add passkey** — sign in (Google), navigate to Settings → Authentication, click Add, mock the Webauthn ceremony, assert list shows the new entry with the auto-generated label.
- **Relabel** — click label, type new value, blur, reload, assert persisted.
- **Remove** — click Remove, confirm, assert row gone.
- **Sign in with Passkey** — log out, on login page click "Sign in with Passkey", mock browser picker, land on dashboard.
- **Browser unsupported** — mock `doesBrowserSupportWebAuthn` returning false; assert button disabled with tooltip.
- Depends on A, B, C, D.

## Success criteria

- Logged-in user can add a passkey from Settings → Authentication on first try with no manual config; the ceremony triggers the OS biometric prompt.
- The new passkey appears in the list with the auto-generated label and "current device" badge.
- Logged out, the user can sign in with a single click + biometric on the login page.
- Removing a passkey from Settings causes it to no longer work for sign-in (verified by E2E).
- Google sign-in continues to work exactly as before.
- Migration `00054` applies cleanly on a fresh DB and a populated one.
- All existing tests still pass; new tests added per spec.
- Frontend lint + `golangci-lint --new-from-rev=origin/main` pass.
- New Playwright scenarios pass headless in CI.
