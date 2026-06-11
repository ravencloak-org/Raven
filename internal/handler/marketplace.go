package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// Marketplace listing defaults. The HTTP-side ceiling is 50; the SECURITY
// DEFINER function in migration 00052 also clamps to 50, so the two
// numbers are deliberately equal — every layer rejects over-large pages
// instead of one silently shrinking what the other accepted.
const (
	marketplaceDefaultLimit = 25
	marketplaceMaxLimit     = 50
)

// validSortValues mirrors the four-entry whitelist enforced by the SQL
// function. We validate Go-side so a malformed query string returns 400
// before any database round trip and so we surface a stable
// error_code clients can branch on; the SQL function's
// invalid_parameter_value path is the backstop.
var validSortValues = map[string]marketplace.ListSort{
	string(marketplace.SortNewest):          marketplace.SortNewest,
	string(marketplace.SortMostImported):    marketplace.SortMostImported,
	string(marketplace.SortRecentlyUpdated): marketplace.SortRecentlyUpdated,
	string(marketplace.SortAlphabetic):      marketplace.SortAlphabetic,
}

// MarketplaceLister is the narrow slice of marketplace.Queries the
// Marketplace handler needs for the listing endpoint. Defined here so
// handler tests can pass a stub without standing up a real pgxpool.
type MarketplaceLister interface {
	ListPublicKBs(ctx context.Context, f marketplace.ListFilters) ([]marketplace.ListItem, error)
	PreviewKB(ctx context.Context, publicKBID uuid.UUID) ([]marketplace.PreviewChunk, error)
}

// SlugResolver wraps marketplace.MarketplaceSlugStatus so the handler can
// be unit-tested with a stub that returns canned (404 / 410 / detail)
// outcomes without standing up the database.
type SlugResolver func(ctx context.Context, orgSlug, kbSlug string) (marketplace.SlugStatus, error)

// MarketplaceHandler serves the cross-tenant Marketplace discovery
// surface: listing, detail, and ≤3-chunk preview. All three are
// authenticated (per CONTEXT.md the Marketplace is "walled" — only
// logged-in users see it) but carry no admin gate; the route group in
// main.go applies the standard SessionMiddleware + UserLookup chain
// shared by every other /api/v1 handler.
type MarketplaceHandler struct {
	queries  MarketplaceLister
	resolver SlugResolver
}

// NewMarketplaceHandler wires the handler to a Queries instance and a
// slug resolver. The resolver is a function (not a method on Queries)
// because the stub MarketplaceSlugStatus lives at the package level
// rather than on Queries — this lets tests inject a deterministic
// resolver without dragging a partial Queries mock through every case.
func NewMarketplaceHandler(q MarketplaceLister, resolver SlugResolver) *MarketplaceHandler {
	return &MarketplaceHandler{queries: q, resolver: resolver}
}

// MarketplaceListResponse is the envelope returned by the listing
// endpoint. The `items` field carries one row per ADR-0005's listing
// row shape (also surfaced as marketplace.ListItem). `next_offset` is
// the offset to pass for the next page, or null when no further pages
// exist.
type MarketplaceListResponse struct {
	Items      []MarketplaceListItem `json:"items"`
	NextOffset *int                  `json:"next_offset"`
}

// MarketplaceListItem is the JSON projection of marketplace.ListItem.
// Defined here (not on the marketplace package) because the JSON shape
// is an HTTP-surface concern; the marketplace package owns the Go
// types, the handler owns the wire format.
type MarketplaceListItem struct {
	KBID                 uuid.UUID  `json:"kb_id"`
	OrgSlug              string     `json:"org_slug"`
	OrgDisplayName       string     `json:"org_display_name"`
	KBSlug               string     `json:"kb_slug"`
	KBName               string     `json:"kb_name"`
	Description          *string    `json:"description,omitempty"`
	LicenseSPDXID        *string    `json:"license_spdx_id,omitempty"`
	LastModifiedAt       string     `json:"last_modified_at"`
	ImportCount          int        `json:"import_count"`
	SourcePublicKBID     *uuid.UUID `json:"source_public_kb_id,omitempty"`
	SourceOrgSlug        *string    `json:"source_org_slug,omitempty"`
	SourceOrgDisplayName *string    `json:"source_org_display_name,omitempty"`
}

