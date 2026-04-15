# SuperTokens Migration -- Design Spec

**Date**: 2026-04-14
**Status**: Approved
**Supersedes**: 2026-04-13-zitadel-migration-design.md

---

## Overview

Replace Zitadel with SuperTokens as the auth backend for Raven. Zitadel failed in production: PG18 incompatibility requiring a separate PG16 container, opaque token behaviour, broken `start-from-init` steps, and forced MFA on passkey registration. SuperTokens is simpler to self-host, shares the existing PostgreSQL instance, and uses cookie-based sessions instead of JWTs.

SuperTokens is **not an OIDC provider**. It is a session management and auth recipes API. The frontend talks to SuperTokens through the Go API (which proxies `/auth/*` to the SuperTokens core), not directly to an IdP. There is no OIDC redirect dance.

Auth methods: **Google social login** and **passkeys (WebAuthn)**.

## Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Database | SuperTokens shares Raven PG18 | No separate PG container; ST creates its own `supertokens_*` tables |
| Login UI | Custom Vue pages | SuperTokens is backend-only; no hosted login page dependency |
| Session model | Cookie-based (httpOnly, CSRF) | Simpler than JWT; automatic refresh; no token storage in frontend |
| Backend integration | Go `AuthProvider` interface | Pluggable; swap providers without touching handler/middleware code |
| Frontend SDK | `supertokens-web-js` | Vanilla JS SDK for non-React SPAs (Vue 3) |
| SuperTokens deployment | Sidecar container (port 3567) | Same Docker network, loopback-only in production |
| API routing | Go API proxies `/auth/*` to SuperTokens | Frontend SDK expects auth APIs on the same domain as the app API |

## Domain Map

| Domain | Service | Internal Port |
|--------|---------|---------------|
| `api.ravencloak.org` | Go API (proxies `/auth/*` to SuperTokens) | 8081 |
| `app.ravencloak.org` | Vue SPA | 3000 |
| `raven.ravencloak.org` | Landing page | Static (Cloudflare Pages) |

No separate auth domain needed. SuperTokens is accessed exclusively through the Go API proxy.

## Request Flow

```
Browser (app.ravencloak.org)
  |
  |-- Google login -----> POST api.ravencloak.org/auth/signinup
  |                          |-> Go proxy -> SuperTokens :3567
  |                          |<- session cookies set (sAccessToken, sRefreshToken)
  |
  |-- Passkey login ----> POST api.ravencloak.org/auth/webauthn/signin
  |                          |-> Go proxy -> SuperTokens :3567
  |                          |<- session cookies set
  |
  |-- API call ----------> GET api.ravencloak.org/api/v1/orgs
  |                          |-> SessionMiddleware reads cookies
  |                          |-> calls SuperTokens /recipe/session/verify
  |                          |-> extracts user info
  |                          |-> sets context keys (external_id, email, name)
  |                          |-> UserLookup middleware (unchanged)
  |                          |-> handler
```

---

## 1. Infrastructure

### 1.1 SuperTokens Docker Container

**Image**: `registry.supertokens.io/supertokens/supertokens-postgresql`

SuperTokens runs as a sidecar container on port 3567. It connects to the existing Raven PostgreSQL instance and auto-creates its tables (`supertokens_*` prefix) on first boot.

### 1.2 Docker Compose -- Development (`docker-compose.yml`)

**Remove:**
- `zitadel` service (entire block)
- Zitadel environment variables from `go-api` (`ZITADEL_EXTERNALDOMAIN`, `ZITADEL_CLIENT_ID`)
- `go-api` dependency on `zitadel`

**Add:**

```yaml
# --- SuperTokens Auth Backend ---
supertokens:
  image: registry.supertokens.io/supertokens/supertokens-postgresql
  environment:
    POSTGRESQL_CONNECTION_URI: "postgresql://${POSTGRES_USER:-raven}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB:-raven}"
    API_KEYS: "${SUPERTOKENS_API_KEY:-supertokens-dev-key-replace-me}"
    DISABLE_TELEMETRY: "true"
  depends_on:
    postgres:
      condition: service_healthy
  networks:
    - raven-internal
  restart: unless-stopped
  healthcheck:
    test: ["CMD", "sh", "-c", "curl -sf http://localhost:3567/hello || exit 1"]
    interval: 10s
    timeout: 5s
    retries: 5
    start_period: 10s
```

**Modify `go-api` service:**

