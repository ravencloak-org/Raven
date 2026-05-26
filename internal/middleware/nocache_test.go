package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/middleware"
)

// NoStoreAPI is mounted on every /api/v1 endpoint to keep authed JSON
// responses out of the browser and shared HTTP caches. Pins the exact header
// set so we can't silently weaken it later (e.g. someone setting just
// "no-cache" which still allows storage with revalidation).
func TestNoStoreAPI_SetsAllCacheBlockingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.NoStoreAPI())
	r.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := map[string]string{
		"Cache-Control": "no-store, max-age=0, private, must-revalidate",
		"Pragma":        "no-cache",
		"Expires":       "0",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
}
