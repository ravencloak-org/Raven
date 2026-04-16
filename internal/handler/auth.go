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

// AuthHandler handles authentication callback endpoints.
type AuthHandler struct {
	svc      AuthServicer
	demoJoin DemoOrgJoiner
}

// NewAuthHandler creates a new AuthHandler.
// Pass optional DemoOrgJoiner instances (at most one) to enable auto-join on signup.
func NewAuthHandler(svc AuthServicer, opts ...DemoOrgJoiner) *AuthHandler {
	h := &AuthHandler{svc: svc}
	if len(opts) > 0 {
		h.demoJoin = opts[0]
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
