package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// runRavenAdminTest sets up a Gin router with RequireRavenAdmin and an
// optional email injector to simulate the SessionMiddleware setting
// ContextKeyEmail.
func runRavenAdminTest(t *testing.T, email string, allowlist string) int {
	t.Helper()
	t.Setenv("RAVEN_ADMIN_EMAILS", allowlist)
	middleware.ResetAdminEmailsCacheForTest()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if email != "" {
			c.Set(string(middleware.ContextKeyEmail), email)
		}
		c.Next()
	})
	r.GET("/admin", middleware.RequireRavenAdmin(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestRequireRavenAdmin_NoEmail(t *testing.T) {
	if got := runRavenAdminTest(t, "", "admin@raven.org"); got != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", got)
	}
}

func TestRequireRavenAdmin_NotInAllowlist(t *testing.T) {
	if got := runRavenAdminTest(t, "user@example.com", "admin@raven.org"); got != http.StatusForbidden {
		t.Errorf("want 403, got %d", got)
	}
}

func TestRequireRavenAdmin_EmptyAllowlist(t *testing.T) {
	// Fail-closed: empty env means no one is admin, even if a session email exists.
	if got := runRavenAdminTest(t, "admin@raven.org", ""); got != http.StatusForbidden {
		t.Errorf("want 403, got %d", got)
	}
}

func TestRequireRavenAdmin_Allowed(t *testing.T) {
	if got := runRavenAdminTest(t, "admin@raven.org", "admin@raven.org"); got != http.StatusOK {
		t.Errorf("want 200, got %d", got)
	}
}

func TestRequireRavenAdmin_AllowedCaseInsensitive(t *testing.T) {
	if got := runRavenAdminTest(t, "Admin@Raven.ORG", "admin@raven.org,other@raven.org"); got != http.StatusOK {
		t.Errorf("want 200, got %d", got)
	}
}

func TestRequireRavenAdmin_AllowedWithMultipleAndWhitespace(t *testing.T) {
	if got := runRavenAdminTest(t, "second@raven.org", " first@raven.org , second@raven.org , "); got != http.StatusOK {
		t.Errorf("want 200, got %d", got)
	}
}
