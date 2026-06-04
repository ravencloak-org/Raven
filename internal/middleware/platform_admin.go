package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ContextKeyIsPlatformAdmin is the context key that the platform-admin
// middleware sets to true when the caller passes the allow-list check.
// Downstream handlers can branch on it without re-reading the env var.
const ContextKeyIsPlatformAdmin contextKey = "is_platform_admin"

// RequirePlatformAdmin returns a Gin handler that aborts with 403 unless
// the caller's session email is in adminEmails. Returns 401 when no
// session email is in context.
//
// ADR-0008 §line 35 mandates `raven_admin` as the only admin tier — no
// granular admin roles in MVP. The allow-list is config-driven
// (RAVEN_ADMIN_EMAILS) rather than DB-backed so revoking admin is a
// config push, not a SQL UPDATE that might race a long-lived session.
//
// Case-insensitive comparison: email addresses are case-insensitive in
// the local-part per RFC 5321 §2.3.11 in practice; matching strictly
// would surprise an operator whose IdP normalises to lower-case.
//
// Empty adminEmails = nobody is admin (every request gets 403). That
// is the safe default for environments where the env var has not been
// set.
func RequirePlatformAdmin(adminEmails []string) gin.HandlerFunc {
	allow := make(map[string]struct{}, len(adminEmails))
	for _, e := range adminEmails {
		trimmed := strings.TrimSpace(strings.ToLower(e))
		if trimmed != "" {
			allow[trimmed] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		emailVal, _ := c.Get(string(ContextKeyEmail))
		email, _ := emailVal.(string)
		if email == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{
				Code:    http.StatusUnauthorized,
				Message: "Unauthorized",
				Detail:  "session required",
			})
			return
		}
		if _, ok := allow[strings.ToLower(email)]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, apierror.AppError{
				Code:    http.StatusForbidden,
				Message: "Forbidden",
				Detail:  "platform admin required",
			})
			return
		}
		c.Set(string(ContextKeyIsPlatformAdmin), true)
		c.Next()
	}
}

// IsPlatformAdmin returns true when the caller's session email is in
// adminEmails. Exported so non-middleware code paths (e.g. the GetMe
// handler reporting `is_platform_admin` in the response so the frontend
// can hide the admin nav) can run the same check without duplicating
// the normalisation rules.
func IsPlatformAdmin(email string, adminEmails []string) bool {
	if email == "" {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(email))
	for _, e := range adminEmails {
		if strings.ToLower(strings.TrimSpace(e)) == target {
			return true
		}
	}
	return false
}
