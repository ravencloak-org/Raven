package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestRouter(corsConfig *config.CORSConfig) *gin.Engine {
	r := gin.New()
	r.Use(middleware.SecurityHeadersMiddleware())
	r.Use(middleware.CORSMiddleware(corsConfig))
	r.GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": "v1", "status": "ok"})
	})
	return r
}

func defaultCORSConfig() *config.CORSConfig {
	return &config.CORSConfig{
		AllowedOrigins: []string{
			"http://localhost:5173",
			"https://raven-frontend.pages.dev",
		},
	}
}

// TestSecurityHeaders verifies that all required security headers are present
// in a normal (non-preflight) response.
func TestSecurityHeaders(t *testing.T) {
	r := newTestRouter(defaultCORSConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	want := map[string]string{
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-Xss-Protection":          "1; mode=block",
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Permissions-Policy":        "geolocation=(), microphone=(), camera=()",
	}

	for header, expected := range want {
		got := w.Header().Get(header)
		if got != expected {
			t.Errorf("header %q: got %q, want %q", header, got, expected)
		}
	}
}

// TestCORSAllowedOrigin verifies that a preflight from an allowed origin
// receives a 204 with proper CORS headers.
func TestCORSAllowedOrigin(t *testing.T) {
	r := newTestRouter(defaultCORSConfig())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/ping", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// gin-contrib/cors returns 204 for successful pre-flight
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for allowed origin preflight, got %d", w.Code)
	}

	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", allowOrigin, "http://localhost:5173")
	}

	allowCreds := w.Header().Get("Access-Control-Allow-Credentials")
	if allowCreds != "true" {
		t.Errorf("Access-Control-Allow-Credentials: got %q, want %q", allowCreds, "true")
	}
}

// TestCORSDisallowedOrigin verifies that a preflight from an unknown origin
// does not receive an Access-Control-Allow-Origin header.
func TestCORSDisallowedOrigin(t *testing.T) {
	r := newTestRouter(defaultCORSConfig())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/ping", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin == "https://evil.example.com" {
		t.Error("disallowed origin should not appear in Access-Control-Allow-Origin")
	}
}

// TestCORSSecondAllowedOrigin ensures the pages.dev origin also works.
func TestCORSSecondAllowedOrigin(t *testing.T) {
	r := newTestRouter(defaultCORSConfig())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/ping", nil)
	req.Header.Set("Origin", "https://raven-frontend.pages.dev")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for allowed origin preflight, got %d", w.Code)
	}

	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "https://raven-frontend.pages.dev" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", allowOrigin, "https://raven-frontend.pages.dev")
	}
}