```yaml
go-api:
  environment:
    RAVEN_SUPERTOKENS_CONNECTION_URI: "http://supertokens:3567"
    RAVEN_SUPERTOKENS_API_KEY: "${SUPERTOKENS_API_KEY:-supertokens-dev-key-replace-me}"
    GOOGLE_CLIENT_ID: "${GOOGLE_CLIENT_ID}"
    GOOGLE_CLIENT_SECRET: "${GOOGLE_CLIENT_SECRET}"
  depends_on:
    supertokens:
      condition: service_healthy
    # ... (keep postgres, valkey, openobserve)
```

### 1.3 Docker Compose -- Production (`deploy/ec2/docker-compose.server.yml`)

**Remove:**
- `zitadel` service
- `zitadel-db` service (the separate PG16 container)
- `zitadel-pg-data` volume

**Add:**

```yaml
supertokens:
  image: registry.supertokens.io/supertokens/supertokens-postgresql
  environment:
    POSTGRESQL_CONNECTION_URI: "postgresql://${POSTGRES_USER:-raven}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB:-raven}"
    API_KEYS: "${SUPERTOKENS_API_KEY}"
    DISABLE_TELEMETRY: "true"
  depends_on:
    postgres:
      condition: service_healthy
  networks:
    - raven-net
  restart: unless-stopped
  healthcheck:
    test: ["CMD", "sh", "-c", "curl -sf http://localhost:3567/hello || exit 1"]
    interval: 10s
    timeout: 5s
    retries: 5
    start_period: 10s
  # No port exposure -- only reachable inside raven-net
```

### 1.4 Cloudflare Tunnel

No changes needed. SuperTokens is not exposed to the internet. The Go API proxies all `/auth/*` requests to the SuperTokens container internally. The existing `api.ravencloak.org` tunnel route handles everything.

### 1.5 PostgreSQL Init Script (`deploy/postgres/init.sql`)

**Remove:**
- Zitadel database creation (`CREATE DATABASE zitadel`)
- Zitadel user creation (`CREATE USER zitadel`)
- Zitadel privilege grants

SuperTokens auto-creates its tables in the Raven database. No init script changes needed beyond removing Zitadel entries.

---

## 2. Backend -- AuthProvider Interface

### 2.1 Interface Definition (`internal/auth/provider.go`)

```go
package auth

import "net/http"

// SessionInfo holds user identity data extracted from a verified session.
type SessionInfo struct {
    ExternalID string // SuperTokens user ID
    Email      string
    Name       string
}

// AuthProvider abstracts authentication session verification.
// Implementations may call external services (SuperTokens, etc.) to verify sessions.
type AuthProvider interface {
    // VerifySession validates the session from the HTTP request (cookies or headers)
    // and returns the authenticated user's identity.
    VerifySession(r *http.Request) (*SessionInfo, error)

    // RevokeSession invalidates the current session from the HTTP request.
    RevokeSession(r *http.Request) error
}
```

### 2.2 SuperTokens Implementation (`internal/auth/supertokens.go`)

The SuperTokens implementation calls the SuperTokens Core HTTP API at `http://supertokens:3567`.

Key operations:

| Operation | SuperTokens Core API Endpoint |
|-----------|------------------------------|
| Verify session | `POST /recipe/session/verify` |
| Get user info | `GET /recipe/user?userId=<id>` |
| Revoke session | `POST /recipe/session/remove` |

```go
package auth

import (
    "encoding/json"
    "fmt"
    "net/http"
)

// SuperTokensProvider implements AuthProvider using the SuperTokens Core HTTP API.
type SuperTokensProvider struct {
    connectionURI string // e.g. "http://supertokens:3567"
    apiKey        string
    httpClient    *http.Client
}

func NewSuperTokensProvider(connectionURI, apiKey string) *SuperTokensProvider {
    return &SuperTokensProvider{
        connectionURI: connectionURI,
        apiKey:        apiKey,
        httpClient:    &http.Client{},
    }
}

func (p *SuperTokensProvider) VerifySession(r *http.Request) (*SessionInfo, error) {
    // 1. Extract sAccessToken from cookies
    // 2. POST /recipe/session/verify with the access token
    // 3. Extract userId from session response
    // 4. GET /recipe/user?userId=<id> for email/name
    // 5. Return SessionInfo
}

func (p *SuperTokensProvider) RevokeSession(r *http.Request) error {
    // POST /recipe/session/remove with session handle
}
```

