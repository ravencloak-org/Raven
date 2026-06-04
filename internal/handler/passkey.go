// Package handler — passkey.go is the HTTP layer for the passkey
// management endpoints under /api/v1/me/passkeys. All routes assume the
// session middleware has populated ContextKeyUserID + ContextKeyExternalID
// (i.e. they sit under the standard authenticated /api/v1 group).
package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// PasskeyServicer is the subset of *service.PasskeyService the handler
// needs. Defined as an interface so handler tests can inject a stub
// without touching the live SuperTokens core.
type PasskeyServicer interface {
	ListByUser(ctx context.Context, externalUserID string) ([]service.PasskeyCredential, error)
	DeleteCredential(ctx context.Context, externalUserID, credentialID string) error
}

// PasskeyLabelStore is the subset of *pgxpool.Pool the handler exercises.
// Allowing nil + a labelStoreOverride field on the handler keeps tests free
// of the pool entirely — they can simulate the DB by injecting a fake.
type PasskeyLabelStore interface {
	ListLabelsForUser(ctx context.Context, userID string) (map[string]passkeyLabelRow, error)
	UpsertLabel(ctx context.Context, userID, credentialID, label string) error
	DeleteLabel(ctx context.Context, userID, credentialID string) error
}

// passkeyLabelRow is the projected shape we need from
// user_passkey_labels for the GET join.
type passkeyLabelRow struct {
	Label      string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// PasskeyHandler wires the SuperTokens core (via PasskeyServicer) to the
// local user_passkey_labels table.
type PasskeyHandler struct {
	svc   PasskeyServicer
	store PasskeyLabelStore
}

// NewPasskeyHandler constructs a handler backed by the supplied service and
// a real pgxpool-backed label store. Tests construct the handler with
// NewPasskeyHandlerWithStore so they can substitute a fake store.
func NewPasskeyHandler(svc PasskeyServicer, pool *pgxpool.Pool) *PasskeyHandler {
	return &PasskeyHandler{svc: svc, store: &pgxPasskeyLabelStore{pool: pool}}
}

// NewPasskeyHandlerWithStore is the test seam — pass a fake PasskeyLabelStore
// and the handler will route every DB call through it.
func NewPasskeyHandlerWithStore(svc PasskeyServicer, store PasskeyLabelStore) *PasskeyHandler {
	return &PasskeyHandler{svc: svc, store: store}
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

	labels, err := h.store.ListLabelsForUser(c.Request.Context(), userID)
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
// @Summary     Relabel a passkey owned by the caller
// @Tags        passkeys
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       credential_id path string true "Credential ID"
// @Param       request body passkeyPatchRequest true "New label"
// @Success     204
// @Failure     403 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /me/passkeys/{credential_id} [patch]
func (h *PasskeyHandler) Patch(c *gin.Context) {
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

	if !h.ownsCredential(c, externalID, credentialID) {
		return
	}

	if err := h.store.UpsertLabel(c.Request.Context(), userID, credentialID, req.Label); err != nil {
		_ = c.Error(apierror.NewInternal("passkey label upsert failed: " + err.Error()))
		c.Abort()
		return
	}

	c.Status(http.StatusNoContent)
}

// Delete handles DELETE /api/v1/me/passkeys/:credential_id.
//
// @Summary     Delete a passkey owned by the caller
// @Tags        passkeys
// @Security    BearerAuth
// @Param       credential_id path string true "Credential ID"
// @Success     204
// @Failure     403 {object} apierror.AppError
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

	if !h.ownsCredential(c, externalID, credentialID) {
		return
	}

	// Core delete first: if the core fails we don't want to leave Raven's
	// label row pointing at a credential the user still has. The label-row
	// delete is best-effort — if it fails after the core succeeded we
	// surface the error but the credential is already gone from the core
	// and a stale label row is harmless (the next GET will simply not see
	// it since the join is by credential_id).
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

	if err := h.store.DeleteLabel(c.Request.Context(), userID, credentialID); err != nil {
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

// ownsCredential confirms the caller actually owns the credential by
// listing the core's credentials for the session user and checking
// membership. Returns true when ownership is confirmed; otherwise writes
// a 403 (or 500 on lookup failure) and returns false.
//
// This is the authorisation backstop: the routes already sit under the
// session middleware, but PATCH/DELETE accept the credential ID as a URL
// parameter and we must not trust it.
func (h *PasskeyHandler) ownsCredential(c *gin.Context, externalID, credentialID string) bool {
	creds, err := h.svc.ListByUser(c.Request.Context(), externalID)
	if err != nil {
		_ = c.Error(apierror.NewInternal("passkey ownership check failed: " + err.Error()))
		c.Abort()
		return false
	}
	for _, cred := range creds {
		if cred.CredentialID == credentialID {
			return true
		}
	}
	_ = c.Error(&apierror.AppError{
		Code:    http.StatusForbidden,
		Message: "Forbidden",
		Detail:  "credential does not belong to caller",
	})
	c.Abort()
	return false
}

// pgxPasskeyLabelStore implements PasskeyLabelStore against the live
// user_passkey_labels table. RLS is enforced by setting
// app.current_user_id inside the wrapping transaction (db.WithUserID).
type pgxPasskeyLabelStore struct {
	pool *pgxpool.Pool
}

// ListLabelsForUser returns label rows keyed by credential_id.
func (s *pgxPasskeyLabelStore) ListLabelsForUser(ctx context.Context, userID string) (map[string]passkeyLabelRow, error) {
	out := make(map[string]passkeyLabelRow)
	err := db.WithUserID(ctx, s.pool, userID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT credential_id, label, created_at, last_used_at
			 FROM user_passkey_labels
			 WHERE user_id = $1`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				credID     string
				row        passkeyLabelRow
				lastUsedAt *time.Time
			)
			if err := rows.Scan(&credID, &row.Label, &row.CreatedAt, &lastUsedAt); err != nil {
				return err
			}
			row.LastUsedAt = lastUsedAt
			out[credID] = row
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertLabel inserts a new row or updates the label on an existing one.
// Behaviour is intentionally "label only" — created_at is never touched on
// update, and last_used_at is owned by the SuperTokens hook path (set when
// the credential is used for sign-in).
func (s *pgxPasskeyLabelStore) UpsertLabel(ctx context.Context, userID, credentialID, label string) error {
	return db.WithUserID(ctx, s.pool, userID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO user_passkey_labels (user_id, credential_id, label)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (credential_id) DO UPDATE SET label = EXCLUDED.label`,
			userID, credentialID, label)
		return err
	})
}

// DeleteLabel removes a single label row. It is the caller's responsibility
// to have already deleted the credential from the SuperTokens core.
func (s *pgxPasskeyLabelStore) DeleteLabel(ctx context.Context, userID, credentialID string) error {
	return db.WithUserID(ctx, s.pool, userID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`DELETE FROM user_passkey_labels WHERE user_id = $1 AND credential_id = $2`,
			userID, credentialID)
		return err
	})
}
