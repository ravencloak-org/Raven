package turnstile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequired_AllowsBypassHeaderWhenSecretMatches(t *testing.T) {
	r := gin.New()
	r.POST("/x",
		Required(&Verifier{SecretKey: "k"}, "bypass-it"),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/x", nil)
	req.Header.Set("X-Demo-Bypass", "bypass-it")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid bypass header, got %d", w.Code)
	}
}

func TestRequired_RejectsBypassHeaderWhenSecretWrong(t *testing.T) {
	r := gin.New()
	r.POST("/x",
		Required(&Verifier{SecretKey: "k"}, "bypass-it"),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/x", nil)
	req.Header.Set("X-Demo-Bypass", "wrong")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 with wrong bypass header, got %d", w.Code)
	}
}

func TestRequired_MissingTokenReturns403(t *testing.T) {
	r := gin.New()
	r.POST("/x",
		Required(&Verifier{SecretKey: "k"}, ""),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(w.Body).Decode(&body)
	if !strings.Contains(strings.ToLower(body["error"]), "turnstile") {
		t.Fatalf("expected error to mention turnstile, got %q", body["error"])
	}
}

func TestRequired_NoSecretConfiguredAllowsThrough(t *testing.T) {
	// When SecretKey is empty (dev / single-user), the middleware is a
	// pass-through. Otherwise the dev workflow would break.
	r := gin.New()
	r.POST("/x",
		Required(&Verifier{SecretKey: ""}, ""),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with no secret configured, got %d", w.Code)
	}
}

func TestSignupOnly_AppliesOnlyToSignupPaths(t *testing.T) {
	r := gin.New()
	r.Use(SignupOnly(&Verifier{SecretKey: "k"}, ""))
	r.POST("/auth/signinup", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/auth/signin", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/auth/session/refresh", func(c *gin.Context) { c.Status(http.StatusOK) })

	cases := []struct {
		method, path string
		wantStatus   int
	}{
		// Signup POSTs require Turnstile.
		{http.MethodPost, "/auth/signinup", http.StatusForbidden},
		{http.MethodPost, "/auth/signinup/somesub", http.StatusForbidden},
		// Sign-in POST does NOT require Turnstile (existing account).
		{http.MethodPost, "/auth/signin", http.StatusOK},
		// Session refresh does not require Turnstile.
		{http.MethodGet, "/auth/session/refresh", http.StatusOK},
	}
	for _, tc := range cases {
		req, _ := http.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.wantStatus {
			t.Errorf("%s %s: got %d, want %d", tc.method, tc.path, w.Code, tc.wantStatus)
		}
	}
}
