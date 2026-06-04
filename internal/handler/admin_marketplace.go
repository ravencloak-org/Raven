package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// AdminMarketplaceService is the slice of *marketplace.AdminModeration
// this handler depends on. Declared as an interface so the unit tests
// can pass a stub that returns canned errors (404, 409 mapping) without
// standing up a Postgres testcontainer.
type AdminMarketplaceService interface {
	ListReports(ctx context.Context, status marketplace.ReportStatus, limit, offset int) ([]marketplace.Report, error)
	Approve(ctx context.Context, reportID uuid.UUID) (marketplace.ApproveResult, error)
	Dismiss(ctx context.Context, reportID uuid.UUID) error
}

// NewAdminMarketplaceHandler builds a handler ready to be wired into
// the admin route group. The service argument may not be nil; main.go
// constructs the service and passes it.
func NewAdminMarketplaceHandler(svc AdminMarketplaceService) *AdminMarketplaceHandler {
	return &AdminMarketplaceHandler{svc: svc}
}

// AdminMarketplaceHandler implements the three admin review-queue
// endpoints described in plan §4.
type AdminMarketplaceHandler struct {
	svc AdminMarketplaceService
}

// ListReports handles GET /api/v1/admin/marketplace/reports.
//
// Query parameters:
//   - status: one of open/reviewing/resolved/dismissed. Defaults to
//     `open` since the queue's default view is the live work-list.
//   - limit: max rows per page. Defaults to 50, capped at 200 (keeps
//     a single response payload bounded for the SPA).
//   - offset: zero-based offset for pagination. Defaults to 0. We do
//     not yet support cursor-based pagination — created_at ASC + offset
//     is sufficient for the MVP volumes.
//
// Returns 400 on invalid status / negative limit / non-numeric inputs.
// The platform-admin gate runs upstream so 401/403 are already handled.
func (h *AdminMarketplaceHandler) ListReports(c *gin.Context) {
	statusStr := c.DefaultQuery("status", string(marketplace.ReportStatusOpen))
	status := marketplace.ReportStatus(statusStr)
	if !status.IsValid() {
		_ = c.Error(apierror.NewBadRequest("invalid status query parameter"))
		c.Abort()
		return
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

	reports, err := h.svc.ListReports(c.Request.Context(), status, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
		"limit":   limit,
		"offset":  offset,
	})
}

// Approve handles POST /api/v1/admin/marketplace/reports/:id/approve.
//
// Runs the four-side-effect atomic transaction described in ADR-0006.
// On success returns {takedown_id, target_kb_id, strikes_after} so the
// SPA can update the queue row + show a strike-count toast in one
// round-trip.
//
// 404 when the report is missing; 409 on an illegal state-machine
// transition (the row is already resolved/dismissed); 500 otherwise.
func (h *AdminMarketplaceHandler) Approve(c *gin.Context) {
	reportID, err := parseReportID(c)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	result, err := h.svc.Approve(c.Request.Context(), reportID)
	if err != nil {
		_ = c.Error(translateModerationError(err))
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, result)
}

// Dismiss handles POST /api/v1/admin/marketplace/reports/:id/dismiss.
//
// 404 / 409 / 500 mapping identical to Approve. No body on success
// beyond the canonical {report_id, status: 'dismissed'} echo so the SPA
// can confirm the action without a follow-up GET.
func (h *AdminMarketplaceHandler) Dismiss(c *gin.Context) {
	reportID, err := parseReportID(c)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	if err := h.svc.Dismiss(c.Request.Context(), reportID); err != nil {
		_ = c.Error(translateModerationError(err))
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"report_id": reportID,
		"status":    marketplace.ReportStatusDismissed,
	})
}

// parseReportID pulls :id from the URL and validates it is a UUID.
// Returns a 400 AppError on malformed input.
func parseReportID(c *gin.Context) (uuid.UUID, error) {
	raw := c.Param("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apierror.NewBadRequest("invalid report id")
	}
	return id, nil
}

// parsePageInt reads an optional integer query parameter. Returns the
// default when missing, an error when present-but-malformed, and clamps
// at the supplied max so a degenerate ?limit=2147483647 cannot blow up
// a pgx prepared statement.
func parsePageInt(c *gin.Context, name string, def, max int) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, apierror.NewBadRequest("invalid " + name + " query parameter")
	}
	if n < 0 {
		return 0, apierror.NewBadRequest(name + " must be non-negative")
	}
	if n > max {
		n = max
	}
	return n, nil
}

// translateModerationError maps marketplace sentinel errors to API
// errors so the handler doesn't need a manual switch in every method.
// Anything unknown becomes an opaque 500.
func translateModerationError(err error) error {
	switch {
	case errors.Is(err, marketplace.ErrReportNotFound):
		return apierror.NewNotFound("report not found")
	case errors.Is(err, marketplace.ErrIllegalTransition):
		return apierror.NewConflict("illegal report status transition")
	case errors.Is(err, marketplace.ErrInvalidReportStatus):
		return apierror.NewBadRequest("invalid report status")
	default:
		return err
	}
}
