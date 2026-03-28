package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Context keys for API-key-authenticated requests.
const (
	ContextKeyAPIKeyID contextKey = "api_key_id"
	ContextKeyKBID     contextKey = "kb_id"
)

// ApiKeyLookupResult holds the fields the middleware needs after looking up
// a hashed API key. This is deliberately a plain struct rather than the full
// model.ApiKey so the middleware has no import dependency on the model package.
type ApiKeyLookupResult struct {
	ID              string
	OrgID           string
	WorkspaceID     string
	KnowledgeBaseID string
	AllowedDomains  []string
	RateLimit       int
	Status          string
}

// ApiKeyLookup is the interface the API key middleware requires for looking
// up keys by hash. The repository layer implements this.
type ApiKeyLookup interface {
	LookupByHash(ctx context.Context, keyHash string) (*ApiKeyLookupResult, error)
}

// hashAPIKey returns the lowercase hex SHA-256 digest of the raw API key.
func hashAPIKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

// ApiKeyAuth returns a Gin middleware that authenticates requests via the
// X-API-Key header. It hashes the raw key with SHA-256, looks it up in the
// database, validates the domain allowlist against the Origin/Referer header,
// and sets org/workspace/kb context keys for downstream handlers.
//
// Usage:
//
//	router.Use(middleware.ApiKeyAuth(repo))
func ApiKeyAuth(lookup ApiKeyLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawKey := c.GetHeader("X-API-Key")
		if rawKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing_api_key"})
			return
		}

		keyHash := hashAPIKey(rawKey)

		result, err := lookup.LookupByHash(c.Request.Context(), keyHash)
		if err != nil || result == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_api_key"})
			return
		}

		if result.Status != "active" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "api_key_revoked"})
			return
		}

		// Domain allowlist validation.
		if len(result.AllowedDomains) > 0 {
			origin := c.GetHeader("Origin")
			if origin == "" {
				origin = c.GetHeader("Referer")
			}
			if !isDomainAllowed(origin, result.AllowedDomains) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "domain_not_allowed"})
				return
			}
		}

		// Store identity in context for downstream handlers.
		c.Set(string(ContextKeyOrgID), result.OrgID)
		c.Set(string(ContextKeyAPIKeyID), result.ID)
		c.Set(string(ContextKeyKBID), result.KnowledgeBaseID)
		c.Set(ContextKeyAPIKey, rawKey)

		// Store workspace ID if available.
		if result.WorkspaceID != "" {
			c.Set("workspace_id", result.WorkspaceID)
		}

		c.Next()
	}
}

// isDomainAllowed checks whether the given origin/referer matches any entry
// in the allowlist. Each entry in allowed is compared against the hostname
// extracted from the origin URL. An empty origin is rejected when the
// allowlist is non-empty.
func isDomainAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}

	host := extractHost(origin)
	if host == "" {
		return false
	}

	for _, d := range allowed {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		// Wildcard match: *.example.com matches sub.example.com
		if strings.HasPrefix(d, "*.") {
			suffix := d[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) || host == d[2:] {
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

// extractHost returns the hostname from a URL string. If parsing fails it
// attempts to use the raw value as a plain hostname.
func extractHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// Might be a bare hostname (no scheme).
		return strings.TrimSpace(raw)
	}
	// Strip port if present.
	h := u.Hostname()
	return h
}
