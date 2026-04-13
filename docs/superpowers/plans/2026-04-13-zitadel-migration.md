# Zitadel Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Keycloak with Zitadel as the identity provider, add Cloudflare Tunnels for routing, update Go backend JWT middleware, and rebuild the Vue frontend auth flow with a 2-step onboarding wizard.

**Architecture:** Three-phase sequential migration: (1) Infrastructure — swap Docker Compose services, (2) Backend — replace JWT middleware and provisioning endpoints, (3) Frontend — swap OIDC client and rebuild onboarding. Zitadel handles identity only; Raven continues to manage multi-tenancy via PostgreSQL RLS.

**Tech Stack:** Zitadel v2.71.12, `github.com/zitadel/oidc/v3` (Go), `oidc-client-ts` (Vue), Cloudflare Tunnels (`cloudflared`), PostgreSQL 18, Gin, Vue 3 + Pinia.

**Spec:** `docs/superpowers/specs/2026-04-13-zitadel-migration-design.md`

---

## File Structure

### Phase 1: Infrastructure
```
deploy/
├── zitadel/
│   ├── zitadel-config.yaml       # Static Zitadel config (pool sizes, logging)
│   └── zitadel-init-steps.yaml   # Auto-creates OIDC app + machine user on first boot
├── postgres/
│   └── init.sql                  # MODIFY: add zitadel database + user, remove keycloak
docker-compose.yml                # MODIFY: remove keycloak, add zitadel + cloudflared
.env.example                      # MODIFY: replace keycloak vars with zitadel + CF vars
```

### Phase 2: Backend
```
migrations/
└── 00034_zitadel_migration.sql   # Rename columns, make org_id nullable, drop keycloak_realm
internal/
├── config/
│   └── config.go                 # MODIFY: ZitadelConfig replaces KeycloakConfig, port → 8081
├── middleware/
│   └── auth.go                   # MODIFY: Zitadel JWKS validation, OptionalOrgContext
├── model/
│   ├── user.go                   # MODIFY: ExternalID replaces KeycloakSub, remove webhook types
│   └── org.go                    # MODIFY: remove KeycloakRealm field
├── repository/
│   ├── user.go                   # MODIFY: external_id queries replace keycloak_sub
│   └── org.go                    # MODIFY: remove keycloak_realm from columns
├── handler/
│   ├── auth.go                   # CREATE: auth callback + passkey proxy endpoints
│   └── provision.go              # MODIFY: remove Keycloak realm provisioning
├── service/
│   ├── user.go                   # MODIFY: remove HandleKeycloakEvent
│   └── provision.go              # MODIFY: remove Keycloak admin client
cmd/api/
└── main.go                       # MODIFY: routes, middleware setup, remove keycloak webhook
go.mod                            # MODIFY: add zitadel/oidc, remove keycloak deps
```

### Phase 3: Frontend
```
frontend/
├── src/
│   ├── stores/
│   │   └── auth.ts               # MODIFY: replace keycloak-js with oidc-client-ts
│   ├── composables/
│   │   └── useAuth.ts            # MODIFY: update to work with new auth store
│   ├── pages/
│   │   ├── callback/
│   │   │   └── CallbackPage.vue  # CREATE: OIDC callback handler
│   │   ├── login/
│   │   │   └── LoginPage.vue     # CREATE: auto-redirect to Google via Zitadel
│   │   └── onboarding/
│   │       └── OnboardingWizard.vue  # MODIFY: replace 5-step with 2-step wizard
│   ├── router/
│   │   └── index.ts              # MODIFY: update guards, add callback/login routes
│   └── silent-renew.html         # CREATE: silent token renewal page
├── package.json                  # MODIFY: remove keycloak-js, add oidc-client-ts
└── .env.example                  # MODIFY: Zitadel vars replace Keycloak vars
```

---

## Phase 1: Infrastructure

### Task 1: Zitadel Configuration Files

**Files:**
- Create: `deploy/zitadel/zitadel-config.yaml`
- Create: `deploy/zitadel/zitadel-init-steps.yaml`

- [ ] **Step 1: Create zitadel-config.yaml**

