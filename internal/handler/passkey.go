// Package handler — passkey.go is the HTTP layer for the passkey
// management endpoints under /api/v1/me/passkeys. All routes assume the
// session middleware has populated ContextKeyUserID + ContextKeyExternalID
// (i.e. they sit under the standard authenticated /api/v1 group).
//
// Ownership model:
//
//   - GET is naturally scoped to the session user (the core list call
//     takes the external user ID, and the label join is by user_id).
//   - PATCH and DELETE prove ownership atomically inside each write,
//     rather than via a pre-flight ListByUser round-trip to the
//     SuperTokens core. PATCH relies on the (credential_id, user_id)
//     WHERE clause of the UPDATE — 0 rows means "not yours" (or
//     "doesn't exist", which is the same 404 from the caller's
//     point of view). DELETE relies on the core's own ownership
//     check inside its DELETE /recipe/webauthn/user/credential
//     handler; a credential_id that does not belong to the calling
//     user surfaces as ErrPasskeyNotFound, which we map to 404.
package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// PasskeyService is the full interface the handler requires. One
// implementation (service.PasskeyService) bundles both the SuperTokens
// core HTTP client and the label repository, and one fake serves both
// sides in tests.
type PasskeyService interface {
	// Core (SuperTokens) side.
	ListByUser(ctx context.Context, externalUserID string) ([]service.PasskeyCredential, error)
	DeleteCredential(ctx context.Context, externalUserID, credentialID string) error

	// Label persistence side.
	ListLabelsForUser(ctx context.Context, userID string) (map[string]service.PasskeyLabel, error)
	UpdateLabel(ctx context.Context, userID, credentialID, label string) error
	DeleteLabel(ctx context.Context, userID, credentialID string) error
}

// PasskeyHandler wires the unified PasskeyService into Gin routes.
type PasskeyHandler struct {
	svc PasskeyService
}

// NewPasskeyHandler constructs a handler backed by the supplied service.
// Production wiring passes *service.PasskeyService (which itself holds
// the core client + label repo); tests pass a hand-written fake.
func NewPasskeyHandler(svc PasskeyService) *PasskeyHandler {
	return &PasskeyHandler{svc: svc}
}

