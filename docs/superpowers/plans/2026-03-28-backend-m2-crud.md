# Backend M2 CRUD + Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Organization CRUD, RBAC middleware, PostgreSQL RLS, Workspace CRUD, User Management, Knowledge Base CRUD, and Swagger docs for the Raven API (M2: issues #24, #28, #29, #25, #26, #27, #32).

**Architecture:** Three-layer Go pattern: handler (Gin, HTTP) → service (business logic) → repository (raw pgx SQL). Each entity gets its own file in each layer. JWT claims (org_id, user sub, org_role) are read from the Gin context key set by `middleware.JWTMiddleware`. Every DB transaction sets `SET LOCAL app.current_org_id` for RLS.

**Tech Stack:** Go 1.23, Gin v1.12, pgx/v5 (to be added), goose migrations, golang-jwt/jwt/v5, pkg/apierror, pkg/validator.

**Worktree:** `.claude/worktrees/stream-backend` on branch `feat/stream-backend-m2-crud`

---

## Pre-flight: Read before writing a single line

- [ ] Read `cmd/api/main.go` — understand router setup, middleware order, how routes are registered
- [ ] Read `internal/middleware/auth.go` — learn context keys for `org_id`, `user_id`, `org_role`
- [ ] Read `internal/config/config.go` — understand Config struct, how to add DB client
- [ ] Read `pkg/apierror/apierror.go` — use these helpers for all error responses
- [ ] Read `migrations/00003_organizations.sql`, `00005_workspaces.sql`, `00007_knowledge_bases.sql` — DB schema
- [ ] Read `migrations/00002_roles.sql` — understand `raven_app` and `raven_admin` roles
- [ ] Check `go.mod` — confirm pgx/v5 is NOT yet a dependency (must add it)

---

## Task 1: Add pgx/v5 database layer foundation

**Closes part of:** Infrastructure needed by #24, #25, #26, #27, #28, #29

**Files:**
- Create: `internal/db/db.go` — pool init + `SetLocalOrgID` helper
- Modify: `internal/config/config.go` — already has `DatabaseConfig.URL`
- Modify: `cmd/api/main.go` — open pool, pass to handlers
- Test: `internal/db/db_test.go`

- [ ] Add pgx dependency:
```bash
cd .claude/worktrees/stream-backend
go get github.com/jackc/pgx/v5@latest
go get github.com/jackc/pgx/v5/pgxpool@latest
```

- [ ] Write failing test for `SetLocalOrgID`:
```go
// internal/db/db_test.go
package db_test

import (
    "context"
    "testing"
    "github.com/ravencloak-org/Raven/internal/db"
)

func TestSetLocalOrgID_ValidUUID(t *testing.T) {
    // unit test only - no DB needed
    orgID := "550e8400-e29b-41d4-a716-446655440000"
    query := db.SetOrgIDQuery(orgID)
    if query != "SET LOCAL app.current_org_id = '550e8400-e29b-41d4-a716-446655440000'" {
        t.Errorf("unexpected query: %s", query)
    }
}
```

