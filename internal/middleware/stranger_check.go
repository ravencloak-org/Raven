package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/ravencloak-org/Raven/internal/model"
)

// strangerRateLua atomically increments a counter and sets a 60-second TTL on
// first creation. Returns the new counter value.
// KEYS[1] — rate-limit key; ARGV[1] — TTL in seconds.
const strangerRateLua = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('EXPIRE', KEYS[1], tonumber(ARGV[1]))
end
return current
`

// suspiciousThreshold is the number of requests in SuspiciousWindowSec seconds
// that triggers automatic throttling and suspicious-behavior flagging.
const suspiciousThreshold = 30

// suspiciousWindowSec is the sliding-window size used for suspicious-behavior
// detection. When a stranger sends more than suspiciousThreshold requests in
// this many seconds, they are auto-throttled.
const suspiciousWindowSec = 60

// StrangerServiceInterface is the subset of StrangerService the middleware requires.
type StrangerServiceInterface interface {
	Upsert(ctx context.Context, orgID string, req model.UpsertStrangerRequest) (*model.StrangerUser, error)
	// FlagSuspicious upgrades an active stranger to throttled status when
	// suspicious burst behaviour is detected. It is a best-effort call;
	// errors are logged but do not block the current request.
	FlagSuspicious(ctx context.Context, orgID, strangerID string) error
}

// StrangerCheck tracks anonymous sessions and enforces block/throttle rules.
// It reads X-Session-ID header; upserts the stranger record on each request.
// Returns 403 if status is blocked/banned.
//
// Suspicious-behavior detection: active strangers that exceed
// suspiciousThreshold requests within suspiciousWindowSec seconds are
// automatically promoted to "throttled" and the event is logged. This
// provides baseline protection without requiring operator action.
//
// For throttled users, per-session rate limiting is enforced via a Valkey
// INCR counter with a 60-second TTL.
func StrangerCheck(strangerSvc StrangerServiceInterface, valkey *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgIDVal, _ := c.Get(string(ContextKeyOrgID))
		orgID, _ := orgIDVal.(string)
		if orgID == "" {
			// Fall back to route param for chat API group which uses :kb_id, not :org_id.
			// The org_id is stored in context by APIKeyAuth.
			c.Next()
			return
		}

		sessionID := c.GetHeader("X-Session-ID")
		if sessionID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "X-Session-ID header is required"})
			return
		}

		ipAddr := c.GetHeader("X-Real-IP")
		if ipAddr == "" {
			ipAddr = c.ClientIP()
		}

		req := model.UpsertStrangerRequest{
			SessionID:      sessionID,
			IPAddress:      &ipAddr,
			UserAgent:      c.GetHeader("User-Agent"),
			IncrementCount: c.Request.Method == http.MethodPost,
		}
		user, err := strangerSvc.Upsert(c.Request.Context(), orgID, req)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "stranger check: upsert failed, denying request",
				slog.String("org_id", orgID),
				slog.String("session_id", sessionID),
				slog.String("error", err.Error()),
			)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "service temporarily unavailable"})
			return
		}

		if user.Status == model.StrangerStatusBlocked || user.Status == model.StrangerStatusBanned {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		// --- Suspicious-behavior detection (active sessions only) ---
		// We track every request in a Valkey counter keyed by org+session.
		// If an active stranger blows past the threshold they are auto-throttled
		// and the event is logged. Throttled/blocked/banned users skip this
		// check because they are already under tighter controls.
		if user.Status == model.StrangerStatusActive {
			burstKey := fmt.Sprintf("stranger_burst:%s:%s", orgID, sessionID)
			script := redis.NewScript(strangerRateLua)
			burstCount, burstErr := script.Run(
				c.Request.Context(), valkey, []string{burstKey},
				fmt.Sprintf("%d", suspiciousWindowSec),
			).Int64()
			if burstErr != nil {
				// Fail-open: log the error but allow the request through.
				slog.WarnContext(c.Request.Context(), "stranger check: burst-detection Valkey error, allowing request",
					slog.String("org_id", orgID),
					slog.String("session_id", sessionID),
					slog.String("error", burstErr.Error()),
				)
			} else if int(burstCount) > suspiciousThreshold {
				slog.WarnContext(c.Request.Context(), "stranger check: suspicious burst detected, auto-throttling",
					slog.String("org_id", orgID),
					slog.String("stranger_id", user.ID),
					slog.String("session_id", sessionID),
					slog.Int64("burst_count", burstCount),
				)
				// Best-effort: flag the stranger as suspicious (throttled).
				// Any DB error is logged but does not block the request.
				if flagErr := strangerSvc.FlagSuspicious(c.Request.Context(), orgID, user.ID); flagErr != nil {
					slog.WarnContext(c.Request.Context(), "stranger check: FlagSuspicious failed",
						slog.String("org_id", orgID),
						slog.String("stranger_id", user.ID),
						slog.String("error", flagErr.Error()),
					)
				} else {
					// Update local copy so the rate-limit check below takes effect
					// immediately for the current request.
					user.Status = model.StrangerStatusThrottled
				}
			}
		}

		if user.Status == model.StrangerStatusThrottled {
			rpm := 60
			if user.RateLimitRPM != nil {
				rpm = *user.RateLimitRPM
			}
			key := fmt.Sprintf("stranger_rate:%s:%s", orgID, sessionID)
			script := redis.NewScript(strangerRateLua)
			count, err := script.Run(c.Request.Context(), valkey, []string{key}, "60").Int64()
			if err != nil {
				slog.ErrorContext(c.Request.Context(), "stranger check: Valkey rate-limit script failed, denying request",
					slog.String("org_id", orgID),
					slog.String("session_id", sessionID),
					slog.String("error", err.Error()),
				)
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "service temporarily unavailable"})
				return
			}
			if int(count) > rpm {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
				return
			}
		}

		c.Set("stranger_user", user)
		c.Next()
	}
}
