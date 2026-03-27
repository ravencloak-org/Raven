package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// workspaceRoleRank maps workspace role names to their permission level.
// Higher index means more permissions; an org_admin bypasses all workspace checks.
var workspaceRoleRank = map[string]int{
	"viewer": 0,
	"member": 1,
	"admin":  2,
	"owner":  3,
}

// RequireOrgRole returns a Gin middleware that allows only requests where the
// caller holds the given org-level role (or is org_admin, which bypasses all
// org-role checks). Returns 403 when the requirement is not met.
func RequireOrgRole(required string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(string(ContextKeyOrgRole))
		roleStr, _ := role.(string)
		if roleStr == "org_admin" || roleStr == required {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, apierror.AppError{
			Code:    http.StatusForbidden,
			Message: "Forbidden",
			Detail:  "requires org role: " + required,
		})
	}
}

// RequireWorkspaceRole returns a Gin middleware that enforces a minimum
// workspace role. org_admin always bypasses workspace role checks.
// Returns 403 when the requirement is not met.
func RequireWorkspaceRole(minimum string) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgRole, _ := c.Get(string(ContextKeyOrgRole))
		if orgRole == "org_admin" {
			c.Next()
			return
		}
		wsRole, _ := c.Get(string(ContextKeyWorkspaceRole))
		wsRoleStr, _ := wsRole.(string)
		if workspaceRoleRank[wsRoleStr] >= workspaceRoleRank[minimum] {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, apierror.AppError{
			Code:    http.StatusForbidden,
			Message: "Forbidden",
			Detail:  "requires workspace role: " + minimum,
		})
	}
}
