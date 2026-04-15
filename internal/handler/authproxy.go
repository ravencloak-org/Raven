package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/supertokens/supertokens-golang/supertokens"
)

// SuperTokensMiddleware returns a Gin middleware that delegates all SuperTokens
// auth routes (e.g. /auth/signinup, /auth/session/refresh) to the SuperTokens
// Go SDK. The SDK must be initialised via auth.InitSuperTokens before use.
//
// Register with router.Use(handler.SuperTokensMiddleware()) BEFORE other routes
// so the SDK can intercept /auth/* requests before Gin processes them.
func SuperTokensMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		supertokens.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This inner handler is only called when SuperTokens did NOT handle
			// the request (i.e. the path is not an /auth/* SDK endpoint).
		})).ServeHTTP(c.Writer, c.Request)

		// If the response was already written by the SDK, abort the Gin chain
		// to prevent double-writing.
		if c.Writer.Written() {
			c.Abort()
			return
		}

		c.Next()
	}
}
