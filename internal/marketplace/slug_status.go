// Marketplace KB slug-status resolution (issue #731 stub for #727).
//
// MarketplaceSlugStatus is the seam the Marketplace HTTP handlers
// (#731) use to translate a URL slug pair `(org_slug, kb_slug)` into one
// of three outcomes:
//
//   - HTTP 410 — slug is in `kb_slug_holds` (the publisher unpublished
//     within the hold window). ADR-0007 §"Slug hold semantics".
//   - HTTP 404 — slug pair does not resolve to any Public KB and is not
//     held. Includes "private KB exists at this org slug but no public
//     KB at this kb slug" — by design we can't distinguish that without
//     leaking existence.
//   - HTTP 200/403 — slug resolves to a Public KB; caller proceeds.
//
// Issue #727 will replace the resolution body with a direct join against
// `knowledge_bases` (via the unpublish-aware SECURITY DEFINER it
// introduces). Until then this file holds the stub:
//
//   - The hold check is a plain JOIN to `organizations` because
//     `kb_slug_holds` has no RLS (cross-tenant by design) and the orgs
//     table is publicly readable for this kind of lookup (mirrors the
//     pattern in ResolveOrgSlug).
//   - The KB detail lookup walks `marketplace_list_public_kbs` (the
//     existing SECURITY DEFINER from #728) page-by-page, filtering for
//     matching org_slug + kb_slug Go-side. Inefficient but correct, and
//     bounded by stubWalkPageCap pages so a pathological query cannot
//     stall a request indefinitely. The TODO below documents the
//     intended replacement.

package marketplace

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// stubWalkPageLimit and stubWalkPageCap bound the paginated walk in the
// stub MarketplaceSlugStatus. The defaults (50 × 10 = 500 KBs) cover the
// MVP Marketplace size with margin. Increase only if you understand the
// quadratic cost and have a measured reason — #727 removes the walk
// entirely.
const (
	stubWalkPageLimit = 50
	stubWalkPageCap   = 10
)

// SlugStatus is the resolved view of a `(org_slug, kb_slug)` Marketplace
// URL pair. Exactly one of `Detail` and `IsHeld` is meaningful at a time:
// when `IsHeld` is true the slug is in `kb_slug_holds` and the handler
// must return 410; otherwise `Detail` carries the public listing row
// (nil if neither lookup matched, which the handler maps to 404).
//
// We return the full ListItem here (not just the KBID) so the detail
// handler does not have to re-query the same row through a second
// SECURITY DEFINER call: the slug resolution and the detail body share
// one round trip in the common (200) case.
type SlugStatus struct {
	// KBID is the resolved KB's id when either the listing matched or
	// the hold row records a non-NULL kb_id. uuid.Nil otherwise.
	KBID uuid.UUID
	// IsHeld is true when an active row exists in `kb_slug_holds` for
	// the requested slug pair (`held_until > NOW()`). The handler maps
	// this to HTTP 410 with `error_code: "slug_held"`.
	IsHeld bool
	// Detail is the Public KB listing row matching the slug pair, or
	// nil when no Public KB matches (either because the slug is held,
	// or because the pair never resolved at all). When IsHeld is true,
	// callers must NOT inspect Detail — there is no live row to show.
	Detail *ListItem
}

// MarketplaceSlugStatus resolves a `(org_slug, kb_slug)` URL pair to a
// SlugStatus.
//
// Order of operations:
//  1. Probe `kb_slug_holds` for an active hold; if found, return
//     IsHeld=true immediately so the 410 fast-path doesn't pay for a
//     listing walk.
//  2. Walk `marketplace_list_public_kbs` for a matching Public KB.
//     Returns SlugStatus{Detail: nil} when no row matches (handler →
//     404).
//
// TODO(#727): replace the listing walk with a direct SECURITY DEFINER
// lookup (`marketplace_resolve_public_kb(org_slug, kb_slug)` or
// equivalent) once the unpublish path lands. The current implementation
// is O(rows / page) per call and exists only so the #731 handlers can
// ship before #727 merges.
//
// The exported name `MarketplaceSlugStatus` is fixed by the issue #731
// brief — #727 replaces this implementation in place. The revive
// stutter (marketplace.MarketplaceSlugStatus) is intentional and
// scoped to this transitional helper.
//
//nolint:revive // name fixed by #731 brief; replaced by #727.
func MarketplaceSlugStatus(ctx context.Context, pool *pgxpool.Pool, orgSlug, kbSlug string) (SlugStatus, error) {
	// Cheap input validation — refuse to touch the database for a slug
	// we know cannot exist. Matches the guard in ResolveOrgSlug; KB
	// slugs do not yet share the org slug regex (VARCHAR(100) in
	// migration 00007), so we only reject the obvious empty cases here
	// and let the listing function return zero rows for malformed
	// kb_slug values.
	if orgSlug == "" || kbSlug == "" {
		return SlugStatus{}, nil
	}

	// 1. Hold lookup. We join to `organizations` so a hold whose owning
	//    org has been deactivated is treated as a miss — mirrors the
	//    same defence in ResolveOrgSlug for org_slug_holds. Held rows
	//    short-circuit before we pay for the listing walk.
	var heldKBID *uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT h.kb_id
		 FROM kb_slug_holds h
		 JOIN organizations o ON o.id = h.org_id
		 WHERE o.slug = $1
		   AND h.slug = $2
		   AND h.held_until > NOW()
		   AND o.status != 'deactivated'`,
		orgSlug, kbSlug,
	).Scan(&heldKBID)
	switch {
	case err == nil:
		status := SlugStatus{IsHeld: true}
		if heldKBID != nil {
			status.KBID = *heldKBID
		}
		return status, nil
	case errors.Is(err, pgx.ErrNoRows):
		// Fall through to listing walk.
	default:
		return SlugStatus{}, fmt.Errorf("MarketplaceSlugStatus: hold lookup: %w", err)
	}

	// 2. Listing walk. We use SortAlphabetic for a stable order across
	//    pages so a row inserted between two `Query` calls cannot cause
	//    a duplicate or a skip in the matching probe. Each page is
	//    capped at stubWalkPageLimit, and we bail after stubWalkPageCap
	//    pages so a pathological large Marketplace does not stall a
	//    request indefinitely.
	q := NewQueries(pool)
	for page := 0; page < stubWalkPageCap; page++ {
		rows, err := q.ListPublicKBs(ctx, ListFilters{
			Sort:   SortAlphabetic,
			Limit:  stubWalkPageLimit,
			Offset: page * stubWalkPageLimit,
		})
		if err != nil {
			return SlugStatus{}, fmt.Errorf("MarketplaceSlugStatus: list walk page %d: %w", page, err)
		}
		for i := range rows {
			it := rows[i]
			if it.OrgSlug == orgSlug && it.KBSlug == kbSlug {
				return SlugStatus{KBID: it.KBID, Detail: &it}, nil
			}
		}
		if len(rows) < stubWalkPageLimit {
			// Reached the end of the listing — no further pages exist,
			// so the slug pair definitively does not resolve to a
			// Public KB. Map to 404 in the handler.
			break
		}
	}
	return SlugStatus{}, nil
}