// MarketplacePreviewResponse is the JSON envelope for the preview
// endpoint. The `chunks` field is always present and may be empty when
// the KB has no chunks yet; an empty slice (not null) keeps the wire
// shape stable for clients.
type MarketplacePreviewResponse struct {
	Chunks []MarketplacePreviewChunk `json:"chunks"`
}

// MarketplacePreviewChunk is the JSON projection of
// marketplace.PreviewChunk.
type MarketplacePreviewChunk struct {
	ChunkID uuid.UUID `json:"chunk_id"`
	Ordinal int       `json:"ordinal"`
	Text    string    `json:"text"`
}

// List handles GET /api/v1/marketplace.
//
// Query string:
//   - q         — free-form search (FTS); empty means no filter.
//   - sort      — one of newest|most_imported|recently_updated|alphabetic
//     (default: newest).
//   - license[] — repeated SPDX identifier; each must be in the 7-item
//     allow-list.
//   - limit     — page size, default 25, hard cap 50.
//   - offset    — page start, default 0.
//
// @Summary     List public Marketplace KBs
// @Tags        marketplace
// @Produce     json
// @Security    BearerAuth
// @Param       q       query string  false "Free-form FTS search"
// @Param       sort    query string  false "newest|most_imported|recently_updated|alphabetic"
// @Param       license query []string false "SPDX identifier(s) to filter by" collectionFormat(multi)
// @Param       limit   query int     false "Page size, max 50"
// @Param       offset  query int     false "Page start"
// @Success     200 {object} MarketplaceListResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /marketplace [get]
func (h *MarketplaceHandler) List(c *gin.Context) {
	filters, appErr := parseListFilters(c)
	if appErr != nil {
		_ = c.Error(appErr)
		c.Abort()
		return
	}

	items, err := h.queries.ListPublicKBs(c.Request.Context(), filters)
	if err != nil {
		// The SQL function raises invalid_parameter_value on a bad sort,
		// which the Queries layer maps to ErrUnknownSort. We already
		// rejected those Go-side above so reaching this branch is a
		// drift signal — surface 400 with the same error_code rather
		// than 500, so the cause is obvious in logs.
		if errors.Is(err, marketplace.ErrUnknownSort) {
			_ = c.Error(&apierror.AppError{
				Code:      http.StatusBadRequest,
				Message:   "Bad Request",
				Detail:    err.Error(),
				ErrorCode: "invalid_sort",
			})
			c.Abort()
			return
		}
		_ = c.Error(err)
		c.Abort()
		return
	}

	resp := MarketplaceListResponse{
		Items: make([]MarketplaceListItem, 0, len(items)),
	}
	for i := range items {
		resp.Items = append(resp.Items, toMarketplaceListItem(items[i]))
	}
	// `next_offset` is set when the page came back full. The SQL
	// function may have clamped Limit, but we mirror what we asked for
	// (filters.Limit), so the client's next request will pass the same
	// effective page size.
	if len(items) == filters.Limit {
		next := filters.Offset + filters.Limit
		resp.NextOffset = &next
	}
	c.JSON(http.StatusOK, resp)
}

