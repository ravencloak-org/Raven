package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// MarketplaceImporter is the narrow slice of marketplace.Importer the
// handler needs. Defined here so the handler test can pass a stub instead
// of standing up the full pgx + plan-resolver wiring; production wiring
// passes a real *marketplace.Importer.
type MarketplaceImporter interface {
	Import(
		ctx context.Context,
		sourceKBID, dstOrgID, dstWorkspaceID, dstUserID uuid.UUID,
	) (*marketplace.ImportResult, error)
}

// MarketplaceHandler owns the HTTP surface for the Marketplace endpoints
// that wrap cross-tenant operations. Listing / preview / publish live in
// separate handlers (issue #731 / #726); this file is import-only by
// design — the Marketplace surface is mounted from several issue-scoped
// PRs and a single mega-handler would force every PR to touch the same
// file.
type MarketplaceHandler struct {
	importer MarketplaceImporter
}

// NewMarketplaceHandler constructs the handler. The importer must be
// non-nil; production wiring should pass *marketplace.Importer.
func NewMarketplaceHandler(importer MarketplaceImporter) *MarketplaceHandler {
	if importer == nil {
		panic("handler.NewMarketplaceHandler: nil importer")
	}
	return &MarketplaceHandler{importer: importer}
}

// ImportKBRequest is the payload for POST /api/v1/marketplace/import/:public_kb_id.
// The workspace_id is the destination workspace inside the importer's own
// Org (resolved from session) — it identifies WHERE the new local KB will
// land, not WHAT will be imported.
type ImportKBRequest struct {
	WorkspaceID string `json:"workspace_id" binding:"required,uuid"`
	// OrgID is optional — when present the handler trusts the caller's
	// own session resolution of "which Org owns this workspace". The
	// session middleware sets ContextKeyOrgID by default; we allow the
	// body to override only when the session has not been bound to a
	// single Org (multi-Org users on first login). Empty falls back to
	// the session value.
	OrgID string `json:"org_id,omitempty" binding:"omitempty,uuid"`
}

// Import handles POST /api/v1/marketplace/import/:public_kb_id.
//
// Errors per docs/plans/marketplace-mvp.md §4:
//
//	401 — actor is unauthenticated (handled by the session middleware
//	      before we get here; we surface a defensive 401 if the user id
//	      is missing).
//	403 — actor is not a member of the target workspace, OR the source
//	      KB is not public (deliberately indistinguishable per ADR-0001).
//	404 — public_kb_id is not a valid UUID (parser rejection).
//	409 — the same public KB has already been imported into the target
//	      workspace (use the Re-import endpoint, #730).
//	422 — license missing on the source, source has zero documents,
//	      embedding model mismatch.
//
// The handler is a thin wrapper around marketplace.Importer.Import. All
// policy lives there; the only reason this file exists is to bind the
// HTTP shape to the service shape.
//
// @Summary     Import a Public Knowledge Base from the Marketplace
// @Description Forks the chosen Public KB into the caller's workspace.
// @Tags        marketplace
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       public_kb_id path string true "Source Public KB UUID"
// @Param       request body ImportKBRequest true "Destination workspace"
// @Success     201 {object} marketplace.ImportResult
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     409 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /marketplace/import/{public_kb_id} [post]
func (h *MarketplaceHandler) Import(c *gin.Context) {
	// 1. Parse + validate path param.
	rawPublicKBID := c.Param("public_kb_id")
	publicKBID, err := uuid.Parse(rawPublicKBID)
	if err != nil {
		_ = c.Error(apierror.NewNotFound("invalid public_kb_id"))
		c.Abort()
		return
	}

	// 2. Bind body. The workspace_id binding tag catches malformed UUIDs
	// here so the importer never sees a junk argument.
	var req ImportKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierror.NewUnprocessableEntity(err.Error()))
		c.Abort()
		return
	}
	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		_ = c.Error(apierror.NewUnprocessableEntity("workspace_id is not a valid UUID"))
		c.Abort()
		return
	}

	// 3. Resolve actor + destination Org from session context. The
	// session middleware sets both keys; missing means an unauthenticated
	// request slipped past — surface 401 rather than a 500 with a NULL
	// org id.
	userIDRaw, _ := c.Get(string(middleware.ContextKeyUserID))
	userIDStr, _ := userIDRaw.(string)
	if userIDStr == "" {
		_ = c.Error(apierror.NewUnauthorized("user context missing"))
		c.Abort()
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		_ = c.Error(apierror.NewUnauthorized("invalid user id in session"))
		c.Abort()
		return
	}

	orgIDStr := req.OrgID
	if orgIDStr == "" {
		orgIDRaw, _ := c.Get(string(middleware.ContextKeyOrgID))
		orgIDStr, _ = orgIDRaw.(string)
	}
	if orgIDStr == "" {
		// No body OrgID and no session OrgID — surface 422 (the request
		// shape parsed but is incomplete) rather than 401 (the session
		// is fine, the caller just has no Org bound).
		_ = c.Error(apierror.NewUnprocessableEntity("org_id missing from request and session"))
		c.Abort()
		return
	}
	dstOrgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		_ = c.Error(apierror.NewUnprocessableEntity("org_id is not a valid UUID"))
		c.Abort()
		return
	}

	// 4. Delegate. All policy lives in the service.
	res, err := h.importer.Import(c.Request.Context(), publicKBID, dstOrgID, workspaceID, userID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, res)
}
