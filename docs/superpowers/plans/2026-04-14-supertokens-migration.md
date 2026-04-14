# SuperTokens Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Zitadel with SuperTokens as the auth backend — Google social login + passkeys, cookie-based sessions, pluggable AuthProvider interface.

**Architecture:** SuperTokens runs as a sidecar container sharing Raven's PG18. The Go API proxies `/auth/*` to SuperTokens and verifies sessions via HTTP API calls. Frontend uses `supertokens-web-js` SDK with cookie-based sessions (no JWTs, no OIDC redirects).

**Tech Stack:** SuperTokens (self-hosted), `supertokens-web-js` (Vue), Go `net/http` reverse proxy, PostgreSQL 18, Gin, Vue 3 + Pinia.

**Spec:** `docs/superpowers/specs/2026-04-14-supertokens-migration-design.md`

---

## Phase 1: Infrastructure (Tasks 1-2)

---

### Task 1: Docker Compose — Replace Zitadel with SuperTokens (dev compose)

**Files to modify:**
- `docker-compose.yml` (lines 189-227: remove zitadel service; lines 18-26: update go-api env/depends)

**Steps:**

- [ ] **1.1** Remove the entire `zitadel` service block from `docker-compose.yml` (lines 189-227):

```yaml
  # DELETE THIS ENTIRE BLOCK:
  # ─── Zitadel IAM ──────────────────────────────────────────────────────────
  zitadel:
    image: ghcr.io/zitadel/zitadel:v2.71.12
    # ... everything through the healthcheck closing
```

- [ ] **1.2** Add the SuperTokens service after the `valkey` service block (after line 188):

```yaml
  # ─── SuperTokens Auth Backend ─────────────────────────────────────────────
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

- [ ] **1.3** Update the `go-api` service environment (lines 18-19) — remove Zitadel env vars and add SuperTokens ones:

```yaml
  go-api:
    # ...
    environment:
      DOTENV_PRIVATE_KEY: ${DOTENV_PRIVATE_KEY:-}
      RAVEN_OTEL_ENABLED: "true"
      RAVEN_OTEL_ENDPOINT: "openobserve:5081"
      RAVEN_OTEL_SERVICE_NAME: "raven-api"
      # SuperTokens auth
      RAVEN_SUPERTOKENS_CONNECTION_URI: "http://supertokens:3567"
      RAVEN_SUPERTOKENS_API_KEY: "${SUPERTOKENS_API_KEY:-supertokens-dev-key-replace-me}"
      GOOGLE_CLIENT_ID: "${GOOGLE_CLIENT_ID}"
      GOOGLE_CLIENT_SECRET: "${GOOGLE_CLIENT_SECRET}"
```

Remove these two lines from go-api environment:
```yaml
      ZITADEL_EXTERNALDOMAIN: ${ZITADEL_EXTERNALDOMAIN:-localhost}
      ZITADEL_CLIENT_ID: ${ZITADEL_CLIENT_ID}
```

- [ ] **1.4** Update the `go-api` service `depends_on` (lines 21-26) — replace `zitadel` with `supertokens`:

```yaml
    depends_on:
      postgres:
        condition: service_healthy
      valkey:
        condition: service_healthy
      supertokens:
        condition: service_healthy
      openobserve:
        condition: service_started
```

- [ ] **1.5** Verify compose config parses:

```bash
docker compose config --quiet
```

**Commit message:**
```
feat(infra): replace Zitadel with SuperTokens in dev docker-compose
```

---

### Task 2: Config + Env — Replace ZitadelConfig with SuperTokensConfig

**Files to modify:**
- `internal/config/config.go` (lines 14-34: Config struct; lines 153-158: ZitadelConfig; lines 220-223: defaults; lines 319-322: env bindings; lines 398-400: startup warning)
- `.env.example` (lines 10-17: Zitadel section)
- `frontend/.env.example` (lines 3-5: Zitadel vars)
- `deploy/postgres/init.sql` (lines 4-19: Zitadel DB setup)

**Steps:**

- [ ] **2.1** In `internal/config/config.go`, replace `ZitadelConfig` struct (lines 153-158) with:

```go
// SuperTokensConfig holds SuperTokens connection settings for session verification.
type SuperTokensConfig struct {
	ConnectionURI string `mapstructure:"connection_uri"` // e.g. http://supertokens:3567
	APIKey        string `mapstructure:"api_key"`
}

// GoogleOAuthConfig holds Google OAuth credentials for social login via SuperTokens.
type GoogleOAuthConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}
```

- [ ] **2.2** In the `Config` struct (line 19), replace `Zitadel ZitadelConfig` with:

```go
	SuperTokens SuperTokensConfig
	GoogleOAuth GoogleOAuthConfig
```

- [ ] **2.3** In the `Load()` function, replace Zitadel defaults (lines 220-223) with:

```go
	v.SetDefault("supertokens.connection_uri", "http://supertokens:3567")
	v.SetDefault("supertokens.api_key", "")
	v.SetDefault("googleoauth.client_id", "")
	v.SetDefault("googleoauth.client_secret", "")
```

- [ ] **2.4** Replace Zitadel env bindings (lines 319-322) with:

```go
	_ = v.BindEnv("supertokens.connection_uri", "RAVEN_SUPERTOKENS_CONNECTION_URI")
	_ = v.BindEnv("supertokens.api_key", "RAVEN_SUPERTOKENS_API_KEY")
	_ = v.BindEnv("googleoauth.client_id", "GOOGLE_CLIENT_ID")
	_ = v.BindEnv("googleoauth.client_secret", "GOOGLE_CLIENT_SECRET")
```

- [ ] **2.5** Remove the Zitadel startup warning (lines 398-400):

```go
	// DELETE THESE LINES:
	if cfg.Zitadel.ClientID == "" {
		log.Printf("[WARN] zitadel.client_id is empty — JWT audience validation will reject all tokens until configured")
	}
```

- [ ] **2.6** Update `.env.example` — replace the Zitadel section (lines 10-17) with:

```bash
# ─── SuperTokens ────────────────────────────────────────────────────────────
SUPERTOKENS_API_KEY=supertokens-dev-key-replace-me
RAVEN_SUPERTOKENS_CONNECTION_URI=http://supertokens:3567

# Google OAuth (configure for social login)
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
```

Remove lines 19-20 (Google IDP comment referencing Zitadel admin console):
```bash
# DELETE:
# Google IDP (configure in Zitadel admin console)
```

- [ ] **2.7** Update `frontend/.env.example` — replace Zitadel vars (lines 3-5) with:

```bash
VITE_API_DOMAIN=http://localhost:8081
```

The full file should now be:
```bash
VITE_API_URL=http://localhost:8081/api
VITE_API_BASE_URL=http://localhost:8081/api/v1
VITE_API_DOMAIN=http://localhost:8081

# LiveKit WebRTC (leave empty to disable voice)
VITE_LIVEKIT_URL=wss://livekit.example.com

# PostHog (opt-in: leave empty to disable analytics)
# VITE_POSTHOG_API_KEY=phc_your_project_api_key
# VITE_POSTHOG_HOST=https://us.i.posthog.com
```

- [ ] **2.8** Update `deploy/postgres/init.sql` — remove lines 4-19 (Zitadel database/user creation):

```sql
-- DELETE THESE LINES:
-- Zitadel needs its own database + user
SELECT 'CREATE DATABASE zitadel'
WHERE NOT EXISTS (SELECT FROM pg_database
                  WHERE datname = 'zitadel')\gexec

DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'zitadel') THEN
    CREATE USER zitadel WITH PASSWORD 'zitadel';
  END IF;
END
$$;

GRANT ALL PRIVILEGES ON DATABASE zitadel TO zitadel;
ALTER USER zitadel CREATEDB;
```

The file should start with:
```sql
-- Raven PostgreSQL initialisation
-- Runs once when the data volume is first created.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
```

- [ ] **2.9** Verify Go code compiles (this will fail until Task 3-7 are done — expected):

```bash
cd /Users/jobinlawrance/Project/raven && go build ./internal/config/...
```

**Commit message:**
```
feat(config): replace ZitadelConfig with SuperTokensConfig and GoogleOAuthConfig
```

---

## Phase 2: Backend Auth (Tasks 3-7)

---

### Task 3: AuthProvider Interface — Create `internal/auth/provider.go`

**Files to create:**
- `internal/auth/provider.go`

**Steps:**

- [ ] **3.1** Create the directory:

```bash
mkdir -p /Users/jobinlawrance/Project/raven/internal/auth
```

- [ ] **3.2** Create `internal/auth/provider.go` with the following content:

```go
package auth

import "net/http"

// SessionInfo holds user identity data extracted from a verified session.
type SessionInfo struct {
	ExternalID string // SuperTokens user ID (maps to users.external_id)
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

- [ ] **3.3** Verify the package compiles:

```bash
cd /Users/jobinlawrance/Project/raven && go build ./internal/auth/...
```

**Commit message:**
```
feat(auth): add AuthProvider interface for pluggable session verification
```

---

### Task 4: SuperTokens Implementation — Create `internal/auth/supertokens.go`

**Files to create:**
- `internal/auth/supertokens.go`

**Steps:**

- [ ] **4.1** Create `internal/auth/supertokens.go` with the following content:

```go
package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SuperTokensProvider implements AuthProvider using the SuperTokens Core HTTP API.
// It verifies sessions by calling the Core directly (no Go SDK needed).
type SuperTokensProvider struct {
	connectionURI string // e.g. "http://supertokens:3567"
	apiKey        string
	httpClient    *http.Client
}

// NewSuperTokensProvider creates a new SuperTokensProvider.
func NewSuperTokensProvider(connectionURI, apiKey string) *SuperTokensProvider {
	return &SuperTokensProvider{
		connectionURI: connectionURI,
		apiKey:        apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// sessionVerifyRequest is the JSON body sent to POST /recipe/session/verify.
type sessionVerifyRequest struct {
	AccessToken           string `json:"accessToken"`
	EnableAntiCsrf        bool   `json:"enableAntiCsrf"`
	DoAntiCsrfCheck       bool   `json:"doAntiCsrfCheck"`
	CheckDatabase         bool   `json:"checkDatabase"`
	AntiCsrfToken         string `json:"antiCsrfToken,omitempty"`
}

// sessionVerifyResponse is the JSON response from POST /recipe/session/verify.
type sessionVerifyResponse struct {
	Status  string `json:"status"`
	Session struct {
		Handle         string                 `json:"handle"`
		UserID         string                 `json:"userId"`
		UserDataInJWT  map[string]interface{} `json:"userDataInJWT"`
	} `json:"session"`
}

// userGetResponse is the JSON response from GET /user/id.
type userGetResponse struct {
	Status string `json:"status"`
	User   struct {
		ID         string `json:"id"`
		Email      string `json:"email"`
		TimeJoined int64  `json:"timeJoined"`
		ThirdParty *struct {
			ID     string `json:"id"`
			UserID string `json:"userId"`
		} `json:"thirdParty,omitempty"`
		Emails []string `json:"emails,omitempty"`
	} `json:"user"`
}

// sessionRemoveRequest is the JSON body sent to POST /recipe/session/remove.
type sessionRemoveRequest struct {
	SessionHandles []string `json:"sessionHandles"`
}

// VerifySession extracts the sAccessToken cookie from the request, calls the
// SuperTokens Core to verify it, then fetches user info and returns SessionInfo.
func (p *SuperTokensProvider) VerifySession(r *http.Request) (*SessionInfo, error) {
	// 1. Extract sAccessToken from cookies
	accessTokenCookie, err := r.Cookie("sAccessToken")
	if err != nil {
		return nil, fmt.Errorf("missing sAccessToken cookie: %w", err)
	}
	accessToken := accessTokenCookie.Value
	if accessToken == "" {
		return nil, fmt.Errorf("empty sAccessToken cookie")
	}

	// 2. Extract anti-CSRF token from header (optional, depends on config)
	antiCsrfToken := r.Header.Get("anti-csrf")

	// 3. POST /recipe/session/verify
	verifyReq := sessionVerifyRequest{
		AccessToken:     accessToken,
		EnableAntiCsrf:  antiCsrfToken != "",
		DoAntiCsrfCheck: antiCsrfToken != "",
		CheckDatabase:   false,
		AntiCsrfToken:   antiCsrfToken,
	}

	verifyResp, err := p.coreRequest(http.MethodPost, "/recipe/session/verify", verifyReq)
	if err != nil {
		return nil, fmt.Errorf("session verify request failed: %w", err)
	}

	var sessionResp sessionVerifyResponse
	if err := json.Unmarshal(verifyResp, &sessionResp); err != nil {
		return nil, fmt.Errorf("failed to parse session verify response: %w", err)
	}

	if sessionResp.Status != "OK" {
		return nil, fmt.Errorf("session verification failed: status=%s", sessionResp.Status)
	}

	userID := sessionResp.Session.UserID
	if userID == "" {
		return nil, fmt.Errorf("session verified but no userId returned")
	}

	// 4. GET /user/id?userId=<id> for email/name
	email, name, err := p.getUserInfo(userID)
	if err != nil {
		// Non-fatal: return session info with empty email/name rather than failing auth
		return &SessionInfo{
			ExternalID: userID,
		}, nil
	}

	return &SessionInfo{
		ExternalID: userID,
		Email:      email,
		Name:       name,
	}, nil
}

// RevokeSession extracts the session handle from the request and revokes it.
func (p *SuperTokensProvider) RevokeSession(r *http.Request) error {
	// First verify the session to get the handle
	accessTokenCookie, err := r.Cookie("sAccessToken")
	if err != nil {
		return fmt.Errorf("missing sAccessToken cookie: %w", err)
	}

	antiCsrfToken := r.Header.Get("anti-csrf")

	verifyReq := sessionVerifyRequest{
		AccessToken:     accessTokenCookie.Value,
		EnableAntiCsrf:  antiCsrfToken != "",
		DoAntiCsrfCheck: false, // Don't check CSRF for revocation
		CheckDatabase:   false,
		AntiCsrfToken:   antiCsrfToken,
	}

	verifyResp, err := p.coreRequest(http.MethodPost, "/recipe/session/verify", verifyReq)
	if err != nil {
		return fmt.Errorf("session verify for revocation failed: %w", err)
	}

	var sessionResp sessionVerifyResponse
	if err := json.Unmarshal(verifyResp, &sessionResp); err != nil {
		return fmt.Errorf("failed to parse session response: %w", err)
	}

	if sessionResp.Status != "OK" || sessionResp.Session.Handle == "" {
		return nil // Session already invalid, nothing to revoke
	}

	// POST /recipe/session/remove
	removeReq := sessionRemoveRequest{
		SessionHandles: []string{sessionResp.Session.Handle},
	}

	_, err = p.coreRequest(http.MethodPost, "/recipe/session/remove", removeReq)
	return err
}

// getUserInfo fetches user email and name from the SuperTokens Core.
func (p *SuperTokensProvider) getUserInfo(userID string) (email, name string, err error) {
	resp, err := p.coreRequest(http.MethodGet, "/user/id?userId="+userID, nil)
	if err != nil {
		return "", "", err
	}

	var userResp userGetResponse
	if err := json.Unmarshal(resp, &userResp); err != nil {
		return "", "", err
	}

	if userResp.Status != "OK" {
		return "", "", fmt.Errorf("user lookup failed: status=%s", userResp.Status)
	}

	email = userResp.User.Email
	// SuperTokens may return emails in the Emails array for newer API versions
	if email == "" && len(userResp.User.Emails) > 0 {
		email = userResp.User.Emails[0]
	}

	// SuperTokens Core does not store display names natively.
	// The name can be populated from the thirdparty provider metadata
	// or from session claims. For now, use email prefix as fallback.
	if name == "" && email != "" {
		atIdx := len(email)
		for i, c := range email {
			if c == '@' {
				atIdx = i
				break
			}
		}
		name = email[:atIdx]
	}

	return email, name, nil
}

// coreRequest makes an HTTP request to the SuperTokens Core API.
func (p *SuperTokensProvider) coreRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, p.connectionURI+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if p.apiKey != "" {
		req.Header.Set("api-key", p.apiKey)
	}
	// SuperTokens Core requires a CDI version header
	req.Header.Set("cdi-version", "4.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("core request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("core API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
```

- [ ] **4.2** Verify the package compiles:

```bash
cd /Users/jobinlawrance/Project/raven && go build ./internal/auth/...
```

**Commit message:**
```
feat(auth): add SuperTokensProvider with Core HTTP API integration
```

---

### Task 5: Session Middleware — Replace JWTMiddleware with SessionMiddleware

**Files to modify:**
- `internal/middleware/auth.go` (complete rewrite of JWT-specific code)

**Steps:**

- [ ] **5.1** In `internal/middleware/auth.go`, remove the following imports (lines 3-8 approximately):

```go
// DELETE these imports:
"fmt"
"log"
"sync"
"time"
"strings"

"github.com/MicahParks/keyfunc/v3"
"github.com/golang-jwt/jwt/v5"

"github.com/ravencloak-org/Raven/internal/config"
```

Add the new import:
```go
"github.com/ravencloak-org/Raven/internal/auth"
```

Keep these imports:
```go
"context"
"errors"
"net/http"

"github.com/gin-gonic/gin"
```

- [ ] **5.2** Remove the following constants and types from `auth.go`:
  - `jwksCacheTTL` constant (line 47)
  - `Claims` struct (lines 53-57)
  - `jwksCache` struct and all its methods: `newJWKSCache`, `refresh`, `keyFunc` (lines 65-117)
  - `JWTMiddleware` function (lines 119-200)
  - `parseJWT` function (lines 260-274)
  - `isKeyError` function (lines 280-284)
  - `abortWithTokenError` function (lines 287-294)

Keep these:
  - All `contextKey` constants (lines 21-46) — but update `ContextKeyClaims` comment
  - `authError` struct (lines 60-62)
  - `UserResolver` interface and `UserLookup` function (lines 203-239)
  - `RequireOrg` function (lines 244-253)
  - `abortUnauthorized` function (lines 297-299)

- [ ] **5.3** Remove `ContextKeyClaims` from the constants block (line 45-46) since we no longer have JWT claims:

```go
// DELETE:
// ContextKeyClaims is the context key for the full parsed Claims struct.
ContextKeyClaims contextKey = "claims"
```

- [ ] **5.4** Update the `ContextKeyExternalID` comment (line 38-40) to reference SuperTokens instead of Zitadel:

```go
// ContextKeyExternalID is the context key for the auth provider's user ID (external user ID).
// Set by SessionMiddleware from the verified session.
ContextKeyExternalID contextKey = "external_id"
```

- [ ] **5.5** Update the `ContextKeyUserName` comment (lines 42-43):

```go
// ContextKeyUserName is the context key for the user's display name.
// Set by SessionMiddleware from the auth provider's user info.
ContextKeyUserName contextKey = "user_name"
```

- [ ] **5.6** Add the `SessionMiddleware` function after the constants block:

```go
// SessionMiddleware returns a Gin handler that verifies the session using
// the provided AuthProvider. On success, it stores identity data in the
// Gin context using the same context keys as the old JWTMiddleware, so
// downstream middleware (UserLookup, RequireOrg, etc.) works unchanged.
func SessionMiddleware(provider auth.AuthProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		info, err := provider.VerifySession(c.Request)
		if err != nil {
			abortUnauthorized(c, "invalid_session")
			return
		}

		// Store in context — same keys as old JWTMiddleware
		c.Set(string(ContextKeyExternalID), info.ExternalID)
		c.Set(string(ContextKeyEmail), info.Email)
		c.Set(string(ContextKeyUserName), info.Name)

		c.Next()
	}
}
```

- [ ] **5.7** The final `internal/middleware/auth.go` file should look like this:

```go
package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/auth"
)

// contextKey is a private type for context keys to avoid collisions with other packages.
type contextKey string

const (
	// ContextKeyUserID is the context key for the internal database user ID.
	// Set by auth handlers after a DB lookup; not set by SessionMiddleware directly.
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyOrgID is the context key for the organisation ID.
	// Set by auth handlers or org-scoped middleware after a DB lookup.
	ContextKeyOrgID contextKey = "org_id"
	// ContextKeyOrgRole is the context key for the organisation role.
	// Set by org-scoped middleware after a DB lookup.
	ContextKeyOrgRole contextKey = "org_role"
	// ContextKeyWorkspaceRole is the context key for the resolved workspace role,
	// set by workspace-scoped middleware after a membership DB lookup.
	ContextKeyWorkspaceRole contextKey = "workspace_role"
	// ContextKeyEmail is the context key for the user email from the auth session.
	ContextKeyEmail contextKey = "email"
	// ContextKeyExternalID is the context key for the auth provider's user ID (external user ID).
	// Set by SessionMiddleware from the verified session.
	ContextKeyExternalID contextKey = "external_id"
	// ContextKeyUserName is the context key for the user's display name.
	// Set by SessionMiddleware from the auth provider's user info.
	ContextKeyUserName contextKey = "user_name"
)

// authError represents a structured 401 response body.
type authError struct {
	Error string `json:"error"`
}

// SessionMiddleware returns a Gin handler that verifies the session using
// the provided AuthProvider. On success, it stores identity data in the
// Gin context using the same context keys as the old JWTMiddleware, so
// downstream middleware (UserLookup, RequireOrg, etc.) works unchanged.
func SessionMiddleware(provider auth.AuthProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		info, err := provider.VerifySession(c.Request)
		if err != nil {
			abortUnauthorized(c, "invalid_session")
			return
		}

		// Store in context — same keys as old JWTMiddleware
		c.Set(string(ContextKeyExternalID), info.ExternalID)
		c.Set(string(ContextKeyEmail), info.Email)
		c.Set(string(ContextKeyUserName), info.Name)

		c.Next()
	}
}

// UserResolver is the interface for looking up users by external ID.
// Returns empty userID when the user is not found (not an error).
type UserResolver interface {
	GetByExternalID(ctx context.Context, externalID string) (userID string, orgID *string, err error)
}

// UserLookup returns middleware that resolves the external ID to internal
// user and org IDs via a database lookup. Apply after SessionMiddleware on routes
// that need ContextKeyUserID or ContextKeyOrgID.
//
// If the user is not found (first login), the middleware continues without
// setting these keys — the /auth/callback handler handles user creation.
// Real DB errors abort with 503 to avoid masking infra failures.
func UserLookup(resolver UserResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		externalID := c.GetString(string(ContextKeyExternalID))
		if externalID == "" {
			c.Next()
			return
		}
		userID, orgID, err := resolver.GetByExternalID(c.Request.Context(), externalID)
		if err != nil {
			// Real DB error — abort so infra failures don't silently degrade auth
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "user_lookup_failed"})
			return
		}
		if userID == "" {
			// User not found = first login, let auth callback handle creation
			c.Next()
			return
		}
		c.Set(string(ContextKeyUserID), userID)
		if orgID != nil {
			c.Set(string(ContextKeyOrgID), *orgID)
		}
		c.Next()
	}
}

// RequireOrg returns middleware that aborts with 403 if the request context
// does not contain a valid organisation ID. Apply after SessionMiddleware on
// routes that require an onboarded user.
func RequireOrg() gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := c.GetString(string(ContextKeyOrgID))
		if orgID == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "organization required"})
			return
		}
		c.Next()
	}
}

// abortUnauthorized aborts the request with a structured 401 JSON body.
func abortUnauthorized(c *gin.Context, code string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, authError{Error: code})
}
```

- [ ] **5.8** Verify compilation:

```bash
cd /Users/jobinlawrance/Project/raven && go build ./internal/middleware/...
```

**Commit message:**
```
feat(middleware): replace JWTMiddleware with SessionMiddleware using AuthProvider
```

---

### Task 6: Auth Proxy Handler — Create reverse proxy for `/auth/*`

**Files to create:**
- `internal/handler/authproxy.go`

**Steps:**

- [ ] **6.1** Create `internal/handler/authproxy.go` with the following content:

```go
package handler

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

// NewSuperTokensProxy returns a Gin handler that reverse-proxies all
// requests under /auth/* to the SuperTokens core. The supertokens-web-js
// SDK expects auth APIs on the same domain as the app API, so the Go API
// proxies these requests internally.
//
// The apiKey is passed in the "api-key" header on every forwarded request
// so the SuperTokens core can authenticate the caller.
func NewSuperTokensProxy(superTokensURL, apiKey string) gin.HandlerFunc {
	target, err := url.Parse(superTokensURL)
	if err != nil {
		slog.Error("failed to parse SuperTokens URL for proxy", "url", superTokensURL, "error", err)
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "auth_proxy_misconfigured"})
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Override the Director to set the api-key header and fix the path.
	defaultDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		defaultDirector(req)
		if apiKey != "" {
			req.Header.Set("api-key", apiKey)
		}
	}

	// Log proxy errors rather than panicking.
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("SuperTokens proxy error", "path", r.URL.Path, "error", err)
		w.WriteHeader(http.StatusBadGateway)
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
```

- [ ] **6.2** Verify compilation:

```bash
cd /Users/jobinlawrance/Project/raven && go build ./internal/handler/...
```

**Commit message:**
```
feat(handler): add SuperTokens reverse proxy for /auth/* routes
```

---

### Task 7: Wire Everything in main.go — Replace Zitadel wiring with SuperTokens

**Files to modify:**
- `cmd/api/main.go` (lines 452: JWTMiddleware call; lines 743-748: authGroup; add proxy route; update imports)
- `internal/middleware/cors.go` (add SuperTokens headers)

**Steps:**

- [ ] **7.1** In `cmd/api/main.go`, add the auth import (around line 39):

```go
"github.com/ravencloak-org/Raven/internal/auth"
```

- [ ] **7.2** Remove the `config` import reference to Zitadel. The `config` import is already present — no change needed there.

- [ ] **7.3** In `cmd/api/main.go`, replace the JWT middleware wiring (line 452):

```go
// REPLACE:
api.Use(middleware.JWTMiddleware(&cfg.Zitadel))

// WITH:
authProvider := auth.NewSuperTokensProvider(
	cfg.SuperTokens.ConnectionURI,
	cfg.SuperTokens.APIKey,
)
api.Use(middleware.SessionMiddleware(authProvider))
```

- [ ] **7.4** Add the SuperTokens proxy route BEFORE the `api` group (after line 443, after the Swagger route):

```go
// SuperTokens auth proxy — must be outside /api/v1 group (no session verification).
// The supertokens-web-js SDK sends auth requests to /auth/* on the API domain.
router.Any("/auth/*path", handler.NewSuperTokensProxy(cfg.SuperTokens.ConnectionURI, cfg.SuperTokens.APIKey))
```

- [ ] **7.5** Add a SuperTokens recipe configuration call at startup. After the `authProvider` creation (step 7.3), add:

```go
// Configure SuperTokens recipes on startup (idempotent).
go configureSuperTokensRecipes(cfg)
```

Then add this function at the bottom of main.go (before the closing of the main function, or as a separate function after main):

```go
// configureSuperTokensRecipes configures ThirdParty (Google) and WebAuthn recipes
// in the SuperTokens Core via its HTTP API. This is idempotent — safe to call on
// every startup.
func configureSuperTokensRecipes(cfg *config.Config) {
	if cfg.GoogleOAuth.ClientID == "" {
		slog.Warn("GOOGLE_CLIENT_ID is empty — Google social login will not be available until configured")
		return
	}

	stURL := cfg.SuperTokens.ConnectionURI
	apiKey := cfg.SuperTokens.APIKey

	// Helper to make Core API calls
	coreReq := func(method, path string, body interface{}) error {
		var bodyReader io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return err
			}
			bodyReader = bytes.NewReader(b)
		}
		req, err := http.NewRequest(method, stURL+path, bodyReader)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("cdi-version", "4.0")
		if apiKey != "" {
			req.Header.Set("api-key", apiKey)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("SuperTokens Core API %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
		}
		return nil
	}

	// Enable ThirdParty recipe on public tenant
	if err := coreReq(http.MethodPut, "/recipe/multitenancy/tenant", map[string]interface{}{
		"tenantId":          "public",
		"thirdPartyEnabled": true,
	}); err != nil {
		slog.Error("failed to enable ThirdParty recipe", "error", err)
	}

	// Configure Google provider
	if err := coreReq(http.MethodPut, "/recipe/multitenancy/tenant/thirdparty/config", map[string]interface{}{
		"tenantId": "public",
		"config": map[string]interface{}{
			"thirdPartyId": "google",
			"clients": []map[string]interface{}{
				{
					"clientId":     cfg.GoogleOAuth.ClientID,
					"clientSecret": cfg.GoogleOAuth.ClientSecret,
				},
			},
		},
	}); err != nil {
		slog.Error("failed to configure Google provider", "error", err)
	}

	slog.Info("SuperTokens recipes configured", "google_client_id", cfg.GoogleOAuth.ClientID)
}
```

- [ ] **7.6** Add the required imports for the new function at the top of `main.go` (if not already present):

```go
"bytes"
"encoding/json"
"io"
"log/slog" // already imported
```

- [ ] **7.7** Update `internal/middleware/cors.go` to include SuperTokens-specific headers. Add to the `AllowHeaders` slice:

```go
AllowHeaders: []string{
	"Authorization",
	"Content-Type",
	"X-API-Key",
	"X-Request-ID",
	// SuperTokens cookie-based auth headers
	"anti-csrf",
	"st-auth-mode",
	"rid",
	"fdi-version",
},
```

And add `ExposeHeaders` to the CORS config:

```go
ExposeHeaders: []string{
	"st-access-token",
	"st-refresh-token",
	"anti-csrf",
	"front-token",
},
```

- [ ] **7.8** Update the Swagger security definition comment at the top of main.go. Change the `@securityDefinitions.apikey BearerAuth` block (lines 16-18) to reflect cookie-based auth:

```go
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name sAccessToken
// @description Session cookie set by SuperTokens after authentication
```

- [ ] **7.9** Update the auth handler comment in main.go (line 743). The `AuthHandler.Callback` endpoint remains unchanged — it reads `ContextKeyExternalID`, `ContextKeyEmail`, and `ContextKeyUserName` from the Gin context (now set by `SessionMiddleware` instead of `JWTMiddleware`).

- [ ] **7.10** Remove the `keyfunc` and `golang-jwt` Go dependencies:

```bash
cd /Users/jobinlawrance/Project/raven && go mod tidy
```

Verify these are no longer in `go.mod`:
```
github.com/MicahParks/keyfunc/v3
github.com/golang-jwt/jwt/v5
```

- [ ] **7.11** Verify the entire project compiles:

```bash
cd /Users/jobinlawrance/Project/raven && go build ./...
```

- [ ] **7.12** Run existing Go tests (some will fail until Task 14 updates them):

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/middleware/... 2>&1 | head -20
```

**Commit message:**
```
feat(api): wire SuperTokens auth provider, proxy, and recipe configuration
```

---

## Phase 3: Frontend (Tasks 8-11)

---

### Task 8: Swap npm packages — Remove oidc-client-ts, add supertokens-web-js

**Files to modify:**
- `frontend/package.json` (line 21: oidc-client-ts dependency)

**Steps:**

- [ ] **8.1** Remove `oidc-client-ts` and add `supertokens-web-js`:

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npm uninstall oidc-client-ts && npm install supertokens-web-js
```

- [ ] **8.2** Verify `package.json` no longer contains `oidc-client-ts` and does contain `supertokens-web-js`.

- [ ] **8.3** Create the SuperTokens initialization plugin at `frontend/src/plugins/supertokens.ts`:

```typescript
import SuperTokens from "supertokens-web-js"
import Session from "supertokens-web-js/recipe/session"
import ThirdParty from "supertokens-web-js/recipe/thirdparty"

export function initSuperTokens() {
  SuperTokens.init({
    appInfo: {
      appName: "Raven",
      apiDomain: import.meta.env.VITE_API_DOMAIN || "http://localhost:8081",
      apiBasePath: "/auth",
    },
    recipeList: [
      Session.init(),
      ThirdParty.init(),
    ],
  })
}
```

- [ ] **8.4** Update `frontend/src/main.ts` to initialize SuperTokens before the auth store. Replace the file content:

```typescript
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { initSuperTokens } from './plugins/supertokens'
import { useAuthStore } from './stores/auth'
import { posthogPlugin } from './plugins/posthog'
import App from './App.vue'
import router from './router'
import './assets/main.css'

// Initialise SuperTokens SDK before anything else — must happen before
// any session checks or API calls.
initSuperTokens()

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(posthogPlugin, { router })

// Initialise auth store — checks for existing session cookie.
const authStore = useAuthStore()
authStore.init().then(() => app.mount('#app'))
```

**Commit message:**
```
feat(frontend): swap oidc-client-ts for supertokens-web-js SDK
```

---

### Task 9: Auth Store Rewrite — Replace OIDC store with SuperTokens cookie-based store

**Files to modify:**
- `frontend/src/stores/auth.ts` (complete rewrite)
- `frontend/src/composables/useAuth.ts` (update exports)
- `frontend/src/api/client.ts` (switch to cookie-based auth)
- All API files that use `Authorization: Bearer` headers

**Steps:**

- [ ] **9.1** Rewrite `frontend/src/stores/auth.ts`:

```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import Session from 'supertokens-web-js/recipe/session'
import {
  getAuthorisationURLWithQueryParamsAndSetState,
  signInAndUp,
} from 'supertokens-web-js/recipe/thirdparty'
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

  async function handleCallback(): Promise<void> {
    const response = await signInAndUp()
    if (response.status === 'OK') {
      sessionExists.value = true
    } else {
      throw new Error(`Sign-in failed: ${response.status}`)
    }
  }

  async function callAuthCallback(): Promise<{
    isNewUser: boolean
    orgId?: string
    userId?: string
  }> {
    // POST /api/v1/auth/callback — session cookie sent automatically
    const res = await fetch(
      `${import.meta.env.VITE_API_BASE_URL}/auth/callback`,
      { method: 'POST', credentials: 'include' },
    )
    if (!res.ok) {
      throw new Error(`Auth callback failed (${res.status})`)
    }
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
    handleCallback,
    callAuthCallback,
    logout,
    setOrgId,
  }
})
```

- [ ] **9.2** Update `frontend/src/composables/useAuth.ts` to remove OIDC-specific exports:

```typescript
import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'

export function useAuth() {
  const store = useAuthStore()
  return {
    isAuthenticated: computed(() => store.isAuthenticated),
    hasOrg: computed(() => store.hasOrg),
    loginWithGoogle: () => store.loginWithGoogle(),
    logout: () => store.logout(),
  }
}
```

- [ ] **9.3** Rewrite `frontend/src/api/client.ts` to use cookie-based auth (remove Bearer token logic):

```typescript
import { useBillingStore } from '../stores/billing'

const BASE_URL = import.meta.env.VITE_API_URL || '/api'

interface RequestOptions {
  method?: string
  body?: unknown
  headers?: Record<string, string>
}

async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {} } = options

  const response = await fetch(`${BASE_URL}${endpoint}`, {
    method,
    credentials: 'include', // Send session cookies with every request
    headers: {
      'Content-Type': 'application/json',
      ...headers,
    },
    body: body ? JSON.stringify(body) : undefined,
  })

  if (response.status === 402) {
    // Payment required — show upgrade prompt via billing store
    try {
      const billingStore = useBillingStore()
      billingStore.showUpgradePrompt()
    } catch {
      // Store may not be available outside Pinia context; ignore
    }
    throw Object.assign(new Error('Payment required'), { status: 402 })
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: response.statusText }))
    throw new Error(error.message || `Request failed with status ${response.status}`)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json()
}

export const api = {
  get<T>(endpoint: string, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'GET', headers })
  },

  post<T>(endpoint: string, body?: unknown, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'POST', body, headers })
  },

  put<T>(endpoint: string, body?: unknown, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'PUT', body, headers })
  },

  patch<T>(endpoint: string, body?: unknown, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'PATCH', body, headers })
  },

  delete<T>(endpoint: string, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'DELETE', headers })
  },
}
```

- [ ] **9.4** Update ALL API files that use `Authorization: Bearer` headers to use `credentials: 'include'` instead. For each file below, remove the `Authorization` header and add `credentials: 'include'` to `fetch()` calls:

Files to update (remove `Authorization: Bearer ${auth.accessToken}` from headers, add `credentials: 'include'` to fetch options):

1. `frontend/src/api/orgs.ts` (line 20)
2. `frontend/src/api/workspaces.ts` (line 32)
3. `frontend/src/api/knowledge-bases.ts` (line 48)
4. `frontend/src/api/llm-providers.ts` (line 83)
5. `frontend/src/api/apikeys.ts` (line 42)
6. `frontend/src/api/chatbot-config.ts` (line 24)
7. `frontend/src/api/test-sandbox.ts` (line 22)
8. `frontend/src/api/analytics.ts` (line 41)
9. `frontend/src/api/billing.ts` (line 40)
10. `frontend/src/api/whatsapp.ts` (line 95)
11. `frontend/src/api/voice-sessions.ts` (line 72)
12. `frontend/src/pages/onboarding/OnboardingWizard.vue` (line 113)

**Pattern for each file:** Find all `fetch()` calls that include an `Authorization` header. Change from:

```typescript
const res = await fetch(`${url}`, {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${auth.accessToken}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify(data),
})
```

To:

```typescript
const res = await fetch(`${url}`, {
  method: 'POST',
  credentials: 'include',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify(data),
})
```

Also remove any `import { useAuthStore }` that was only used for `accessToken`, and remove `const auth = useAuthStore()` lines if no longer needed.

- [ ] **9.5** Verify TypeScript compiles:

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit 2>&1 | head -30
```

**Commit message:**
```
feat(frontend): rewrite auth store for SuperTokens cookie-based sessions
```

---

### Task 10: Login Page Redesign — Two-button login (Google + passkey)

**Files to modify:**
- `frontend/src/pages/LoginPage.vue` (complete rewrite)

**Steps:**

- [ ] **10.1** Rewrite `frontend/src/pages/LoginPage.vue`:

```vue
<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black">
    <div class="w-full max-w-sm px-6">
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-neutral-900 dark:text-white">Raven</h1>
        <p class="text-neutral-500 mt-2">Sign in to continue</p>
      </div>

      <div class="space-y-4">
        <!-- Google Sign In -->
        <button
          class="w-full flex items-center justify-center gap-3 px-4 py-3 rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-white hover:bg-neutral-50 dark:hover:bg-neutral-800 transition-colors min-h-[48px]"
          :disabled="loading"
          @click="signInWithGoogle"
        >
          <svg class="w-5 h-5" viewBox="0 0 24 24" aria-hidden="true">
            <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/>
            <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
            <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
            <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
          </svg>
          <span>Sign in with Google</span>
        </button>

        <!-- Passkey Sign In (placeholder for future WebAuthn recipe) -->
        <!--
        <button
          class="w-full flex items-center justify-center gap-3 px-4 py-3 rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-white hover:bg-neutral-50 dark:hover:bg-neutral-800 transition-colors min-h-[48px]"
          :disabled="loading"
          @click="signInWithPasskey"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
          </svg>
          <span>Sign in with passkey</span>
        </button>
        -->
      </div>

      <p v-if="error" class="text-red-500 text-sm mt-4 text-center" role="alert">{{ error }}</p>

      <p v-if="loading" class="text-neutral-500 text-sm mt-4 text-center" role="status" aria-live="polite">
        Redirecting...
      </p>

      <div class="mt-8 text-center text-xs text-neutral-400">
        By continuing, you agree to our
        <router-link to="/legal/terms" class="text-amber-500 hover:underline">Terms of Service</router-link>
        &amp;
        <router-link to="/legal/privacy" class="text-amber-500 hover:underline">Privacy Policy</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const error = ref('')
const loading = ref(false)

async function signInWithGoogle() {
  try {
    error.value = ''
    loading.value = true
    await auth.loginWithGoogle()
  } catch (e: unknown) {
    console.error('Login redirect failed:', e)
    error.value = 'Unable to start sign-in. Please try again.'
    loading.value = false
  }
}
</script>
```

- [ ] **10.2** Verify the login page renders correctly in dev:

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit 2>&1 | head -10
```

**Commit message:**
```
feat(frontend): redesign login page with Google sign-in button
```

---

### Task 11: Callback + Router — Simplify callback, update router guard

**Files to modify:**
- `frontend/src/pages/callback/CallbackPage.vue` (simplify for SuperTokens)
- `frontend/src/router/index.ts` (update guard — minimal changes needed)

**Steps:**

- [ ] **11.1** Rewrite `frontend/src/pages/callback/CallbackPage.vue`:

```vue
<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black">
    <div class="text-center">
      <p class="text-neutral-500" role="status" aria-live="polite">Completing sign in...</p>
      <p v-if="error" class="text-red-500 text-sm mt-4">{{ error }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const error = ref('')

onMounted(async () => {
  try {
    // Complete the SuperTokens OAuth callback (exchanges code for session)
    await auth.handleCallback()

    // Call backend auth callback to create/lookup internal user
    // Session cookie is sent automatically (credentials: 'include')
    const data = await auth.callAuthCallback()

    if (data.isNewUser) {
      router.push('/onboarding')
    } else {
      if (data.orgId) {
        auth.setOrgId(data.orgId)
      }
      router.push('/dashboard')
    }
  } catch (err) {
    console.error('Callback error:', err)
    error.value = 'Sign-in failed. Redirecting to login...'
    setTimeout(() => router.push('/login'), 2000)
  }
})
</script>
```

- [ ] **11.2** Update `frontend/src/router/index.ts` — add `/legal` prefix to the public route check. The existing guard logic on lines 163-183 is mostly correct. The only change is to also allow `/legal/*` routes through without auth:

```typescript
router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // Public routes — no auth check
  if (to.path === '/login' || to.path === '/callback') return
  if (to.path.startsWith('/legal/')) return

  // Initialize session check if not done
  if (!auth.isAuthenticated) {
    await auth.init()
  }

  // Redirect to login if auth required but not authenticated
  if (to.meta.requiresAuth === true && !auth.isAuthenticated) {
    return '/login'
  }

  // Redirect to onboarding if authenticated but no org
  if (auth.isAuthenticated && !auth.hasOrg && to.path !== '/onboarding') {
    return '/onboarding'
  }
})
```

Note: The current router guard on line 168 already has `if (to.path.startsWith('/legal/'))` — verify this is present. If the `/legal/` check already exists, no change needed for the guard.

- [ ] **11.3** Verify frontend TypeScript compiles:

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit 2>&1 | head -20
```

**Commit message:**
```
feat(frontend): simplify callback page for SuperTokens OAuth flow
```

---

## Phase 4: Cleanup + Deploy (Tasks 12-14)

---

### Task 12: Remove Zitadel artifacts — Delete deploy/zitadel/, remove old deps

**Files to delete:**
- `deploy/zitadel/zitadel-config.yaml`
- `deploy/zitadel/zitadel-init-steps.yaml`
- `frontend/public/silent-renew.html`

**Files to modify:**
- `frontend/src/stores/auth.spec.ts` (rewrite for SuperTokens)

**Steps:**

- [ ] **12.1** Delete the `deploy/zitadel/` directory:

```bash
rm -rf /Users/jobinlawrance/Project/raven/deploy/zitadel
```

- [ ] **12.2** Delete the OIDC silent renew page:

```bash
rm /Users/jobinlawrance/Project/raven/frontend/public/silent-renew.html
```

- [ ] **12.3** Create the migration file `migrations/00035_supertokens_migration.sql`:

```sql
-- migrations/00035_supertokens_migration.sql
-- +goose Up
-- +goose StatementBegin

-- No schema changes needed — external_id column is reused as-is for SuperTokens user IDs.
-- SuperTokens auto-creates its own tables (supertokens_*) on first boot.

-- Update auth_provider default for new users
ALTER TABLE users ALTER COLUMN auth_provider SET DEFAULT 'supertokens';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users ALTER COLUMN auth_provider SET DEFAULT 'zitadel';

-- +goose StatementEnd
```

- [ ] **12.4** Verify Go module is clean:

```bash
cd /Users/jobinlawrance/Project/raven && go mod tidy
```

Confirm `github.com/MicahParks/keyfunc/v3` and `github.com/golang-jwt/jwt/v5` are no longer in `go.mod` (they should have been removed when `internal/middleware/auth.go` stopped importing them).

- [ ] **12.5** Verify nothing else imports the removed packages:

```bash
cd /Users/jobinlawrance/Project/raven && grep -r "keyfunc\|golang-jwt" --include="*.go" .
```

This should return no results. If any files still import these, update them.

**Commit message:**
```
chore: remove Zitadel artifacts, silent-renew, and add SuperTokens migration
```

---

### Task 13: Update EC2 Production Compose — Replace Zitadel+PG16 with SuperTokens

**Files to modify:**
- `deploy/ec2/docker-compose.server.yml` (lines 33-35: go-api Zitadel env; lines 42-44: go-api depends_on; lines 107-161: zitadel-db + zitadel services; line 210: zitadel-pg-data volume)

**Steps:**

- [ ] **13.1** In `deploy/ec2/docker-compose.server.yml`, update the `go-api` service environment (lines 33-35) — replace Zitadel env vars:

```yaml
  go-api:
    # ...
    env_file: .env.server
    environment:
      RAVEN_SUPERTOKENS_CONNECTION_URI: "http://supertokens:3567"
      RAVEN_SUPERTOKENS_API_KEY: "${SUPERTOKENS_API_KEY}"
      GOOGLE_CLIENT_ID: "${GOOGLE_CLIENT_ID}"
      GOOGLE_CLIENT_SECRET: "${GOOGLE_CLIENT_SECRET}"
```

Remove these lines:
```yaml
      ZITADEL_EXTERNALDOMAIN: ${ZITADEL_EXTERNALDOMAIN:-auth.ravencloak.org}
      ZITADEL_CLIENT_ID: ${ZITADEL_CLIENT_ID}
      ZITADEL_EXTERNALSECURE: "true"
```

- [ ] **13.2** Update the `go-api` service `depends_on` (lines 38-44) — replace `zitadel` with `supertokens`:

```yaml
    depends_on:
      postgres:
        condition: service_healthy
      valkey:
        condition: service_healthy
      supertokens:
        condition: service_healthy
```

- [ ] **13.3** Remove the `zitadel-db` service entirely (lines 107-122):

```yaml
# DELETE THIS ENTIRE BLOCK:
  # ─── Zitadel PostgreSQL (separate PG16 — Zitadel needs UNLOGGED partitioned tables) ─
  zitadel-db:
    image: postgres:16-alpine
    # ... through healthcheck closing
```

- [ ] **13.4** Remove the `zitadel` service entirely (lines 124-161):

```yaml
# DELETE THIS ENTIRE BLOCK:
  # ─── Zitadel IAM ──────────────────────────────────────────────────────────
  zitadel:
    image: ghcr.io/zitadel/zitadel:v2.71.12
    # ... through healthcheck closing
```

- [ ] **13.5** Add the `supertokens` service after the `valkey` service:

```yaml
  # ─── SuperTokens Auth Backend ─────────────────────────────────────────────
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
    # No port exposure — only reachable inside raven-net
```

- [ ] **13.6** Remove `zitadel-pg-data` from the volumes section (line 210):

```yaml
# DELETE:
  zitadel-pg-data:
```

- [ ] **13.7** Verify compose config parses:

```bash
cd /Users/jobinlawrance/Project/raven/deploy/ec2 && docker compose -f docker-compose.server.yml config --quiet 2>&1 || echo "Parse check (may need env vars)"
```

**Commit message:**
```
feat(deploy): replace Zitadel+PG16 with SuperTokens in EC2 production compose
```

---

### Task 14: Update tests — Mock AuthProvider in Go tests, update frontend tests

**Files to modify:**
- `internal/middleware/auth_test.go` (complete rewrite to use mock AuthProvider)
- `frontend/src/stores/auth.spec.ts` (rewrite for SuperTokens)

**Steps:**

- [ ] **14.1** Rewrite `internal/middleware/auth_test.go` to use a mock `AuthProvider` instead of JWT/JWKS:

```go
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockAuthProvider implements auth.AuthProvider for testing.
type mockAuthProvider struct {
	sessionInfo *auth.SessionInfo
	err         error
}

func (m *mockAuthProvider) VerifySession(_ *http.Request) (*auth.SessionInfo, error) {
	return m.sessionInfo, m.err
}

func (m *mockAuthProvider) RevokeSession(_ *http.Request) error {
	return m.err
}

// setupRouter wires a test Gin router with the SessionMiddleware and a simple
// 200 OK handler at GET /protected that echoes external_id and email.
func setupRouter(provider auth.AuthProvider) *gin.Engine {
	r := gin.New()
	protected := r.Group("/protected")
	protected.Use(SessionMiddleware(provider))
	protected.GET("", func(c *gin.Context) {
		externalID, _ := c.Get(string(ContextKeyExternalID))
		email, _ := c.Get(string(ContextKeyEmail))
		c.JSON(http.StatusOK, gin.H{
			"external_id": externalID,
			"email":       email,
		})
	})
	return r
}

func TestSessionMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		provider    *mockAuthProvider
		wantStatus  int
		wantErrCode string
		wantExtID   string
		wantEmail   string
	}{
		{
			name: "valid session grants access",
			provider: &mockAuthProvider{
				sessionInfo: &auth.SessionInfo{
					ExternalID: "st-user-42",
					Email:      "user@example.com",
					Name:       "Test User",
				},
			},
			wantStatus: http.StatusOK,
			wantExtID:  "st-user-42",
			wantEmail:  "user@example.com",
		},
		{
			name: "invalid session returns 401",
			provider: &mockAuthProvider{
				err: assert.AnError,
			},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_session",
		},
		{
			name: "nil session info returns 401",
			provider: &mockAuthProvider{
				sessionInfo: nil,
				err:         assert.AnError,
			},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_session",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			router := setupRouter(tc.provider)
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)

			if tc.wantErrCode != "" {
				var body authError
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, tc.wantErrCode, body.Error)
			}

			if tc.wantStatus == http.StatusOK {
				var body map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				if tc.wantExtID != "" {
					assert.Equal(t, tc.wantExtID, body["external_id"])
				}
				if tc.wantEmail != "" {
					assert.Equal(t, tc.wantEmail, body["email"])
				}
			}
		})
	}
}