Note: The Go API also needs to **proxy** all `/auth/*` requests to SuperTokens. This is handled by a reverse proxy handler, separate from the `AuthProvider` interface. See section 2.4.

### 2.3 Session Middleware (`internal/middleware/auth.go`)

Replace `JWTMiddleware` with `SessionMiddleware` that uses the `AuthProvider` interface.

**Remove:**
- `jwksCache` struct and all JWKS logic
- `parseJWT` function
- `Claims` struct (JWT-specific)
- `keyfunc` and `golang-jwt` imports

**Replace with:**

```go
// SessionMiddleware returns a Gin handler that verifies the session using
// the provided AuthProvider. On success, it stores identity data in the
// Gin context using the same context keys as the old JWTMiddleware, so
// downstream middleware (UserLookup, RequireOrg, etc.) works unchanged.
func SessionMiddleware(provider auth.AuthProvider) gin.HandlerFunc {
    return func(c *gin.Context) {
        info, err := provider.VerifySession(c.Request)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, authError{Error: "invalid_session"})
            return
        }

        // Store in context -- same keys as old JWTMiddleware
        c.Set(string(ContextKeyExternalID), info.ExternalID)
        c.Set(string(ContextKeyEmail), info.Email)
        c.Set(string(ContextKeyUserName), info.Name)

        c.Next()
    }
}
```

**Unchanged middleware:**
- `UserLookup` -- continues to resolve `external_id` to internal user/org IDs
- `RequireOrg` -- continues to check for org_id in context
- `RequireOrgRole`, `ResolveWorkspaceRole`, `RequireWorkspaceRole` -- unchanged

### 2.4 Auth Proxy Handler

The Go API must proxy `/auth/*` requests to the SuperTokens core so the frontend SDK works. The `supertokens-web-js` SDK expects to call auth endpoints on the same domain as the API.

```go
// internal/handler/authproxy.go

// NewSuperTokensProxy returns a gin handler that reverse-proxies all
// requests under /auth/* to the SuperTokens core.
func NewSuperTokensProxy(superTokensURL string) gin.HandlerFunc {
    target, _ := url.Parse(superTokensURL)
    proxy := httputil.NewSingleHostReverseProxy(target)
    return func(c *gin.Context) {
        proxy.ServeHTTP(c.Writer, c.Request)
    }
}
```

Registered in `main.go`:

```go
// SuperTokens auth proxy -- must be outside the /api/v1 group (no session verification)
router.Any("/auth/*path", handler.NewSuperTokensProxy(cfg.SuperTokens.ConnectionURI))
```

### 2.5 Wiring in `main.go`

```go
// Replace:
//   api.Use(middleware.JWTMiddleware(&cfg.Zitadel))
// With:
authProvider := auth.NewSuperTokensProvider(
    cfg.SuperTokens.ConnectionURI,
    cfg.SuperTokens.APIKey,
)
api.Use(middleware.SessionMiddleware(authProvider))
api.Use(middleware.UserLookup(&userLookupAdapter{repo: userRepo}))
```

The `AuthHandler.Callback` endpoint (`POST /api/v1/auth/callback`) remains unchanged. It reads `ContextKeyExternalID`, `ContextKeyEmail`, and `ContextKeyUserName` from the Gin context (now set by `SessionMiddleware` instead of `JWTMiddleware`) and creates/looks up the internal user record.

---

## 3. Backend -- SuperTokens Recipe Configuration

SuperTokens recipes are configured by calling the SuperTokens Core API at startup (or via the Go SDK `supertokens.Init`). Since Raven uses a raw HTTP integration (no Go SDK middleware), recipe configuration is done through the Core API.

### 3.1 ThirdParty Recipe -- Google OAuth

Configure via Core API on startup or through environment variables:

```
GOOGLE_CLIENT_ID=<your-google-client-id>
GOOGLE_CLIENT_SECRET=<your-google-client-secret>
```

The Go API configures the Google provider by calling the SuperTokens Core API:

```
PUT /recipe/multitenancy/tenant
{
  "tenantId": "public",
  "thirdPartyEnabled": true
}
```

And configuring the Google provider:

```
PUT /recipe/multitenancy/tenant/thirdparty/config
{
  "tenantId": "public",
  "config": {
    "thirdPartyId": "google",
    "clients": [{
      "clientId": "<GOOGLE_CLIENT_ID>",
      "clientSecret": "<GOOGLE_CLIENT_SECRET>"
    }]
  }
}
```

