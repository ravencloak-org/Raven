package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/config"
)

// CORSMiddleware returns a Gin handler that applies CORS policy based on the
// provided CORSConfig. Origins are validated against cfg.AllowedOrigins.
// AllowOriginFunc mirrors the same list so that pre-flight and actual requests
// both use one consistent source of truth; the comment block marks the place
// where per-key allowed_domains will be wired in during a later milestone.
func CORSMiddleware(cfg *config.CORSConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = &config.CORSConfig{}
	}

	allowedSet := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowedSet[o] = struct{}{}
	}

	corsConfig := cors.Config{
		AllowMethods: []string{
			"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH",
		},
		AllowHeaders: []string{
			"Authorization",
			"Content-Type",
			"X-API-Key",
			"X-Request-ID",
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
