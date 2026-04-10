package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ContextKeyAPIKey is the Gin context key under which the raw API key is stored
// by the auth middleware so the rate limiter can hash and bucket by it.
const ContextKeyAPIKey = "api_key"

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
// Returns a three-element array: {current_count, remaining, oldest_ms}.
//   - Admitted:  remaining >= 0  (0 means the last slot was just consumed)
//   - Rejected:  remaining == -1 (count >= limit, request was NOT recorded)
//   - oldest_ms: Unix timestamp in milliseconds of the oldest surviving hit,
//     or 0 if the set is empty. Callers use this to compute the accurate reset
//     time: oldest_ms + window_ms gives the moment the oldest hit expires.
const slidingWindowLua = `
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local count = redis.call('ZCARD', key)
local oldest = 0
local oldest_entry = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
if #oldest_entry > 0 then
  oldest = tonumber(oldest_entry[2])
end
if count < limit then
  redis.call('ZADD', key, now, now .. math.random())
  redis.call('EXPIRE', key, math.ceil(window/1000) + 1)
  return {count + 1, limit - count - 1, oldest}
end
return {count, -1, oldest}
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
	// resetAt is the Unix timestamp (seconds) at which the oldest in-window hit
	// expires, i.e. when the rate-limit counter first drops.  Falls back to
	// now+window when Valkey is unavailable or the window is empty.
	resetAt int64
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
	fallbackResetAt := time.Now().Add(time.Duration(windowMs) * time.Millisecond).Unix()

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
		return rateLimitResult{count: 0, remaining: int64(limit), admitted: true, resetAt: fallbackResetAt}
	}

	count := res[0]
	remaining := res[1]
	oldestMs := res[2]

	// Compute the precise reset time: the oldest surviving hit expires at
	// oldest_ms + window_ms.  When the window is empty (oldestMs == 0) fall
	// back to now + window.
	var resetAt int64
	if oldestMs > 0 {
		resetAt = (oldestMs + windowMs) / 1000
	} else {
		resetAt = fallbackResetAt
	}

	// remaining == -1 is the sentinel the Lua script uses for "rejected".
	admitted := remaining >= 0
	if remaining < 0 {
		remaining = 0 // clamp for header display
	}
	return rateLimitResult{count: count, remaining: remaining, admitted: admitted, resetAt: resetAt}
}

// keyPrefixFallback is the default fallback prefix used when no scope-specific
// prefix is provided to RateLimitMiddleware.
const keyPrefixFallback = "raven:rl:fallback:"

// fallbackLogSeen deduplicates the "identity lookup missed" warning so it fires
// at most once per IP per 5-minute window rather than on every request.
var fallbackLogSeen sync.Map

// RateLimitMiddleware is the generic Gin middleware factory.
//
//   - limit          — maximum requests per minute for this scope
//   - keyFn          — extracts the rate-limit key from the request context;
//     returning "" means identity could not be determined; a fallback
//     IP-based key is used instead so the limiter is never silently skipped.
//   - fallbackPrefix — key prefix used when keyFn returns ""; each scope
//     should pass its own prefix so that stacked middlewares do not share
//     the same anonymous bucket (e.g. keyPrefixUser+"fallback:" for ByUserID).
func RateLimitMiddleware(rl *RateLimiter, limit int, keyFn func(*gin.Context) string, fallbackPrefix string) gin.HandlerFunc {
	const windowMs = int64(60_000) // 1 minute sliding window
	if fallbackPrefix == "" {
		fallbackPrefix = keyPrefixFallback
	}

	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			// Identity lookup returned no key — fall back to the client IP so
			// the limiter still applies.  Log a warning, but deduplicate per IP
			// to avoid flooding logs under sustained anonymous traffic.
			ip := c.ClientIP()
			const logTTL = 5 * time.Minute
			now := time.Now()
			if v, loaded := fallbackLogSeen.LoadOrStore(ip, now); loaded {
				if now.Sub(v.(time.Time)) >= logTTL {
					// Entry is stale — refresh and log again.
					fallbackLogSeen.Store(ip, now)
					rl.logger.WarnContext(c.Request.Context(),
						"rate limiter: identity lookup missed, using IP fallback",
						slog.String("ip", ip),
						slog.String("path", c.FullPath()),
					)
				}
				// else: recently logged for this IP — stay silent
			} else {
				// First time we've seen this IP fall back — log immediately.
				rl.logger.WarnContext(c.Request.Context(),
					"rate limiter: identity lookup missed, using IP fallback",
					slog.String("ip", ip),
					slog.String("path", c.FullPath()),
				)
			}
			key = fallbackPrefix + ip
		}

		result := rl.check(c.Request.Context(), key, limit, windowMs)

		// Set X-RateLimit-* headers, keeping the tightest (lowest remaining)
		// when multiple rate-limit middlewares are stacked on the same route.
		// This ensures clients always observe the binding constraint.
		prevRemStr := c.Writer.Header().Get("X-RateLimit-Remaining")
		if prevRemStr == "" {
			c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
			c.Header("X-RateLimit-Remaining", strconv.FormatInt(result.remaining, 10))
			c.Header("X-RateLimit-Reset", strconv.FormatInt(result.resetAt, 10))
		} else if prevRem, err := strconv.ParseInt(prevRemStr, 10, 64); err == nil && result.remaining < prevRem {
			c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
			c.Header("X-RateLimit-Remaining", strconv.FormatInt(result.remaining, 10))
			c.Header("X-RateLimit-Reset", strconv.FormatInt(result.resetAt, 10))
		}

		if !result.admitted {
			retryAfter := result.resetAt - time.Now().Unix()
			if retryAfter < 1 {
				retryAfter = 1
			}
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
	}, keyPrefixAPIKey+"fallback:")
}

// ByUserID returns a Gin middleware that rate-limits by authenticated user ID.
// The user ID is read from gin.Context key ContextKeyUserID (set by JWT middleware).
func ByUserID(rl *RateLimiter, limit int) gin.HandlerFunc {
	return RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
		raw, ok := c.Get(string(ContextKeyUserID))
		if !ok {
			return ""
		}
		id, ok := raw.(string)
		if !ok || id == "" {
			return ""
		}
		return keyPrefixUser + id
	}, keyPrefixUser+"fallback:")
}

// ByOrgID returns a Gin middleware that rate-limits by organisation ID.
// The org ID is read from gin.Context key ContextKeyOrgID (set by JWT middleware).
func ByOrgID(rl *RateLimiter, limit int) gin.HandlerFunc {
	return RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
		raw, ok := c.Get(string(ContextKeyOrgID))
		if !ok {
			return ""
		}
		id, ok := raw.(string)
		if !ok || id == "" {
			return ""
		}
		return keyPrefixOrg + id
	}, keyPrefixOrg+"fallback:")
}

// ── Per-org tier-based rate limiting ────────────────────────────────────────

// RouteGroup identifies the category of endpoint being rate-limited. Each group
// can have different per-tier limits (e.g. completions are more expensive than
// general CRUD).
type RouteGroup string

const (
	// RouteGroupGeneral is the default route group for standard API endpoints.
	RouteGroupGeneral RouteGroup = "general"
	// RouteGroupCompletion is the route group for AI completion endpoints.
	RouteGroupCompletion RouteGroup = "completion"
	// RouteGroupWidget is the route group for the public chatbot widget endpoint.
	RouteGroupWidget RouteGroup = "widget"
)

// TierLimits defines the rate limits for a single subscription tier.
type TierLimits struct {
	GeneralRPM    int // requests per minute for general API endpoints
	CompletionRPM int // requests per minute for AI completion endpoints; -1 = unlimited
}

// TierConfig maps each PlanTier to its TierLimits.
type TierConfig struct {
	Free       TierLimits
	Pro        TierLimits
	Enterprise TierLimits
	WidgetRPM  int // separate stricter limit for widget endpoints
}

// DefaultTierConfig returns the issue-specified tier limits.
func DefaultTierConfig() TierConfig {
	return TierConfig{
		Free:       TierLimits{GeneralRPM: 60, CompletionRPM: 10},
		Pro:        TierLimits{GeneralRPM: 600, CompletionRPM: 120},
		Enterprise: TierLimits{GeneralRPM: 6000, CompletionRPM: -1},
		WidgetRPM:  30,
	}
}

// TierResolver looks up the subscription tier for an organisation.
// Implementations may read from Valkey cache, the database, or a static mapping.
type TierResolver interface {
	// Resolve returns the PlanTier for the given org ID.
	// On error, implementations should return a fallback tier (e.g. "free").
	Resolve(ctx context.Context, orgID string) string
}

// StaticTierResolver always returns a fixed tier. Useful for testing and as the
// default when no billing integration is wired up.
type StaticTierResolver struct {
	Tier string
}

// Resolve implements TierResolver by returning the static tier.
func (s *StaticTierResolver) Resolve(_ context.Context, _ string) string {
	return s.Tier
}

// ValkeyTierResolver reads the subscription tier from a Valkey key.
// The key format is "raven:org_tier:{org_id}".
// If the key does not exist, it falls back to "free".
type ValkeyTierResolver struct {
	client redis.Cmdable
	logger *slog.Logger
}

// NewValkeyTierResolver constructs a ValkeyTierResolver.
func NewValkeyTierResolver(client redis.Cmdable, logger *slog.Logger) *ValkeyTierResolver {
	if logger == nil {
		logger = slog.Default()
	}
	return &ValkeyTierResolver{client: client, logger: logger}
}

// Resolve reads the tier from Valkey. Falls back to "free" on miss or error.
func (v *ValkeyTierResolver) Resolve(ctx context.Context, orgID string) string {
	callCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	tier, err := v.client.Get(callCtx, "raven:org_tier:"+orgID).Result()
	if err != nil {
		// Key not found or Valkey error — default to free tier.
		return "free"
	}
	switch tier {
	case "free", "pro", "enterprise":
		return tier
	default:
		v.logger.WarnContext(ctx, "rate limiter: unknown tier in Valkey, defaulting to free",
			slog.String("org_id", orgID),
			slog.String("tier", tier),
		)
		return "free"
	}
}

// limitsForTier returns the TierLimits for the given tier string.
func limitsForTier(tc TierConfig, tier string) TierLimits {
	switch tier {
	case "pro":
		return tc.Pro
	case "enterprise":
		return tc.Enterprise
	default:
		return tc.Free
	}
}

// limitForRouteGroup returns the per-minute limit for the given route group and tier.
// Returns -1 for unlimited.
func limitForRouteGroup(tc TierConfig, tier string, group RouteGroup) int {
	if group == RouteGroupWidget {
		return tc.WidgetRPM
	}
	tl := limitsForTier(tc, tier)
	switch group {
	case RouteGroupCompletion:
		return tl.CompletionRPM
	default:
		return tl.GeneralRPM
	}
}

// keyPrefixTier is the Valkey key prefix for per-org tier-based rate limiting.
const keyPrefixTier = "ratelimit:"

// ByOrgTier returns a Gin middleware that rate-limits per org based on
// subscription tier and route group. The key format is:
//
//	ratelimit:{org_id}:{route_group}:{window}
//
// where window is always "60s" for the 1-minute sliding window.
//
// Enterprise completion endpoints with CompletionRPM == -1 are not limited.
func ByOrgTier(rl *RateLimiter, tierResolver TierResolver, tierCfg TierConfig, group RouteGroup) gin.HandlerFunc {
	const windowMs = int64(60_000) // 1 minute sliding window

	return func(c *gin.Context) {
		orgID := ""
		if raw, ok := c.Get(string(ContextKeyOrgID)); ok {
			orgID, _ = raw.(string)
		}
		if orgID == "" {
			// No org context — fall back to IP-based limiting with free-tier general limits.
			ip := c.ClientIP()
			key := keyPrefixTier + "anon:" + string(group) + ":60s:" + ip
			limit := limitForRouteGroup(tierCfg, "free", group)
			if limit < 0 {
				c.Next()
				return
			}
			result := rl.check(c.Request.Context(), key, limit, windowMs)
			setRateLimitHeaders(c, limit, result)
			if !result.admitted {
				rejectRateLimited(c, result)
				return
			}
			c.Next()
			return
		}

		tier := tierResolver.Resolve(c.Request.Context(), orgID)
		limit := limitForRouteGroup(tierCfg, tier, group)

		// -1 means unlimited — skip rate limiting entirely.
		if limit < 0 {
			c.Next()
			return
		}

		key := fmt.Sprintf("%s%s:%s:60s", keyPrefixTier, orgID, string(group))
		result := rl.check(c.Request.Context(), key, limit, windowMs)

		setRateLimitHeaders(c, limit, result)
		if !result.admitted {
			rejectRateLimited(c, result)
			return
		}

		c.Next()
	}
}

// setRateLimitHeaders writes X-RateLimit-* headers, preserving the tightest
// constraint when multiple rate-limit middlewares are stacked.
func setRateLimitHeaders(c *gin.Context, limit int, result rateLimitResult) {
	prevRemStr := c.Writer.Header().Get("X-RateLimit-Remaining")
	if prevRemStr == "" {
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(result.remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(result.resetAt, 10))
	} else if prevRem, err := strconv.ParseInt(prevRemStr, 10, 64); err == nil && result.remaining < prevRem {
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(result.remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(result.resetAt, 10))
	}
}

// rejectRateLimited aborts with 429 and sets the Retry-After header.
func rejectRateLimited(c *gin.Context, result rateLimitResult) {
	retryAfter := result.resetAt - time.Now().Unix()
	if retryAfter < 1 {
		retryAfter = 1
	}
	c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error":       "rate_limit_exceeded",
		"retry_after": retryAfter,
	})
}