### 3.2 WebAuthn/Passkey Recipe

SuperTokens supports WebAuthn via the `webauthn` recipe. Configuration:

```
PUT /recipe/multitenancy/tenant
{
  "tenantId": "public",
  "webauthnEnabled": true
}
```

The WebAuthn recipe requires:
- `rpId`: The relying party ID (domain, e.g., `ravencloak.org`)
- `rpName`: Display name (e.g., `Raven`)
- `origin`: The frontend origin (e.g., `https://app.ravencloak.org`)

These are configured via SuperTokens Core config or environment variables.

### 3.3 Session Recipe

SuperTokens sessions use httpOnly cookies by default. Configuration:

| Setting | Dev | Production |
|---------|-----|------------|
| `cookie_secure` | `false` | `true` |
| `cookie_same_site` | `lax` | `lax` |
| `cookie_domain` | `localhost` | `.ravencloak.org` |
| `access_token_validity` | 3600 (1 hour) | 3600 |
| `refresh_token_validity` | 144000 (100 days, minutes) | 144000 |

### 3.4 CORS Configuration

Cookie-based auth requires specific CORS settings. Update `middleware.CORSMiddleware`:

```go
// Existing AllowedOrigins from config +
AllowCredentials: true  // Required for cookies
ExposeHeaders: []string{
    "st-access-token",
    "st-refresh-token",
    "anti-csrf",
    "front-token",
}
AllowHeaders: append(existingHeaders,
    "anti-csrf",
    "st-auth-mode",
    "rid",
    "fdi-version",
)
```

---

## 4. Frontend -- Auth Store Rewrite

### 4.1 Package Changes

**Remove:** `oidc-client-ts`
**Add:** `supertokens-web-js`

```bash
npm uninstall oidc-client-ts
npm install supertokens-web-js
```

### 4.2 SuperTokens Initialization (`main.ts` or `plugins/supertokens.ts`)

```typescript
import SuperTokens from "supertokens-web-js"
import Session from "supertokens-web-js/recipe/session"
import ThirdParty from "supertokens-web-js/recipe/thirdparty"
import WebAuthn from "supertokens-web-js/recipe/webauthn"

SuperTokens.init({
  appInfo: {
    appName: "Raven",
    apiDomain: import.meta.env.VITE_API_DOMAIN,  // https://api.ravencloak.org
    apiBasePath: "/auth",
  },
  recipeList: [
    Session.init(),
    ThirdParty.init(),
    WebAuthn.init(),
  ],
})
```

### 4.3 Auth Store (`stores/auth.ts`)

Replace the entire OIDC-based store:

```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import Session from 'supertokens-web-js/recipe/session'
import {
  getAuthorisationURLWithQueryParamsAndSetState,
  signInAndUp,
} from 'supertokens-web-js/recipe/thirdparty'
import {
  authenticateCredentialWithSignIn,
  doesBrowserSupportWebAuthn,
} from 'supertokens-web-js/recipe/webauthn'
import { usePostHog } from '../plugins/posthog'

export const useAuthStore = defineStore('auth', () => {
  const sessionExists = ref(false)
  const orgId = ref<string | null>(sessionStorage.getItem('raven_org_id'))
  const isAuthenticated = computed(() => sessionExists.value)
  const hasOrg = computed(() => !!orgId.value)

  async function init() {
    sessionExists.value = await Session.doesSessionExist()
  }

  async function loginWithGoogle() {
    const authUrl = await getAuthorisationURLWithQueryParamsAndSetState({
      thirdPartyId: 'google',
      frontendRedirectURI: `${window.location.origin}/callback`,
    })
    window.location.assign(authUrl)
  }

  async function loginWithPasskey() {
    const support = await doesBrowserSupportWebAuthn({ userContext: {} })
    if (support.status !== 'OK' || !support.browserSupportsWebauthn) {
      throw new Error('WebAuthn not supported by this browser')
    }
    const response = await authenticateCredentialWithSignIn({ userContext: {} })
    if (response.status === 'OK') {
      sessionExists.value = true
      // Call auth callback to resolve internal user
      await callAuthCallback()
    }
  }

  async function handleCallback(): Promise<void> {
    const response = await signInAndUp()
    if (response.status === 'OK') {
      sessionExists.value = true
      // Call auth callback to resolve internal user
      await callAuthCallback()
    }
  }

  async function callAuthCallback(): Promise<{
    isNewUser: boolean
    orgId?: string
  }> {
    // POST /api/v1/auth/callback -- session cookie sent automatically
    const res = await fetch(
      `${import.meta.env.VITE_API_BASE_URL}/auth/callback`,
      { method: 'POST', credentials: 'include' },
    )
    return res.json()
  }

  async function logout() {
    const { reset: resetPostHog } = usePostHog()
    resetPostHog()
    await Session.signOut()
    sessionExists.value = false
    orgId.value = null
    sessionStorage.removeItem('raven_org_id')
    window.location.href = '/login'
  }

  function setOrgId(id: string) {
    orgId.value = id
    sessionStorage.setItem('raven_org_id', id)
  }

  return {
    sessionExists,
    orgId,
    isAuthenticated,
    hasOrg,
    init,
    loginWithGoogle,
    loginWithPasskey,
    handleCallback,
    logout,
    setOrgId,
  }
})
```