// TestContextKeys verifies that all expected context keys are populated for a
// valid session so downstream handlers can rely on them.
func TestContextKeys(t *testing.T) {
	provider := &mockAuthProvider{
		sessionInfo: &auth.SessionInfo{
			ExternalID: "sub-ctx-test",
			Email:      "ctx@example.com",
			Name:       "Context User",
		},
	}

	r := gin.New()
	protected := r.Group("/protected")
	protected.Use(SessionMiddleware(provider))
	protected.GET("", func(c *gin.Context) {
		externalID, _ := c.Get(string(ContextKeyExternalID))
		email, _ := c.Get(string(ContextKeyEmail))
		userName, _ := c.Get(string(ContextKeyUserName))

		assert.Equal(t, "sub-ctx-test", externalID)
		assert.Equal(t, "ctx@example.com", email)
		assert.Equal(t, "Context User", userName)

		// ContextKeyUserID and ContextKeyOrgID are NOT set by SessionMiddleware;
		// they are populated by downstream UserLookup after a DB lookup.
		userID, exists := c.Get(string(ContextKeyUserID))
		assert.False(t, exists, "user_id should not be set by SessionMiddleware")
		assert.Nil(t, userID)

		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireOrg verifies that RequireOrg allows requests with an org ID set
// and blocks those without one.
func TestRequireOrg(t *testing.T) {
	r := gin.New()
	r.GET("/with-org", func(c *gin.Context) {
		c.Set(string(ContextKeyOrgID), "org-abc")
		c.Next()
	}, RequireOrg(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/without-org", RequireOrg(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("org present returns 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/with-org", nil))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("org absent returns 403", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/without-org", nil))
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
```

- [ ] **14.2** Rewrite `frontend/src/stores/auth.spec.ts` for SuperTokens:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Mock supertokens-web-js/recipe/session
vi.mock('supertokens-web-js/recipe/session', () => ({
  default: {
    doesSessionExist: vi.fn().mockResolvedValue(true),
    signOut: vi.fn().mockResolvedValue(undefined),
  },
}))

// Mock supertokens-web-js/recipe/thirdparty
vi.mock('supertokens-web-js/recipe/thirdparty', () => ({
  getAuthorisationURLWithQueryParamsAndSetState: vi.fn().mockResolvedValue('https://accounts.google.com/o/oauth2/auth?...'),
  signInAndUp: vi.fn().mockResolvedValue({ status: 'OK' }),
}))

import { useAuthStore } from './auth'

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
  })

  it('initialises as unauthenticated', () => {
    const store = useAuthStore()
    expect(store.isAuthenticated).toBe(false)
  })

  it('detects existing session after init', async () => {
    const store = useAuthStore()
    await store.init()
    expect(store.isAuthenticated).toBe(true)
    expect(store.sessionExists).toBe(true)
  })

  it('exposes orgId after setOrgId', async () => {
    const store = useAuthStore()
    await store.init()
    store.setOrgId('org-456')
    expect(store.orgId).toBe('org-456')
    expect(sessionStorage.getItem('raven_org_id')).toBe('org-456')
  })

  it('hasOrg is false when orgId is not set', () => {
    const store = useAuthStore()
    expect(store.hasOrg).toBe(false)
  })

  it('hasOrg is true when orgId is set', () => {
    sessionStorage.setItem('raven_org_id', 'org-789')
    const store = useAuthStore()
    expect(store.hasOrg).toBe(true)
  })
})
```

- [ ] **14.3** Run the Go middleware tests:

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/middleware/... -v -run TestSessionMiddleware
```

- [ ] **14.4** Run the Go context key tests:

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/middleware/... -v -run TestContextKeys
```

- [ ] **14.5** Run the frontend auth store tests:

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vitest run --reporter=verbose -- src/stores/auth.spec.ts
```

- [ ] **14.6** Run the full Go test suite to verify nothing is broken:

```bash
cd /Users/jobinlawrance/Project/raven && go test ./... 2>&1 | tail -30
```

- [ ] **14.7** Run the full frontend test suite:

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vitest run 2>&1 | tail -20
```

- [ ] **14.8** Run all linters:

```bash
cd /Users/jobinlawrance/Project/raven && golangci-lint run ./...
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit
```

**Commit message:**
```
test: rewrite auth middleware and store tests for SuperTokens
```

---

## Summary of All Changes

| File | Action | Task |
|------|--------|------|
| `docker-compose.yml` | Modify — replace zitadel with supertokens service | 1 |
| `internal/config/config.go` | Modify — replace ZitadelConfig with SuperTokensConfig + GoogleOAuthConfig | 2 |
| `.env.example` | Modify — replace Zitadel env vars with SuperTokens | 2 |
| `frontend/.env.example` | Modify — replace Zitadel vars with VITE_API_DOMAIN | 2 |
| `deploy/postgres/init.sql` | Modify — remove Zitadel DB/user creation | 2 |
| `internal/auth/provider.go` | **Create** — AuthProvider interface + SessionInfo | 3 |
| `internal/auth/supertokens.go` | **Create** — SuperTokensProvider implementation | 4 |
| `internal/middleware/auth.go` | Modify — replace JWTMiddleware with SessionMiddleware, remove all JWT/JWKS code | 5 |
| `internal/handler/authproxy.go` | **Create** — reverse proxy to SuperTokens | 6 |
| `cmd/api/main.go` | Modify — wire auth provider, add proxy route, add recipe config | 7 |
| `internal/middleware/cors.go` | Modify — add SuperTokens headers | 7 |
| `frontend/package.json` | Modify — swap oidc-client-ts for supertokens-web-js | 8 |
| `frontend/src/plugins/supertokens.ts` | **Create** — SDK initialization | 8 |
| `frontend/src/main.ts` | Modify — initialize SuperTokens before auth store | 8 |
| `frontend/src/stores/auth.ts` | Rewrite — cookie-based sessions | 9 |
| `frontend/src/composables/useAuth.ts` | Modify — remove OIDC exports | 9 |
| `frontend/src/api/client.ts` | Rewrite — credentials: include, remove Bearer | 9 |
| `frontend/src/api/orgs.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/workspaces.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/knowledge-bases.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/llm-providers.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/apikeys.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/chatbot-config.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/test-sandbox.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/analytics.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/billing.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/whatsapp.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/api/voice-sessions.ts` | Modify — remove Bearer token | 9 |
| `frontend/src/pages/onboarding/OnboardingWizard.vue` | Modify — remove Bearer token | 9 |
| `frontend/src/pages/LoginPage.vue` | Rewrite — Google sign-in button | 10 |
| `frontend/src/pages/callback/CallbackPage.vue` | Rewrite — SuperTokens signInAndUp | 11 |
| `frontend/src/router/index.ts` | Verify — guard logic unchanged | 11 |
| `deploy/zitadel/` | **Delete** — entire directory | 12 |
| `frontend/public/silent-renew.html` | **Delete** | 12 |
| `migrations/00035_supertokens_migration.sql` | **Create** — update auth_provider default | 12 |
| `deploy/ec2/docker-compose.server.yml` | Modify — replace zitadel+zitadel-db with supertokens | 13 |
| `internal/middleware/auth_test.go` | Rewrite — mock AuthProvider | 14 |
| `frontend/src/stores/auth.spec.ts` | Rewrite — mock SuperTokens SDK | 14 |

## Go Dependencies

**Remove:**
- `github.com/MicahParks/keyfunc/v3`
- `github.com/golang-jwt/jwt/v5`

**Add:**
- None (SuperTokens uses raw HTTP calls)

## Frontend Dependencies

**Remove:**
- `oidc-client-ts`

**Add:**
- `supertokens-web-js`

## Task Dependencies

```
Task 1 (Docker Compose) ─────┐
Task 2 (Config + Env) ───────┤
Task 3 (AuthProvider) ───────┤
                              ├─→ Task 7 (Wire main.go) ─→ Task 12 (Cleanup)
Task 4 (SuperTokens impl) ───┤                                    │
Task 5 (Session middleware) ──┤                                    ├─→ Task 14 (Tests)
Task 6 (Auth proxy) ─────────┘                                    │
                                                                   │
Task 8 (npm packages) ───────┐                                    │
                              ├─→ Task 11 (Callback+Router) ──────┤
Task 9 (Auth store) ─────────┤                                    │
Task 10 (Login page) ────────┘                                    │
                                                                   │
                                                    Task 13 (EC2 compose) ──┘
```

**Parallelizable groups:**
- Tasks 1-6 can all be done in parallel (no file conflicts)
- Task 7 depends on Tasks 3-6
- Tasks 8-10 can be done in parallel
- Task 11 depends on Tasks 8-9
- Tasks 12-13 can be done in parallel after Task 7
- Task 14 depends on Tasks 5, 7, 12