```yaml
# deploy/zitadel/zitadel-config.yaml
# Static defaults — dynamic values (domain, DB creds) come from env vars
Log:
  Level: info

Database:
  postgres:
    MaxOpenConns: 10
    MaxIdleConns: 5
    MaxConnLifetime: 30m
    MaxConnIdleTime: 5m
```

- [ ] **Step 2: Create zitadel-init-steps.yaml**

This file auto-creates the OIDC application and machine user on first boot. Reference Zitadel docs for the exact format — the key structure is:

```yaml
# deploy/zitadel/zitadel-init-steps.yaml
FirstInstance:
  Org:
    Human:
      UserName: "admin@ravencloak.org"
      FirstName: "Raven"
      LastName: "Admin"
      Password: "RavenAdmin1!"  # Change on first login
    Machine:
      UserName: "raven-api"
      Name: "Raven API Service User"
      PAT:
        ExpirationDate: "2030-01-01T00:00:00Z"
  OIDCApp:
    Name: "raven-app"
    RedirectURIs:
      - "https://app.ravencloak.org/callback"
      - "http://localhost:5173/callback"
    PostLogoutRedirectURIs:
      - "https://app.ravencloak.org"
      - "http://localhost:5173"
    ResponseTypes:
      - "OIDC_RESPONSE_TYPE_CODE"
    GrantTypes:
      - "OIDC_GRANT_TYPE_AUTHORIZATION_CODE"
    AuthMethodType: "OIDC_AUTH_METHOD_TYPE_NONE"
    AppType: "OIDC_APP_TYPE_USER_AGENT"
    DevMode: true
```

Note: The exact YAML schema depends on Zitadel version. The implementer should verify against Zitadel v2.71.12 docs. The key requirements are: PKCE-enabled public SPA client, correct redirect URIs, and a machine user for backend API calls.

- [ ] **Step 3: Commit**

```bash
git add deploy/zitadel/
git commit -m "feat(infra): add Zitadel configuration files"
```

---

### Task 2: Docker Compose — Swap Keycloak for Zitadel + Cloudflare Tunnels

**Files:**
- Modify: `docker-compose.yml` (lines 181-210: keycloak service, line 20-21: go-api depends_on)
- Modify: `deploy/postgres/init.sql` (line 5: keycloak database creation)
- Modify: `.env.example` (lines 10-24: Keycloak vars)

- [ ] **Step 1: Read current docker-compose.yml**

Read the full file to understand the structure before making changes.

- [ ] **Step 2: Remove Keycloak service from docker-compose.yml**

Remove the entire `keycloak` service block (lines 181-210) and the `kc-data` volume.

- [ ] **Step 3: Add Zitadel service to docker-compose.yml**

Add after the postgres service:

```yaml
  zitadel:
    image: ghcr.io/zitadel/zitadel:v2.71.12
    restart: always
    command: >
      start-from-init
      --config /zitadel-config.yaml
      --steps /zitadel-init-steps.yaml
      --masterkey "${ZITADEL_MASTERKEY}"
      --tlsMode disabled
    ports:
      - "8080:8080"
    volumes:
      - ./deploy/zitadel/zitadel-config.yaml:/zitadel-config.yaml:ro
      - ./deploy/zitadel/zitadel-init-steps.yaml:/zitadel-init-steps.yaml:ro
    depends_on:
      postgres:
        condition: service_healthy
    networks:
      - raven-internal
    environment:
      - ZITADEL_MASTERKEY=${ZITADEL_MASTERKEY}
      - ZITADEL_EXTERNALDOMAIN=${ZITADEL_EXTERNALDOMAIN:-localhost}
      - ZITADEL_EXTERNALPORT=${ZITADEL_EXTERNALPORT:-8080}
      - ZITADEL_EXTERNALSECURE=${ZITADEL_EXTERNALSECURE:-false}
      - ZITADEL_DATABASE_POSTGRES_HOST=postgres
      - ZITADEL_DATABASE_POSTGRES_PORT=5432
      - ZITADEL_DATABASE_POSTGRES_DATABASE=zitadel
      - ZITADEL_DATABASE_POSTGRES_USER_USERNAME=zitadel
      - ZITADEL_DATABASE_POSTGRES_USER_PASSWORD=${ZITADEL_DB_PASSWORD:-zitadel}
      - ZITADEL_DATABASE_POSTGRES_ADMIN_USERNAME=${POSTGRES_USER:-raven}
      - ZITADEL_DATABASE_POSTGRES_ADMIN_PASSWORD=${POSTGRES_PASSWORD}
    healthcheck:
      test: ["CMD", "/app/zitadel", "ready"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s
```