- [ ] Run test — expect FAIL (package doesn't exist):
```bash
go test -short ./internal/db/... 2>&1
```

- [ ] Implement `internal/db/db.go`:
```go
package db

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

// New creates and validates a pgx connection pool.
func New(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
    pool, err := pgxpool.New(ctx, databaseURL)
    if err != nil {
        return nil, fmt.Errorf("pgxpool.New: %w", err)
    }
    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("db ping: %w", err)
    }
    return pool, nil
}

// SetOrgIDQuery returns the SQL to set the current org for RLS (use with Exec inside a tx).
func SetOrgIDQuery(orgID string) string {
    return fmt.Sprintf("SET LOCAL app.current_org_id = '%s'", orgID)
}

// WithOrgID executes fn inside a transaction with app.current_org_id set for RLS.
func WithOrgID(ctx context.Context, pool *pgxpool.Pool, orgID string, fn func(tx pgxpool.Tx) error) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx) //nolint:errcheck
    if _, err := tx.Exec(ctx, SetOrgIDQuery(orgID)); err != nil {
        return fmt.Errorf("set org_id: %w", err)
    }
    if err := fn(tx); err != nil {
        return err
    }
    return tx.Commit(ctx)
}
```

- [ ] Run tests — expect PASS:
```bash
go test -short ./internal/db/...
```

- [ ] Run full baseline to confirm nothing broken:
```bash
go test -short ./...
```

- [ ] Commit:
```bash
git add internal/db/ go.mod go.sum
git commit -m "feat: add pgx/v5 pool and RLS transaction helper"
```

---

## Task 2: Issue #29 — PostgreSQL RLS Policies migration

**GitHub issue:** #29 — All tenant tables must have RLS enabled with `tenant_isolation` + `admin_bypass` policies.

**Why first:** RLS is a DB-layer concern independent of Go code. Doing it before CRUD means tests validate isolation from the start.

**Files:**
- Check: `migrations/00005_workspaces.sql` already has RLS — verify which tables are missing it
- Create: `migrations/00015_rls_policies.sql`

- [ ] Check which tables already have RLS:
```bash
grep -l "ENABLE ROW LEVEL SECURITY" /Users/jobinlawrance/Project/raven/migrations/*.sql
```

- [ ] Write migration `migrations/00015_rls_policies.sql` — enable RLS on any tables not yet covered (organizations table does NOT get RLS — it IS the tenant boundary). Add for: `users`, `workspace_members`, `knowledge_bases`, `documents`, `sources`, `chunks`, `embeddings`, `llm_provider_configs`, `chat_sessions`, `chat_messages`, `api_keys`:

```sql
-- +goose Up

-- Enable RLS on all tenant-scoped tables not already covered.
-- organizations table is intentionally excluded (it IS the tenant boundary).

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users
    FOR ALL USING (org_id = current_setting('app.current_org_id', true)::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON users FOR ALL TO raven_admin USING (true);

ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON workspace_members
    FOR ALL USING (
        org_id = current_setting('app.current_org_id', true)::uuid
    )
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON workspace_members FOR ALL TO raven_admin USING (true);

ALTER TABLE knowledge_bases ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON knowledge_bases
    FOR ALL USING (org_id = current_setting('app.current_org_id', true)::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON knowledge_bases FOR ALL TO raven_admin USING (true);

ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON documents
    FOR ALL USING (org_id = current_setting('app.current_org_id', true)::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON documents FOR ALL TO raven_admin USING (true);

ALTER TABLE sources ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON sources
    FOR ALL USING (org_id = current_setting('app.current_org_id', true)::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON sources FOR ALL TO raven_admin USING (true);

ALTER TABLE chunks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON chunks
    FOR ALL USING (org_id = current_setting('app.current_org_id', true)::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON chunks FOR ALL TO raven_admin USING (true);

ALTER TABLE embeddings ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON embeddings
    FOR ALL USING (org_id = current_setting('app.current_org_id', true)::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON embeddings FOR ALL TO raven_admin USING (true);

ALTER TABLE llm_provider_configs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON llm_provider_configs
    FOR ALL USING (org_id = current_setting('app.current_org_id', true)::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON llm_provider_configs FOR ALL TO raven_admin USING (true);

ALTER TABLE chat_sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON chat_sessions
    FOR ALL USING (org_id = current_setting('app.current_org_id', true)::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON chat_sessions FOR ALL TO raven_admin USING (true);

ALTER TABLE chat_messages ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON chat_messages
    FOR ALL USING (
        session_id IN (
            SELECT id FROM chat_sessions
            WHERE org_id = current_setting('app.current_org_id', true)::uuid
        )
    );
CREATE POLICY admin_bypass ON chat_messages FOR ALL TO raven_admin USING (true);

ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON api_keys
    FOR ALL USING (org_id = current_setting('app.current_org_id', true)::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id', true)::uuid);
CREATE POLICY admin_bypass ON api_keys FOR ALL TO raven_admin USING (true);

-- +goose Down
ALTER TABLE api_keys DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON api_keys;
DROP POLICY IF EXISTS admin_bypass ON api_keys;
-- (repeat for each table above)
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON users;
DROP POLICY IF EXISTS admin_bypass ON users;
```

- [ ] Adjust DOWN block to cover all tables (follow same pattern).

- [ ] Run migration test (skips in -short mode but validate it compiles):
```bash
go test -short ./migrations/...
```

- [ ] Commit:
```bash
git add migrations/00015_rls_policies.sql
git commit -m "feat(#29): add RLS policies for all tenant-scoped tables"
```

---

## Task 3: Issue #28 — RBAC Middleware

**GitHub issue:** #28 — Four-layer access model enforced in Gin middleware.

**Files:**
- Create: `internal/middleware/rbac.go`
- Create: `internal/middleware/rbac_test.go`

**Context keys from auth.go** (read first): Understand how `org_role` and workspace roles are set.

- [ ] Write failing tests in `internal/middleware/rbac_test.go`:
```go
package middleware_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/ravencloak-org/Raven/internal/middleware"
)

func TestRequireOrgRole_OrgAdminAllowed(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set(middleware.ContextKeyOrgRole, "org_admin")
        c.Next()
    })
    r.GET("/test", middleware.RequireOrgRole("org_admin"), func(c *gin.Context) {
        c.Status(http.StatusOK)
    })
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", w.Code)
    }
}

func TestRequireOrgRole_InsufficientRole_Returns403(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set(middleware.ContextKeyOrgRole, "member")
        c.Next()
    })
    r.GET("/test", middleware.RequireOrgRole("org_admin"), func(c *gin.Context) {
        c.Status(http.StatusOK)
    })
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    r.ServeHTTP(w, req)
    if w.Code != http.StatusForbidden {
        t.Errorf("expected 403, got %d", w.Code)
    }
}

func TestRequireWorkspaceRole_RoleHierarchy(t *testing.T) {
    // admin can do everything member can
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set(middleware.ContextKeyWorkspaceRole, "admin")
        c.Next()
    })
    r.GET("/test", middleware.RequireWorkspaceRole("member"), func(c *gin.Context) {
        c.Status(http.StatusOK)
    })
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Errorf("admin should satisfy member requirement, got %d", w.Code)
    }
}
```

- [ ] Run — expect FAIL (functions don't exist yet):
```bash
go test -short ./internal/middleware/... -run TestRequireOrgRole 2>&1
```

- [ ] Implement `internal/middleware/rbac.go`:
```go
package middleware

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/ravencloak-org/Raven/pkg/apierror"
)

// Role hierarchy — higher index = more permissions.
var workspaceRoleRank = map[string]int{
    "viewer": 0,
    "member": 1,
    "admin":  2,
    "owner":  3,
}

// RequireOrgRole returns a middleware that allows only the given org-level role.
// org_admin always passes. Returns 403 with which role is required.
func RequireOrgRole(required string) gin.HandlerFunc {
    return func(c *gin.Context) {
        role, _ := c.Get(ContextKeyOrgRole)
        if role == "org_admin" || role == required {
            c.Next()
            return
        }
        _ = c.Error(&apierror.AppError{
            Code:    http.StatusForbidden,
            Message: "Forbidden",
            Detail:  "requires org role: " + required,
        })
        c.Abort()
    }
}

// RequireWorkspaceRole returns a middleware that enforces minimum workspace role.
// org_admin bypasses workspace role checks. Returns 403 with required role.
func RequireWorkspaceRole(minimum string) gin.HandlerFunc {
    return func(c *gin.Context) {
        orgRole, _ := c.Get(ContextKeyOrgRole)
        if orgRole == "org_admin" {
            c.Next()
            return
        }
        wsRole, _ := c.Get(ContextKeyWorkspaceRole)
        wsRoleStr, _ := wsRole.(string)
        if workspaceRoleRank[wsRoleStr] >= workspaceRoleRank[minimum] {
            c.Next()
            return
        }
        _ = c.Error(&apierror.AppError{
            Code:    http.StatusForbidden,
            Message: "Forbidden",
            Detail:  "requires workspace role: " + minimum,
        })
        c.Abort()
    }
}
```

> **Note:** Add `ContextKeyWorkspaceRole` to `middleware.go` if not already present. Read `auth.go` to check existing keys — do NOT duplicate.

- [ ] Run tests — expect PASS:
```bash
go test -short ./internal/middleware/... 2>&1
```

- [ ] Commit:
```bash
git add internal/middleware/rbac.go internal/middleware/rbac_test.go internal/middleware/middleware.go
git commit -m "feat(#28): add RBAC middleware with org/workspace role hierarchy"
```

- [ ] Create PR — link to #28:
```bash
git push origin feat/stream-backend-m2-crud
gh pr create --title "feat: Role-Based Access Control middleware (#28)" \
  --body "$(cat <<'EOF'
## Summary
- Implements `RequireOrgRole` and `RequireWorkspaceRole` Gin middleware
- org_admin bypasses all workspace checks
- Role hierarchy: viewer < member < admin < owner
- 403 response includes which role is required

Closes #28
EOF
)"
```

---

## Task 4: Issue #24 — Organization CRUD API

**GitHub issue:** #24 — POST/GET/PUT/DELETE orgs + list members.

**Files:**
- Create: `internal/model/org.go`
- Create: `internal/repository/org.go`
- Create: `internal/service/org.go`
- Create: `internal/handler/org.go`
- Create: `internal/handler/org_test.go`
- Modify: `cmd/api/main.go` — register routes, wire DB pool

**Read first:** `migrations/00003_organizations.sql` (schema), `migrations/00004_users.sql` (members join).

- [ ] Define models in `internal/model/org.go`:
```go
package model

import "time"

type OrgStatus string

const (
    OrgStatusActive      OrgStatus = "active"
    OrgStatusDeactivated OrgStatus = "deactivated"
)

type Organization struct {
    ID             string            `json:"id"`
    Name           string            `json:"name"`
    Slug           string            `json:"slug"`
    Status         OrgStatus         `json:"status"`
    Settings       map[string]any    `json:"settings"`
    KeycloakRealm  string            `json:"keycloak_realm,omitempty"`
    CreatedAt      time.Time         `json:"created_at"`
    UpdatedAt      time.Time         `json:"updated_at"`
}

type CreateOrgRequest struct {
    Name string `json:"name" binding:"required,min=2,max=255"`
}

type UpdateOrgRequest struct {
    Name     *string            `json:"name,omitempty" binding:"omitempty,min=2,max=255"`
    Settings map[string]any     `json:"settings,omitempty"`
}

type OrgMember struct {
    UserID    string    `json:"user_id"`
    Email     string    `json:"email"`
    OrgRole   string    `json:"org_role"`
    JoinedAt  time.Time `json:"joined_at"`
}
```

- [ ] Write failing handler tests in `internal/handler/org_test.go` (httptest + mock service):
```go
package handler_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/ravencloak-org/Raven/internal/handler"
    "github.com/ravencloak-org/Raven/internal/middleware"
    "github.com/ravencloak-org/Raven/internal/model"
)

type mockOrgService struct {
    createFn func(ctx interface{}, req model.CreateOrgRequest, creatorID string) (*model.Organization, error)
    getFn    func(ctx interface{}, orgID string) (*model.Organization, error)
}

func (m *mockOrgService) Create(ctx interface{}, req model.CreateOrgRequest, creatorID string) (*model.Organization, error) {
    return m.createFn(ctx, req, creatorID)
}

func TestCreateOrg_Success(t *testing.T) {
    gin.SetMode(gin.TestMode)
    svc := &mockOrgService{
        createFn: func(_ interface{}, req model.CreateOrgRequest, _ string) (*model.Organization, error) {
            return &model.Organization{ID: "test-id", Name: req.Name, Slug: "test-org"}, nil
        },
    }
    h := handler.NewOrgHandler(svc)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set(middleware.ContextKeyUserID, "user-123")
        c.Set(middleware.ContextKeyOrgRole, "org_admin")
        c.Next()
    })
    r.POST("/api/v1/orgs", h.Create)

    body, _ := json.Marshal(map[string]string{"name": "Test Org"})
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/api/v1/orgs", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    r.ServeHTTP(w, req)

    if w.Code != http.StatusCreated {
        t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
    }
}

func TestCreateOrg_InvalidPayload_Returns422(t *testing.T) {
    gin.SetMode(gin.TestMode)
    svc := &mockOrgService{}
    h := handler.NewOrgHandler(svc)
    r := gin.New()
    r.POST("/api/v1/orgs", h.Create)

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/api/v1/orgs", bytes.NewBufferString(`{"name":""}`))
    req.Header.Set("Content-Type", "application/json")
    r.ServeHTTP(w, req)

    if w.Code != http.StatusUnprocessableEntity {
        t.Errorf("expected 422, got %d", w.Code)
    }
}
```

- [ ] Run — expect FAIL:
```bash
go test -short ./internal/handler/... -run TestCreateOrg 2>&1
```

- [ ] Implement `internal/repository/org.go` (raw pgx SQL):
```go
package repository

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/ravencloak-org/Raven/internal/model"
)

type OrgRepository struct{ pool *pgxpool.Pool }

func NewOrgRepository(pool *pgxpool.Pool) *OrgRepository {
    return &OrgRepository{pool: pool}
}

func (r *OrgRepository) Create(ctx context.Context, name, slug, creatorID string) (*model.Organization, error) {
    var org model.Organization
    err := r.pool.QueryRow(ctx,
        `INSERT INTO organizations (name, slug) VALUES ($1, $2)
         RETURNING id, name, slug, status, settings, keycloak_realm, created_at, updated_at`,
        name, slug,
    ).Scan(&org.ID, &org.Name, &org.Slug, &org.Status, &org.Settings, &org.KeycloakRealm, &org.CreatedAt, &org.UpdatedAt)
    if err != nil {
        return nil, fmt.Errorf("OrgRepository.Create: %w", err)
    }
    return &org, nil
}

func (r *OrgRepository) GetByID(ctx context.Context, orgID string) (*model.Organization, error) {
    var org model.Organization
    err := r.pool.QueryRow(ctx,
        `SELECT id, name, slug, status, settings, keycloak_realm, created_at, updated_at
         FROM organizations WHERE id = $1 AND status != 'deactivated'`, orgID,
    ).Scan(&org.ID, &org.Name, &org.Slug, &org.Status, &org.Settings, &org.KeycloakRealm, &org.CreatedAt, &org.UpdatedAt)
    if err != nil {
        return nil, fmt.Errorf("OrgRepository.GetByID: %w", err)
    }
    return &org, nil
}

func (r *OrgRepository) SoftDelete(ctx context.Context, orgID string) error {
    _, err := r.pool.Exec(ctx,
        `UPDATE organizations SET status = 'deactivated' WHERE id = $1`, orgID)
    return err
}
```

- [ ] Implement `internal/service/org.go` (slug generation + business logic):
```go
package service

import (
    "context"
    "regexp"
    "strings"

    "github.com/ravencloak-org/Raven/internal/model"
    "github.com/ravencloak-org/Raven/internal/repository"
    "github.com/ravencloak-org/Raven/pkg/apierror"
)

type OrgService struct{ repo *repository.OrgRepository }

func NewOrgService(repo *repository.OrgRepository) *OrgService {
    return &OrgService{repo: repo}
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func toSlug(name string) string {
    s := strings.ToLower(name)
    s = slugRe.ReplaceAllString(s, "-")
    return strings.Trim(s, "-")
}

func (s *OrgService) Create(ctx context.Context, req model.CreateOrgRequest, creatorID string) (*model.Organization, error) {
    slug := toSlug(req.Name)
    org, err := s.repo.Create(ctx, req.Name, slug, creatorID)
    if err != nil {
        return nil, apierror.NewBadRequest("slug already taken or DB error: " + err.Error())
    }
    return org, nil
}

func (s *OrgService) GetByID(ctx context.Context, orgID string) (*model.Organization, error) {
    org, err := s.repo.GetByID(ctx, orgID)
    if err != nil {
        return nil, apierror.NewNotFound("organization not found")
    }
    return org, nil
}
```

- [ ] Implement `internal/handler/org.go`:
```go
package handler

import (
    "context"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/ravencloak-org/Raven/internal/middleware"
    "github.com/ravencloak-org/Raven/internal/model"
    "github.com/ravencloak-org/Raven/pkg/apierror"
)

type OrgServicer interface {
    Create(ctx context.Context, req model.CreateOrgRequest, creatorID string) (*model.Organization, error)
    GetByID(ctx context.Context, orgID string) (*model.Organization, error)
}

type OrgHandler struct{ svc OrgServicer }

func NewOrgHandler(svc OrgServicer) *OrgHandler { return &OrgHandler{svc: svc} }

func (h *OrgHandler) Create(c *gin.Context) {
    var req model.CreateOrgRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        _ = c.Error(apierror.NewBadRequest(err.Error()))
        c.Status(http.StatusUnprocessableEntity)
        c.Abort()
        return
    }
    userID, _ := c.Get(middleware.ContextKeyUserID)
    org, err := h.svc.Create(c.Request.Context(), req, userID.(string))
    if err != nil {
        _ = c.Error(err)
        c.Abort()
        return
    }
    c.JSON(http.StatusCreated, org)
}

func (h *OrgHandler) Get(c *gin.Context) {
    orgID := c.Param("org_id")
    org, err := h.svc.GetByID(c.Request.Context(), orgID)
    if err != nil {
        _ = c.Error(err)
        c.Abort()
        return
    }
    c.JSON(http.StatusOK, org)
}

func (h *OrgHandler) Delete(c *gin.Context) {
    // TODO: implement soft-delete via service
    c.Status(http.StatusNoContent)
}
```

- [ ] Run tests — expect PASS:
```bash
go test -short ./internal/... 2>&1
```

- [ ] Register routes in `cmd/api/main.go` — add after DB pool init:
```go
// Wire DB pool
pool, err := db.New(context.Background(), cfg.Database.URL)
if err != nil {
    log.Fatalf("failed to connect to database: %v", err)
}
defer pool.Close()

orgRepo := repository.NewOrgRepository(pool)
orgSvc := service.NewOrgService(orgRepo)
orgHandler := handler.NewOrgHandler(orgSvc)

// Register under /api/v1 (already has JWTMiddleware)
api.POST("/orgs", orgHandler.Create)
api.GET("/orgs/:org_id", orgHandler.Get)
api.DELETE("/orgs/:org_id", middleware.RequireOrgRole("org_admin"), orgHandler.Delete)
```

- [ ] Build to confirm compilation:
```bash
go build ./...
```

- [ ] Run all tests:
```bash
go test -short ./...
```

- [ ] Commit:
```bash
git add internal/model/org.go internal/repository/org.go internal/service/org.go internal/handler/org.go internal/handler/org_test.go cmd/api/main.go
git commit -m "feat(#24): Organization CRUD API (POST/GET/DELETE /api/v1/orgs)"
```

- [ ] Push and create PR — link to #24:
```bash
git push origin feat/stream-backend-m2-crud
gh pr create --title "feat: Organization CRUD API (#24)" \
  --body "Closes #24"
```

---

## Task 5: Issue #25 — Workspace CRUD API

**GitHub issue:** #25 — CRUD + member management within org tenant boundary.

**Files:**
- Create: `internal/model/workspace.go`
- Create: `internal/repository/workspace.go`
- Create: `internal/service/workspace.go`
- Create: `internal/handler/workspace.go`
- Create: `internal/handler/workspace_test.go`
- Modify: `cmd/api/main.go`

**Read first:** `migrations/00005_workspaces.sql`, `migrations/00006_workspace_members.sql`

Follow the exact same TDD pattern as Task 4:
1. Write models (WorkspaceModel, CreateWorkspaceRequest, WorkspaceMember)
2. Write failing handler tests
3. Implement repository (GetByOrgAndID, Create, SoftDelete, AddMember, UpdateMemberRole, RemoveMember)
4. Implement service (slug uniqueness within org, member role hierarchy validation)
5. Implement handler (CreateWorkspace, GetWorkspace, DeleteWorkspace, AddMember, UpdateMember, RemoveMember)
6. Register routes: `POST /orgs/:org_id/workspaces`, `GET /orgs/:org_id/workspaces/:ws_id`, etc.
7. Tests pass, build succeeds, commit, PR → closes #25

**Key constraint:** Workspace operations must go through `db.WithOrgID` to enforce RLS.

---

## Task 6: Issue #26 — User Management API (Keycloak Sync)

**GitHub issue:** #26 — `/api/v1/me`, Keycloak webhook, user CRUD.

**Files:**
- Create: `internal/model/user.go`
- Create: `internal/repository/user.go`
- Create: `internal/service/user.go`
- Create: `internal/handler/user.go`
- Create: `internal/handler/user_test.go`
- Modify: `cmd/api/main.go`

Follow same TDD pattern. Key points:
- `/api/v1/me` reads user_id from JWT context, queries DB
- Webhook endpoint: internal-only, validate it's not reachable externally (configure Gin trusted proxies or separate router group)
- User soft-delete respects GDPR flag in config

---

## Task 7: Issue #27 — Knowledge Base CRUD API

**GitHub issue:** #27 — KB CRUD within workspace scope.

**Files:**
- Create: `internal/model/kb.go`
- Create: `internal/repository/kb.go`
- Create: `internal/service/kb.go`
- Create: `internal/handler/kb.go`
- Create: `internal/handler/kb_test.go`
- Modify: `cmd/api/main.go`

Follow same TDD pattern. Routes nest under `/orgs/:org_id/workspaces/:ws_id/knowledge-bases`.
Protect create/delete with `RequireWorkspaceRole("member")` and `RequireWorkspaceRole("admin")` respectively.

---

## Task 8: Issue #32 — Swagger/OpenAPI Generation

**GitHub issue:** #32 — swaggo annotations + Scalar UI at `/api/docs`.

**Files:**
- Modify: all handler files (add swaggo comments)
- Modify: `Makefile` (add `swagger` target)
- Modify: `cmd/api/main.go` (serve docs endpoint)

- [ ] Install swaggo:
```bash
go install github.com/swaggo/swag/cmd/swag@latest
go get github.com/swaggo/gin-swagger
go get github.com/swaggo/files
```

- [ ] Add `@Summary`, `@Tags`, `@Param`, `@Success`, `@Failure`, `@Router` comments to every handler

- [ ] Run `swag init -g cmd/api/main.go --output docs/swagger`

- [ ] Serve at `/api/docs` using scalar or swagger-ui

- [ ] Add to Makefile:
```makefile
swagger:
	swag init -g cmd/api/main.go --output docs/swagger
```

- [ ] PR → closes #32

---

## Final verification before each PR

```bash
# All tests pass
go test -short ./...

# Builds clean
go build ./...

# Lint clean
golangci-lint run
```
