package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/ravencloak-org/Raven/internal/middleware"
)

// ---------------------------------------------------------------------------
// Mock PoolQuerier for ResolveWorkspaceRole tests
// ---------------------------------------------------------------------------

// mockRow implements pgx.Row, returning a preconfigured role or error.
type mockRow struct {
	role string
	err  error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > 0 {
		if p, ok := dest[0].(*string); ok {
			*p = r.role
		}
	}
	return nil
}

// mockPool implements middleware.PoolQuerier for testing.
type mockPool struct {
	role string
	err  error
}

func (m *mockPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockRow{role: m.role, err: m.err}
}

func TestRequireOrgRole_OrgAdminAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Next()
	})
	r.GET("/test", middleware.RequireOrgRole("org_admin"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("org_admin should be allowed, got %d", w.Code)
	}
}

func TestRequireOrgRole_ExactRoleAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "billing_admin")
		c.Next()
	})
	r.GET("/test", middleware.RequireOrgRole("billing_admin"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("exact required role should be allowed, got %d", w.Code)
	}
}

func TestRequireOrgRole_InsufficientRole_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	r.GET("/test", middleware.RequireOrgRole("org_admin"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("insufficient role should return 403, got %d", w.Code)
	}
}

func TestRequireOrgRole_MissingRole_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// no role set in context
	r.GET("/test", middleware.RequireOrgRole("org_admin"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("missing role should return 403, got %d", w.Code)
	}
}

func TestRequireWorkspaceRole_OrgAdminBypasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		// workspace_role intentionally not set
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceRole("owner"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("org_admin should bypass workspace role check, got %d", w.Code)
	}
}

func TestRequireWorkspaceRole_RoleHierarchy_AdminSatisfiesMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "admin")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceRole("member"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("admin should satisfy member requirement, got %d", w.Code)
	}
}

func TestRequireWorkspaceRole_InsufficientWorkspaceRole_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "viewer")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceRole("admin"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("viewer should not satisfy admin requirement, got %d", w.Code)
	}
}

func TestRequireWorkspaceRole_ExactMinimumAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "member")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceRole("member"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("exact minimum role should be allowed, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// ResolveWorkspaceRole tests
// ---------------------------------------------------------------------------

func TestResolveWorkspaceRole_SetsRoleFromDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := &mockPool{role: "admin"}

	var capturedRole string
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-123")
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	r.GET("/orgs/:org_id/workspaces/:ws_id", middleware.ResolveWorkspaceRole(pool), func(c *gin.Context) {
		role, _ := c.Get(string(middleware.ContextKeyWorkspaceRole))
		capturedRole, _ = role.(string)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/orgs/org-1/workspaces/ws-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if capturedRole != "admin" {
		t.Errorf("expected workspace role 'admin', got %q", capturedRole)
	}
}

func TestResolveWorkspaceRole_OrgAdminBypassesDBLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// The mock returns an error to prove the DB is never queried.
	pool := &mockPool{err: fmt.Errorf("should not be called")}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-123")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Next()
	})
	r.GET("/orgs/:org_id/workspaces/:ws_id", middleware.ResolveWorkspaceRole(pool), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/orgs/org-1/workspaces/ws-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("org_admin should bypass DB lookup, got %d", w.Code)
	}
}

func TestResolveWorkspaceRole_NonMember_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := &mockPool{err: pgx.ErrNoRows}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-456")
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	r.GET("/orgs/:org_id/workspaces/:ws_id", middleware.ResolveWorkspaceRole(pool), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/orgs/org-1/workspaces/ws-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("non-member should get 403, got %d", w.Code)
	}
}

func TestResolveWorkspaceRole_DBError_Returns500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := &mockPool{err: fmt.Errorf("connection refused")}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-789")
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	r.GET("/orgs/:org_id/workspaces/:ws_id", middleware.ResolveWorkspaceRole(pool), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/orgs/org-1/workspaces/ws-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("DB error should return 500, got %d", w.Code)
	}
}

func TestResolveWorkspaceRole_MissingUserID_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := &mockPool{role: "member"}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		// user_id intentionally not set
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	r.GET("/orgs/:org_id/workspaces/:ws_id", middleware.ResolveWorkspaceRole(pool), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/orgs/org-1/workspaces/ws-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing user_id should return 401, got %d", w.Code)
	}
}

func TestResolveWorkspaceRole_MissingWsID_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := &mockPool{role: "member"}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-123")
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	// Route without :ws_id param
	r.GET("/orgs/:org_id/workspaces", middleware.ResolveWorkspaceRole(pool), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/orgs/org-1/workspaces", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("missing ws_id should return 400, got %d", w.Code)
	}
}

