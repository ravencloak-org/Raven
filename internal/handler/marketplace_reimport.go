package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ReImporterService is the marketplace re-import contract the handler depends
// on. Keeping the surface narrow (one method) lets tests substitute an
// in-memory fake and keeps the dependency direction from handler → service.
type ReImporterService interface {
	ReImport(ctx context.Context, orgID, kbID, actorUserID string) (marketplace.ReImportResult, error)
}

// MarketplaceReImportHandler serves POST /api/v1/knowledge_bases/{id}/re-import.
// See ADR-0007 §7b and issue #730.
type MarketplaceReImportHandler struct {
	svc ReImporterService
}

// NewMarketplaceReImportHandler constructs the handler. The service is the
// only collaborator — by design the HTTP boundary has no direct DB access.
func NewMarketplaceReImportHandler(svc ReImporterService) *MarketplaceReImportHandler {
	return &MarketplaceReImportHandler{svc: svc}
}

// ReImportRequest is the body shape. The single field is a deliberate guardrail:
// Re-import is irreversibly destructive on the importer's local content, so
// the API refuses to fire on an empty body and the frontend must echo back a
// modal-confirmation toggle. ADR-0007 calls this out: "UI requires explicit
// confirmation"; this is the server-side mirror.
type ReImportRequest struct {
	Confirm bool `json:"confirm"`
}

// ReImportResponse is the success shape mandated by §4 of the plan and the
// acceptance criteria on #730: `{ kb_id, imported_from_revision_at }`. The
// chunks_projected field is additive — clients that ignore it stay correct.
type ReImportResponse struct {
	KBID                   string    `json:"kb_id"`
	ImportedFromRevisionAt time.Time `json:"imported_from_revision_at"`
	ChunksProjected        int       `json:"chunks_projected,omitempty"`
}

// ReImport is the POST handler. Routes are wired in cmd/api/main.go.
//
// Errors map per ADR-0007 §7b:
//
//   - 400 — confirm flag missing or false (UX guardrail).
//   - 401 — no session (handled by middleware before this method runs).
//   - 403 — caller's org cannot resolve / not the KB's owning org. The
//     existing org middleware already enforces session→org binding, so a
//     mismatched org_id surfaces as a missing KB (404) at the service layer;
//     a 403 here would mean a future code path attached the wrong org to
//     the gin context.
//   - 404 — KB id does not exist for caller's org.
//   - 409 — KB is not an import (source_public_kb_id IS NULL).
//   - 410 — source public KB has been unpublished.
//
// @Summary     Re-import a knowledge base from its Marketplace source
// @Description Destructive overwrite: drops every Source / Document / Chunk
// @Description on the local KB and re-projects from the publisher's current
// @Description state. Preserves the KB id and every runtime artefact that
// @Description FKs to it (chats, widgets, API keys, routing rules, webhooks,
// @Description response cache). See ADR-0007 §7b.
// @Tags        marketplace
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id      path string          true "Imported KB id"
// @Param       request body ReImportRequest true "Confirmation payload"
// @Success     200 {object} ReImportResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     409 {object} apierror.AppError "KB is not an import"
// @Failure     410 {object} apierror.AppError "Source KB unpublished"
// @Router      /knowledge_bases/{id}/re-import [post]
func (h *MarketplaceReImportHandler) ReImport(c *gin.Context) {
	kbID := c.Param("kb_id")
	if kbID == "" {
		_ = c.Error(apierror.NewBadRequest("knowledge base id is required"))
		c.Abort()
		return
	}

	orgID := c.GetString(string(middleware.ContextKeyOrgID))
	if orgID == "" {
		// Session middleware must populate org_id before this handler runs.
		// Empty here means the session is unauthenticated or has no org
		// binding — surface as 401 (the auth-layer error) rather than 500.
		_ = c.Error(apierror.NewUnauthorized("missing org binding on session"))
		c.Abort()
		return
	}
	actorUserID := c.GetString(string(middleware.ContextKeyUserID))

	var req ReImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierror.NewBadRequest("request body must be JSON with a {confirm: true} field: " + err.Error()))
		c.Abort()
		return
	}
	// The confirm flag is the server-side mirror of the ADR-required UI
	// confirmation. Missing or false → 400, no destructive work fires.
	if !req.Confirm {
		_ = c.Error(apierror.NewBadRequest(`re-import is destructive; set "confirm": true to proceed`))
		c.Abort()
		return
	}

	res, err := h.svc.ReImport(c.Request.Context(), orgID, kbID, actorUserID)
	if err != nil {
		// Map the service's typed errors to the wire contract. The default
		// branch falls through to the generic error middleware (500).
		switch {
		case errors.Is(err, marketplace.ErrKBNotFound):
			_ = c.Error(apierror.NewNotFound("knowledge base not found"))
		case errors.Is(err, marketplace.ErrNotAnImport):
			_ = c.Error(apierror.NewConflict("knowledge base was authored locally, not imported from the Marketplace"))
		case errors.Is(err, marketplace.ErrSourceUnpublished):
			_ = c.Error(apierror.NewGone("source public KB has been unpublished"))
		default:
			_ = c.Error(apierror.NewInternal("failed to re-import knowledge base: " + err.Error()))
		}
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, ReImportResponse{
		KBID:                   res.KBID,
		ImportedFromRevisionAt: res.ImportedFromRevisionAt,
		ChunksProjected:        res.ChunksProjected,
	})
}
