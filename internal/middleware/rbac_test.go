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