- [ ] **Step 4: Add cloudflared service (production profile)**

```yaml
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

- [ ] **Step 5: Update go-api service**

- Change port mapping from `8080:8080` to `8081:8081`
- Update `depends_on`: replace `keycloak` with `zitadel` (condition: `service_healthy`)
- Add Zitadel env vars: `ZITADEL_EXTERNALDOMAIN`, `ZITADEL_CLIENT_ID`

- [ ] **Step 6: Update deploy/postgres/init.sql**

Replace the `CREATE DATABASE keycloak;` line with:

```sql
CREATE DATABASE zitadel;
CREATE USER zitadel WITH PASSWORD 'zitadel';
GRANT ALL PRIVILEGES ON DATABASE zitadel TO zitadel;
ALTER USER zitadel CREATEDB;
```

- [ ] **Step 7: Update .env.example**

Remove all `KEYCLOAK_*`, `KC_*`, `RAVEN_KEYCLOAK_*` variables. Add:

```env
# Zitadel
ZITADEL_MASTERKEY=MustBeAtLeast32CharactersLongKey!
ZITADEL_EXTERNALDOMAIN=localhost
ZITADEL_EXTERNALPORT=8080
ZITADEL_EXTERNALSECURE=false
ZITADEL_DB_PASSWORD=zitadel
ZITADEL_CLIENT_ID=  # Set after first Zitadel boot (from init output)

# Google IDP (configure in Zitadel admin console)
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# Cloudflare Tunnels (production only)
CF_TUNNEL_TOKEN=
```

- [ ] **Step 8: Remove deploy/keycloak/ directory**

```bash
rm -rf deploy/keycloak/
```

- [ ] **Step 9: Test Docker Compose**

```bash
docker compose up postgres zitadel -d
# Wait for zitadel healthy
docker compose ps
# Expected: postgres (healthy), zitadel (healthy)
# Verify Zitadel UI: open http://localhost:8080 in browser
```

- [ ] **Step 10: Commit**

```bash
git add docker-compose.yml deploy/ .env.example
git commit -m "feat(infra): replace Keycloak with Zitadel, add Cloudflare Tunnels"
```

---

## Phase 2: Backend

### Task 3: Database Migration

**Files:**
- Create: `migrations/00034_zitadel_migration.sql`

- [ ] **Step 1: Create migration file**

```sql
-- migrations/00034_zitadel_migration.sql
-- +goose Up
-- +goose StatementBegin

-- Rename keycloak_sub to external_id in users table
ALTER TABLE users RENAME COLUMN keycloak_sub TO external_id;
ALTER TABLE users ADD COLUMN auth_provider TEXT NOT NULL DEFAULT 'zitadel';
DROP INDEX IF EXISTS idx_users_keycloak_sub;
CREATE UNIQUE INDEX idx_users_external_id ON users(external_id) WHERE external_id IS NOT NULL;

-- Make org_id nullable for pre-onboarding users
ALTER TABLE users ALTER COLUMN org_id DROP NOT NULL;

-- Drop Keycloak-specific column from organizations
ALTER TABLE organizations DROP COLUMN IF EXISTS keycloak_realm;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE organizations ADD COLUMN keycloak_realm TEXT;
ALTER TABLE users ALTER COLUMN org_id SET NOT NULL;
DROP INDEX IF EXISTS idx_users_external_id;
ALTER TABLE users DROP COLUMN IF EXISTS auth_provider;
ALTER TABLE users RENAME COLUMN external_id TO keycloak_sub;
CREATE INDEX idx_users_keycloak_sub ON users(keycloak_sub) WHERE keycloak_sub IS NOT NULL;

