// Admin takedown audit-log endpoint (issue #735, M3).
//
// GET /api/v1/admin/marketplace/takedowns?limit=&cursor=
//
// Read-only paginated audit log of marketplace_takedowns joined with the
// target KB and publisher Org. The HTTP gate is RequireRavenAdmin
// (env-allowlist match on the session email); the repository layer
// switches to the raven_admin DB role inside its own short-lived
// transaction so the SELECT can cross every tenant.
//
// This file is intentionally tiny — the heavy lifting lives in
// internal/marketplace/takedowns.go (ListAudit + cursor helpers). The
// handler's only job is to parse query params, call the repo, and
// translate the cursor + rows into the JSON shape #735 documents.

package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// TakedownsAuditLister is the slice of *marketplace.Takedowns the
// handler depends on. Defined as an interface so tests can stub it
// without spinning up a Postgres testcontainer for the HTTP-layer
// assertions; the integration tests in internal/marketplace cover the
// real SQL.
type TakedownsAuditLister interface {
	ListAudit(ctx context.Context, limit int, cursor string) ([]marketplace.AuditRow, string, error)
}

// AdminTakedownsHandler exposes the read-only audit endpoint.
type AdminTakedownsHandler struct {
	repo TakedownsAuditLister
}

// NewAdminTakedownsHandler returns a handler bound to repo.
func NewAdminTakedownsHandler(repo TakedownsAuditLister) *AdminTakedownsHandler {
	return &AdminTakedownsHandler{repo: repo}
}

// adminTakedownRow is the JSON wire shape per #735's "Response row shape"
// in the issue. Snake_case keys to match the rest of the API.
type adminTakedownRow struct {
	TakedownID            string `json:"takedown_id"`
	TargetKBID            string `json:"target_kb_id"`
	TargetKBName          string `json:"target_kb_name"`
	TargetOrgSlug         string `json:"target_org_slug"`
	TargetOrgDisplayName  string `json:"target_org_display_name"`
	Source                string `json:"source"`
	Notes                 string `json:"notes"`
	CreatedAt             string `json:"created_at"`
	StrikesAfterOrgTotal  int64  `json:"strikes_after_org_total"`
}

type adminTakedownsResponse struct {
	Items      []adminTakedownRow `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

// List handles GET /api/v1/admin/marketplace/takedowns.
//
// @Summary     Paginated audit log of Marketplace takedowns
// @Tags        admin,marketplace
// @Produce     json
// @Security    BearerAuth
// @Param       limit  query int    false "Page size (1..100, default 25)"
// @Param       cursor query string false "Opaque keyset cursor from prior response"
// @Success     200 {object} adminTakedownsResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /admin/marketplace/takedowns [get]
func (h *AdminTakedownsHandler) List(c *gin.Context) {
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			_ = c.Error(&apierror.AppError{
				Code:    http.StatusBadRequest,
				Message: "Bad Request",
				Detail:  "limit must be a non-negative integer",
			})
			c.Abort()
			return
		}
		limit = n
	}
	cursor := c.Query("cursor")

	rows, nextCursor, err := h.repo.ListAudit(c.Request.Context(), limit, cursor)
	if err != nil {
		// Opaque cursors are a contract: parse failures are a client bug,
		// not a server one — surface 400 rather than 500.
		if isInvalidCursor(err) {
			_ = c.Error(&apierror.AppError{
				Code:    http.StatusBadRequest,
				Message: "Bad Request",
				Detail:  "invalid cursor",
			})
			c.Abort()
			return
		}
		_ = c.Error(err)
		c.Abort()
		return
	}

	out := adminTakedownsResponse{
		Items:      make([]adminTakedownRow, 0, len(rows)),
		NextCursor: nextCursor,
	}
	for _, r := range rows {
		out.Items = append(out.Items, adminTakedownRow{
			TakedownID:           r.TakedownID.String(),
			TargetKBID:           r.TargetKBID.String(),
			TargetKBName:         r.TargetKBName,
			TargetOrgSlug:        r.TargetOrgSlug,
			TargetOrgDisplayName: r.TargetOrgDisplayName,
			Source:               string(r.Source),
			Notes:                r.Notes,
			CreatedAt:            r.CreatedAt.UTC().Format("2006-01-02T15:04:05.000000Z"),
			StrikesAfterOrgTotal: r.StrikesAfterOrgTotal,
		})
	}
	c.JSON(http.StatusOK, out)
}

// isInvalidCursor detects the audit-cursor parse-error sentinel.
// ListAudit wraps it with fmt.Errorf("...: %w", ...) so errors.Is is the
// right tool.
func isInvalidCursor(err error) bool {
	return errors.Is(err, marketplace.ErrInvalidAuditCursor)
}
