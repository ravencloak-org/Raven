package middleware

import (
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/supertokens/supertokens-golang/supertokens"

	"github.com/ravencloak-org/Raven/internal/config"
)

// stCORSHeadersOnce caches the result of supertokens.GetAllCORSHeaders() so
// we only call it once per process and do not pay the overhead on every CORS
// middleware construction.
var (
	stCORSOnce    sync.Once
	stCORSHeaders []string
)

// getSuperTokensCORSHeaders returns the headers required by the SuperTokens Go
// SDK. It is computed lazily on first call; if the SDK is not yet initialised
// (e.g. in unit tests that construct the CORS middleware without a running
// Core) it falls back to the well-known static set so tests remain stable.
func getSuperTokensCORSHeaders() []string {
	stCORSOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				// SDK not initialised (e.g. unit test context). Fall back to
				// the well-known static set that SuperTokens always requires.
				stCORSHeaders = []string{
					"anti-csrf",
					"st-auth-mode",
					"rid",
					"fdi-version",
				}
			}
		}()
		stCORSHeaders = supertokens.GetAllCORSHeaders()
	})
	return stCORSHeaders
}

// CORSMiddleware returns a Gin handler that applies CORS policy based on the
// provided CORSConfig. Origins are validated against cfg.AllowedOrigins.
// AllowOriginFunc mirrors the same list so that pre-flight and actual requests
// both use one consistent source of truth; the comment block marks the place
// where per-key allowed_domains will be wired in during a later milestone.
func CORSMiddleware(cfg *config.CORSConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = &config.CORSConfig{}
	}

	allowedSet := lo.SliceToMap(cfg.AllowedOrigins, func(o string) (string, struct{}) {
		return o, struct{}{}
	})

	// Merge the SDK-required headers with our application-specific headers so
	// that both sets are always in sync with the SuperTokens version in use.
	appHeaders := []string{
		"Authorization",
		"Content-Type",
		"X-API-Key",
		"X-Request-ID",
	}
	allowHeaders := lo.Uniq(append(appHeaders, getSuperTokensCORSHeaders()...))

	corsConfig := cors.Config{
		AllowMethods: []string{
			"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH",
		},
		AllowHeaders: allowHeaders,
		ExposeHeaders: []string{
			"st-access-token",
			"st-refresh-token",
			"anti-csrf",
			"front-token",
		},
		AllowCredentials: true,
		MaxAge:           time.Duration(3600) * time.Second,
		// AllowOriginFunc takes precedence over AllowOrigins when set.
		// It checks the request origin against the configured allow-list.
		// TODO(M2): also check api_keys.allowed_domains from the database.
		AllowOriginFunc: func(origin string) bool {
			_, ok := allowedSet[origin]
			return ok
		},
	}

	return cors.New(corsConfig)
}
