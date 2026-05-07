package resilience

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
