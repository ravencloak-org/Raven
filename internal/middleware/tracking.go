package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/telemetry"
)

// TrackingMiddleware returns a Gin middleware that sends API-usage events to
// PostHog.  Each request is captured as an "api_request" event with the
// endpoint, HTTP method, response status, latency, and (when available) the
// authenticated user and organisation IDs.
//
// The middleware is a no-op when the PostHog client is disabled (i.e. no API
// key was configured), so it is safe to install unconditionally.
func TrackingMiddleware(ph *telemetry.PostHogClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Short-circuit when PostHog is not configured.
		if ph == nil || !ph.Enabled() {
			c.Next()
			return
		}

		start := time.Now()

		// Process the request first so we can capture the status code.
		c.Next()

		// Extract identity from the Gin context (set by JWTMiddleware).
		userID, _ := c.Get(string(ContextKeyUserID))
		orgID, _ := c.Get(string(ContextKeyOrgID))

		distinctID := "anonymous"
		if uid, ok := userID.(string); ok && uid != "" {
			distinctID = uid
		}

		latencyMs := float64(time.Since(start).Milliseconds())

		properties := map[string]interface{}{
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"route":      c.FullPath(),
			"status":     c.Writer.Status(),
			"latency_ms": latencyMs,
			"user_agent": c.Request.UserAgent(),
		}

		if oid, ok := orgID.(string); ok && oid != "" {
			properties["org_id"] = oid
		}

		// Fire-and-forget; the PostHog client logs errors internally.
		ph.TrackEvent(c.Request.Context(), distinctID, "api_request", properties)
	}
}
