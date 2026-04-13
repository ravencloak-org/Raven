# Zitadel Migration — Design Spec

**Date**: 2026-04-13
**Status**: Approved
**Sub-project**: 1 of 3 (Zitadel migration → Google IDP + WebAuthn → Registration + Org flow)

---

## Overview

Replace Keycloak with Zitadel as the identity provider for Raven. Zitadel handles identity (authentication, Google IDP, passkeys). Raven handles tenancy (org/workspace membership, RLS, billing). This is a full replacement — no dual-auth period.

The migration is delivered in **3 phases** (separate PRs):
1. Infrastructure — Docker Compose + Zitadel config + Cloudflare Tunnels
2. Backend — Go middleware swap + post-signup provisioning endpoints
3. Frontend — OIDC client swap + 2-step onboarding wizard + passkey registration

## Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Multi-tenancy | Single Zitadel org, Raven manages tenancy | Simpler ops, existing RLS model works, no Zitadel API calls for org creation |
| First signup | Google OAuth only | Eliminates choice paralysis, fastest onboarding, passkeys added later |
| Passkey timing | Post-registration in settings | WebAuthn requires existing account, power-user feature |
| Onboarding | 2-step wizard (name org → create KB) | Minimal friction, gets user invested fast |
| Deployment | Docker Compose + Cloudflare Tunnels | Same config local and prod, no TLS management, no exposed ports |
| Reverse proxy | Cloudflare Tunnels (replaces Traefik) | Zero-config TLS, DDoS protection, no port exposure |

## Domain Map

| Domain | Service | Internal Port |
|--------|---------|---------------|
| `auth.ravencloak.org` | Zitadel | 8080 |
| `api.ravencloak.org` | Raven Go API | 8081 |
| `app.ravencloak.org` | Vue.js frontend | 3000 |
| `raven.ravencloak.org` | Landing page | Static (Cloudflare Pages) |

---

## Phase 1: Infrastructure

### Docker Compose Changes

**Remove:**
- `keycloak` service
- Keycloak-specific PostgreSQL database/config
- `deploy/keycloak/` directory (realm config, themes)
- Traefik service and config (if present)

**Add:**

```yaml
zitadel:
  image: ghcr.io/zitadel/zitadel:v2.71.12
  restart: always
  command: >
    start-from-init
    --config /zitadel-config.yaml
    --config /zitadel-secrets.yaml
    --steps /zitadel-init-steps.yaml
    --masterkey "${ZITADEL_MASTERKEY}"
    --tlsMode disabled
  ports:
    - "8080:8080"
  volumes:
    - ./deploy/zitadel/zitadel-config.yaml:/zitadel-config.yaml:ro
    - ./deploy/zitadel/zitadel-secrets.yaml:/zitadel-secrets.yaml:ro
    - ./deploy/zitadel/zitadel-init-steps.yaml:/zitadel-init-steps.yaml:ro
  depends_on:
    postgres:
      condition: service_healthy
  networks:
    - raven-internal
  environment:
    - ZITADEL_MASTERKEY=${ZITADEL_MASTERKEY}
    - ZITADEL_EXTERNALDOMAIN=${ZITADEL_EXTERNALDOMAIN}
    - ZITADEL_EXTERNALPORT=${ZITADEL_EXTERNALPORT:-443}
    - ZITADEL_EXTERNALSECURE=${ZITADEL_EXTERNALSECURE:-true}
    - ZITADEL_DATABASE_POSTGRES_HOST=postgres
    - ZITADEL_DATABASE_POSTGRES_PORT=5432
    - ZITADEL_DATABASE_POSTGRES_DATABASE=zitadel
    - ZITADEL_DATABASE_POSTGRES_USER_USERNAME=zitadel
    - ZITADEL_DATABASE_POSTGRES_USER_PASSWORD=${ZITADEL_DB_PASSWORD}
    - ZITADEL_DATABASE_POSTGRES_ADMIN_USERNAME=postgres
    - ZITADEL_DATABASE_POSTGRES_ADMIN_PASSWORD=${POSTGRES_PASSWORD}
  healthcheck:
    test: ["CMD", "/app/zitadel", "ready"]
    interval: 10s
    timeout: 5s
    retries: 5
    start_period: 30s

cloudflared:
  image: cloudflare/cloudflared:latest
  restart: always
  command: tunnel run
  environment:
    - TUNNEL_TOKEN=${CF_TUNNEL_TOKEN}
  networks:
    - raven-internal
  profiles:
    - production
```

