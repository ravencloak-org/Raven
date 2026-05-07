package resilience

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HTTPClient returns an *http.Client configured with the policy timeout
// and a transport with sensible per-stage timeouts. Use this for any
// outbound HTTP call (LiveKit, SeaweedFS, third-party APIs).
func HTTPClient(p *Policy) *http.Client {
	return &http.Client{
		Timeout:   p.Timeout,
		Transport: defaultTransport(),
	}
}

// HTTPClientWithBreaker wraps HTTPClient's transport with a breaker-aware
// RoundTripper. 5xx responses count toward breaker failures.
func HTTPClientWithBreaker(p *Policy, br *Breaker) *http.Client {
	return &http.Client{
		Timeout: p.Timeout,
		Transport: &breakerTransport{
			next:    defaultTransport(),
			breaker: br,
		},
	}
}

func defaultTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
	}
}

type breakerTransport struct {
	next    http.RoundTripper
	breaker *Breaker
}

func (t *breakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	out, err := t.breaker.Execute(req.Context(), func(_ context.Context) (any, error) {
		resp, rerr := t.next.RoundTrip(req)
		if rerr != nil {
			return nil, rerr
		}
		if resp.StatusCode >= 500 {
			return resp, fmt.Errorf("upstream %d", resp.StatusCode)
		}
		return resp, nil
	})
	if errors.Is(err, ErrCircuitOpen) {
		return nil, err
	}
	if err != nil {
		// breakerTransport.Execute may return a non-nil response alongside
		// the synthetic 5xx error; surface that response.
		if r, ok := out.(*http.Response); ok && r != nil {
			return r, nil
		}
		return nil, err
	}
	resp, _ := out.(*http.Response)
	return resp, nil
}