Key differences from the OIDC store:
- No `UserManager`, no token management, no `accessToken` computed
- Session state is a boolean (`doesSessionExist`), not a User object
- No Bearer token needed for API calls -- cookies are sent automatically with `credentials: 'include'`
- `handleCallback()` no longer returns a User object
- API calls use `fetch` with `credentials: 'include'` instead of `Authorization: Bearer <token>`

### 4.4 API Client Changes

All API calls must include `credentials: 'include'` to send session cookies. If using an axios instance:

```typescript
const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL,
  withCredentials: true,  // Send cookies with every request
})
```

Remove any request interceptor that adds `Authorization: Bearer <token>` headers.

---

## 5. Frontend -- Login Page Redesign

### 5.1 LoginPage

Replace the current auto-redirect flow with an intentional login page:

```
+----------------------------------+
|          Raven                    |
|                                  |
|  [G] Sign in with Google         |
|                                  |
|  [*] Sign in with passkey        |
|                                  |
|  --------------------------------|
|  By continuing, you agree to our |
|  Terms of Service & Privacy      |
+----------------------------------+
```

- No auto-redirect to Google on page load
- Two distinct buttons for each login method
- Design: black/white/amber theme consistent with existing Raven UI

### 5.2 CallbackPage

Simplified to handle only the Google OAuth callback:

```typescript
// pages/callback/CallbackPage.vue
onMounted(async () => {
  try {
    await auth.handleCallback()
    // handleCallback calls /api/v1/auth/callback internally
    // which returns { isNewUser, orgId }
    // Route based on response
  } catch (err) {
    router.push('/login')
  }
})
```

No passkey callback needed -- passkey auth completes synchronously in the browser.

---

## 6. Frontend -- Router Guard Changes

### 6.1 Updated Guard Logic

```typescript
router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // Public routes -- no auth check
  if (to.path === '/login' || to.path === '/callback') return
  if (to.path.startsWith('/legal/')) return

  // Initialize session check if not done
  if (!auth.isAuthenticated) {
    await auth.init()
  }

  // Redirect to login if auth required but no session
  if (to.meta.requiresAuth === true && !auth.isAuthenticated) {
    return '/login'
  }

  // Redirect to onboarding if authenticated but no org
  if (auth.isAuthenticated && !auth.hasOrg && to.path !== '/onboarding') {
    return '/onboarding'
  }
})
```

The logic is identical to the current guard. The only change is that `auth.isAuthenticated` now reads from `doesSessionExist()` instead of checking OIDC token expiry.

---

## 7. Config Changes

### 7.1 Go Config (`internal/config/config.go`)

**Remove:**

```go
type ZitadelConfig struct {
    Domain   string `mapstructure:"domain"`
    ClientID string `mapstructure:"client_id"`
    Secure   bool   `mapstructure:"secure"`
    KeyPath  string `mapstructure:"key_path"`
}
```

**Replace with:**

```go
type SuperTokensConfig struct {
    ConnectionURI string `mapstructure:"connection_uri"` // http://supertokens:3567
    APIKey        string `mapstructure:"api_key"`
}

type GoogleOAuthConfig struct {
    ClientID     string `mapstructure:"client_id"`
    ClientSecret string `mapstructure:"client_secret"`
}
```

**Update `Config` struct:**

```go
type Config struct {
    // ...
    SuperTokens SuperTokensConfig
    GoogleOAuth GoogleOAuthConfig
    // Remove: Zitadel ZitadelConfig
}
```