**Zitadel uses Raven's existing PostgreSQL** — separate `zitadel` database on the same instance. The `zitadel` database and user are created automatically by Zitadel's `start-from-init` using the Admin credentials (postgres user with `CREATEDB` privilege). Add `CREATE USER zitadel WITH PASSWORD ... CREATEDB;` to `deploy/postgres/init.sql` as a safety net.

**Port conflict resolution**: The Go API must move from port `8080` to `8081` in both `docker-compose.yml` and `internal/config/config.go` (update `v.SetDefault("server.port", 8081)`). Zitadel takes `8080`.

**Zitadel config uses env vars, not YAML interpolation**: Zitadel does NOT support `${VAR}` syntax in YAML config files. Instead, use Zitadel's built-in env var mapping — every config key maps to an env var (e.g., `ExternalDomain` → `ZITADEL_EXTERNALDOMAIN`). The `zitadel-config.yaml` should contain only static defaults; dynamic values come from Docker Compose environment variables.

**`cloudflared` uses `profiles: [production]`** — only starts when running `docker compose --profile production up`. Local dev accesses services directly on localhost ports.

### Zitadel Configuration Files

**`deploy/zitadel/zitadel-config.yaml`** (static defaults only — dynamic values via env vars):
```yaml
Log:
  Level: info

# ExternalDomain, ExternalSecure, ExternalPort are set via env vars:
# ZITADEL_EXTERNALDOMAIN, ZITADEL_EXTERNALSECURE, ZITADEL_EXTERNALPORT
# Database connection is also set via env vars (see docker-compose environment block)

Database:
  postgres:
    MaxOpenConns: 10
    MaxIdleConns: 5
    MaxConnLifetime: 30m
    MaxConnIdleTime: 5m
```

**`deploy/zitadel/zitadel-secrets.yaml`:** Not needed — all secrets passed via environment variables in Docker Compose.

**`deploy/zitadel/zitadel-init-steps.yaml`:**
Auto-creates on first boot:
- Default OIDC application `raven-app` with:
  - Redirect URIs: `https://app.ravencloak.org/callback`, `http://localhost:5173/callback`
  - Post-logout URIs: `https://app.ravencloak.org`, `http://localhost:5173`
  - Grant type: Authorization Code + PKCE
  - Response type: Code
- Machine user `raven-api` with key (for backend-to-Zitadel API calls if needed)

**`deploy/zitadel/zitadel-secrets.yaml`:**
```yaml
Database:
  postgres:
    User:
      Password: ${ZITADEL_DB_PASSWORD}
    Admin:
      Password: ${POSTGRES_PASSWORD}
```

### Google IDP Setup

Configured via Zitadel admin console (post-init) or API:
- Add Google as generic OIDC IDP
- Client ID/Secret from environment: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`
- Scopes: `openid`, `profile`, `email`
- Set as default IDP for the organization
- Configure auto-linking: match by email

### Cloudflare Tunnel Routes

Single tunnel with three public hostname routes:

| Public Hostname | Service | Internal URL |
|-----------------|---------|-------------|
| `auth.ravencloak.org` | Zitadel | `http://zitadel:8080` |
| `api.ravencloak.org` | Raven API | `http://api:8081` |
| `app.ravencloak.org` | Frontend | `http://frontend:3000` |

Configured in Cloudflare Zero Trust dashboard. The `cloudflared` container just needs `CF_TUNNEL_TOKEN`.

### Environment Variables

```env
# Zitadel
ZITADEL_MASTERKEY=<32-char-random-key>
ZITADEL_EXTERNALDOMAIN=auth.ravencloak.org  # or localhost for dev
ZITADEL_DB_PASSWORD=<password>

# Google IDP
GOOGLE_CLIENT_ID=<from-google-cloud-console>
GOOGLE_CLIENT_SECRET=<from-google-cloud-console>

# Cloudflare (production only)
CF_TUNNEL_TOKEN=<from-cloudflare-dashboard>

# Existing
POSTGRES_PASSWORD=<existing-pg-password>
```

