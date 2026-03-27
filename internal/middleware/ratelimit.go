package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Context keys used by upstream JWT/auth middleware to store identity values.
const (
	ContextKeyUserID = "user_id"
	ContextKeyOrgID  = "org_id"
	ContextKeyAPIKey = "api_key"
)

// Valkey key prefixes for each rate-limit scope.
const (
	keyPrefixAPIKey = "raven:rl:apikey:"
	keyPrefixUser   = "raven:rl:user:"
	keyPrefixOrg    = "raven:rl:org:"
)

// slidingWindowLua is an atomic Lua script that implements a sliding-window
// counter using a Valkey sorted set.
//
// KEYS[1] — the rate-limit key
// ARGV[1] — current time in milliseconds (Unix epoch)
// ARGV[2] — window size in milliseconds
// ARGV[3] — request limit
//
// Returns a two-element array: {current_count, remaining}.
//   - Admitted:  remaining >= 0  (0 means the last slot was just consumed)
//   - Rejected:  remaining == -1 (count >= limit, request was NOT recorded)
const slidingWindowLua = `
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local count = redis.call('ZCARD', key)
if count < limit then
  redis.call('ZADD', key, now, now .. math.random())
  redis.call('EXPIRE', key, math.ceil(window/1000) + 1)
  return {count + 1, limit - count - 1}
end
return {count, -1}
`

// RateLimiter holds the Valkey client and logger used by rate-limit middleware.
type RateLimiter struct {
	client redis.Cmdable
	logger *slog.Logger
}

// NewRateLimiter constructs a RateLimiter from any redis.Cmdable (real client
// or miniredis stub).
func NewRateLimiter(client redis.Cmdable, logger *slog.Logger) *RateLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &RateLimiter{client: client, logger: logger}
}

// rateLimitResult is the decoded response from the Lua script.
type rateLimitResult struct {
	count     int64
	remaining int64
	admitted  bool
}

// redactKey returns a safe representation of a rate-limit key for logging.
// It shows only the first 8 characters followed by "..." to avoid leaking
// user identity (user IDs, org IDs, hashed API keys) into log sinks.
func redactKey(key string) string {
	const maxVisible = 8
	if len(key) <= maxVisible {
		return key
	}
	return key[:maxVisible] + "..."
}

// check runs the sliding-window Lua script against Valkey and returns the
// result.  On any Valkey error the request is admitted (fail-open) and the
// error is logged as a warning.
func (rl *RateLimiter) check(ctx context.Context, key string, limit int, windowMs int64) rateLimitResult {
	now := time.Now().UnixMilli()

	// Apply a short timeout to the Valkey call so a slow/unavailable server
	// does not stall the request indefinitely.
	callCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	script := redis.NewScript(slidingWindowLua)
	res, err := script.Run(callCtx, rl.client,
		[]string{key},
		strconv.FormatInt(now, 10),
		strconv.FormatInt(windowMs, 10),
		strconv.Itoa(limit),
	).Int64Slice()

	if err != nil {
		rl.logger.WarnContext(ctx, "rate limiter: valkey unavailable, allowing request",
			slog.String("key", redactKey(key)),
			slog.String("error", err.Error()),
		)
		// Fail-open: treat as admitted with full remaining quota.
		return rateLimitResult{count: 0, remaining: int64(limit), admitted: true}
	}

	count := res[0]
	remaining := res[1]
	// remaining == -1 is the sentinel the Lua script uses for "rejected".
	admitted := remaining >= 0
	if remaining < 0 {
		remaining = 0 // clamp for header display
	}
	return rateLimitResult{count: count, remaining: remaining, admitted: admitted}
}

// fallbackKey builds a rate-limit key from the request's remote address so
// that anonymous or unidentified callers are still rate-limited rather than
// silently bypassing the limiter.
const keyPrefixFallback = "raven:rl:fallback:"

// RateLimitMiddleware is the generic Gin middleware factory.
//
//   - limit    — maximum requests per minute for this scope
//   - keyFn    — extracts the rate-limit key from the request context;
//     returning "" means identity could not be determined; a fallback
//     IP-based key is used instead so the limiter is never silently skipped.
func RateLimitMiddleware(rl *RateLimiter, limit int, keyFn func(*gin.Context) string) gin.HandlerFunc {
	const windowMs = int64(60_000) // 1 minute sliding window

	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			// Identity lookup returned no key — log a warning and fall back to
			// the client IP so the limiter still applies.  This avoids silently
			// failing open for unauthenticated or misconfigured callers.
			ip := c.ClientIP()
			rl.logger.WarnContext(c.Request.Context(),
				"rate limiter: identity lookup missed, using IP fallback",
				slog.String("ip", ip),
				slog.String("path", c.FullPath()),
			)
			key = keyPrefixFallback + ip
		}

		resetAt := time.Now().Add(time.Minute).Unix()
		result := rl.check(c.Request.Context(), key, limit, windowMs)

		// Always set informational headers.
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(result.remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))

		if !result.admitted {
			retryAfter := int64(60) // worst-case: a full window
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}

// apiKeyHash returns the SHA-256 hex digest of an API key, used as the Valkey
// key suffix so that raw secrets are never stored in Valkey.
func apiKeyHash(apiKey string) string {
	h := sha256.Sum256([]byte(apiKey))
	return fmt.Sprintf("%x", h)
}

// ByAPIKey returns a Gin middleware that rate-limits by API key.
// The raw API key is read from gin.Context key ContextKeyAPIKey and hashed
// with SHA-256 before being used as the Valkey key suffix.
func ByAPIKey(rl *RateLimiter, limit int) gin.HandlerFunc {
	return RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
		raw, ok := c.Get(ContextKeyAPIKey)
		if !ok {
			return ""
		}
		key, ok := raw.(string)
		if !ok || key == "" {
			return ""
		}
		return keyPrefixAPIKey + apiKeyHash(key)
	})
}

// ByUserID returns a Gin middleware that rate-limits by authenticated user ID.
// The user ID is read from gin.Context key ContextKeyUserID (set by JWT middleware).
func ByUserID(rl *RateLimiter, limit int) gin.HandlerFunc {
	return RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
		raw, ok := c.Get(ContextKeyUserID)
		if !ok {
			return ""
		}
		id, ok := raw.(string)
		if !ok || id == "" {
			return ""
		}
		return keyPrefixUser + id
	})
}

// ByOrgID returns a Gin middleware that rate-limits by organisation ID.
// The org ID is read from gin.Context key ContextKeyOrgID (set by JWT middleware).
func ByOrgID(rl *RateLimiter, limit int) gin.HandlerFunc {
	return RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
		raw, ok := c.Get(ContextKeyOrgID)
		if !ok {
			return ""
		}
		id, ok := raw.(string)
		if !ok || id == "" {
			return ""
		}
		return keyPrefixOrg + id
	})
}

