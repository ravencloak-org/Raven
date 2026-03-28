package telemetry

// PostHog server-side event tracking for the Raven API.
//
// Events are sent via plain HTTP POST to the PostHog /capture and /batch
// endpoints.  This avoids pulling in a heavyweight SDK dependency and keeps
// the implementation transparent.
//
// The client is opt-in: when no API key is configured, every method is a
// no-op.  All public methods are safe for concurrent use.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// PostHogClient sends events to a PostHog instance.
type PostHogClient struct {
	apiKey  string
	host    string
	client  *http.Client
	logger  *slog.Logger
	enabled bool
}

// PostHogConfig holds the configuration for a PostHogClient.
type PostHogConfig struct {
	APIKey string
	Host   string // e.g. "https://us.i.posthog.com"
}

// NewPostHogClient creates a new client.  If cfg.APIKey is empty the
// returned client is a no-op -- every method returns nil immediately.
func NewPostHogClient(cfg PostHogConfig, logger *slog.Logger) *PostHogClient {
	if logger == nil {
		logger = slog.Default()
	}
	host := cfg.Host
	if host == "" {
		host = "https://us.i.posthog.com"
	}

	return &PostHogClient{
		apiKey:  cfg.APIKey,
		host:    host,
		logger:  logger,
		enabled: cfg.APIKey != "",
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Enabled reports whether the client will actually send events.
func (c *PostHogClient) Enabled() bool {
	return c.enabled
}

// capturePayload is the JSON body sent to /capture.
type capturePayload struct {
	APIKey     string                 `json:"api_key"`
	Event      string                 `json:"event"`
	DistinctID string                 `json:"distinct_id"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Timestamp  string                 `json:"timestamp,omitempty"`
}

// identifyPayload is the JSON body sent to /capture with a $identify event.
type identifyPayload struct {
	APIKey     string                 `json:"api_key"`
	Event      string                 `json:"event"`
	DistinctID string                 `json:"distinct_id"`
	Set        map[string]interface{} `json:"$set,omitempty"`
}

// TrackEvent sends a single event to PostHog.
//
// The call is fire-and-forget: errors are logged but not returned so that
// analytics failures never break application logic.
func (c *PostHogClient) TrackEvent(ctx context.Context, distinctID, event string, properties map[string]interface{}) {
	if !c.enabled {
		return
	}

	payload := capturePayload{
		APIKey:     c.apiKey,
		Event:      event,
		DistinctID: distinctID,
		Properties: properties,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	c.post(ctx, "/capture", payload)
}

// IdentifyUser associates properties with a distinct user ID.
func (c *PostHogClient) IdentifyUser(ctx context.Context, distinctID string, properties map[string]interface{}) {
	if !c.enabled {
		return
	}

	payload := identifyPayload{
		APIKey:     c.apiKey,
		Event:      "$identify",
		DistinctID: distinctID,
		Set:        properties,
	}

	c.post(ctx, "/capture", payload)
}

// post marshals body to JSON and sends it to the given PostHog path.
func (c *PostHogClient) post(ctx context.Context, path string, body interface{}) {
	data, err := json.Marshal(body)
	if err != nil {
		c.logger.ErrorContext(ctx, "posthog: failed to marshal payload",
			slog.String("error", err.Error()),
		)
		return
	}

	url := c.host + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		c.logger.ErrorContext(ctx, "posthog: failed to create request",
			slog.String("url", url),
			slog.String("error", err.Error()),
		)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.WarnContext(ctx, "posthog: request failed",
			slog.String("url", url),
			slog.String("error", err.Error()),
		)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		c.logger.WarnContext(ctx, "posthog: non-OK response",
			slog.String("url", url),
			slog.Int("status", resp.StatusCode),
		)
	}
}

// Close releases resources held by the HTTP client.  Currently a no-op but
// provided so the client satisfies io.Closer-style shutdown patterns.
func (c *PostHogClient) Close() error {
	return nil
}

// String implements fmt.Stringer for logging / debugging.
func (c *PostHogClient) String() string {
	if !c.enabled {
		return "PostHogClient(disabled)"
	}
	return fmt.Sprintf("PostHogClient(host=%s)", c.host)
}