func TestResolveWorkspaceRole_AllRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	roles := []string{"viewer", "member", "admin", "owner"}
	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			pool := &mockPool{role: role}
			var capturedRole string

			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUserID), "user-123")
				c.Set(string(middleware.ContextKeyOrgRole), "member")
				c.Next()
			})
			r.GET("/orgs/:org_id/workspaces/:ws_id", middleware.ResolveWorkspaceRole(pool), func(c *gin.Context) {
				v, _ := c.Get(string(middleware.ContextKeyWorkspaceRole))
				capturedRole, _ = v.(string)
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/orgs/org-1/workspaces/ws-1", nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if capturedRole != role {
				t.Errorf("expected role %q, got %q", role, capturedRole)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// End-to-end: ResolveWorkspaceRole + RequireWorkspaceRole chained
// ---------------------------------------------------------------------------

func TestResolveAndRequire_MemberAccessesMemberRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := &mockPool{role: "member"}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-123")
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	r.GET("/orgs/:org_id/workspaces/:ws_id/kb",
		middleware.ResolveWorkspaceRole(pool),
		middleware.RequireWorkspaceRole("member"),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/orgs/org-1/workspaces/ws-1/kb", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("member should access member route, got %d", w.Code)
	}
}

func TestResolveAndRequire_ViewerBlockedFromAdminRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := &mockPool{role: "viewer"}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-123")
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	r.PUT("/orgs/:org_id/workspaces/:ws_id",
		middleware.ResolveWorkspaceRole(pool),
		middleware.RequireWorkspaceRole("admin"),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/orgs/org-1/workspaces/ws-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("viewer should be blocked from admin route, got %d", w.Code)
	}
}

func TestResolveAndRequire_OwnerAccessesAdminRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := &mockPool{role: "owner"}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-123")
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	r.PUT("/orgs/:org_id/workspaces/:ws_id",
		middleware.ResolveWorkspaceRole(pool),
		middleware.RequireWorkspaceRole("admin"),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/orgs/org-1/workspaces/ws-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("owner should access admin route (hierarchy), got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Convenience wrapper tests
// ---------------------------------------------------------------------------

func TestRequireOrgAdmin_AllowsOrgAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Next()
	})
	r.GET("/test", middleware.RequireOrgAdmin(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("RequireOrgAdmin should allow org_admin, got %d", w.Code)
	}
}

func TestRequireOrgAdmin_BlocksNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	r.GET("/test", middleware.RequireOrgAdmin(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("RequireOrgAdmin should block non-admin, got %d", w.Code)
	}
}

func TestRequireWorkspaceOwner_AllowsOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "owner")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceOwner(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("RequireWorkspaceOwner should allow owner, got %d", w.Code)
	}
}

func TestRequireWorkspaceOwner_BlocksAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "admin")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceOwner(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("RequireWorkspaceOwner should block admin, got %d", w.Code)
	}
}

func TestRequireWorkspaceAdmin_AllowsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "admin")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceAdmin(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("RequireWorkspaceAdmin should allow admin, got %d", w.Code)
	}
}

func TestRequireWorkspaceAdmin_BlocksMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "member")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceAdmin(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("RequireWorkspaceAdmin should block member, got %d", w.Code)
	}
}

func TestRequireWorkspaceMember_AllowsMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "member")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceMember(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("RequireWorkspaceMember should allow member, got %d", w.Code)
	}
}

func TestRequireWorkspaceMember_BlocksViewer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "viewer")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceMember(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("RequireWorkspaceMember should block viewer, got %d", w.Code)
	}
}

func TestRequireWorkspaceViewer_AllowsViewer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "viewer")
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceViewer(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("RequireWorkspaceViewer should allow viewer, got %d", w.Code)
	}
}

func TestRequireWorkspaceViewer_NoRoleFallsThroughAtViewerLevel(t *testing.T) {
	// When workspace_role is not set, wsRoleStr is "" and workspaceRoleRank[""]
	// returns 0 (Go zero value). Viewer also has rank 0, so the check passes.
	// In practice, ResolveWorkspaceRole always runs first and either sets a
	// valid role or aborts the request before RequireWorkspaceRole executes.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		// workspace_role intentionally not set
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceViewer(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("unset role maps to rank 0 which equals viewer rank, got %d", w.Code)
	}
}

func TestRequireWorkspaceMember_BlocksNoRole(t *testing.T) {
	// When workspace_role is not set, the rank is 0 which is below member (1).
	// This confirms that ResolveWorkspaceRole is essential: without it, a user
	// with no workspace membership would pass viewer-level checks but not member+.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		// workspace_role intentionally not set
		c.Next()
	})
	r.GET("/test", middleware.RequireWorkspaceMember(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("unset role should not satisfy member requirement, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Full role hierarchy test
// ---------------------------------------------------------------------------

func TestRoleHierarchy_FullMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type testCase struct {
		wsRole   string
		required string
		allowed  bool
	}

	tests := []testCase{
		// viewer
		{"viewer", "viewer", true},
		{"viewer", "member", false},
		{"viewer", "admin", false},
		{"viewer", "owner", false},
		// member
		{"member", "viewer", true},
		{"member", "member", true},
		{"member", "admin", false},
		{"member", "owner", false},
		// admin
		{"admin", "viewer", true},
		{"admin", "member", true},
		{"admin", "admin", true},
		{"admin", "owner", false},
		// owner
		{"owner", "viewer", true},
		{"owner", "member", true},
		{"owner", "admin", true},
		{"owner", "owner", true},
	}

	for _, tc := range tests {
		name := fmt.Sprintf("ws=%s_requires=%s", tc.wsRole, tc.required)
		t.Run(name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyOrgRole), "member")
				c.Set(string(middleware.ContextKeyWorkspaceRole), tc.wsRole)
				c.Next()
			})
			r.GET("/test", middleware.RequireWorkspaceRole(tc.required), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/test", nil)
			r.ServeHTTP(w, req)

			if tc.allowed && w.Code != http.StatusOK {
				t.Errorf("role %q should satisfy %q, got %d", tc.wsRole, tc.required, w.Code)
			}
			if !tc.allowed && w.Code != http.StatusForbidden {
				t.Errorf("role %q should NOT satisfy %q, got %d", tc.wsRole, tc.required, w.Code)
			}
		})
	}
}
