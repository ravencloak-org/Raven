package middleware_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
		"X-XSS-Protection":          "1; mode=block",
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
// is rejected with a 403 and no Access-Control-Allow-Origin header.
func TestCORSDisallowedOrigin(t *testing.T) {
	r := newTestRouter(defaultCORSConfig())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/ping", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed origin preflight, got %d", w.Code)
	}

	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "" {
		t.Errorf("disallowed origin must not appear in Access-Control-Allow-Origin, got %q", allowOrigin)
	}
}

// TestCORSActualRequest verifies that an actual (non-preflight) cross-origin
// request from an allowed origin receives the Access-Control-Allow-Origin header.
func TestCORSActualRequest(t *testing.T) {
	r := newTestRouter(defaultCORSConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
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

// --- Per-API-key origin allowlist (docs-audit #9) ----------------------------

// fakeAPIKeyLookup is a test double for middleware.APIKeyOriginLookup that
// returns canned responses keyed by the SHA-256 hash of the raw key.
type fakeAPIKeyLookup struct {
	byHash map[string]*middleware.APIKeyLookupResult
	err    error
}

func (f *fakeAPIKeyLookup) LookupByHash(_ context.Context, hash string) (*middleware.APIKeyLookupResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if r, ok := f.byHash[hash]; ok {
		return r, nil
	}
	return nil, nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// newKeyAwareRouter wires a CORS middleware with both a server allowlist and a
// per-key lookup so the per-key branch can be exercised end-to-end.
func newKeyAwareRouter(corsConfig *config.CORSConfig, lookup middleware.APIKeyOriginLookup) *gin.Engine {
	r := gin.New()
	r.Use(middleware.SecurityHeadersMiddleware())
	r.Use(middleware.CORSMiddleware(corsConfig, lookup))
	r.GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// Use Any to also serve OPTIONS via gin's NoMethod fallback path so the
	// CORS middleware can return its own 204/403 verdict without us masking
	// it as a 404.
	r.NoRoute(func(c *gin.Context) { c.Status(http.StatusNotFound) })
	return r
}

// xOriginRequest builds a test request and pins Host to a value distinct from
// the supplied Origin so the gin-contrib/cors "same-origin" shortcut at
// applyCors() does not silently bypass our validator. (httptest.NewRequest
// defaults Host to "example.com", which collides with our example origins.)
func xOriginRequest(method, target, origin string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "api.raven.test"
	req.Header.Set("Origin", origin)
	return req
}

// TestCORSPerKey_NoKeyOnRequest_FallsBackToServerAllowlist verifies that a
// request without X-API-Key keeps the legacy server-allowlist semantics even
// when a key lookup is wired in.
func TestCORSPerKey_NoKeyOnRequest_FallsBackToServerAllowlist(t *testing.T) {
	lookup := &fakeAPIKeyLookup{}
	r := newKeyAwareRouter(defaultCORSConfig(), lookup)

	// Allowed by server allowlist.
	req := xOriginRequest(http.MethodGet, "/api/v1/ping", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for server-allowed origin without key, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", got, "http://localhost:5173")
	}

	// Rejected: not in server allowlist, no key supplied.
	req = xOriginRequest(http.MethodOptions, "/api/v1/ping", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for unknown origin without key, got %d", w.Code)
	}
}

// TestCORSPerKey_KeyWithEmptyAllowlist_FallsBackToServer verifies that an
// API key whose allowed_domains is empty is treated as "no per-key
// restriction" and the server allowlist still governs.
func TestCORSPerKey_KeyWithEmptyAllowlist_FallsBackToServer(t *testing.T) {
	const rawKey = "rk_test_empty"
	lookup := &fakeAPIKeyLookup{byHash: map[string]*middleware.APIKeyLookupResult{
		sha256Hex(rawKey): {ID: "k1", AllowedDomains: nil, Status: "active"},
	}}
	r := newKeyAwareRouter(defaultCORSConfig(), lookup)

	// Origin is in the server allowlist -> allowed.
	req := xOriginRequest(http.MethodGet, "/api/v1/ping", "http://localhost:5173")
	req.Header.Set("X-API-Key", rawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (server allowlist applies), got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", got, "http://localhost:5173")
	}

	// Origin not in server allowlist -> rejected (empty per-key list does
	// NOT widen the allowlist).
	req = xOriginRequest(http.MethodOptions, "/api/v1/ping", "https://example.com")
	req.Header.Set("X-API-Key", rawKey)
	req.Header.Set("Access-Control-Request-Method", "GET")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 (empty allowlist doesn't widen server set), got %d", w.Code)
	}
}

// TestCORSPerKey_MatchingOrigin_Allowed verifies that an Origin which matches
// the per-key allowed_domains is accepted EVEN IF it is absent from the
// server-wide RAVEN_CORS_ALLOWED_ORIGINS allowlist.
func TestCORSPerKey_MatchingOrigin_Allowed(t *testing.T) {
	const rawKey = "rk_test_example"
	lookup := &fakeAPIKeyLookup{byHash: map[string]*middleware.APIKeyLookupResult{
		sha256Hex(rawKey): {ID: "k1", AllowedDomains: []string{"example.com"}, Status: "active"},
	}}
	// Server allowlist intentionally excludes example.com.
	r := newKeyAwareRouter(defaultCORSConfig(), lookup)

	req := xOriginRequest(http.MethodGet, "/api/v1/ping", "https://example.com")
	req.Header.Set("X-API-Key", rawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for per-key allowed origin, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", got, "https://example.com")
	}
}

// TestCORSPerKey_NonMatchingOrigin_Rejected verifies that an Origin which does
// NOT match the per-key allowed_domains is rejected, even when the server
// allowlist would otherwise admit it. This is the load-bearing case: a widget
// key bound to example.com must not be usable from attacker.com.
func TestCORSPerKey_NonMatchingOrigin_Rejected(t *testing.T) {
	const rawKey = "rk_test_bound"
	lookup := &fakeAPIKeyLookup{byHash: map[string]*middleware.APIKeyLookupResult{
		sha256Hex(rawKey): {ID: "k1", AllowedDomains: []string{"example.com"}, Status: "active"},
	}}
	// Even though the server allowlist contains localhost:5173, a key bound
	// to example.com must not be usable from there.
	r := newKeyAwareRouter(defaultCORSConfig(), lookup)

	req := xOriginRequest(http.MethodOptions, "/api/v1/ping", "http://localhost:5173")
	req.Header.Set("X-API-Key", rawKey)
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-matching origin under bound key, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin must not appear in Access-Control-Allow-Origin, got %q", got)
	}

	// And the attacker case from the task description.
	req = xOriginRequest(http.MethodGet, "/api/v1/ping", "https://attacker.com")
	req.Header.Set("X-API-Key", rawKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("attacker origin must be rejected, got Allow-Origin %q", got)
	}
}

// TestCORSPerKey_WildcardMatch verifies the `*.example.com` wildcard rule:
// it matches direct sub-domains, deeper sub-domains, and the apex.
func TestCORSPerKey_WildcardMatch(t *testing.T) {
	const rawKey = "rk_test_wild"
	lookup := &fakeAPIKeyLookup{byHash: map[string]*middleware.APIKeyLookupResult{
		sha256Hex(rawKey): {ID: "k1", AllowedDomains: []string{"*.example.com"}, Status: "active"},
	}}
	r := newKeyAwareRouter(defaultCORSConfig(), lookup)

	cases := []struct {
		origin string
		want   int
	}{
		{"https://example.com", http.StatusOK},        // apex
		{"https://www.example.com", http.StatusOK},    // direct sub
		{"https://a.b.example.com", http.StatusOK},    // deeper sub
		{"https://notexample.com", http.StatusForbidden},
		{"https://example.com.attacker.com", http.StatusForbidden},
	}
	for _, tc := range cases {
		req := xOriginRequest(http.MethodOptions, "/api/v1/ping", tc.origin)
		req.Header.Set("X-API-Key", rawKey)
		req.Header.Set("Access-Control-Request-Method", "GET")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// For preflight, OK preflight returns 204; we check >= 400 for deny.
		if tc.want == http.StatusOK {
			if w.Code != http.StatusNoContent {
				t.Errorf("origin %q: expected 204 preflight, got %d", tc.origin, w.Code)
			}
		} else if w.Code < 400 {
			t.Errorf("origin %q: expected denied preflight, got %d", tc.origin, w.Code)
		}
	}
}

// TestCORSPerKey_LookupError_FailsClosed verifies that an infrastructure
// failure in the key lookup denies the request rather than silently falling
// through to the server allowlist (which an attacker who hammered the DB
// could otherwise weaponise).
func TestCORSPerKey_LookupError_FailsClosed(t *testing.T) {
	lookup := &fakeAPIKeyLookup{err: errors.New("db down")}
	r := newKeyAwareRouter(defaultCORSConfig(), lookup)

	req := xOriginRequest(http.MethodGet, "/api/v1/ping", "http://localhost:5173") // would be allowed without a key
	req.Header.Set("X-API-Key", "rk_anything")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("lookup error must fail closed, got Allow-Origin %q", got)
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