-- +goose StatementEnd
```

- [ ] **Step 2: Run migration locally**

```bash
~/go/bin/goose -dir migrations postgres "postgresql://raven:changeme@127.0.0.1:15432/raven?sslmode=disable" up
```

Expected: Migration 00034 applied successfully.

- [ ] **Step 3: Commit**

```bash
git add migrations/00034_zitadel_migration.sql
git commit -m "feat(db): migration for Zitadel — rename keycloak columns, nullable org_id"
```

---

### Task 4: Config — Replace KeycloakConfig with ZitadelConfig

**Files:**
- Modify: `internal/config/config.go` (lines 151-170: KeycloakConfig)

- [ ] **Step 1: Read current config.go**

Read the full file to understand the config struct and Viper setup.

- [ ] **Step 2: Replace KeycloakConfig struct with ZitadelConfig**

Replace the `KeycloakConfig` struct and its Viper bindings with:

```go
type ZitadelConfig struct {
    Domain   string `mapstructure:"domain"`
    ClientID string `mapstructure:"client_id"`
    Secure   bool   `mapstructure:"secure"`
    KeyPath  string `mapstructure:"key_path"` // path to machine user key JSON (optional)
}
```

Update Viper defaults:
```go
v.SetDefault("zitadel.domain", "localhost:8080")
v.SetDefault("zitadel.client_id", "")
v.SetDefault("zitadel.secure", false)
v.SetDefault("server.port", 8081) // was 8080, Zitadel takes that now
```

Update env var bindings:
```go
v.BindEnv("zitadel.domain", "ZITADEL_EXTERNALDOMAIN")
v.BindEnv("zitadel.client_id", "ZITADEL_CLIENT_ID")
v.BindEnv("zitadel.secure", "ZITADEL_EXTERNALSECURE")
```

Replace `Keycloak KeycloakConfig` field in the main `Config` struct with `Zitadel ZitadelConfig`.

- [ ] **Step 3: Remove all Keycloak config references**

Search for and remove any `cfg.Keycloak.*` references throughout the codebase. Update callers to use `cfg.Zitadel.*`.

- [ ] **Step 4: Test build compiles**

```bash
go build ./...
```

Expected: Build errors for files still referencing Keycloak — that's OK, we fix them in subsequent tasks.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): replace KeycloakConfig with ZitadelConfig, port to 8081"
```

---

### Task 5: Model — Update User and Org Structs

**Files:**
- Modify: `internal/model/user.go` (lines 15-42)
- Modify: `internal/model/org.go` (lines 16-25)

- [ ] **Step 1: Update User struct**

In `internal/model/user.go`:
- Rename `KeycloakSub string` → `ExternalID string` (update json/db tags: `json:"external_id" db:"external_id"`)
- Add `AuthProvider string` field (`json:"auth_provider" db:"auth_provider"`)
- Make `OrgID` a pointer type: `OrgID *uuid.UUID` (nullable)
- Remove `KeycloakWebhookEvent` struct entirely (lines 34-42)

- [ ] **Step 2: Update Org struct**

In `internal/model/org.go`:
- Remove `KeycloakRealm string` field (line 22)

- [ ] **Step 3: Test build**

```bash
go build ./...
```

Expected: Build errors in repository and service layers — addressed in next tasks.

- [ ] **Step 4: Commit**

```bash
git add internal/model/user.go internal/model/org.go
git commit -m "feat(model): rename KeycloakSub to ExternalID, remove Keycloak fields"
```

---

### Task 6: Repository — Update User and Org Queries

**Files:**
- Modify: `internal/repository/user.go` (lines 23, 44-59, 77-90)
- Modify: `internal/repository/org.go` (line 24)

- [ ] **Step 1: Update user repository**

In `internal/repository/user.go`:
- Line 23: Change `keycloak_sub` → `external_id` in column list, add `auth_provider`
- Rename `UpsertByKeycloakSub()` → `UpsertByExternalID()` — update INSERT/ON CONFLICT to use `external_id`
- Rename `GetByKeycloakSub()` → `GetByExternalID()` — update WHERE clause
- Handle nullable `org_id` in scan (use `sql.NullString` or pointer)

- [ ] **Step 2: Update org repository**

In `internal/repository/org.go`:
- Line 24: Remove `keycloak_realm` from `orgColumns`
- Remove `keycloak_realm` from any INSERT/UPDATE queries
- Remove `keycloak_realm` from scan targets

