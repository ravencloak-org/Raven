package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestDeadline_AppliesTimeoutToRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Deadline(50 * time.Millisecond))
	r.GET("/", func(c *gin.Context) {
		dl, ok := c.Request.Context().Deadline()
		if !ok {
			t.Errorf("request ctx has no deadline")
		}
		if remaining := time.Until(dl); remaining > 60*time.Millisecond {
			t.Errorf("deadline too far away: %v", remaining)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

func TestDeadline_PropagatesCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Deadline(20 * time.Millisecond))

	var observed error
	r.GET("/", func(c *gin.Context) {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-c.Request.Context().Done():
			observed = c.Request.Context().Err()
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !errors.Is(observed, context.DeadlineExceeded) {
		t.Errorf("ctx err = %v, want DeadlineExceeded", observed)
	}
}
