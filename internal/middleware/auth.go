package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/auth"
)

// contextKey is a private type for context keys to avoid collisions with other packages.
type contextKey string

const (
	// ContextKeyUserID is the context key for the internal database user ID.
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyOrgID is the context key for the organisation ID.
	ContextKeyOrgID contextKey = "org_id"
	// ContextKeyOrgRole is the context key for the organisation role.
	ContextKeyOrgRole contextKey = "org_role"
	// ContextKeyWorkspaceRole is the context key for the resolved workspace role.
	ContextKeyWorkspaceRole contextKey = "workspace_role"
	// ContextKeyEmail is the context key for the user email from the session.
	ContextKeyEmail contextKey = "email"
	// ContextKeyExternalID is the context key for the provider-specific user ID.
	ContextKeyExternalID contextKey = "external_id"
	// ContextKeyUserName is the context key for the user's display name.
	ContextKeyUserName contextKey = "user_name"
	// ContextKeyClaims is the context key for the full parsed claims.
	ContextKeyClaims contextKey = "claims"
)

// authError represents a structured 401 response body.
type authError struct {
	Error string `json:"error"`
}

// SessionMiddleware returns a Gin handler that verifies the session using
// the provided AuthProvider. On success, it stores identity data in the
// Gin context using the same context keys as the old JWTMiddleware.
func SessionMiddleware(provider auth.Provider) gin.HandlerFunc {
	return func(c *gin.Context) {
		info, err := provider.VerifySession(c.Request)
		if err != nil || info == nil {
			abortUnauthorized(c, "invalid_session")
			return
		}

		c.Set(string(ContextKeyExternalID), info.ExternalID)
		c.Set(string(ContextKeyEmail), info.Email)
		c.Set(string(ContextKeyUserName), info.Name)

		c.Next()
	}
}

// UserResolver is the interface for looking up users by external ID.
// Returns empty userID when the user is not found (not an error).
type UserResolver interface {
	GetByExternalID(ctx context.Context, externalID string) (userID string, orgID *string, err error)
}

// UserLookup returns middleware that resolves the session external ID to internal
// user and org IDs via a database lookup. Apply after SessionMiddleware on routes
// that need ContextKeyUserID or ContextKeyOrgID.
//
// If the user is not found (first login), the middleware continues without
// setting these keys — the /auth/callback handler handles user creation.
// Real DB errors abort with 503 to avoid masking infra failures.
func UserLookup(resolver UserResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		externalID := c.GetString(string(ContextKeyExternalID))
		if externalID == "" {
			c.Next()
			return
		}
		userID, orgID, err := resolver.GetByExternalID(c.Request.Context(), externalID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "user_lookup_failed"})
			return
		}
		if userID == "" {
			c.Next()
			return
		}
		c.Set(string(ContextKeyUserID), userID)
		if orgID != nil {
			c.Set(string(ContextKeyOrgID), *orgID)
		}
		c.Next()
	}
}

// RequireOrg returns middleware that aborts with 403 if the request context
// does not contain a valid organisation ID.
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

// abortUnauthorized aborts the request with a structured 401 JSON body.
func abortUnauthorized(c *gin.Context, code string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, authError{Error: code})
}
