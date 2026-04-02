package middleware

import (
	"context"
	"fmt"
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

// StrangerServiceInterface is the subset of StrangerService the middleware requires.
type StrangerServiceInterface interface {
	Upsert(ctx context.Context, orgID string, req model.UpsertStrangerRequest) (*model.StrangerUser, error)
}

// StrangerCheck tracks anonymous sessions and enforces block/throttle rules.
// It reads X-Session-ID header; upserts the stranger record on each request.
// Returns 403 if status is blocked/banned.
// For throttled users, enforces rate limit via Valkey INCR with 60s TTL.
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
			c.Next()
			return
		}

		ipAddr := c.GetHeader("X-Real-IP")
		if ipAddr == "" {
			ipAddr = c.ClientIP()
		}

		req := model.UpsertStrangerRequest{
			SessionID: sessionID,
			IPAddress: &ipAddr,
			UserAgent: c.GetHeader("User-Agent"),
		}
		user, err := strangerSvc.Upsert(c.Request.Context(), orgID, req)
		if err != nil {
			// Fail open — do not block the request if tracking fails.
			c.Next()
			return
		}

		if user.Status == model.StrangerStatusBlocked || user.Status == model.StrangerStatusBanned {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
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
				// Valkey unavailable — fail open rather than blocking valid users.
				c.Next()
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