- [ ] **Step 3: Test build compiles**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/repository/user.go internal/repository/org.go
git commit -m "feat(repo): update queries for external_id, remove keycloak columns"
```

---

### Task 7: Middleware — Zitadel JWT Validation

**Files:**
- Modify: `internal/middleware/auth.go` (lines 45-210)

- [ ] **Step 1: Read current auth.go**

Read the full file to understand the JWKS validation flow, claims parsing, and context key setup.

- [ ] **Step 2: Replace Keycloak JWKS validation with Zitadel**

Update the middleware to:
- Build JWKS URL from Zitadel domain: `https://{domain}/.well-known/openid-configuration` (or use `github.com/zitadel/oidc/v3` for automatic discovery)
- Validate JWT: verify `iss` matches Zitadel domain, `aud` contains client ID
- Extract claims: `sub` (Zitadel user ID), `email`, `name`
- Store in Gin context: `ContextKeyUserID` = sub, `ContextKeyEmail` = email

- [ ] **Step 3: Add user lookup from database**

After JWT validation, resolve `external_id` → internal user. Add to middleware:

```go
// After JWT validation, look up internal user
user, err := userRepo.GetByExternalID(c.Request.Context(), claims.Subject)
if err != nil {
    // User not found = first login, let /auth/callback handle creation
    c.Set(ContextKeyExternalID, claims.Subject)
    c.Set(ContextKeyEmail, claims.Email)
    c.Set(ContextKeyUserName, claims.Name)
    c.Next()
    return
}
c.Set(ContextKeyUserID, user.ID.String())
if user.OrgID != nil {
    c.Set(ContextKeyOrgID, user.OrgID.String())
}
```

- [ ] **Step 4: Add OptionalOrgContext helper**

Create a middleware that doesn't require `ContextKeyOrgID` to be set — for routes accessible by users without an org (auth callback, org creation):

```go
func RequireOrg() gin.HandlerFunc {
    return func(c *gin.Context) {
        orgID := c.GetString(ContextKeyOrgID)
        if orgID == "" {
            c.AbortWithStatusJSON(403, gin.H{"error": "organization required"})
            return
        }
        c.Next()
    }
}
```

- [ ] **Step 5: Remove all Keycloak-specific code**

Remove `raven-org` scope parsing, realm-based issuer construction, and any Keycloak-specific claim extraction.

- [ ] **Step 6: Update go.mod**

```bash
go get github.com/zitadel/oidc/v3
go mod tidy
```

- [ ] **Step 7: Test build**

```bash
go build ./...
```

- [ ] **Step 8: Commit**

```bash
git add internal/middleware/ go.mod go.sum
git commit -m "feat(auth): Zitadel JWT validation middleware with user lookup"
```

---

### Task 8: Auth Handler — Callback + Org Creation Endpoints

**Files:**
- Create: `internal/handler/auth.go`
- Modify: `cmd/api/main.go` (routes)

- [ ] **Step 1: Create auth handler**

Create `internal/handler/auth.go` with:

```go
type AuthHandler struct {
    userService    *service.UserService
    orgService     *service.OrgService
    billingService *service.BillingService
}

// POST /api/v1/auth/callback
// Called by frontend after OIDC callback
func (h *AuthHandler) Callback(c *gin.Context) {
    externalID := c.GetString(middleware.ContextKeyExternalID)
    email := c.GetString(middleware.ContextKeyEmail)
    name := c.GetString(middleware.ContextKeyUserName)

    // Check if user exists
    user, err := h.userService.GetByExternalID(c.Request.Context(), externalID)
    if err != nil {
        // New user — create record with nil org_id
        user, err = h.userService.Create(c.Request.Context(), externalID, email, name)
        if err != nil {
            c.JSON(500, gin.H{"error": "failed to create user"})
            return
        }
        c.JSON(200, gin.H{"isNewUser": true})
        return
    }

    // Existing user
    if user.OrgID == nil {
        // Abandoned onboarding — re-enter
        c.JSON(200, gin.H{"isNewUser": true})
        return
    }

    c.JSON(200, gin.H{
        "isNewUser": false,
        "orgId":     user.OrgID.String(),
    })
}
```

