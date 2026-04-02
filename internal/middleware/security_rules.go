package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SecurityRuleAction is the result of evaluating security rules against a request.
type SecurityRuleAction struct {
	Block    bool
	Throttle bool
	RuleID   string
	RuleName string
}

// SecurityEvaluator is the interface the WAF middleware requires for evaluating
// requests against security rules. The service layer implements this.
type SecurityEvaluator interface {
	EvaluateRequest(ctx context.Context, orgID, clientIP, path, method, userAgent string) (*SecurityRuleAction, error)
}

// SecurityRulesMiddleware returns a Gin middleware that evaluates incoming
// requests against the organisation's security rules (IP allow/deny lists,
// pattern matching, etc.).
//
// The middleware is fail-open: if rule evaluation errors out, the request is
// allowed through to avoid breaking the endpoint.
func SecurityRulesMiddleware(evaluator SecurityEvaluator) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgIDVal, exists := c.Get(string(ContextKeyOrgID))
		if !exists {
			// No org context — skip security evaluation (public endpoints).
			c.Next()
			return
		}
		orgID, ok := orgIDVal.(string)
		if !ok || orgID == "" {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		action, err := evaluator.EvaluateRequest(
			c.Request.Context(),
			orgID,
			clientIP,
			c.Request.URL.Path,
			c.Request.Method,
			c.GetHeader("User-Agent"),
		)
		if err != nil {
			// Fail open — log error but don't block the request.
			slog.WarnContext(c.Request.Context(), "security rules: evaluation failed, allowing request",
				slog.String("org_id", orgID),
				slog.String("error", err.Error()),
			)
			c.Next()
			return
		}

		if action != nil && action.Block {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "request blocked by security policy",
			})
			return
		}

		c.Next()
	}
}
