// Package posthog provides a lightweight HTTP client for the PostHog analytics API.
// When no API key is configured the client operates as a no-op so callers never
// need nil-checks.
package posthog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a thin wrapper around the PostHog HTTP capture API.
// All methods are safe for concurrent use.
type Client struct {
	apiKey   string
	endpoint string
	http     *http.Client
	noop     bool
}

// NewClient creates a new PostHog client.
// If apiKey is empty the client is a no-op — all methods return nil immediately.
func NewClient(apiKey, endpoint string) *Client {
	if endpoint == "" {
		endpoint = "https://us.i.posthog.com"
	}
	return &Client{
		apiKey:   apiKey,
		endpoint: endpoint,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
		noop: apiKey == "",
	}
}

// Enabled returns true when the client has a valid API key and will send events.
func (c *Client) Enabled() bool {
	return !c.noop
}

// capturePayload is the JSON body sent to POST /capture.
type capturePayload struct {
	APIKey     string         `json:"api_key"`
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Properties map[string]any `json:"properties,omitempty"`
	Timestamp  string         `json:"timestamp,omitempty"`
}

// Capture sends an event to PostHog.
func (c *Client) Capture(ctx context.Context, distinctID string, event string, properties map[string]any) error {
	if c.noop {
		return nil
	}
	payload := capturePayload{
		APIKey:     c.apiKey,
		Event:      event,
		DistinctID: distinctID,
		Properties: properties,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	return c.post(ctx, "/capture", payload)
}

// identifyPayload is the JSON body for an $identify event.
type identifyPayload struct {
	APIKey     string         `json:"api_key"`
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Set        map[string]any `json:"$set,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

// Identify links properties to a user in PostHog via the $identify event.
func (c *Client) Identify(ctx context.Context, distinctID string, properties map[string]any) error {
	if c.noop {
		return nil
	}
	payload := identifyPayload{
		APIKey:     c.apiKey,
		Event:      "$identify",
		DistinctID: distinctID,
		Set:        properties,
		Properties: map[string]any{
			"$set": properties,
		},
	}
	return c.post(ctx, "/capture", payload)
}

// aliasPayload is the JSON body for a $create_alias event.
type aliasPayload struct {
	APIKey     string         `json:"api_key"`
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Properties map[string]any `json:"properties"`
}

// Alias links two distinct IDs (e.g., anonymous session to authenticated user).
func (c *Client) Alias(ctx context.Context, distinctID, alias string) error {
	if c.noop {
		return nil
	}
	payload := aliasPayload{
		APIKey:     c.apiKey,
		Event:      "$create_alias",
		DistinctID: distinctID,
		Properties: map[string]any{
			"alias": alias,
		},
	}
	return c.post(ctx, "/capture", payload)
}

// post sends a JSON payload to the PostHog HTTP API.
func (c *Client) post(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("posthog: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("posthog: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("posthog: send request: %w", err)
	}
	// Drain body so connection can be reused, then close explicitly.
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("posthog: unexpected status %d for %s", resp.StatusCode, path)
	}
	return nil
}
