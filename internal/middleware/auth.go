package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	// ContextKeyUserID is the context key for the internal database user ID.
	// Set by auth handlers after a DB lookup; not set by JWTMiddleware directly.
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyOrgID is the context key for the organisation ID.
	// Set by auth handlers or org-scoped middleware after a DB lookup.
	ContextKeyOrgID contextKey = "org_id"
	// ContextKeyOrgRole is the context key for the organisation role.
	// Set by org-scoped middleware after a DB lookup.
	ContextKeyOrgRole contextKey = "org_role"
	// ContextKeyWorkspaceRole is the context key for the resolved workspace role,
	// set by workspace-scoped middleware after a membership DB lookup.
	ContextKeyWorkspaceRole contextKey = "workspace_role"
	// ContextKeyEmail is the context key for the user email claim from the JWT.
	ContextKeyEmail contextKey = "email"
	// ContextKeyExternalID is the context key for the Zitadel subject (external user ID).
	// Set by JWTMiddleware from the JWT sub claim.
	ContextKeyExternalID contextKey = "external_id"
	// ContextKeyUserName is the context key for the user's display name from the JWT.
	// Set by JWTMiddleware from the JWT name claim.
	ContextKeyUserName contextKey = "user_name"
	// ContextKeyClaims is the context key for the full parsed Claims struct.
	ContextKeyClaims contextKey = "claims"

	jwksCacheTTL = time.Hour
)

// Claims holds the standard JWT registered claims from Zitadel.
// Org and workspace context is derived from the database after JWT validation,
// not from custom JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Name  string `json:"name"`
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
// A 10-second timeout is applied to the fetch to prevent indefinite blocking.
func (c *jwksCache) refresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	kf, err := keyfunc.NewDefaultCtx(ctx, []string{c.jwksURL})
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
		// Refresh in place; log any error but continue with stale keys.
		if err := c.refresh(); err != nil {
			log.Printf("[WARN] JWT middleware: JWKS refresh from %s failed: %v — continuing with cached keys", c.jwksURL, err)
		}
		c.mu.RLock()
		kf = c.keyfunc
		c.mu.RUnlock()
	}

	return kf.Keyfunc
}

// JWTMiddleware returns a Gin handler that validates Bearer JWTs against the
// Zitadel JWKS endpoint.
//
// The middleware is intended to be applied per route-group, not globally.
//
// On success, the following values are stored in the Gin context:
//
//	ContextKeyExternalID → string (JWT sub — Zitadel user ID)
//	ContextKeyEmail      → string
//	ContextKeyUserName   → string
//	ContextKeyClaims     → *Claims
//
// NOTE: ContextKeyUserID and ContextKeyOrgID are populated by downstream
// auth handlers after a database lookup, not by this middleware.
func JWTMiddleware(cfg *config.ZitadelConfig) gin.HandlerFunc {
	scheme := "https"
	if !cfg.Secure {
		scheme = "http"
	}
	issuerURL := fmt.Sprintf("%s://%s", scheme, cfg.Domain)
	jwksURL := issuerURL + "/oauth/v2/keys"

	const maxAttempts = 3
	var (
		cache *jwksCache
		err   error
	)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		cache, err = newJWKSCache(jwksURL)
		if err == nil {
			break
		}
		log.Printf("[WARN] JWT middleware: JWKS fetch attempt %d/%d from %s failed: %v", attempt, maxAttempts, jwksURL, err)
		if attempt < maxAttempts {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		// All attempts exhausted — serve 503 on every protected request until restart.
		log.Printf("[ERROR] JWT middleware: all %d JWKS fetch attempts failed; all protected endpoints will return 503", maxAttempts)
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, authError{Error: "jwks_unavailable"})
		}
	}

	return func(c *gin.Context) {
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
		claims, err := parseJWT(rawToken, issuerURL, cfg.ClientID, cache, false)
		if err != nil {
			// On any key-related error, retry once with a forced JWKS refresh
			// (handles key rotation / cache staleness).
			if isKeyError(err) {
				claims, err = parseJWT(rawToken, issuerURL, cfg.ClientID, cache, true)
			}
			if err != nil {
				abortWithTokenError(c, err)
				return
			}
		}

		// --- Store claims in Gin context ---
		c.Set(string(ContextKeyExternalID), claims.Subject)
		c.Set(string(ContextKeyEmail), claims.Email)
		c.Set(string(ContextKeyUserName), claims.Name)
		c.Set(string(ContextKeyClaims), claims)

		c.Next()
	}
}

// RequireOrg returns middleware that aborts with 403 if the request context
// does not contain a valid organisation ID. Apply after JWTMiddleware on
// routes that require an onboarded user.
func RequireOrg() gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := c.GetString(string(ContextKeyOrgID))
		if orgID == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "organization required"})
			return
		}
		c.Next()
	}
}

// parseJWT validates the raw token string and returns the parsed Claims.
// It validates the issuer, audience, and expiry claims in addition to the signature.
func parseJWT(rawToken, issuerURL, audience string, cache *jwksCache, forceRefresh bool) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		cache.keyFunc(forceRefresh),
		jwt.WithIssuer(issuerURL),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(5*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// isKeyError reports whether the error is likely caused by an unknown/unresolvable
// key or a stale key (suggesting a JWKS rotation rather than a malformed token).
// ErrTokenSignatureInvalid is included because a key rotation can produce it when
// the cached JWKS is stale.
func isKeyError(err error) bool {
	return strings.Contains(err.Error(), "unable to find") ||
		errors.Is(err, jwt.ErrTokenUnverifiable) ||
		errors.Is(err, jwt.ErrTokenSignatureInvalid)
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
