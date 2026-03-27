package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/ravencloak-org/Raven/internal/config"
)

// contextKey is a private type for context keys to avoid collisions with other packages.
type contextKey string

const (
	// ContextKeyUserID is the context key for the JWT subject (user ID).
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyOrgID is the context key for the organisation ID claim.
	ContextKeyOrgID contextKey = "org_id"
	// ContextKeyOrgRole is the context key for the organisation role claim.
	ContextKeyOrgRole contextKey = "org_role"
	// ContextKeyWorkspaceIDs is the context key for the workspace IDs claim.
	ContextKeyWorkspaceIDs contextKey = "workspace_ids"
	// ContextKeyKBPermissions is the context key for the knowledge-base permissions claim.
	ContextKeyKBPermissions contextKey = "kb_permissions"
	// ContextKeyEmail is the context key for the user email claim.
	ContextKeyEmail contextKey = "email"
	// ContextKeyClaims is the context key for the full parsed Claims struct.
	ContextKeyClaims contextKey = "claims"

	jwksCacheTTL = time.Hour
)

// Claims holds the standard JWT registered claims plus custom Raven/Keycloak claims.
type Claims struct {
	jwt.RegisteredClaims

	// Custom Keycloak / Raven claims.
	OrgID          string   `json:"org_id"`
	OrgRole        string   `json:"org_role"`
	WorkspaceIDs   []string `json:"workspace_ids"`
	KBPermissions  []string `json:"kb_permissions"`
	Email          string   `json:"email"`
}

// authError represents a structured 401 response body.
type authError struct {
	Error string `json:"error"`
}

// jwksCache wraps a keyfunc.Keyfunc with TTL-based refresh logic.
type jwksCache struct {
	mu          sync.RWMutex
	keyfunc     keyfunc.Keyfunc
	lastRefresh time.Time
	jwksURL     string
}

// newJWKSCache creates a new cache and performs an initial fetch.
func newJWKSCache(jwksURL string) (*jwksCache, error) {
	c := &jwksCache{jwksURL: jwksURL}
	if err := c.refresh(); err != nil {
		return nil, fmt.Errorf("initial JWKS fetch failed: %w", err)
	}
	return c, nil
}

// refresh fetches the JWKS from the remote endpoint and replaces the cached keyfunc.
func (c *jwksCache) refresh() error {
	kf, err := keyfunc.NewDefaultCtx(context.Background(), []string{c.jwksURL})
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.keyfunc = kf
	c.lastRefresh = time.Now()
	c.mu.Unlock()
	return nil
}

// keyFunc returns the jwt.Keyfunc, refreshing the cache if TTL has elapsed or
// if forceRefresh is true.
func (c *jwksCache) keyFunc(forceRefresh bool) jwt.Keyfunc {
	c.mu.RLock()
	expired := time.Since(c.lastRefresh) > jwksCacheTTL
	kf := c.keyfunc
	c.mu.RUnlock()

	if forceRefresh || expired {
		// Refresh in place; ignore error — continue with stale keys.
		_ = c.refresh()
		c.mu.RLock()
		kf = c.keyfunc
		c.mu.RUnlock()
	}

	return kf.Keyfunc
}

// JWTMiddleware returns a Gin handler that validates Bearer JWTs against the
// Keycloak JWKS endpoint, or stubs through API-key requests.
//
// The middleware is intended to be applied per route-group, not globally.
//
// On success, the following values are stored in the Gin context:
//
//	ContextKeyUserID        → string (JWT sub)
//	ContextKeyOrgID         → string
//	ContextKeyOrgRole       → string
//	ContextKeyWorkspaceIDs  → []string
//	ContextKeyKBPermissions → []string
//	ContextKeyEmail         → string
//	ContextKeyClaims        → *Claims
//
// NOTE: RLS enforcement (`SET LOCAL app.current_org_id`) must be applied by
// the repository layer after retrieving ContextKeyOrgID from the context,
// because the DB connection is not accessible here.
func JWTMiddleware(cfg *config.KeycloakConfig) gin.HandlerFunc {
	jwksURL := cfg.IssuerURL + "/protocol/openid-connect/certs"

	cache, err := newJWKSCache(jwksURL)
	if err != nil {
		// If JWKS is unavailable at startup, log a warning and serve 503 on
		// every protected request until the cache can be populated.
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, authError{Error: "jwks_unavailable"})
		}
	}

	return func(c *gin.Context) {
		// --- Detect auth method ---
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			handleAPIKey(c, apiKey)
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortUnauthorized(c, "missing_token")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			abortUnauthorized(c, "missing_token")
			return
		}
		rawToken := parts[1]

		// --- Parse & validate JWT ---
		claims, err := parseJWT(rawToken, cfg.IssuerURL, cache, false)
		if err != nil {
			// On any key-related error, retry once with a forced JWKS refresh
			// (handles key rotation / cache staleness).
			if isKeyError(err) {
				claims, err = parseJWT(rawToken, cfg.IssuerURL, cache, true)
			}
			if err != nil {
				abortWithTokenError(c, err)
				return
			}
		}

		// --- Store claims in Gin context ---
		c.Set(string(ContextKeyUserID), claims.Subject)
		c.Set(string(ContextKeyOrgID), claims.OrgID)
		c.Set(string(ContextKeyOrgRole), claims.OrgRole)
		c.Set(string(ContextKeyWorkspaceIDs), claims.WorkspaceIDs)
		c.Set(string(ContextKeyKBPermissions), claims.KBPermissions)
		c.Set(string(ContextKeyEmail), claims.Email)
		c.Set(string(ContextKeyClaims), claims)

		c.Next()
	}
}

// parseJWT validates the raw token string and returns the parsed Claims.
func parseJWT(rawToken, issuerURL string, cache *jwksCache, forceRefresh bool) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		cache.keyFunc(forceRefresh),
		jwt.WithIssuer(issuerURL),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(5*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// isKeyError reports whether the error is likely caused by an unknown/unresolvable
// key (suggesting a JWKS rotation rather than a malformed token).
func isKeyError(err error) bool {
	return strings.Contains(err.Error(), "unable to find") ||
		strings.Contains(err.Error(), "key") ||
		errors.Is(err, jwt.ErrTokenUnverifiable)
}

// abortWithTokenError maps jwt parse errors to structured 401 responses.
func abortWithTokenError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		abortUnauthorized(c, "token_expired")
	default:
		abortUnauthorized(c, "invalid_token")
	}
}

// abortUnauthorized aborts the request with a structured 401 JSON body.
func abortUnauthorized(c *gin.Context, code string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, authError{Error: code})
}

// handleAPIKey stubs the API-key authentication path.
// The lookup against the database is not yet implemented; a placeholder
// Claims struct is stored in the context so downstream handlers can read it.
//
// TODO(issue-24): replace stub with real DB lookup and key hashing.
func handleAPIKey(c *gin.Context, _ string) {
	stub := &Claims{}
	stub.Subject = "api-key-subject-placeholder"

	c.Set(string(ContextKeyUserID), stub.Subject)
	c.Set(string(ContextKeyOrgID), stub.OrgID)
	c.Set(string(ContextKeyOrgRole), stub.OrgRole)
	c.Set(string(ContextKeyWorkspaceIDs), stub.WorkspaceIDs)
	c.Set(string(ContextKeyKBPermissions), stub.KBPermissions)
	c.Set(string(ContextKeyEmail), stub.Email)
	c.Set(string(ContextKeyClaims), stub)

	c.Next()
}
