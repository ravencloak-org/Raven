// Package account implements user-account self-service endpoints used
// by the public demo: DSAR export and delete, and the retention purge
// service. All data-access goes through the DSARRepo / RetentionRepo
// interfaces so handlers stay testable without a real database.
package account

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ravencloak-org/Raven/internal/mail"
)

// UserExport is the JSON payload returned by GET /account/export.
type UserExport struct {
	UserID string                      `json:"user_id"`
	Email  string                      `json:"email"`
	Rows   map[string][]map[string]any `json:"rows"`
}

// DSARRepo is the persistence surface for DSAR operations.
type DSARRepo interface {
	ExportUser(ctx context.Context, userID string) (UserExport, error)
	ScheduleDelete(ctx context.Context, userID string) error
}

// DSARHandler exposes /account/export and /account/delete.
type DSARHandler struct {
	Repo DSARRepo
	Mail mail.Sender
}

// NewDSARHandler returns a handler wired to repo + mailer. The mailer may
// be nil; Delete simply skips the confirmation email in that case.
func NewDSARHandler(repo DSARRepo, mailer mail.Sender) *DSARHandler {
	return &DSARHandler{Repo: repo, Mail: mailer}
}

type ctxKey string

const (
	userIDKey    ctxKey = "user_id"
	userEmailKey ctxKey = "user_email"
)

// WithUserID puts a SuperTokens-resolved user ID into ctx for handlers.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserIDFrom extracts the user ID set by WithUserID. Returns ok=false
// when no user is bound (treat as unauthenticated).
func UserIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok && v != ""
}

// WithUserEmail puts the signed-in user's email into ctx so the delete
// confirmation email can be sent.
func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, userEmailKey, email)
}

// UserEmailFrom extracts the email set by WithUserEmail.
func UserEmailFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userEmailKey).(string)
	return v, ok && v != ""
}

// Export writes a JSON file containing every row the repo associates
// with the authenticated user. Streamed so the response stays small for
// any one query but the full payload may be large.
func (h *DSARHandler) Export(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	out, err := h.Repo.ExportUser(r.Context(), uid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=raven-export.json")
	_ = json.NewEncoder(w).Encode(out)
}

// Delete schedules irreversible deletion of the authenticated user's
// data in 24h (the grace window gives users a chance to recover from
// an accidental click via reply-to-email). Responds 202 Accepted.
func (h *DSARHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	if err := h.Repo.ScheduleDelete(r.Context(), uid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if email, ok := UserEmailFrom(r.Context()); ok && h.Mail != nil {
		_ = h.Mail.Send(r.Context(), mail.Message{
			To:      email,
			Subject: "Your Raven delete request",
			Text: "We received your request to delete your Raven account. " +
				"Your data will be removed in 24 hours. " +
				"Reply to this email if you didn't request this.",
		})
	}
	w.WriteHeader(http.StatusAccepted)
}
