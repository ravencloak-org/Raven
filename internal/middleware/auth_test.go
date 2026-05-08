package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/auth"
	"github.com/ravencloak-org/Raven/internal/middleware"
)

type mockAuthProvider struct {
	info *auth.SessionInfo
	err  error
}

func (m *mockAuthProvider) VerifySession(r *http.Request) (*auth.SessionInfo, error) {
	return m.info, m.err
}

func (m *mockAuthProvider) RevokeSession(r *http.Request) error {
	return m.err
}

func setupRouter(provider auth.Provider) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SessionMiddleware(provider))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"external_id": c.GetString(string(middleware.ContextKeyExternalID)),
			"email":       c.GetString(string(middleware.ContextKeyEmail)),
			"name":        c.GetString(string(middleware.ContextKeyUserName)),
		})
	})
	return r
}

func TestSessionMiddleware_ValidSession(t *testing.T) {
	provider := &mockAuthProvider{
		info: &auth.SessionInfo{
			ExternalID: "user-123",
			Email:      "test@example.com",
			Name:       "Test User",
		},
	}
	r := setupRouter(provider)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSessionMiddleware_InvalidSession(t *testing.T) {
	provider := &mockAuthProvider{
		err: fmt.Errorf("invalid session"),
	}
	r := setupRouter(provider)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSingleUserMiddleware_SetsLocalIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SingleUserMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"user_id": c.GetString(string(middleware.ContextKeyUserID)),
			"org_id":  c.GetString(string(middleware.ContextKeyOrgID)),
			"role":    c.GetString(string(middleware.ContextKeyOrgRole)),
			"email":   c.GetString(string(middleware.ContextKeyEmail)),
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "00000000-0000-0000-0000-000000000002") {
		t.Errorf("expected local user_id in response body, got: %s", body)
	}
	if !strings.Contains(body, "00000000-0000-0000-0000-000000000001") {
		t.Errorf("expected local org_id in response body, got: %s", body)
	}
	if !strings.Contains(body, "org_admin") {
		t.Errorf("expected org_admin role in response body, got: %s", body)
	}
}

func TestSingleUserMiddleware_NoAuthHeaderRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SingleUserMiddleware())
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// No Authorization header, no cookies — must still succeed.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200 with no auth headers, got %d", w.Code)
	}
}

func TestRequireOrg_WithOrg(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-123")
		c.Next()
	})
	r.Use(middleware.RequireOrg())
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireOrg_WithoutOrg(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequireOrg())
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}
