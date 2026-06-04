package marketplace

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SlugHoldStatus is the typed return shape for SlugHoldStatusLookup.
// A small struct beats a (uuid, bool, error) chain at call sites: the
// 410-Gone handler in #731 branches on `IsHeld` and reaches for
// `KBID` for the audit row, and giving each a name makes the call
// site read like English. Leaves room to add fields (e.g. the
// canonical published-at the visitor's link expected to find) without
// breaking signatures, exactly as `ResolvedOrg` does for org slugs.
type SlugHoldStatus struct {
	// OrgID is the publishing Org. Always populated when the lookup
	// succeeds, even when IsHeld is false — the caller usually wants
	// to know that the Org slug resolved, so it can render a "no
	// such KB in this org" 404 instead of a generic miss.
	OrgID uuid.UUID
	// KBID is the KB that vacated the slug, or uuid.Nil if the hold
	// row carries a NULL kb_id (the KB was hard-deleted after the
	// hold was recorded). Only populated when IsHeld is true.
	KBID uuid.UUID
	// IsHeld is true when (orgSlug, kbSlug) currently maps to a row
	// in `kb_slug_holds` whose `held_until > now()`. The 410-Gone
	// path branches on this single field — keeping the contract
	// thin lets the handler stay a single switch.
	IsHeld bool
}

// ErrKBSlugNotFound is returned by LookupKBSlugHold when the Org slug
// does not resolve to a live Org (or active hold), OR when the Org
// exists but the KB slug has no current hold row. The Marketplace
// detail/preview handlers turn this into HTTP 404 — distinguishing it
// from the `IsHeld=true` path that turns into 410.
var ErrKBSlugNotFound = errors.New("marketplace: kb slug not found")

// LookupKBSlugHold answers the question "does (orgSlug, kbSlug)
// currently resolve to a 90-day-held URL, or is it a 404?". Used by
// the Marketplace detail and preview handlers (#731) to render the
// canonical 410 Gone — the helper deliberately does NOT consult
// `knowledge_bases` itself, because resolution of a live Public KB is
// already owned by the listing/detail handlers and overlapping that
// logic here would duplicate routing.
//
// Resolution order:
//  1. Resolve orgSlug via ResolveOrgSlug. A miss surfaces
//     ErrKBSlugNotFound so the caller renders 404 (not 410).
//  2. Probe `kb_slug_holds` for (org_id, kbSlug, held_until > NOW()).
//     A hit returns IsHeld=true; a miss returns ErrKBSlugNotFound.
//
// Expired rows (`held_until <= NOW()`) are treated as not-held even
// if the sweeper has not yet deleted the row. This keeps the 90-day
// window honest regardless of cleanup-job latency — identical to how
// `ResolveOrgSlug` treats expired `org_slug_holds`.
func LookupKBSlugHold(
	ctx context.Context,
	q SlugQuerier,
	orgSlug, kbSlug string,
) (SlugHoldStatus, error) {
	// Cheap input validation. The kb-slug shape matches the org-slug
	// regex by ADR-0007 Q7e — refusing junk shapes here saves a DB
	// round trip for noise traffic.
	if !IsValidSlug(orgSlug) || !IsValidSlug(kbSlug) {
		return SlugHoldStatus{}, ErrKBSlugNotFound
	}

	resolved, err := ResolveOrgSlug(ctx, q, orgSlug)
	if err != nil {
		if errors.Is(err, ErrSlugNotFound) {
			return SlugHoldStatus{}, ErrKBSlugNotFound
		}
		return SlugHoldStatus{}, fmt.Errorf("LookupKBSlugHold: resolve org: %w", err)
	}

	orgUUID, err := uuid.Parse(resolved.OrgID)
	if err != nil {
		// ResolveOrgSlug returned a non-UUID string — this is a hard
		// invariant violation from the organizations row. Surface as
		// an internal error rather than 404, so the alerting catches it.
		return SlugHoldStatus{}, fmt.Errorf("LookupKBSlugHold: parse org id %q: %w", resolved.OrgID, err)
	}

	var (
		heldKBID *uuid.UUID
	)
	err = q.QueryRow(ctx,
		`SELECT kb_id
		   FROM kb_slug_holds
		  WHERE org_id     = $1
		    AND slug       = $2
		    AND held_until > NOW()`,
		orgUUID, kbSlug,
	).Scan(&heldKBID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Org resolved but no active hold for this kb slug. Caller
			// renders 404. We still populate OrgID so the handler can
			// avoid a second probe when distinguishing miss types.
			return SlugHoldStatus{OrgID: orgUUID}, ErrKBSlugNotFound
		}
		return SlugHoldStatus{}, fmt.Errorf("LookupKBSlugHold: probe holds: %w", err)
	}

	status := SlugHoldStatus{OrgID: orgUUID, IsHeld: true}
	if heldKBID != nil {
		status.KBID = *heldKBID
	}
	return status, nil
}
