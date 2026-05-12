package middleware

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
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
				slog.Warn("supertokens.GetAllCORSHeaders() panicked, using static fallback", "recovered", r)
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

// APIKeyOriginLookup is the narrow interface the CORS middleware needs to
// resolve a raw widget API key into its allowed-domain list. The repository
// layer satisfies this via APIKeyLookup; we keep a distinct interface so the
// CORS middleware never imports the model or repository packages.
//
// LookupByHash MUST return (nil, nil) when the key is unknown or revoked so
// the CORS middleware can fall back cleanly to the server allowlist; it should
// only return an error for infrastructure-level failures (DB unreachable etc.)
// which will be treated as "deny" to fail closed.
type APIKeyOriginLookup interface {
	LookupByHash(ctx context.Context, keyHash string) (*APIKeyLookupResult, error)
}

// CORSMiddleware returns a Gin handler that applies CORS policy based on the
// provided CORSConfig. Origins are validated against cfg.AllowedOrigins.
//
// When a non-nil keyLookup is supplied, the middleware also honours the
// per-API-key allowed_domains allowlist stored against each public widget
// key. The match rules — applied inside AllowOriginWithContextFunc so a
// single source of truth covers both preflight and actual requests — are:
//
//   - No X-API-Key header on the request  -> fall back to the server-wide
//     RAVEN_CORS_ALLOWED_ORIGINS allowlist only (legacy behaviour).
//   - X-API-Key present and the key has a non-empty allowed_domains -> the
//     Origin MUST match one of those domains. The server allowlist does NOT
//     widen this per-key set; a widget key bound to example.com is unusable
//     from any other origin even if the server allowlist is broader.
//   - X-API-Key present but allowed_domains is empty -> treat as "no per-key
//     restriction" and fall back to the server allowlist.
//
// Domain matching is host-based and supports a single wildcard form,
// `*.example.com`, which matches any direct or deeper sub-domain (e.g.
// `sub.example.com`, `a.b.example.com`) plus the apex `example.com`. This
// mirrors the rule already enforced by APIKeyAuth so per-key allowlists
// behave identically at preflight and at request time.
//
// We deliberately look up the key inside the validator rather than reordering
// middleware: CORS preflights MUST run before any auth middleware (browsers
// strip the X-API-Key header from the OPTIONS probe via the Access-Control-
// Request-Headers list, so APIKeyAuth cannot speak for them), and an actual
// cross-origin POST still needs an Allow-Origin verdict before the route
// handler runs. Keeping CORS as the global gate and pushing the lookup down
// into AllowOriginWithContextFunc is the smaller change.
func CORSMiddleware(cfg *config.CORSConfig, keyLookup ...APIKeyOriginLookup) gin.HandlerFunc {
	if cfg == nil {
		cfg = &config.CORSConfig{}
	}

	var lookup APIKeyOriginLookup
	if len(keyLookup) > 0 {
		lookup = keyLookup[0]
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
		MaxAge:           1 * time.Hour,
		// AllowOriginWithContextFunc takes precedence over AllowOrigins/
		// AllowOriginFunc when set. It checks the request origin against the
		// configured server allow-list AND (when an API key is presented on
		// the request) the per-key allowed_domains stored against the widget
		// API key. See the doc-comment on CORSMiddleware for the exact rules.
		AllowOriginWithContextFunc: func(c *gin.Context, origin string) bool {
			return originAllowed(c, origin, allowedSet, lookup)
		},
	}

	return cors.New(corsConfig)
}

// originAllowed implements the per-key + server-wide CORS verdict. It is
// pulled out of the closure to keep it directly unit-testable.
func originAllowed(c *gin.Context, origin string, serverAllowed map[string]struct{}, lookup APIKeyOriginLookup) bool {
	// The cors package only invokes the validator when an Origin header is
	// present, so we don't need to special-case missing origins here.

	rawKey := ""
	if c != nil && c.Request != nil {
		rawKey = c.Request.Header.Get("X-API-Key")
	}

	if rawKey != "" && lookup != nil {
		// Best-effort lookup; a DB error here fails closed.
		res, err := lookup.LookupByHash(c.Request.Context(), hashAPIKey(rawKey))
		if err != nil {
			slog.Warn("CORS api key lookup failed; denying origin", "err", err)
			return false
		}
		// res == nil means "key not found / revoked"; treat as no per-key
		// restriction and fall through to the server allowlist. The auth
		// middleware will reject the request shortly anyway.
		if res != nil && len(res.AllowedDomains) > 0 {
			return isOriginAllowedByDomains(origin, res.AllowedDomains)
		}
	}

	_, ok := serverAllowed[origin]
	return ok
}

// isOriginAllowedByDomains reports whether the given Origin header value
// matches any entry in the per-key allowed_domains list. Matching is
// host-based; entries beginning with `*.` are treated as sub-domain wildcards
// and also match the apex. Mirrors APIKeyAuth's isDomainAllowed semantics.
func isOriginAllowedByDomains(origin string, allowed []string) bool {
	host := originHost(origin)
	if host == "" {
		return false
	}
	for _, d := range allowed {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if strings.HasPrefix(d, "*.") {
			suffix := d[1:] // ".example.com"
			apex := d[2:]   // "example.com"
			if strings.EqualFold(host, apex) || strings.HasSuffix(strings.ToLower(host), strings.ToLower(suffix)) {
				return true
			}
			continue
		}
		if strings.EqualFold(host, d) {
			return true
		}
	}
	return false
}

// originHost extracts the host portion from an Origin header value. The
// Origin header is always a URL like "https://example.com[:port]"; if parsing
// fails we conservatively treat it as a bare host.
func originHost(origin string) string {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return strings.TrimSpace(origin)
	}
	return u.Hostname()
}