### Local Development

- No `cloudflared` (uses `profiles: [production]`)
- Zitadel accessible at `http://localhost:8080`
- Frontend at `http://localhost:5173` (Vite dev server)
- API at `http://localhost:8081`
- Zitadel config uses `ExternalDomain: localhost`, `ExternalSecure: false`, `ExternalPort: 8080` (overridden via env or local config file)

---

## Phase 2: Backend

### JWT Middleware Replacement

**Remove:**
- `internal/middleware/auth.go` — Keycloak-specific JWT validation
- Keycloak realm-based routing logic
- `keycloak` Go dependencies

**Add:**
- New `internal/middleware/auth.go` using `github.com/zitadel/oidc/v3/pkg/op` for token introspection
- Or use standard JWT validation with Zitadel's JWKS endpoint: `https://auth.ravencloak.org/.well-known/openid-configuration`
- Middleware extracts from JWT: `sub` (Zitadel user ID), `email`, `name`
- Sets on Gin context: `userExternalID`, `userEmail`, `userName`

**Token validation flow:**
1. Extract `Authorization: Bearer <token>` from request header
2. Validate JWT signature against Zitadel JWKS (cached)
3. Verify `iss` matches Zitadel domain, `aud` matches `raven-app` client ID
4. Extract claims, set on context
5. Resolve `external_id` → internal `user_id` + `org_id` via database lookup

### User Model Changes

**Migration:**
```sql
-- Fix: actual column is keycloak_sub, not keycloak_id
ALTER TABLE users RENAME COLUMN keycloak_sub TO external_id;
ALTER TABLE users ADD COLUMN auth_provider TEXT NOT NULL DEFAULT 'zitadel';
DROP INDEX IF EXISTS idx_users_keycloak_sub;
CREATE UNIQUE INDEX idx_users_external_id ON users(external_id);

-- Make org_id nullable for users who have authenticated but not yet created an org
ALTER TABLE users ALTER COLUMN org_id DROP NOT NULL;

-- Drop Keycloak-specific column from organizations
ALTER TABLE organizations DROP COLUMN IF EXISTS keycloak_realm;
```

**Go model updates:**
- `internal/model/user.go`: rename `KeycloakSub` field → `ExternalID`, add `AuthProvider` field
- `internal/model/org.go`: remove `KeycloakRealm` field
- `internal/repository/user.go`: update all queries referencing `keycloak_sub` → `external_id`
- `internal/repository/org.go`: update scan to exclude `keycloak_realm`

**Lookup path:**
- Request → middleware extracts `sub` from JWT → query `users WHERE external_id = sub` → get `user_id` + `org_id`
- If `org_id IS NULL` → user exists but has no org (needs onboarding)
- Uses existing `users.org_id` column directly (no `user_orgs` table — it does not exist)
- Cache: Valkey key `user:{external_id}` → `{user_id, org_id}`, TTL 5 minutes, invalidated on org creation

### New/Modified Endpoints

**`POST /api/v1/auth/callback`** (new — create `internal/handler/auth.go`):
- Called by frontend after OIDC callback
- Checks if `external_id` exists in `users` table
- If new user:
  - Creates `users` record (external_id, email, name from JWT claims, org_id = NULL)
  - Returns `{ isNewUser: true }`
- If existing user with org:
  - Returns `{ isNewUser: false, orgId: "...", orgSlug: "..." }`
- If existing user without org (abandoned onboarding):
  - Returns `{ isNewUser: true }` (re-enter onboarding)

**`POST /api/v1/orgs`** (modified):
- Remove `middleware.RequireOrgRole("org_admin")` guard from route in `cmd/api/main.go`
- New rule: if authenticated user's `org_id IS NULL`, allow org creation
- Creates: org record, updates `users.org_id` to new org, assigns `owner` role, free tier subscription via billing service
- Invalidates Valkey user cache
- Returns: `{ orgId, orgSlug }`

**Middleware context for org-less users:**
- New middleware helper: `OptionalOrgContext` — sets `ContextKeyOrgID` if user has an org, leaves empty if not
- Routes that require an org (workspaces, KBs, billing) keep the existing `RequireOrg` middleware
- Auth callback and org creation routes use `OptionalOrgContext`

