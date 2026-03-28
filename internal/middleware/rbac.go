package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// PoolQuerier abstracts the QueryRow method used by ResolveWorkspaceRole so
// that a *pgxpool.Pool can be passed in production and a lightweight stub in
// tests.
type PoolQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// compile-time check: *pgxpool.Pool satisfies PoolQuerier.
var _ PoolQuerier = (*pgxpool.Pool)(nil)

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

// ---------------------------------------------------------------------------
// ResolveWorkspaceRole — database-backed workspace role resolver
// ---------------------------------------------------------------------------

// ResolveWorkspaceRole returns a Gin middleware that looks up the caller's
// workspace role from the workspace_members table and stores it in the
// context under ContextKeyWorkspaceRole.
//
// It expects the following to already be in the Gin context (set by JWTMiddleware):
//   - ContextKeyUserID  (string)
//   - ContextKeyOrgRole (string)
//
// URL parameter required: :ws_id
//
// Behaviour:
//   - org_admin users bypass the DB lookup entirely; their workspace role is
//     not set (downstream RequireWorkspaceRole already short-circuits for
//     org_admin).
//   - If the user is not a member of the workspace, the request is aborted
//     with 403.
//   - On DB errors the request is aborted with 500.
func ResolveWorkspaceRole(db PoolQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// org_admin bypasses workspace membership check.
		orgRole, _ := c.Get(string(ContextKeyOrgRole))
		if orgRole == "org_admin" {
			c.Next()
			return
		}

		wsID := c.Param("ws_id")
		if wsID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{
				Code:    http.StatusBadRequest,
				Message: "Bad Request",
				Detail:  "missing workspace id in URL",
			})
			return
		}

		userID, _ := c.Get(string(ContextKeyUserID))
		userIDStr, _ := userID.(string)
		if userIDStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{
				Code:    http.StatusUnauthorized,
				Message: "Unauthorized",
				Detail:  "missing user identity",
			})
			return
		}

		var role string
		err := db.QueryRow(
			c.Request.Context(),
			`SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
			wsID, userIDStr,
		).Scan(&role)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.AbortWithStatusJSON(http.StatusForbidden, apierror.AppError{
					Code:    http.StatusForbidden,
					Message: "Forbidden",
					Detail:  "not a member of this workspace",
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, apierror.AppError{
				Code:    http.StatusInternalServerError,
				Message: "Internal Server Error",
				Detail:  "failed to resolve workspace role",
			})
			return
		}

		c.Set(string(ContextKeyWorkspaceRole), role)
		c.Next()
	}
}

// ---------------------------------------------------------------------------
// Convenience wrappers
// ---------------------------------------------------------------------------

// RequireOrgAdmin is a convenience wrapper that requires the org_admin role.
func RequireOrgAdmin() gin.HandlerFunc {
	return RequireOrgRole("org_admin")
}

// RequireWorkspaceOwner is a convenience wrapper that requires at least the
// workspace owner role.
func RequireWorkspaceOwner() gin.HandlerFunc {
	return RequireWorkspaceRole("owner")
}

// RequireWorkspaceAdmin is a convenience wrapper that requires at least the
// workspace admin role.
func RequireWorkspaceAdmin() gin.HandlerFunc {
	return RequireWorkspaceRole("admin")
}

// RequireWorkspaceMember is a convenience wrapper that requires at least the
// workspace member role.
func RequireWorkspaceMember() gin.HandlerFunc {
	return RequireWorkspaceRole("member")
}

// RequireWorkspaceViewer is a convenience wrapper that requires at least the
// workspace viewer role.
func RequireWorkspaceViewer() gin.HandlerFunc {
	return RequireWorkspaceRole("viewer")
}
