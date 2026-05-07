package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Deadline returns a Gin middleware that wraps the request context
// in context.WithTimeout(d). Apply at the route group level so each
// group can have its own budget.
func Deadline(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