**Existing endpoints (unchanged, just work with new middleware):**
- `POST /api/v1/workspaces` — creates workspace in user's org
- `POST /api/v1/knowledge-bases` — creates KB in workspace
- `POST /api/v1/documents` — upload documents to KB

### Keycloak Cleanup

Remove from Go codebase:
- Keycloak admin client usage (realm provisioning in `internal/service/provision.go`)
- Keycloak-specific config in `internal/config/config.go`
- Keycloak health check endpoints
- Keycloak webhook handler at `/api/v1/internal/keycloak-webhook` (`cmd/api/main.go`)
- `KeycloakWebhookEvent` model type (`internal/model/user.go`)
- `HandleKeycloakEvent` method in `UserService` (`internal/service/user.go`)

Add to Go config:
```go
type ZitadelConfig struct {
    Domain   string `env:"ZITADEL_EXTERNALDOMAIN" envDefault:"localhost:8080"`
    ClientID string `env:"ZITADEL_CLIENT_ID"`
    Secure   bool   `env:"ZITADEL_SECURE" envDefault:"true"`
}
```

---

## Phase 3: Frontend

### Auth Library Swap

**Remove:**
- `keycloak-js` dependency
- `frontend/src/auth/keycloak.ts` (or equivalent)
- All `keycloak.init()`, `keycloak.token`, `keycloak.login()` references

**Add:**
- `oidc-client-ts` dependency (standard OIDC Relying Party library)
- `frontend/src/auth/oidc.ts` — OIDC client configuration:
  ```typescript
  const config = {
    authority: import.meta.env.VITE_ZITADEL_URL, // https://auth.ravencloak.org
    client_id: import.meta.env.VITE_ZITADEL_CLIENT_ID,
    redirect_uri: `${window.location.origin}/callback`,
    post_logout_redirect_uri: window.location.origin,
    scope: 'openid profile email',
    response_type: 'code',
  }
  ```

### Login Flow

**`/login` route:**
- On mount: immediately trigger OIDC redirect with `idp_hint` extra param
- **Note**: Zitadel's `idp_hint` value is the IDP's internal Zitadel ID (a numeric/UUID), not the string "google". After configuring Google IDP in Zitadel, obtain the IDP ID and store it as `VITE_GOOGLE_IDP_ID` env var.
- This makes Zitadel skip its own login UI and redirect straight to Google
- User sees: Landing page → Google consent → app callback (no Zitadel UI ever shown)

**`/callback` route:**
- Exchange authorization code for tokens (PKCE)
- Store access token in memory, refresh token in httpOnly cookie (via Go API proxy) or use `oidc-client-ts` silent renewal via iframe
- **Token refresh strategy**: `oidc-client-ts` `automaticSilentRenew: true` with `silent_redirect_uri` pointing to a minimal `/silent-renew.html` page. This renews tokens before expiry without page reload. If silent renewal fails (e.g., Google session expired), redirect to `/login`.
- Call `POST /api/v1/auth/callback` with the access token
- If `isNewUser: true` → navigate to `/onboarding`
- If `isNewUser: false` → navigate to `/dashboard`

**`/logout`:**
- Clear tokens from memory
- Redirect to Zitadel end-session endpoint
- Zitadel redirects back to landing page

### 2-Step Onboarding Wizard

**Replaces** the existing 5-step wizard at `frontend/src/pages/onboarding/`.

**Step 1: "Name your organization"**
- Full-screen centered card
- Text input pre-filled with `${userName}'s Team`
- Validation: 3-100 chars, alphanumeric + spaces
- "Continue" button → `POST /api/v1/orgs` with `{ name: input }`
- On success: store `orgId` in state, advance to step 2

**Step 2: "Create your first knowledge base"**
- KB name input (required, e.g., "Product Docs")
- Drag-and-drop file upload zone (optional, accepts PDF/DOCX/MD)
- "Get Started" button →
  1. `POST /api/v1/workspaces` → creates default workspace
  2. `POST /api/v1/knowledge-bases` → creates KB
  3. If files uploaded: `POST /api/v1/documents` for each file
  4. Navigate to `/dashboard`

**Styling**: follows existing frontend patterns (Vue 3 + Tailwind), black/white/amber accent consistent with landing page.

