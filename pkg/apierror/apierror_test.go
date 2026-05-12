package apierror_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/resilience"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

func TestNewPaymentRequired(t *testing.T) {
	qErr := apierror.NewPaymentRequired("KB limit reached", 3)
	if qErr.Code != http.StatusPaymentRequired {
		t.Errorf("expected code 402, got %d", qErr.Code)
	}
	if qErr.Message != "Payment Required" {
		t.Errorf("expected message 'Payment Required', got %q", qErr.Message)
	}
	if !qErr.UpgradeRequired {
		t.Error("expected upgrade_required to be true")
	}
	if qErr.Limit != 3 {
		t.Errorf("expected limit 3, got %d", qErr.Limit)
	}

	// Verify it satisfies the error interface.
	var e error = qErr
	if e.Error() != "Payment Required: KB limit reached" {
		t.Errorf("unexpected Error() output: %q", e.Error())
	}
}

func TestErrorHandler_CircuitOpenReturns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.GET("/", func(c *gin.Context) {
		_ = c.Error(resilience.ErrCircuitOpen)
	})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Errorf("missing Retry-After header")
	}
}

// TestErrorHandler_CircuitOpenRetryAfterTracksConfiguredCooldown asserts that
// the Retry-After header matches the breaker's configured cooldown and that the
// body Detail does not leak the "circuit breaker" implementation term.
func TestErrorHandler_CircuitOpenRetryAfterTracksConfiguredCooldown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const cooldown = 12 * time.Second

	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.GET("/", func(c *gin.Context) {
		_ = c.Error(&resilience.CircuitOpenError{PolicyName: "ai-worker", RetryAfter: cooldown})
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}

	if got := w.Header().Get("Retry-After"); got != "12" {
		t.Errorf("Retry-After = %q, want %q", got, "12")
	}

	var body struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if !strings.Contains(body.Detail, "retry after 12s") {
		t.Errorf("Detail %q does not contain 'retry after 12s'", body.Detail)
	}
	if strings.Contains(body.Detail, "circuit breaker") {
		t.Errorf("Detail %q leaks 'circuit breaker' implementation detail", body.Detail)
	}
}
