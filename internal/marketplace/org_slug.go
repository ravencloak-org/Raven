// Package marketplace owns the cross-cutting helpers for Raven's public
// Marketplace surface: slug normalisation, collision-suffix logic, and the
// soft-redirect lookup against `org_slug_holds`.
//
// The slug shape matches the CHECK constraint on `organizations.slug` and the
// `org_slug_holds.slug` column (migration 00048) — the regex
//
//	^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$
//
// Slugify is the canonical normaliser. It lives here so the migration-time
// PL/pgSQL backfill, the repository's RenameSlug method, and any future
// admin/API code path all produce identical output for the same input.
package marketplace

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgUniqueViolation is the SQLSTATE for a UNIQUE constraint violation; the
// only source of one inside RenameOrgSlug's UPDATE path is the organizations
// slug unique index, which we surface as ErrSlugTaken so callers can retry.
const pgUniqueViolation = "23505"

// MaxSlugLen is the maximum length of an Org slug in characters. It matches
// the VARCHAR(64) cap and the {0,62} body length in slugRegex (1 leading +
// up-to-62 body + 1 trailing = 64).
const MaxSlugLen = 64

// RedirectHold is the duration a renamed slug is held in `org_slug_holds`
// so external links to the old slug continue to resolve. ADR-0007 Q7e.
const RedirectHold = 30 * 24 * time.Hour

// slugRegex enforces URL-safety: must start and end with [a-z0-9], may
// contain dashes in between, length 1..64.
var slugRegex = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

// nonAlnumRun matches runs of any non-alphanumeric character. The replacement
// pass collapses them into a single dash. Together with the lowercasing pass
// this matches the SQL `regexp_replace(s, '[^a-z0-9]+', '-', 'g')` step.
var nonAlnumRun = regexp.MustCompile(`[^a-z0-9]+`)

// trailingDashRun strips trailing dashes after clipping. We avoid a leading
// counterpart by using TrimLeft below.
var trailingDashRun = regexp.MustCompile(`-+$`)

// Slugify normalises a free-form string into an Org-slug-shaped string:
// lowercased, alphanumeric+dash, with internal non-alphanumeric runs
// collapsed to a single dash. Output is clipped to MaxSlugLen, with any
// resulting trailing dash stripped.
//
// The returned string may be empty if the input contained no alphanumeric
// characters; callers decide how to handle that (typically a fallback to
// a deterministic id-derived stem — see the migration backfill).
//
// Slugify on its own does NOT guarantee uniqueness. Use SlugifyWithSuffix
// to produce a collision-free slug against a known set of taken slugs.
func Slugify(input string) string {
	if input == "" {
		return ""
	}
	s := strings.ToLower(input)
	s = nonAlnumRun.ReplaceAllString(s, "-")
	s = strings.TrimLeft(s, "-")
	s = trailingDashRun.ReplaceAllString(s, "")
	if len(s) > MaxSlugLen {
		s = s[:MaxSlugLen]
		s = trailingDashRun.ReplaceAllString(s, "")
	}
	return s
}

// IsValidSlug reports whether s satisfies the URL-safe Org slug shape. Use
// this in admin / API validation; the database CHECK is the backstop.
func IsValidSlug(s string) bool {
	return slugRegex.MatchString(s)
}

// SlugifyWithSuffix returns Slugify(input) if that slug is not in `taken`,
// otherwise appends `-2`, `-3`, ... until a free slug is found. The
// collision-suffix algorithm matches the migration-time backfill so an
// org first slugified by the migration and a later org slugified by Go
// code produce the same sequence of candidates.
//
// The result is clipped so the suffixed slug never exceeds MaxSlugLen:
// when the base length plus suffix length would overflow, the base is
// shortened first.
func SlugifyWithSuffix(input string, taken map[string]struct{}) string {
	base := Slugify(input)
	if base == "" {
		return ""
	}
	if _, hit := taken[base]; !hit {
		return base
	}
	for n := 2; ; n++ {
		suffix := fmt.Sprintf("-%d", n)
		room := MaxSlugLen - len(suffix)
		if room < 1 {
			// Pathological: suffix alone exceeds the cap. Caller passed a
			// bizarre `taken` set. Bail with the unsuffixed base — the
			// CHECK will reject any further attempt to insert it.
			return base
		}
		stem := base
		if len(stem) > room {
			stem = trailingDashRun.ReplaceAllString(stem[:room], "")
		}
		candidate := stem + suffix
		if _, hit := taken[candidate]; !hit {
			return candidate
		}
	}
}

