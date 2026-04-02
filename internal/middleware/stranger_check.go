package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/ravencloak-org/Raven/internal/model"
)

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
			count, _ := valkey.Incr(c.Request.Context(), key).Result()
			if count == 1 {
				valkey.Expire(c.Request.Context(), key, 60*time.Second) //nolint:errcheck
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