**Update defaults and env bindings:**

```go
// Remove all zitadel.* defaults and bindings
// Add:
v.SetDefault("supertokens.connection_uri", "http://supertokens:3567")
v.SetDefault("supertokens.api_key", "")
_ = v.BindEnv("supertokens.connection_uri", "RAVEN_SUPERTOKENS_CONNECTION_URI")
_ = v.BindEnv("supertokens.api_key", "RAVEN_SUPERTOKENS_API_KEY")
_ = v.BindEnv("googleoauth.client_id", "GOOGLE_CLIENT_ID")
_ = v.BindEnv("googleoauth.client_secret", "GOOGLE_CLIENT_SECRET")
```

**Remove startup warning:**

```go
// Remove:
if cfg.Zitadel.ClientID == "" {
    log.Printf("[WARN] zitadel.client_id is empty ...")
}
```

### 7.2 Environment Variables

**Backend (`.env.example`):**

| Remove | Add |
|--------|-----|
| `ZITADEL_MASTERKEY` | `SUPERTOKENS_API_KEY` |
| `ZITADEL_EXTERNALDOMAIN` | `RAVEN_SUPERTOKENS_CONNECTION_URI` |
| `ZITADEL_EXTERNALPORT` | `GOOGLE_CLIENT_ID` |
| `ZITADEL_EXTERNALSECURE` | `GOOGLE_CLIENT_SECRET` |
| `ZITADEL_DB_PASSWORD` | |
| `ZITADEL_CLIENT_ID` | |

**Frontend (`frontend/.env.example`):**

| Remove | Add |
|--------|-----|
| `VITE_ZITADEL_URL` | `VITE_API_DOMAIN` |
| `VITE_ZITADEL_CLIENT_ID` | |
| `VITE_GOOGLE_IDP_ID` | |

`VITE_API_DOMAIN` is the Go API domain (e.g., `https://api.ravencloak.org` for production, `http://localhost:8081` for development). The `supertokens-web-js` SDK uses this to know where to send auth requests.

---

## 8. Migration Path

### 8.1 Code Removal

**Backend:**
- Delete `deploy/zitadel/` directory (zitadel-config.yaml, zitadel-init-steps.yaml)
- Remove `zitadel` and `zitadel-db` services from all docker-compose files
- Remove JWKS/JWT logic from `internal/middleware/auth.go`
- Remove `keyfunc` and `golang-jwt` Go dependencies

**Frontend:**
- `npm uninstall oidc-client-ts`
- Remove `silent-renew.html` if present
- Remove any Bearer token interceptor logic

### 8.2 Database

The `external_id` column in the `users` table stays. It stores the SuperTokens user ID instead of the Zitadel user ID. The column semantics are identical: opaque string identifying the user in the auth backend.

**No migration script needed for the column itself.** SuperTokens auto-creates its own tables.

### 8.3 Existing Users

Existing users from the Zitadel era will need to **re-register**. A fresh SuperTokens instance has no user data. The `external_id` values will change. This is acceptable because Raven is pre-production and has no end-user data.

### 8.4 Migration SQL

Create `migrations/00035_supertokens_migration.sql`:

```sql
-- Remove Zitadel-specific artifacts from Raven schema.
-- SuperTokens creates its own tables automatically on first boot.

-- Update migration tracking
-- (No schema changes needed -- external_id column is reused as-is)
```

Remove `migrations/00034_zitadel_migration.sql` references if they are Zitadel-specific.

---

## 9. Production Deployment (EC2)

### 9.1 Architecture

```
Internet
  |
  v
Cloudflare Tunnel
  |
  +--> api.ravencloak.org  --> go-api:8081
  |                              |
  |                              +--> /auth/*  --> supertokens:3567 (proxy)
  |                              +--> /api/v1/* --> handlers
  |
  +--> app.ravencloak.org  --> frontend:3080
```

### 9.2 Key Differences from Zitadel Setup

| Aspect | Zitadel | SuperTokens |
|--------|---------|-------------|
| Containers | zitadel + zitadel-db (PG16) | supertokens only |
| Database | Separate PG16 instance required | Shares Raven PG18 |
| External access | Cloudflare Tunnel to `auth.ravencloak.org` | No external access (proxied through Go API) |
| Port exposure | 8080 (loopback) | None (Docker network only) |
| Init steps | Complex YAML config + masterkey | Single env var (connection URI) |
| Health check | `/app/zitadel ready` | `curl http://localhost:3567/hello` |

