// Admin DMCA endpoints (issue #736, launch blocker per ADR-0006).
//
// Three endpoints under /api/v1/admin/marketplace/dmca:
//
//   GET    /admin/marketplace/dmca               — paginated DMCA notice queue.
//   POST   /admin/marketplace/dmca               — record a fresh DMCA notice.
//   POST   /admin/marketplace/dmca/:id/counter-notice — record a publisher
//                                                       counter-notice.
//
// The HTTP gate (RequireRavenAdmin in cmd/api/main.go) runs upstream; this
// file maps the service's structured sentinel errors to HTTP statuses and
// trusts that gate. See admin_marketplace.go for the sibling pattern on
// the reports queue.

package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// AdminDMCAService is the slice of *marketplace.DMCAService this handler
// depends on. Declared as an interface so the unit tests can pass a stub
// that returns canned errors without a Postgres testcontainer.
type AdminDMCAService interface {
	Submit(ctx context.Context, in marketplace.DMCANoticeInput) (marketplace.DMCANotice, error)
	SubmitCounterNotice(ctx context.Context, noticeID uuid.UUID, text string) error
	ListNotices(ctx context.Context, status marketplace.DMCAStatus, limit, offset int) ([]marketplace.DMCANotice, error)
}

// AdminDMCAHandler exposes the three admin DMCA endpoints.
type AdminDMCAHandler struct {
	svc AdminDMCAService
}

// NewAdminDMCAHandler returns a handler bound to svc.
func NewAdminDMCAHandler(svc AdminDMCAService) *AdminDMCAHandler {
	return &AdminDMCAHandler{svc: svc}
}

// dmcaSubmitRequest is the wire shape POSTed by the admin UI.
type dmcaSubmitRequest struct {
	TargetKBID    string `json:"target_kb_id"`
	NoticeText    string `json:"notice_text"`
	ClaimantEmail string `json:"claimant_email"`
	ClaimantName  string `json:"claimant_name"`
}

// dmcaCounterNoticeRequest is the wire shape for the counter-notice
// endpoint. Body is a single string field so the admin UI can capture
// the publisher's reply verbatim.
type dmcaCounterNoticeRequest struct {
	CounterNoticeText string `json:"counter_notice_text"`
}

// Submit handles POST /api/v1/admin/marketplace/dmca.
//
// Body: {target_kb_id, notice_text, claimant_email, claimant_name}.
// On success returns the full DMCANotice (id + counter_notice_window_ends
// in particular, so the UI can show the 14-day countdown without a
// follow-up GET).
//
// Errors:
//   - 400 invalid payload (non-UUID target_kb_id, empty fields, oversize
//     notice text).
//   - 401/403 handled upstream by RequireRavenAdmin.
//   - 404 target KB does not exist.
//   - 409 the KB already has a pending DMCA notice.
func (h *AdminDMCAHandler) Submit(c *gin.Context) {
	var req dmcaSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid DMCA submit body"))
		c.Abort()
		return
	}
	targetKBID, err := uuid.Parse(req.TargetKBID)
	if err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid target_kb_id"))
		c.Abort()
		return
	}

	notice, err := h.svc.Submit(c.Request.Context(), marketplace.DMCANoticeInput{
		TargetKBID:    targetKBID,
		NoticeText:    req.NoticeText,
		ClaimantEmail: req.ClaimantEmail,
		ClaimantName:  req.ClaimantName,
	})
	if err != nil {
		_ = c.Error(translateDMCAError(err))
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, notice)
}

// SubmitCounterNotice handles POST /api/v1/admin/marketplace/dmca/:id/counter-notice.
//
// Body: {counter_notice_text}.
// On success returns 200 with {notice_id, status: 'counter_filed'} so
// the SPA can confirm the pivot without re-fetching the row.
//
// Errors:
//   - 400 invalid payload (empty text, oversize).
//   - 401/403 handled upstream.
//   - 404 notice id does not resolve.
//   - 409 notice is not in `pending` (terminal or already counter-filed).
func (h *AdminDMCAHandler) SubmitCounterNotice(c *gin.Context) {
	noticeID, err := parseDMCANoticeID(c)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	var req dmcaCounterNoticeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierror.NewBadRequest("invalid counter-notice body"))
		c.Abort()
		return
	}

	if err := h.svc.SubmitCounterNotice(c.Request.Context(), noticeID, req.CounterNoticeText); err != nil {
		_ = c.Error(translateDMCAError(err))
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"notice_id": noticeID,
		"status":    marketplace.DMCAStatusCounterFiled,
	})
}

// List handles GET /api/v1/admin/marketplace/dmca.
//
// Query params:
//   - status (optional): one of pending / counter_filed / resolved_take_down /
//     resolved_keep_up / withdrawn. Empty means all.
//   - limit / offset: paginate (defaults 50 / 0, cap 200).
//
// The admin UI uses this to render the DMCA queue. Cross-tenant; gated
// upstream by RequireRavenAdmin.
func (h *AdminDMCAHandler) List(c *gin.Context) {
	statusStr := c.Query("status")
	var status marketplace.DMCAStatus
	if statusStr != "" {
		status = marketplace.DMCAStatus(statusStr)
		if !status.IsValid() {
			_ = c.Error(apierror.NewBadRequest("invalid status query parameter"))
			c.Abort()
			return
		}
	}

	limit, err := parsePageInt(c, "limit", 50, 200)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	offset, err := parsePageInt(c, "offset", 0, 1_000_000)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	notices, err := h.svc.ListNotices(c.Request.Context(), status, limit, offset)
	if err != nil {
		_ = c.Error(translateDMCAError(err))
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"notices": notices,
		"limit":   limit,
		"offset":  offset,
	})
}

// parseDMCANoticeID pulls :id from the URL and validates it is a UUID.
func parseDMCANoticeID(c *gin.Context) (uuid.UUID, error) {
	raw := c.Param("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apierror.NewBadRequest("invalid notice id")
	}
	return id, nil
}

// translateDMCAError maps marketplace DMCA sentinel errors to API
// errors. Unknown errors become opaque 500s via the error handler.
func translateDMCAError(err error) error {
	switch {
	case errors.Is(err, marketplace.ErrDMCAInvalidInput):
		return apierror.NewBadRequest("invalid DMCA input")
	case errors.Is(err, marketplace.ErrDMCATargetKBNotFound):
		return apierror.NewNotFound("target KB not found")
	case errors.Is(err, marketplace.ErrDMCANoticeNotFound):
		return apierror.NewNotFound("DMCA notice not found")
	case errors.Is(err, marketplace.ErrDMCAAlreadyPending):
		return apierror.NewConflict("KB already has a pending DMCA notice")
	case errors.Is(err, marketplace.ErrDMCAIllegalTransition):
		return apierror.NewConflict("illegal DMCA status transition")
	default:
		return err
	}
}
