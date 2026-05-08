package middleware_test

// single_user_integration_test.go validates that SingleUserMiddleware correctly
// injects the local session and that authenticated endpoints work without any
// auth headers when single-user mode is active.
//
// These tests do not require Docker or a real database — they use an in-process
// Gin router with mock handlers to verify the middleware chain behaviour.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
)

// buildSingleUserRouter simulates what main.go does in single-user mode:
// SingleUserMiddleware replaces SessionMiddleware+UserLookup.
func buildSingleUserRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	api := r.Group("/api/v1")
	api.Use(middleware.SingleUserMiddleware())
	{
		api.GET("/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"user_id": c.GetString(string(middleware.ContextKeyUserID)),
				"org_id":  c.GetString(string(middleware.ContextKeyOrgID)),
				"email":   c.GetString(string(middleware.ContextKeyEmail)),
			})
		})

		api.POST("/chat", func(c *gin.Context) {
			orgID := c.GetString(string(middleware.ContextKeyOrgID))
			if orgID == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no org"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
	}

	return r
}

const (
	wantLocalUserID = "00000000-0000-0000-0000-000000000002"
	wantLocalOrgID  = "00000000-0000-0000-0000-000000000001"
)

// TestSingleUserMode_GetMe_Returns200WithoutAuthHeaders verifies that an
// authenticated endpoint returns 200 and the local identity without any
// Authorization header or session cookie.
func TestSingleUserMode_GetMe_Returns200WithoutAuthHeaders(t *testing.T) {
	r := buildSingleUserRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/me", nil)
	// Deliberately no Authorization header, no Cookie.
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["user_id"] != wantLocalUserID {
		t.Errorf("user_id = %q, want %q", body["user_id"], wantLocalUserID)
	}
	if body["org_id"] != wantLocalOrgID {
		t.Errorf("org_id = %q, want %q", body["org_id"], wantLocalOrgID)
	}
	if body["email"] != "local@raven.localhost" {
		t.Errorf("email = %q, want %q", body["email"], "local@raven.localhost")
	}
}

// TestSingleUserMode_PostChat_Returns200WithoutAuthHeaders verifies that a
// representative POST endpoint returns 200 in single-user mode with no headers.
func TestSingleUserMode_PostChat_Returns200WithoutAuthHeaders(t *testing.T) {
	r := buildSingleUserRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat", http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSingleUserMode_ContextKeys_AllSet verifies that all expected context keys
// are populated by SingleUserMiddleware so downstream middleware (RequireOrg,
// RequireOrgRole, etc.) does not abort.
func TestSingleUserMode_ContextKeys_AllSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SingleUserMiddleware())
	r.Use(middleware.RequireOrg())
	r.GET("/guarded", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/guarded", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("RequireOrg blocked single-user request: got %d, want 200", w.Code)
	}
}