// ---------------------------------------------------------------------------
// Soft-redirect resolution.
// ---------------------------------------------------------------------------

// ResolvedOrg is the typed return shape for ResolveOrgSlug. A small struct
// is clearer at the call site than a (uuid, bool, error) chain and leaves
// room to add fields (e.g. canonical slug for the 301 Location header)
// without breaking signatures.
type ResolvedOrg struct {
	// OrgID is the resolved organization's id.
	OrgID string
	// IsRedirect is true when the slug was matched via `org_slug_holds`
	// (i.e. the slug is a stale alias and the Marketplace URL handler
	// should issue a 301 to the current slug).
	IsRedirect bool
	// CanonicalSlug is the current slug on `organizations` — equal to the
	// requested slug when IsRedirect is false, the new slug when IsRedirect
	// is true.
	CanonicalSlug string
}

// ErrSlugNotFound is returned by ResolveOrgSlug when neither a live org row
// nor an active hold matches the requested slug. The Marketplace URL
// handler turns this into HTTP 404.
var ErrSlugNotFound = errors.New("marketplace: org slug not found")

// SlugQuerier is the minimal interface ResolveOrgSlug needs from pgx. Both
// *pgxpool.Pool and pgx.Tx satisfy it, which keeps the resolver usable
// inside transactions (e.g. for tests that wrap-and-rollback).
type SlugQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ResolveOrgSlug maps a URL slug to an Org, returning whether the lookup
// went via the active row or the soft-redirect hold table. The active row
// is checked first so the happy path is a single index probe; the hold
// table is consulted only on miss.
//
// Holds whose `held_until <= now()` are treated as expired — the function
// returns ErrSlugNotFound rather than a stale redirect, even if a sweeper
// has not yet deleted the row. This makes the 30-day window honest
// regardless of cleanup-job latency.
func ResolveOrgSlug(ctx context.Context, q SlugQuerier, slug string) (ResolvedOrg, error) {
	// Cheap input validation — refuse to query the database for a slug we
	// know cannot exist. Guards against incidental SQL load from junk URLs.
	if !IsValidSlug(slug) {
		return ResolvedOrg{}, ErrSlugNotFound
	}

	var orgID string
	err := q.QueryRow(ctx,
		`SELECT id FROM organizations WHERE slug = $1 AND status != 'deactivated'`,
		slug,
	).Scan(&orgID)
	if err == nil {
		return ResolvedOrg{OrgID: orgID, IsRedirect: false, CanonicalSlug: slug}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ResolvedOrg{}, fmt.Errorf("ResolveOrgSlug: live lookup: %w", err)
	}

	// Live miss — try the hold table. We join back to `organizations` so
	// the caller gets the canonical slug for the 301 in a single round
	// trip, and so a hold whose target org has been deactivated is treated
	// as a miss (no point redirecting to a tombstoned org).
	var canonical string
	err = q.QueryRow(ctx,
		`SELECT h.org_id, o.slug
		 FROM org_slug_holds h
		 JOIN organizations o ON o.id = h.org_id
		 WHERE h.slug = $1
		   AND h.held_until > NOW()
		   AND o.status != 'deactivated'`,
		slug,
	).Scan(&orgID, &canonical)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResolvedOrg{}, ErrSlugNotFound
		}
		return ResolvedOrg{}, fmt.Errorf("ResolveOrgSlug: hold lookup: %w", err)
	}
	return ResolvedOrg{OrgID: orgID, IsRedirect: true, CanonicalSlug: canonical}, nil
}

// ---------------------------------------------------------------------------
// Atomic rename.
// ---------------------------------------------------------------------------