- [ ] **Step 2: Modify org creation in cmd/api/main.go**

Update the `POST /orgs` route:
- Remove `middleware.RequireOrgRole("org_admin")` guard
- Add logic: if user has no org (`ContextKeyOrgID` is empty), allow creation
- After org creation, update `users.org_id` and assign free tier billing

- [ ] **Step 3: Add routes to main.go**

```go
// Auth routes (authenticated but org not required)
authGroup := api.Group("/auth")
authGroup.Use(jwtMiddleware) // no RequireOrg
{
    authGroup.POST("/callback", authHandler.Callback)
}
```

- [ ] **Step 4: Remove Keycloak webhook route**

Remove from `cmd/api/main.go`:
- Line 728: `POST /api/v1/internal/keycloak-webhook`
- Remove `HandleKeycloakEvent` from `internal/service/user.go`

- [ ] **Step 5: Test build**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/handler/auth.go internal/service/user.go cmd/api/main.go
git commit -m "feat(auth): auth callback endpoint, remove Keycloak webhook"
```

---

### Task 9: Cleanup — Remove Keycloak Provisioning

**Files:**
- Modify: `internal/handler/provision.go`
- Modify: `internal/service/provision.go`
- Modify: `cmd/api/main.go` (remove provision routes)

- [ ] **Step 1: Gut provision handler**

In `internal/handler/provision.go`:
- Remove `ProvisionRealm()` handler and `RequireInternalAuth()` middleware
- Remove Keycloak admin URL/credentials fields from `ProvisionHandler` struct
- Keep the file if other non-Keycloak provisioning logic exists, otherwise delete it

- [ ] **Step 2: Remove provision service Keycloak client**

In `internal/service/provision.go`:
- Remove all Keycloak admin API calls
- Remove `CreateKeycloakRealm()` and related methods

- [ ] **Step 3: Remove provision routes from main.go**

Remove:
- Line 738: `POST /internal/provision-realm` route
- Any `requireInternalAuth` middleware references

- [ ] **Step 4: Run full test suite**

```bash
go test ./... -count=1
```

Fix any compilation errors from removed types/functions.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/provision.go internal/service/provision.go cmd/api/main.go
git commit -m "refactor(auth): remove Keycloak provisioning and admin client"
```

---

### Task 10: Backend Integration Test

**Files:** None created — verification task.

- [ ] **Step 1: Start services**

```bash
docker compose up postgres zitadel -d
# Wait for both healthy
docker compose ps
```

- [ ] **Step 2: Run migration**

```bash
~/go/bin/goose -dir migrations postgres "postgresql://raven:changeme@127.0.0.1:15432/raven?sslmode=disable" up
```

- [ ] **Step 3: Start Go API**

```bash
go run cmd/api/main.go
```

Expected: API starts on port 8081, no Keycloak errors.

- [ ] **Step 4: Verify Zitadel JWKS discovery**

```bash
curl -s http://localhost:8080/.well-known/openid-configuration | jq .jwks_uri
```

Expected: Returns a valid JWKS URI.

- [ ] **Step 5: Run Go tests**

```bash
go test ./... -count=1
```

Expected: All tests pass (some may need updating if they mock Keycloak).

- [ ] **Step 6: Commit any test fixes**

```bash
git add -A
git commit -m "fix(tests): update tests for Zitadel migration"
```

---

## Phase 3: Frontend

### Task 11: Swap Auth Library

**Files:**
- Modify: `frontend/package.json` (line 20: keycloak-js)
- Modify: `frontend/src/stores/auth.ts` (Keycloak init)
- Modify: `frontend/.env.example`

- [ ] **Step 1: Swap npm dependencies**

```bash
cd frontend
npm uninstall keycloak-js
npm install oidc-client-ts
```

- [ ] **Step 2: Rewrite auth store**

Replace `frontend/src/stores/auth.ts` entirely. The new store uses `oidc-client-ts`:

