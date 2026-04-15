package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// NewSuperTokensProxy returns a Gin handler that reverse-proxies all
// requests to the SuperTokens Core. Register on router.Any("/auth/*path", ...).
//
// The proxy strips the /auth prefix before forwarding, so
// POST /auth/signinup becomes POST /signinup on the SuperTokens Core.
func NewSuperTokensProxy(superTokensURL string) gin.HandlerFunc {
	target, _ := url.Parse(superTokensURL)
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Preserve the original director but override the path
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Strip /auth prefix — SuperTokens Core endpoints don't have it
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/auth")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		req.Host = target.Host
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
