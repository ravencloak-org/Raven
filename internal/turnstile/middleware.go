package turnstile

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Required returns a Gin middleware that 403s any request missing or
// failing a Cloudflare Turnstile token.
//
// The token is read from the ``cf-turnstile-token`` request header — the
// frontend Turnstile widget surfaces the token via its ``callback`` and
// the signup form is responsible for forwarding it.
//
// When ``v.SecretKey`` is empty (dev / single-user runs), the middleware
// is a pass-through so the local workflow isn't broken by accidental
// configuration drift.
//
// When ``bypassSecret`` is non-empty and the request carries a matching
// ``X-Demo-Bypass`` header, verification is skipped. CI uses this for
// the Playwright E2E suite.
func Required(v *Verifier, bypassSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if v.SecretKey == "" {
			c.Next()
			return
		}
		if bypassSecret != "" && c.GetHeader("X-Demo-Bypass") == bypassSecret {
			c.Next()
			return
		}
		token := c.GetHeader("cf-turnstile-token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusForbidden,
				gin.H{"error": "missing turnstile token"})
			return
		}
		ok, err := v.Verify(c.Request.Context(), token, c.ClientIP())
		if err != nil || !ok {
			c.AbortWithStatusJSON(http.StatusForbidden,
				gin.H{"error": "turnstile verification failed"})
			return
		}
		c.Next()
	}
}

// SignupOnly returns a Gin middleware that gates only the signup paths
// inside SuperTokens' ``/auth/*path`` catch-all. Existing-account sign-in,
// session refresh, password reset, etc. are unaffected.
//
// SuperTokens dispatches signup through ``/auth/signinup`` (and its
// recipe-specific variants like ``/auth/signinup/google``). This
// middleware matches any POST whose path begins with ``/auth/signinup``.
func SignupOnly(v *Verifier, bypassSecret string) gin.HandlerFunc {
	required := Required(v, bypassSecret)
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, "/auth/signinup") {
			c.Next()
			return
		}
		required(c)
	}
}