// Detail handles GET /api/v1/marketplace/:org_slug/:kb_slug.
//
// Resolves the slug pair via marketplaceLookupOr410 — a 410 hold
// short-circuits the listing row lookup. A missing pair returns 404; an
// active hold returns 410 with error_code "slug_held".
//
// @Summary     Public KB detail
// @Tags        marketplace
// @Produce     json
// @Security    BearerAuth
// @Param       org_slug path string true "Publisher Org slug"
// @Param       kb_slug  path string true "Public KB slug"
// @Success     200 {object} MarketplaceListItem
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     410 {object} apierror.AppError
// @Router      /marketplace/{org_slug}/{kb_slug} [get]
func (h *MarketplaceHandler) Detail(c *gin.Context) {
	orgSlug := c.Param("org_slug")
	kbSlug := c.Param("kb_slug")

	status, appErr := h.marketplaceLookupOr410(c.Request.Context(), orgSlug, kbSlug)
	if appErr != nil {
		_ = c.Error(appErr)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, toMarketplaceListItem(*status.Detail))
}

// Preview handles GET /api/v1/marketplace/:org_slug/:kb_slug/preview.
//
// Identical 410 / 404 contract to Detail, plus 403 when the underlying
// KB is private — the SECURITY DEFINER function in migration 00052
// raises insufficient_privilege which the Queries layer maps to
// ErrKBNotPublic. Server-side preview-count increment happens inside
// the function regardless of how this handler reads the result.
//
// @Summary     Preview ≤3 sample chunks from a Public KB
// @Tags        marketplace
// @Produce     json
// @Security    BearerAuth
// @Param       org_slug path string true "Publisher Org slug"
// @Param       kb_slug  path string true "Public KB slug"
// @Success     200 {object} MarketplacePreviewResponse
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     410 {object} apierror.AppError
// @Router      /marketplace/{org_slug}/{kb_slug}/preview [get]
func (h *MarketplaceHandler) Preview(c *gin.Context) {
	orgSlug := c.Param("org_slug")
	kbSlug := c.Param("kb_slug")

	status, appErr := h.marketplaceLookupOr410(c.Request.Context(), orgSlug, kbSlug)
	if appErr != nil {
		_ = c.Error(appErr)
		c.Abort()
		return
	}

	chunks, err := h.queries.PreviewKB(c.Request.Context(), status.KBID)
	if err != nil {
		if errors.Is(err, marketplace.ErrKBNotPublic) {
			_ = c.Error(&apierror.AppError{
				Code:    http.StatusForbidden,
				Message: "Forbidden",
				Detail:  "knowledge base is not public",
			})
			c.Abort()
			return
		}
		_ = c.Error(err)
		c.Abort()
		return
	}

	resp := MarketplacePreviewResponse{
		Chunks: make([]MarketplacePreviewChunk, 0, len(chunks)),
	}
	for _, ch := range chunks {
		resp.Chunks = append(resp.Chunks, MarketplacePreviewChunk{
			ChunkID: ch.ChunkID,
			Ordinal: ch.Ordinal,
			Text:    ch.Text,
		})
	}
	c.JSON(http.StatusOK, resp)
}

// marketplaceLookupOr410 is the shared resolver for Detail and Preview.
// Returns the resolved SlugStatus on success, or an *apierror.AppError
// the caller can hand straight to `c.Error`. Extracted so the two
// handlers can't drift on the 410 / 404 boundary — one rule, one
// implementation, surfaced as a single code path.
func (h *MarketplaceHandler) marketplaceLookupOr410(ctx context.Context, orgSlug, kbSlug string) (marketplace.SlugStatus, *apierror.AppError) {
	status, err := h.resolver(ctx, orgSlug, kbSlug)
	if err != nil {
		return marketplace.SlugStatus{}, apierror.NewInternal(err.Error())
	}
	if status.IsHeld {
		return marketplace.SlugStatus{}, apierror.NewGone(
			"this knowledge base has been unpublished",
			"slug_held",
		)
	}
	if status.Detail == nil {
		return marketplace.SlugStatus{}, apierror.NewNotFound("knowledge base not found")
	}
	return status, nil
}

