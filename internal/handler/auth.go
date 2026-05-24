package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// AuthServicer is the interface the auth handler requires from the user service.
type AuthServicer interface {
	GetByExternalID(ctx context.Context, externalID string) (*model.User, error)
	Create(ctx context.Context, externalID, email, displayName string) (*model.User, error)
}

// DemoOrgJoiner is an optional interface that auto-joins new users to the demo
// organisation as viewers. When nil, the auto-join step is skipped.
type DemoOrgJoiner interface {
	// JoinDemoOrg adds the user as a viewer to the first workspace of the demo org.
	// Returns silently on any error (best-effort).
	JoinDemoOrg(ctx context.Context, userID string)
}

// EmailLookup is an optional dependency that fetches the user's email from the
// auth provider's user record by external ID. Used as a fallback when the
// session access-token payload doesn't carry an email — the default for
// SuperTokens' ThirdParty (Google) signin, which doesn't bake claims into the
// JWT. Implementations should return ("", error) on lookup failure so the
// handler can fall back to creating the user with an empty email rather than
// 500-ing the OAuth callback.
type EmailLookup interface {
	LookupEmail(ctx context.Context, externalID string) (string, error)
}

// AuthHandler handles authentication callback endpoints.
type AuthHandler struct {
	svc         AuthServicer
	demoJoin    DemoOrgJoiner
	emailLookup EmailLookup
}

// NewAuthHandler creates a new AuthHandler.
//
// Optional dependencies (zero or one of each):
//   - DemoOrgJoiner: auto-joins new users to the demo org.
//   - EmailLookup:   resolves the user's email from the auth provider's user
//     record when the session claims don't include it (Google/ThirdParty).
//
// Detection is type-based, so callers can pass them in either order.
func NewAuthHandler(svc AuthServicer, opts ...any) *AuthHandler {
	h := &AuthHandler{svc: svc}
	for _, opt := range opts {
		switch v := opt.(type) {
		case DemoOrgJoiner:
			h.demoJoin = v
		case EmailLookup:
			h.emailLookup = v
		}
	}
	return h
}

// Callback handles POST /api/v1/auth/callback.
// Called by the frontend after OIDC redirect callback completes.
// Returns whether the user is new (needs onboarding) or existing.
func (h *AuthHandler) Callback(c *gin.Context) {
	externalID, _ := c.Get(string(middleware.ContextKeyExternalID))
	externalIDStr, _ := externalID.(string)
	if externalIDStr == "" {
		_ = c.Error(apierror.NewUnauthorized("missing external identity"))
		c.Abort()
		return
	}

	email, _ := c.Get(string(middleware.ContextKeyEmail))
	emailStr, _ := email.(string)
	name, _ := c.Get(string(middleware.ContextKeyUserName))
	nameStr, _ := name.(string)

	// SuperTokens' ThirdParty (Google) signin doesn't include email in the
	// access-token payload, so the middleware-populated emailStr is empty for
	// Google logins. Fetch it from the SuperTokens core user record so newly
	// created rows in our `users` table carry the email (used for emailing,
	// audit logs, and the eventual /api/v1/me response).
	if emailStr == "" && h.emailLookup != nil {
		if resolved, err := h.emailLookup.LookupEmail(c.Request.Context(), externalIDStr); err == nil {
			emailStr = resolved
		}
	}

	// Check if user exists
	user, err := h.svc.GetByExternalID(c.Request.Context(), externalIDStr)
	if err != nil {
		// Check if it's a not-found error (service wraps as apierror)
		var appErr *apierror.AppError
		isNotFound := errors.As(err, &appErr) && appErr.Code == http.StatusNotFound
		if !isNotFound {
			_ = c.Error(apierror.NewInternal("failed to look up user: " + err.Error()))
			c.Abort()
			return
		}
		// User not found — first login, create record with nil org_id
		user, err = h.svc.Create(c.Request.Context(), externalIDStr, emailStr, nameStr)
		if err != nil {
			_ = c.Error(apierror.NewInternal("failed to create user: " + err.Error()))
			c.Abort()
			return
		}

		// Auto-join demo org as viewer (best-effort, non-blocking).
		if h.demoJoin != nil {
			h.demoJoin.JoinDemoOrg(c.Request.Context(), user.ID)
		}

		c.JSON(http.StatusOK, gin.H{"isNewUser": true, "userId": user.ID})
		return
	}

	// Existing user
	if user.OrgID == nil {
		// Abandoned onboarding — also try auto-join in case they missed it.
		if h.demoJoin != nil {
			h.demoJoin.JoinDemoOrg(c.Request.Context(), user.ID)
		}
		c.JSON(http.StatusOK, gin.H{"isNewUser": true, "userId": user.ID})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"isNewUser": false,
		"orgId":     *user.OrgID,
		"userId":    user.ID,
	})
}