```typescript
import { defineStore } from 'pinia'
import { UserManager, WebStorageStateStore } from 'oidc-client-ts'
import { ref, computed } from 'vue'

const userManager = new UserManager({
  authority: import.meta.env.VITE_ZITADEL_URL,
  client_id: import.meta.env.VITE_ZITADEL_CLIENT_ID,
  redirect_uri: `${window.location.origin}/callback`,
  post_logout_redirect_uri: window.location.origin,
  silent_redirect_uri: `${window.location.origin}/silent-renew.html`,
  scope: 'openid profile email',
  response_type: 'code',
  automaticSilentRenew: true,
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
})

export const useAuthStore = defineStore('auth', () => {
  const user = ref(null)
  const orgId = ref(null)
  const isAuthenticated = computed(() => !!user.value)
  const hasOrg = computed(() => !!orgId.value)
  const accessToken = computed(() => user.value?.access_token)

  async function init() {
    const existingUser = await userManager.getUser()
    if (existingUser && !existingUser.expired) {
      user.value = existingUser
    }
  }

  async function login(idpHint?: string) {
    const extraParams = idpHint ? { idp_hint: idpHint } : {}
    await userManager.signinRedirect({ extraQueryParams: extraParams })
  }

  async function handleCallback() {
    const callbackUser = await userManager.signinRedirectCallback()
    user.value = callbackUser
    return callbackUser
  }

  async function logout() {
    await userManager.signoutRedirect()
    user.value = null
    orgId.value = null
  }

  function setOrgId(id: string) {
    orgId.value = id
  }

  return { user, orgId, isAuthenticated, hasOrg, accessToken, init, login, handleCallback, logout, setOrgId }
})
```

- [ ] **Step 3: Update useAuth composable**

Update `frontend/src/composables/useAuth.ts` to work with the new store interface.

- [ ] **Step 4: Update frontend/.env.example**

Remove `VITE_KEYCLOAK_*` vars. Add:
```env
VITE_ZITADEL_URL=http://localhost:8080
VITE_ZITADEL_CLIENT_ID=
VITE_GOOGLE_IDP_ID=
VITE_API_URL=http://localhost:8081
```

- [ ] **Step 5: Test build**

```bash
npm run build
```

Expected: Build errors from pages still importing Keycloak — fixed in next tasks.

- [ ] **Step 6: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/stores/auth.ts frontend/src/composables/useAuth.ts frontend/.env.example
git commit -m "feat(frontend): replace keycloak-js with oidc-client-ts auth store"
```

---

### Task 12: Login + Callback Pages

**Files:**
- Create: `frontend/src/pages/login/LoginPage.vue`
- Create: `frontend/src/pages/callback/CallbackPage.vue`
- Create: `frontend/public/silent-renew.html`

- [ ] **Step 1: Create LoginPage.vue**

```vue
<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black">
    <div class="text-center">
      <h1 class="text-2xl font-bold text-neutral-900 dark:text-white mb-4">Redirecting to Google...</h1>
      <p class="text-neutral-500">If you are not redirected, <button @click="login" class="text-amber-500 underline">click here</button>.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const googleIdpId = import.meta.env.VITE_GOOGLE_IDP_ID

function login() {
  auth.login(googleIdpId)
}

onMounted(() => {
  login()
})
</script>
```

- [ ] **Step 2: Create CallbackPage.vue**

```vue
<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black">
    <div class="text-center">
      <p class="text-neutral-500">Completing sign in...</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const apiUrl = import.meta.env.VITE_API_URL

