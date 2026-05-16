package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// SubscriptionStatusChecker is the subset of the billing service the middleware requires.
type SubscriptionStatusChecker interface {
	GetActiveSubscription(ctx context.Context, orgID string) (*model.Subscription, error)
}

// RequireActiveSubscription returns a Gin middleware that rejects requests with HTTP 402
// when the organisation's subscription has status "expired".
//
// Organisations on the free plan (no subscription record) are allowed through.
// This middleware must be placed after the auth middleware so that the org_id
// context key is already populated.
func RequireActiveSubscription(svc SubscriptionStatusChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID, ok := c.Get(string(ContextKeyOrgID))
		if !ok || orgID == "" {
			// No org context — let other middleware handle auth failures.
			c.Next()
			return
		}

		sub, err := svc.GetActiveSubscription(c.Request.Context(), orgID.(string))
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "subscription status check failed",
				"org_id", orgID,
				"error", err,
			)
			// Fail open: do not block the request on a transient DB error.
			c.Next()
			return
		}

		if sub != nil && sub.Status == model.SubscriptionStatusExpired {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, apierror.AppError{
				Code:    http.StatusPaymentRequired,
				Message: "Subscription expired",
			})
			return
		}

		c.Next()
	}
}
