// Package turnstile verifies Cloudflare Turnstile challenge tokens
// against the siteverify endpoint
// (https://developers.cloudflare.com/turnstile/get-started/server-side-validation/).
package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Verifier checks Turnstile tokens server-side.
type Verifier struct {
	// SecretKey is the Turnstile site's secret key. Required.
	SecretKey string

	url    string
	client *http.Client
}

// NewVerifier returns a Verifier pointed at the production Cloudflare
// siteverify endpoint with a 5s request timeout.
func NewVerifier(secretKey string) *Verifier {
	return &Verifier{
		SecretKey: secretKey,
		url:       "https://challenges.cloudflare.com/turnstile/v0/siteverify",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Verify exchanges a Turnstile token for a success/failure verdict.
// remoteIP is optional — pass the empty string to omit it.
//
// Returns (false, err) on transport or decode failures. Returns
// (false, nil) when Cloudflare rejected the token.
func (v *Verifier) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if v.SecretKey == "" {
		return false, errors.New("turnstile: secret not configured")
	}
	if v.client == nil {
		v.client = &http.Client{Timeout: 5 * time.Second}
	}

	form := url.Values{}
	form.Set("secret", v.SecretKey)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.url, strings.NewReader(form.Encode()))
	if err != nil {
		return false, fmt.Errorf("turnstile: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("turnstile: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("turnstile: decode: %w", err)
	}
	return out.Success, nil
}
