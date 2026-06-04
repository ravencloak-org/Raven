package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/middleware"
)

// newAdminRouter spins up a minimal gin router whose only protected
// route is /protected gated by RequirePlatformAdmin(allow). The caller
// provides the seed value for ContextKeyEmail so we can simulate any
// session state.
func newAdminRouter(t *testing.T, allow []string, email string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if email != "" {
			c.Set(string(middleware.ContextKeyEmail), email)
		}
		c.Next()
	})
	r.GET("/protected", middleware.RequirePlatformAdmin(allow), func(c *gin.Context) {
		isAdmin, _ := c.Get(string(middleware.ContextKeyIsPlatformAdmin))
		gotBool, _ := isAdmin.(bool)
		c.JSON(http.StatusOK, gin.H{"ok": true, "admin": gotBool})
	})
	return r
}

func do(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)
	return w
}

// TestRequirePlatformAdmin_NoSession returns 401 when no session email
// is set — the gate is upstream of session middleware but defends in
// depth.
func TestRequirePlatformAdmin_NoSession(t *testing.T) {
	t.Parallel()
	r := newAdminRouter(t, []string{"alice@example.com"}, "")
	w := do(t, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestRequirePlatformAdmin_NotAllowed returns 403 when the session
// email is present but not in the allow-list.
func TestRequirePlatformAdmin_NotAllowed(t *testing.T) {
	t.Parallel()
	r := newAdminRouter(t, []string{"alice@example.com"}, "bob@example.com")
	w := do(t, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("status: want %d, got %d", http.StatusForbidden, w.Code)
	}
}

// TestRequirePlatformAdmin_EmptyAllowList returns 403 for every caller
// when the env var is unset. Safe default per ADR-0008.
func TestRequirePlatformAdmin_EmptyAllowList(t *testing.T) {
	t.Parallel()
	r := newAdminRouter(t, nil, "alice@example.com")
	w := do(t, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("status: want %d, got %d", http.StatusForbidden, w.Code)
	}
}

// TestRequirePlatformAdmin_HappyPath admits the listed email and sets
// the ContextKeyIsPlatformAdmin flag so downstream handlers can branch.
func TestRequirePlatformAdmin_HappyPath(t *testing.T) {
	t.Parallel()
	r := newAdminRouter(t, []string{"Alice@Example.com"}, "alice@example.com")
	w := do(t, r)
	if w.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"admin":true`) {
		t.Errorf("expected admin=true in body, got %s", w.Body.String())
	}
}

// TestRequirePlatformAdmin_CaseInsensitive pins case-insensitive
// matching against the allow-list (operators don't have to fight
// IdP-side email normalisation).
func TestRequirePlatformAdmin_CaseInsensitive(t *testing.T) {
	t.Parallel()
	r := newAdminRouter(t, []string{"alice@example.com"}, "Alice@Example.COM")
	w := do(t, r)
	if w.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// TestIsPlatformAdmin_Standalone covers the standalone helper used by
// GET /me. Same matching rules as the middleware.
func TestIsPlatformAdmin_Standalone(t *testing.T) {
	t.Parallel()
	allow := []string{" alice@example.com ", "bob@example.com"}
	cases := []struct {
		email string
		want  bool
	}{
		{"alice@example.com", true},
		{"ALICE@example.com", true},
		{"bob@example.com", true},
		{"charlie@example.com", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := middleware.IsPlatformAdmin(tc.email, allow); got != tc.want {
			t.Errorf("IsPlatformAdmin(%q): want %v, got %v", tc.email, tc.want, got)
		}
	}
}