// parseListFilters reads the listing query string into a
// marketplace.ListFilters, validating sort + license values against the
// enums in marketplace.ListSort and marketplace.AllowedSPDXLicenses
// respectively. Returns an *apierror.AppError on the first failure so
// the handler can stop on a single drop point.
func parseListFilters(c *gin.Context) (marketplace.ListFilters, *apierror.AppError) {
	filters := marketplace.ListFilters{
		Query:  strings.TrimSpace(c.Query("q")),
		Limit:  marketplaceDefaultLimit,
		Offset: 0,
	}

	// Sort. Empty string defaults to SortNewest — the Marketplace
	// homepage call must work without an explicit sort. Unknown values
	// are rejected here (400) rather than at the SQL boundary so the
	// error_code is stable and the database is not consulted.
	sortRaw := c.Query("sort")
	if sortRaw == "" {
		filters.Sort = marketplace.SortNewest
	} else {
		sort, ok := validSortValues[sortRaw]
		if !ok {
			return marketplace.ListFilters{}, &apierror.AppError{
				Code:      http.StatusBadRequest,
				Message:   "Bad Request",
				Detail:    fmt.Sprintf("sort %q is not one of newest, most_imported, recently_updated, alphabetic", sortRaw),
				ErrorCode: "invalid_sort",
			}
		}
		filters.Sort = sort
	}

	// License filter. Gin's QueryArray returns every value of a
	// repeated parameter, supporting both `?license=MIT&license=Apache-2.0`
	// and `?license[]=MIT&license[]=Apache-2.0` shapes. We validate
	// each against the canonical allow-list — a typo would otherwise
	// silently return zero rows, which is a bad failure mode for
	// callers (no result vs. a 400 they can fix).
	for _, lic := range c.QueryArray("license") {
		if lic == "" {
			continue
		}
		if !marketplace.IsAllowedLicense(lic) {
			return marketplace.ListFilters{}, &apierror.AppError{
				Code:      http.StatusBadRequest,
				Message:   "Bad Request",
				Detail:    fmt.Sprintf("license %q is not in the SPDX allow-list", lic),
				ErrorCode: "invalid_license",
			}
		}
		filters.Licenses = append(filters.Licenses, lic)
	}

	// Limit. A non-numeric value is treated as "use the default"
	// rather than 400; this matches how the rest of the API handles
	// optional pagination knobs. The hard cap mirrors the SQL function
	// clamp at 50 — defence in depth.
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			if n < 1 {
				n = marketplaceDefaultLimit
			} else if n > marketplaceMaxLimit {
				n = marketplaceMaxLimit
			}
			filters.Limit = n
		}
	}

	// Offset. Negative or non-numeric becomes 0 (the SQL function
	// already clamps to >= 0; we mirror that so the response envelope's
	// next_offset arithmetic stays positive).
	if raw := c.Query("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			filters.Offset = n
		}
	}

	return filters, nil
}

// toMarketplaceListItem projects the Go-side marketplace.ListItem into
// its JSON wire shape. We format LastModifiedAt as RFC3339 so the
// frontend can pass it back through Date.parse without parsing pgx's
// time precision quirks; the marketplace.ListItem field is a Go
// time.Time so we don't need to keep two clock representations in sync.
func toMarketplaceListItem(it marketplace.ListItem) MarketplaceListItem {
	return MarketplaceListItem{
		KBID:                 it.KBID,
		OrgSlug:              it.OrgSlug,
		OrgDisplayName:       it.OrgDisplayName,
		KBSlug:               it.KBSlug,
		KBName:               it.KBName,
		Description:          it.Description,
		LicenseSPDXID:        it.LicenseSPDXID,
		LastModifiedAt:       it.LastModifiedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		ImportCount:          it.ImportCount,
		SourcePublicKBID:     it.SourcePublicKBID,
		SourceOrgSlug:        it.SourceOrgSlug,
		SourceOrgDisplayName: it.SourceOrgDisplayName,
	}
}