### 9.3 Cloudflare Tunnel Config

Remove the `auth.ravencloak.org` tunnel route. SuperTokens is not directly accessible. All auth traffic flows through the existing `api.ravencloak.org` tunnel route.

---

## 10. Testing Strategy

### 10.1 Unit Tests

Mock the `AuthProvider` interface for handler/middleware tests:

```go
type mockAuthProvider struct {
    sessionInfo *auth.SessionInfo
    err         error
}

func (m *mockAuthProvider) VerifySession(r *http.Request) (*auth.SessionInfo, error) {
    return m.sessionInfo, m.err
}

func (m *mockAuthProvider) RevokeSession(r *http.Request) error {
    return m.err
}
```

Existing `internal/middleware/auth_test.go` and `internal/handler/user_test.go` should be updated to use the mock instead of constructing JWTs.

### 10.2 Integration Tests

SuperTokens in Docker for real session verification:

```go
// testcontainers setup
supertokensContainer := testcontainers.NewContainer(
    "registry.supertokens.io/supertokens/supertokens-postgresql",
    // ... with PG connection
)
```

Test the full flow:
1. Create a session via SuperTokens Core API
2. Verify the session via `AuthProvider.VerifySession`
3. Revoke the session via `AuthProvider.RevokeSession`

### 10.3 E2E (Playwright)

- **Google OAuth**: Mock the Google OAuth redirect (Playwright can intercept network requests to simulate the OAuth callback)
- **Passkey**: WebAuthn testing requires `authenticatorEnvironment` in Playwright (virtual authenticator)
- Update existing `frontend/e2e/auth.spec.ts` and `frontend/e2e/mobile/login.spec.ts`

---

## File Change Summary

| File | Action |
|------|--------|
| `internal/auth/provider.go` | **Create** -- AuthProvider interface |
| `internal/auth/supertokens.go` | **Create** -- SuperTokens implementation |
| `internal/handler/authproxy.go` | **Create** -- Reverse proxy to SuperTokens |
| `internal/middleware/auth.go` | **Modify** -- Replace JWTMiddleware with SessionMiddleware |
| `internal/config/config.go` | **Modify** -- Replace ZitadelConfig with SuperTokensConfig |
| `cmd/api/main.go` | **Modify** -- Wire new auth provider, add proxy route |
| `docker-compose.yml` | **Modify** -- Replace zitadel with supertokens |
| `deploy/ec2/docker-compose.server.yml` | **Modify** -- Replace zitadel+zitadel-db with supertokens |
| `deploy/postgres/init.sql` | **Modify** -- Remove Zitadel database/user creation |
| `.env.example` | **Modify** -- Replace Zitadel env vars with SuperTokens |
| `frontend/.env.example` | **Modify** -- Replace Zitadel env vars |
| `frontend/src/stores/auth.ts` | **Rewrite** -- SuperTokens SDK, cookie-based sessions |
| `frontend/src/router/index.ts` | **Modify** -- Update guard to use doesSessionExist |
| `frontend/src/pages/LoginPage.vue` | **Rewrite** -- Two-button login (Google + passkey) |
| `frontend/src/pages/callback/CallbackPage.vue` | **Modify** -- Use signInAndUp callback |
| `frontend/src/plugins/supertokens.ts` | **Create** -- SDK initialization |
| `frontend/package.json` | **Modify** -- Swap oidc-client-ts for supertokens-web-js |
| `deploy/zitadel/` | **Delete** -- Entire directory |
| `migrations/00035_supertokens_migration.sql` | **Create** -- Cleanup migration |
| `internal/middleware/auth_test.go` | **Modify** -- Use mock AuthProvider |
| `internal/handler/user_test.go` | **Modify** -- Use mock AuthProvider |
| `frontend/e2e/auth.spec.ts` | **Modify** -- Update for new login flow |

---

## Go Dependencies

**Remove:**
- `github.com/MicahParks/keyfunc/v3`
- `github.com/golang-jwt/jwt/v5`

**Add:**
- No new Go dependencies. The SuperTokens integration uses raw HTTP calls to the Core API. No Go SDK is needed.

## Frontend Dependencies

**Remove:**
- `oidc-client-ts`

**Add:**
- `supertokens-web-js`

---

## Open Questions

None. All decisions are approved and specified above.