// ErrSlugTaken is returned by RenameOrgSlug when `newSlug` is already in use
// by another live org OR is currently held by `org_slug_holds`. The
// Marketplace URL space is global; an active hold blocks reuse for its
// full 30-day window.
var ErrSlugTaken = errors.New("marketplace: slug is taken or held")

// ErrInvalidSlug is returned by RenameOrgSlug when newSlug fails IsValidSlug.
var ErrInvalidSlug = errors.New("marketplace: slug is not URL-safe")

// RenameOrgSlug atomically swaps an Org's slug and registers the previous
// value in `org_slug_holds` with `held_until = now() + 30 days`. All three
// steps run in a single transaction so a crash mid-rename can never leave
// a dead redirect (an old slug recorded for a slug that wasn't actually
// replaced) nor a missing one.
//
// Returns ErrInvalidSlug if newSlug is malformed, ErrSlugTaken if it is
// already claimed, and ErrSlugNotFound if orgID does not resolve to a
// live org. Idempotent only insofar as renaming to the current slug is a
// no-op (no hold row is written).
func RenameOrgSlug(ctx context.Context, pool *pgxpool.Pool, orgID, newSlug string) error {
	if !IsValidSlug(newSlug) {
		return ErrInvalidSlug
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("RenameOrgSlug: begin: %w", err)
	}
	// Rollback is a no-op if Commit succeeded.
	defer func() { _ = tx.Rollback(ctx) }()

	// Read the current slug, locking the org row so two concurrent
	// renames can't race against the uniqueness invariant.
	var oldSlug string
	err = tx.QueryRow(ctx,
		`SELECT slug FROM organizations WHERE id = $1 AND status != 'deactivated' FOR UPDATE`,
		orgID,
	).Scan(&oldSlug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSlugNotFound
		}
		return fmt.Errorf("RenameOrgSlug: load org: %w", err)
	}
	if oldSlug == newSlug {
		// Idempotent no-op. Don't record a hold for the same slug — that
		// would block the org from ever renaming back to it.
		return tx.Commit(ctx)
	}

	// Reject if the new slug is live elsewhere or held. We do both checks
	// inside the transaction so the eventual writes can't race a hold
	// inserted between check and write.
	var taken bool
	err = tx.QueryRow(ctx,
		`SELECT EXISTS (
		    SELECT 1 FROM organizations WHERE slug = $1 AND id <> $2
		    UNION ALL
		    SELECT 1 FROM org_slug_holds  WHERE slug = $1 AND held_until > NOW()
		 )`,
		newSlug, orgID,
	).Scan(&taken)
	if err != nil {
		return fmt.Errorf("RenameOrgSlug: collision check: %w", err)
	}
	if taken {
		return ErrSlugTaken
	}

	// Record the old slug as a 30-day soft-redirect. ON CONFLICT updates
	// the held_until — if this slug was held by the same org previously
	// (rename back-and-forth), refresh the window rather than fail.
	if _, err := tx.Exec(ctx,
		`INSERT INTO org_slug_holds (slug, org_id, held_until)
		 VALUES ($1, $2, NOW() + $3::interval)
		 ON CONFLICT (slug) DO UPDATE
		   SET org_id = EXCLUDED.org_id,
		       held_until = EXCLUDED.held_until`,
		oldSlug, orgID, fmt.Sprintf("%d seconds", int(RedirectHold.Seconds())),
	); err != nil {
		return fmt.Errorf("RenameOrgSlug: insert hold: %w", err)
	}

	// Swap the slug. If the new slug collides at the UNIQUE index, the
	// pre-flight EXISTS check missed a concurrent insert — return
	// ErrSlugTaken so the caller can retry with a fresh suffix.
	if _, err := tx.Exec(ctx,
		`UPDATE organizations SET slug = $1 WHERE id = $2`,
		newSlug, orgID,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return ErrSlugTaken
		}
		return fmt.Errorf("RenameOrgSlug: update slug: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("RenameOrgSlug: commit: %w", err)
	}
	return nil
}
