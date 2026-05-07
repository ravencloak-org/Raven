package resilience

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHTTPClient_TransportError verifies that a connection-level failure
// (unreachable address) surfaces as a non-nil error from HTTPClientWithBreaker.
// This exercises the `return nil, rerr` path inside breakerTransport.RoundTrip.
func TestHTTPClient_TransportError(t *testing.T) {
	// Use an httptest server that is created but never started — its listener is
	// never opened, so any Dial attempt gets a connection-refused error immediately.
	srv := httptest.NewUnstartedServer(nil)
	// srv.URL is empty because it was never started; build a concrete unreachable
	// address using the listener's addr if available, or fall back to a local
	// closed port.
	target := "http://127.0.0.1:1/"

	p, _ := NewPolicy("svc",
		WithTimeout(500*time.Millisecond),
		WithBreakerThreshold(2),
		WithBreakerCooldown(50*time.Millisecond),
	)
	c := HTTPClientWithBreaker(p, NewBreaker(p))

	_, err := c.Get(target)
	if err == nil {
		t.Fatal("expected transport error, got nil")
	}
	// The error must NOT be ErrCircuitOpen — the breaker has not tripped yet.
	if errors.Is(err, ErrCircuitOpen) {
		t.Errorf("got ErrCircuitOpen after a single request; breaker should still be closed")
	}
	_ = srv // referenced to keep the variable used
}

// TestHTTPClient_BreakerOpensOnTransportError verifies that consecutive
// transport-level failures (not 5xx) trip the circuit breaker. This exercises
// the `errors.Is(err, ErrCircuitOpen)` early-return path in RoundTrip as well
// as the `resp, _ := out.(*http.Response); return resp, nil` happy path
// indirectly via the closed-state wrap.
func TestHTTPClient_BreakerOpensOnTransportError(t *testing.T) {
	target := "http://127.0.0.1:1/"

	p, _ := NewPolicy("svc",
		WithTimeout(500*time.Millisecond),
		WithBreakerThreshold(2),
		WithBreakerCooldown(50*time.Millisecond),
	)
	c := HTTPClientWithBreaker(p, NewBreaker(p))

	// Drive two consecutive failures to meet the threshold.
	for i := 0; i < 2; i++ {
		_, _ = c.Get(target)
	}

	// Third call — breaker must be Open.
	_, err := c.Get(target)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("err = %v, want ErrCircuitOpen after %d transport failures", err, 2)
	}
}

func TestHTTPClient_AppliesTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p, _ := NewPolicy("svc", WithTimeout(20*time.Millisecond))
	c := HTTPClient(p)

	resp, err := c.Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHTTPClient_BreakerOpensOn5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _ := NewPolicy("svc",
		WithTimeout(500*time.Millisecond),
		WithBreakerThreshold(2),
		WithBreakerCooldown(50*time.Millisecond),
	)
	c := HTTPClientWithBreaker(p, NewBreaker(p))

	for i := 0; i < 2; i++ {
		resp, _ := c.Get(srv.URL)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	_, err := c.Get(srv.URL)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("err = %v, want ErrCircuitOpen", err)
	}
}