// passkeyResponseItem is one row in the merged GET response.
type passkeyResponseItem struct {
	CredentialID string     `json:"credential_id"`
	Label        string     `json:"label"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

// passkeyPatchRequest is the body of PATCH /api/v1/me/passkeys/:credential_id.
type passkeyPatchRequest struct {
	Label string `json:"label" binding:"required,min=1,max=255"`
}

// List handles GET /api/v1/me/passkeys.
//
// @Summary     List the calling user's passkeys
// @Tags        passkeys
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} passkeyResponseItem
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /me/passkeys [get]
func (h *PasskeyHandler) List(c *gin.Context) {
	userID, externalID, ok := h.requireSession(c)
	if !ok {
		return
	}

	creds, err := h.svc.ListByUser(c.Request.Context(), externalID)
	if err != nil {
		_ = c.Error(apierror.NewInternal("passkey list failed: " + err.Error()))
		c.Abort()
		return
	}

	labels, err := h.svc.ListLabelsForUser(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(apierror.NewInternal("passkey label lookup failed: " + err.Error()))
		c.Abort()
		return
	}

	out := make([]passkeyResponseItem, 0, len(creds))
	for _, cred := range creds {
		// Prefer Raven's label + created_at when present. Fall back to the
		// core's timestamps (and a synthetic "Passkey" label) for newly
		// registered credentials whose label row has not yet been written —
		// this can happen between the core sign-in and the first PATCH from
		// the frontend.
		item := passkeyResponseItem{
			CredentialID: cred.CredentialID,
			Label:        "Passkey",
			CreatedAt:    cred.CreatedAt,
			LastUsedAt:   cred.LastUsedAt,
		}
		if row, found := labels[cred.CredentialID]; found {
			item.Label = row.Label
			if !row.CreatedAt.IsZero() {
				item.CreatedAt = row.CreatedAt
			}
			if row.LastUsedAt != nil {
				item.LastUsedAt = row.LastUsedAt
			}
		}
		out = append(out, item)
	}

	c.JSON(http.StatusOK, out)
}

// Patch handles PATCH /api/v1/me/passkeys/:credential_id.
//
// Ownership is enforced atomically by the UPDATE's WHERE clause: a row
// is only touched when (credential_id, user_id) matches. 0 rows → 404.
// No pre-flight ListByUser call is required.
//
// @Summary     Relabel a passkey owned by the caller
// @Tags        passkeys
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       credential_id path string true "Credential ID"
// @Param       request body passkeyPatchRequest true "New label"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /me/passkeys/{credential_id} [patch]
func (h *PasskeyHandler) Patch(c *gin.Context) {
	userID, _, ok := h.requireSession(c)
	if !ok {
		return
	}
	credentialID := c.Param("credential_id")
	if credentialID == "" {
		_ = c.Error(apierror.NewBadRequest("missing credential_id"))
		c.Abort()
		return
	}

	var req passkeyPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	if err := h.svc.UpdateLabel(c.Request.Context(), userID, credentialID, req.Label); err != nil {
		if errors.Is(err, service.ErrPasskeyLabelNotFound) {
			_ = c.Error(apierror.NewNotFound("passkey not found"))
			c.Abort()
			return
		}
		_ = c.Error(apierror.NewInternal("passkey label update failed: " + err.Error()))
		c.Abort()
		return
	}

	c.Status(http.StatusNoContent)
}

// Delete handles DELETE /api/v1/me/passkeys/:credential_id.
//
// Ownership is enforced by the SuperTokens core's own DELETE handler: a
// credential_id that does not belong to the calling external user is
// reported back as ErrPasskeyNotFound, which we surface as 404. The
// label row delete that follows is best-effort and absorbs missing rows
// at the repository level.
//
// @Summary     Delete a passkey owned by the caller
// @Tags        passkeys
// @Security    BearerAuth
// @Param       credential_id path string true "Credential ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Router      /me/passkeys/{credential_id} [delete]
func (h *PasskeyHandler) Delete(c *gin.Context) {
	userID, externalID, ok := h.requireSession(c)
	if !ok {
		return
	}
	credentialID := c.Param("credential_id")
	if credentialID == "" {
		_ = c.Error(apierror.NewBadRequest("missing credential_id"))
		c.Abort()
		return
	}

	// Core delete first: the core's own ownership check is the
	// authorisation. If the core rejects with "not found" we never touch
	// the label row.
	if err := h.svc.DeleteCredential(c.Request.Context(), externalID, credentialID); err != nil {
		if errors.Is(err, service.ErrPasskeyNotFound) {
			_ = c.Error(apierror.NewNotFound("passkey not found"))
			c.Abort()
			return
		}
		_ = c.Error(apierror.NewInternal("passkey delete failed: " + err.Error()))
		c.Abort()
		return
	}

	// Label-row delete is best-effort. A missing row is harmless (the
	// credential is already gone from the core; the next GET will not
	// see it either way), but a write error after the core succeeded
	// would leave the user looking at a stale label they cannot clear,
	// so we surface those.
	if err := h.svc.DeleteLabel(c.Request.Context(), userID, credentialID); err != nil {
		_ = c.Error(apierror.NewInternal("passkey label cleanup failed: " + err.Error()))
		c.Abort()
		return
	}

	c.Status(http.StatusNoContent)
}

// requireSession pulls both the internal user ID and the SuperTokens
// external ID off the request context. Both are required: external for
// the core API, internal for the label table.
func (h *PasskeyHandler) requireSession(c *gin.Context) (userID, externalID string, ok bool) {
	userIDVal, _ := c.Get(string(middleware.ContextKeyUserID))
	uid, _ := userIDVal.(string)
	extVal, _ := c.Get(string(middleware.ContextKeyExternalID))
	ext, _ := extVal.(string)
	if uid == "" || ext == "" {
		_ = c.Error(apierror.NewUnauthorized("missing user identity"))
		c.Abort()
		return "", "", false
	}
	return uid, ext, true
}
