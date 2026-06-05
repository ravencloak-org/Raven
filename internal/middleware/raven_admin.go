package middleware

import (
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// RAVEN_ADMIN_EMAILS is the comma-separated env var carrying the global
// admin allowlist consulted by RequireRavenAdmin. Empty / unset means
// "no admin endpoints are reachable" — fail-closed by design.
//
// We deliberately do NOT model "raven admin" as a column on the users
// table (a DB column would have to ship with a migration, an RLS policy,
// an API surface to grant/revoke, and a UI — work that has zero MVP
// payoff because the only admins are Raven employees identified by their
// SSO email). The env-allowlist short-circuits all of that for M3 and
// lets #735 ship without coupling to a separate auth-management slice.
//
// When that calculus changes (post-MVP customer admin tiers, e.g. an
// enterprise customer wanting to delegate moderation to one of their
// employees), promote this to a `users.is_raven_admin BOOLEAN` column
// and a DB-backed check — the middleware signature won't change.
const ravenAdminEmailsEnv = "RAVEN_ADMIN_EMAILS"

// adminEmailsCache memoises the parsed allowlist on first lookup. We
// don't watch the env for changes — a restart picks up new emails, which
// is the only mutation path that makes sense for a value that is read on
// every admin request.
var (
	adminEmailsOnce sync.Once
	adminEmails     map[string]struct{}
)

// loadAdminEmails parses RAVEN_ADMIN_EMAILS once and caches the set.
// Emails are normalised to lowercase so the comparison is case-insensitive
// (SSO IdPs are inconsistent about email casing; SuperTokens passes through
// whatever the IdP returns).
func loadAdminEmails() map[string]struct{} {
	adminEmailsOnce.Do(func() {
		raw := os.Getenv(ravenAdminEmailsEnv)
		adminEmails = make(map[string]struct{})
		for _, e := range strings.Split(raw, ",") {
			e = strings.ToLower(strings.TrimSpace(e))
			if e != "" {
				adminEmails[e] = struct{}{}
			}
		}
	})
	return adminEmails
}

// ResetAdminEmailsCacheForTest clears the memoised allowlist. Test-only;
// production code never calls this. Lives here rather than in a _test.go
// file because the middleware tests in this package need it AND the
// integration tests in cmd/api do too.
func ResetAdminEmailsCacheForTest() {
	adminEmailsOnce = sync.Once{}
	adminEmails = nil
}

// RequireRavenAdmin returns a Gin middleware that allows only requests
// whose session email is in the RAVEN_ADMIN_EMAILS allowlist.
//
// Behaviour:
//   - 401 if the session does not carry an email (UserLookup never ran or
//     the SuperTokens session is missing).
//   - 403 if the email is not in the allowlist (or the allowlist is empty —
//     the fail-closed default).
//   - Pass-through otherwise.
//
// The middleware does NOT switch the DB role to raven_admin — that is a
// repository-layer concern (see e.g. internal/jobs/voice_usage.go). The
// HTTP gate here protects the URL surface; the RLS policy on
// marketplace_takedowns is the in-database backstop.
func RequireRavenAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		email, _ := c.Get(string(ContextKeyEmail))
		emailStr, _ := email.(string)
		if emailStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{
				Code:    http.StatusUnauthorized,
				Message: "Unauthorized",
				Detail:  "missing session identity",
			})
			return
		}
		allow := loadAdminEmails()
		if _, ok := allow[strings.ToLower(strings.TrimSpace(emailStr))]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, apierror.AppError{
				Code:    http.StatusForbidden,
				Message: "Forbidden",
				Detail:  "raven admin required",
			})
			return
		}
		c.Next()
	}
}