### Passkey Registration (Settings Page)

Added to existing settings/security page (not part of onboarding):

**"Add Passkey" button:**
1. Calls Go API: `POST /api/v1/auth/passkeys/register` — backend proxies to Zitadel `POST /v2/users/{userId}/passkeys` and returns `publicKeyCredentialCreationOptions` (avoids CORS issues with direct Zitadel calls from frontend)
2. Calls `navigator.credentials.create({ publicKey: options })` in browser
3. Sends credential response to Go API: `POST /api/v1/auth/passkeys/verify` — backend proxies to Zitadel `POST /v2/users/{userId}/passkeys/{passkeyId}`
4. Shows success: "Passkey registered. You can now sign in without Google."

**Login page update (after passkey exists):**
- Check if user has passkeys via Zitadel user info
- If yes: show two buttons — "Continue with Google" + "Sign in with Passkey"
- If no: show only "Continue with Google"
- Passkey login: create Zitadel session with WebAuthn challenge, verify in browser, complete session

### Router Guards

```typescript
router.beforeEach((to) => {
  const auth = useAuthStore()

  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return '/login'
  }

  if (auth.isAuthenticated && !auth.hasOrg && to.path !== '/onboarding') {
    return '/onboarding'
  }
})
```

### Environment Variables (Frontend)

```env
VITE_ZITADEL_URL=https://auth.ravencloak.org
VITE_ZITADEL_CLIENT_ID=<from-zitadel-init>
VITE_GOOGLE_IDP_ID=<zitadel-internal-google-idp-id>
VITE_API_URL=https://api.ravencloak.org
```

### Note: LLM Provider Setup

The existing 5-step onboarding wizard included LLM provider configuration (Step 3). The new 2-step wizard drops this. LLM provider setup (API keys for Anthropic/OpenAI/Cohere) moves to: **Settings → LLM Providers** page in the dashboard. Users can use Raven with default/demo models initially and configure their own keys later. This is already partially built in the settings page.

---

## Files Changed Summary

### Phase 1 (Infrastructure)
- Remove: `deploy/keycloak/` directory
- Create: `deploy/zitadel/zitadel-config.yaml`
- Create: `deploy/zitadel/zitadel-init-steps.yaml`
- Modify: `docker-compose.yml` (remove keycloak, add zitadel + cloudflared, move go-api to port 8081)
- Modify: `deploy/postgres/init.sql` (add zitadel user + database)
- Modify: `.env.example` (new Zitadel + CF vars)

### Phase 2 (Backend)
- Create: DB migration (rename keycloak_sub → external_id, make org_id nullable, drop keycloak_realm, add auth_provider)
- Create: `internal/handler/auth.go` (new — auth callback + passkey proxy endpoints)
- Modify: `internal/middleware/auth.go` (Zitadel JWT validation, OptionalOrgContext)
- Modify: `internal/config/config.go` (ZitadelConfig replaces KeycloakConfig, server.port → 8081)
- Modify: `internal/handler/provision.go` (remove Keycloak realm provisioning)
- Modify: `internal/service/provision.go` (remove Keycloak admin client)
- Modify: `internal/handler/routing.go` (new route for auth callback)
- Modify: `go.mod` (remove keycloak deps, add zitadel/oidc)

### Phase 3 (Frontend)
- Remove: `keycloak-js` dependency
- Add: `oidc-client-ts` dependency
- Create: `frontend/src/auth/oidc.ts` (OIDC client config)
- Modify: `frontend/src/auth/` (replace Keycloak with OIDC)
- Modify: `frontend/src/pages/onboarding/` (2-step wizard replaces 5-step)
- Modify: `frontend/src/pages/settings/` (add passkey registration)
- Modify: `frontend/src/router/` (update guards, add callback route)
- Modify: `frontend/.env.example` (Zitadel vars replace Keycloak vars)
- Modify: `frontend/package.json` (swap deps)

---

## Out of Scope

- Voice/WhatsApp connector setup per org (separate sub-project)
- User invitation / team member management
- SSO/SAML for enterprise orgs (future, Zitadel supports it natively)
- Migrating existing Keycloak users (no production users exist)
- Custom Zitadel branding/themes (not needed — users never see Zitadel UI)
