package handler

import (
	"context"
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

// AuthHandler handles authentication callback endpoints.
type AuthHandler struct {
	svc AuthServicer
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc AuthServicer) *AuthHandler {
	return &AuthHandler{svc: svc}
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
		// New user — create record with nil org_id
		user, err = h.svc.Create(c.Request.Context(), externalIDStr, emailStr, nameStr)
		if err != nil {
			_ = c.Error(apierror.NewInternal("failed to create user: " + err.Error()))
			c.Abort()
			return
		}
		c.JSON(http.StatusOK, gin.H{"isNewUser": true, "userId": user.ID})
		return
	}

	// Existing user
	if user.OrgID == nil {
		// Abandoned onboarding — re-enter
		c.JSON(http.StatusOK, gin.H{"isNewUser": true, "userId": user.ID})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"isNewUser": false,
		"orgId":     *user.OrgID,
		"userId":    user.ID,
	})
}