onMounted(async () => {
  try {
    const user = await auth.handleCallback()

    // Call backend auth callback
    const res = await fetch(`${apiUrl}/api/v1/auth/callback`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${user.access_token}`,
        'Content-Type': 'application/json',
      },
    })
    const data = await res.json()

    if (data.isNewUser) {
      router.push('/onboarding')
    } else {
      auth.setOrgId(data.orgId)
      router.push('/dashboard')
    }
  } catch (err) {
    console.error('Callback error:', err)
    router.push('/login')
  }
})
</script>
```

- [ ] **Step 3: Create silent-renew.html**

```html
<!-- frontend/public/silent-renew.html -->
<!DOCTYPE html>
<html>
<head><title>Silent Renew</title></head>
<body>
  <script src="/node_modules/oidc-client-ts/dist/browser/oidc-client-ts.js"></script>
  <script>
    new oidc.UserManager({ response_mode: 'query' }).signinSilentCallback()
  </script>
</body>
</html>
```

Note: The exact path to `oidc-client-ts` browser bundle depends on the Vite build. The implementer should verify this works or use an alternative approach (service worker or iframe with bundled JS).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/login/ frontend/src/pages/callback/ frontend/public/silent-renew.html
git commit -m "feat(frontend): login, callback, and silent renewal pages"
```

---

### Task 13: 2-Step Onboarding Wizard

**Files:**
- Modify: `frontend/src/pages/onboarding/OnboardingWizard.vue`

- [ ] **Step 1: Read current OnboardingWizard.vue**

Read the full file to understand the current 5-step wizard structure.

- [ ] **Step 2: Replace with 2-step wizard**

Rewrite `OnboardingWizard.vue` with:

**Step 1: "Name your organization"**
- Text input pre-filled with user's display name + "'s Team"
- Validation: 3-100 chars
- "Continue" button → `POST /api/v1/orgs` with `{ name }` + Bearer token
- On success: store orgId, advance to step 2

**Step 2: "Create your first knowledge base"**
- KB name input (required)
- Optional drag-and-drop file upload zone
- "Get Started" button →
  1. `POST /api/v1/workspaces` with `{ name: "Default", orgId }` → workspaceId
  2. `POST /api/v1/knowledge-bases` with `{ name, workspaceId }` → kbId
  3. If files: `POST /api/v1/documents` for each
  4. Navigate to `/dashboard`

Style: black/white + amber accent, consistent with landing page.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/onboarding/
git commit -m "feat(frontend): 2-step onboarding wizard (name org + create KB)"
```

---

### Task 14: Router — Update Guards and Routes

**Files:**
- Modify: `frontend/src/router/index.ts`

- [ ] **Step 1: Read current router**

Read the full file to understand the current route definitions and guards.

- [ ] **Step 2: Add new routes**

```typescript
{
  path: '/login',
  name: 'login',
  component: () => import('@/pages/login/LoginPage.vue'),
  meta: { requiresAuth: false },
},
{
  path: '/callback',
  name: 'callback',
  component: () => import('@/pages/callback/CallbackPage.vue'),
  meta: { requiresAuth: false },
},
```

- [ ] **Step 3: Update router guard**

Replace the Keycloak-specific guard with:

```typescript
router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // Skip auth check for public routes
  if (to.path === '/login' || to.path === '/callback') return

  // Initialize auth if not done
  if (!auth.isAuthenticated) {
    await auth.init()
  }

  // Redirect to login if auth required but not authenticated
  if (to.meta.requiresAuth !== false && !auth.isAuthenticated) {
    return '/login'
  }

  // Redirect to onboarding if authenticated but no org
  if (auth.isAuthenticated && !auth.hasOrg && to.path !== '/onboarding') {
    return '/onboarding'
  }
})
```

- [ ] **Step 4: Remove Keycloak callback handling**

Remove any code that checks for `code=` and `state=` query params (Keycloak callback detection).

- [ ] **Step 5: Test build**

```bash
cd frontend && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/router/
git commit -m "feat(frontend): update router with login/callback routes and Zitadel guards"
```

---

### Task 15: Frontend Integration Test

**Files:** None created — verification task.

- [ ] **Step 1: Start all services**

```bash
docker compose up -d
cd frontend && npm run dev
```

- [ ] **Step 2: Test login flow**

1. Open `http://localhost:5173`
2. Should redirect to `/login` → Zitadel → Google
3. After Google auth → `/callback` → Raven API → `/onboarding`

- [ ] **Step 3: Test onboarding**

1. Name org → "Continue"
2. Name KB → "Get Started"
3. Should land on `/dashboard`

- [ ] **Step 4: Test return visit**

1. Refresh page
2. Should go directly to `/dashboard` (session persists via sessionStorage)

- [ ] **Step 5: Test theme toggle**

Verify dark/light theme still works throughout the app.

- [ ] **Step 6: Run linters**

```bash
cd frontend && npx vue-tsc --noEmit && npx eslint src/
```

- [ ] **Step 7: Commit any fixes**

```bash
git add -A
git commit -m "fix(frontend): integration test fixes for Zitadel auth flow"
```
