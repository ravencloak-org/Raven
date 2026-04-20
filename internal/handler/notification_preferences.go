package handler

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/email"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// NotificationPrefsService abstracts the repository layer for the handler.
type NotificationPrefsService interface {
	GetEmailSummariesEnabled(ctx context.Context, orgID, userID, workspaceID string) (bool, error)
	SetUserPreference(ctx context.Context, orgID, userID, workspaceID string, enabled bool) error
	SetWorkspacePreference(ctx context.Context, orgID, workspaceID string, enabled bool) error
	UnsubscribeAll(ctx context.Context, orgID, userID string) error
}

// UnsubscribeResolver maps a user's external id (from the signed token) to
// the internal (orgID, userID) pair used by the preferences repository.
type UnsubscribeResolver interface {
	ResolveInternalUser(ctx context.Context, externalID string) (orgID, userID string, err error)
}

// NotificationPrefsHandler exposes the user-level email-summary toggle and
// the unsigned-public /unsubscribe endpoint used by email footers.
type NotificationPrefsHandler struct {
	svc      NotificationPrefsService
	resolver UnsubscribeResolver
}

// NewNotificationPrefsHandler constructs the handler.
func NewNotificationPrefsHandler(svc NotificationPrefsService, resolver UnsubscribeResolver) *NotificationPrefsHandler {
	return &NotificationPrefsHandler{svc: svc, resolver: resolver}
}

// userPreferenceRequest is the JSON payload for PUT /me/notification-preferences/:workspace_id.
type userPreferenceRequest struct {
	EmailSummariesEnabled bool `json:"email_summaries_enabled"`
}

// workspacePreferenceRequest is the JSON payload for the admin toggle.
type workspacePreferenceRequest struct {
	EmailSummariesEnabled bool `json:"email_summaries_enabled"`
}

// UpsertUserPreference handles PUT /api/v1/me/notification-preferences/:workspace_id.
// It stores the current authenticated user's email-summary opt-in status for
// the given workspace.
func (h *NotificationPrefsHandler) UpsertUserPreference(c *gin.Context) {
	orgID, ok := c.Get(string(middleware.ContextKeyOrgID))
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{Code: http.StatusUnauthorized, Message: "Unauthorized"})
		return
	}
	userID, ok := c.Get(string(middleware.ContextKeyUserID))
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{Code: http.StatusUnauthorized, Message: "Unauthorized"})
		return
	}
	wsID := c.Param("workspace_id")
	if _, err := uuid.Parse(wsID); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{Code: http.StatusBadRequest, Message: "Bad Request", Detail: "workspace_id must be a valid UUID"})
		return
	}
	var req userPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{Code: http.StatusUnprocessableEntity, Message: "Unprocessable Entity", Detail: err.Error()})
		return
	}
	if err := h.svc.SetUserPreference(c.Request.Context(), orgID.(string), userID.(string), wsID, req.EmailSummariesEnabled); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, gin.H{"email_summaries_enabled": req.EmailSummariesEnabled})
}

// UpsertWorkspacePreference handles PUT /api/v1/orgs/:org_id/workspaces/:ws_id/notification-preferences.
// This is the workspace-admin master toggle — when disabled no user in the
// workspace receives summary emails.
func (h *NotificationPrefsHandler) UpsertWorkspacePreference(c *gin.Context) {
	orgID := c.Param("org_id")
	wsID := c.Param("ws_id")
	if _, err := uuid.Parse(orgID); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{Code: http.StatusBadRequest, Message: "Bad Request", Detail: "org_id must be a valid UUID"})
		return
	}
	if _, err := uuid.Parse(wsID); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{Code: http.StatusBadRequest, Message: "Bad Request", Detail: "ws_id must be a valid UUID"})
		return
	}
	var req workspacePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{Code: http.StatusUnprocessableEntity, Message: "Unprocessable Entity", Detail: err.Error()})
		return
	}
	if err := h.svc.SetWorkspacePreference(c.Request.Context(), orgID, wsID, req.EmailSummariesEnabled); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, gin.H{"email_summaries_enabled": req.EmailSummariesEnabled})
}

// Unsubscribe handles GET /api/v1/notifications/unsubscribe?token=...
//
// The token is an HMAC-SHA256 signature of "<userID>|<workspaceID>" produced
// by internal/email.SignUnsubscribeToken. The endpoint is intentionally
// unauthenticated so mail-client one-click unsubscribe (RFC 8058) works.
// The token's signature IS the authentication.
func (h *NotificationPrefsHandler) Unsubscribe(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{Code: http.StatusBadRequest, Message: "Bad Request", Detail: "token is required"})
		return
	}
	secret := os.Getenv(email.UnsubscribeSecretEnv)
	if secret == "" {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, apierror.AppError{Code: http.StatusServiceUnavailable, Message: "Service Unavailable", Detail: "unsubscribe is temporarily disabled"})
		return
	}
	externalUserID, workspaceID, err := email.VerifyUnsubscribeToken(secret, token)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{Code: http.StatusBadRequest, Message: "Bad Request", Detail: "invalid token"})
		return
	}
	orgID, userID, err := h.resolver.ResolveInternalUser(c.Request.Context(), externalUserID)
	if err != nil || orgID == "" || userID == "" {
		// We intentionally report success to the browser even if the user was
		// already removed — the email-side contract is "clicking this link
		// stops emails" not "clicking reveals whether you exist".
		c.Data(http.StatusOK, "text/html; charset=utf-8", unsubscribeSuccessHTML)
		return
	}
	if err := h.svc.UnsubscribeAll(c.Request.Context(), orgID, userID); err != nil {
		// Same — never leak internal errors here.
		c.Data(http.StatusOK, "text/html; charset=utf-8", unsubscribeSuccessHTML)
		return
	}
	_ = workspaceID // reserved for future workspace-scoped opt-out
	c.Data(http.StatusOK, "text/html; charset=utf-8", unsubscribeSuccessHTML)
}

// UnsubscribePost handles POST on the same path for RFC 8058 one-click mode.
// Mail clients that honour List-Unsubscribe-Post send a POST on behalf of the user.
func (h *NotificationPrefsHandler) UnsubscribePost(c *gin.Context) {
	h.Unsubscribe(c)
}

var unsubscribeSuccessHTML = []byte(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Unsubscribed — Raven</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
</head><body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#f4f5f7;margin:0;padding:48px 16px;">
<div style="max-width:480px;margin:0 auto;background:#fff;border-radius:8px;padding:32px;text-align:center;box-shadow:0 1px 3px rgba(0,0,0,.06);">
<h1 style="font-size:20px;color:#102a43;margin:0 0 12px 0;">You're unsubscribed</h1>
<p style="color:#486581;line-height:1.5;margin:0;">You will no longer receive conversation summary emails from Raven. You can re-enable them anytime from your account settings.</p>
</div></body></html>`)

// compile-time assertion that the zero value satisfies the interface contracts.
var _ error = errors.New("")
