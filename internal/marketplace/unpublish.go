package marketplace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// UnpublishHold is the duration a KB's slug is held in `kb_slug_holds`
// after the publishing Org takes it down. ADR-0007 (90-day Marketplace
// lifecycle window). Kept as a package constant so the unpublish path,
// any future admin re-publish path, and the SlugHoldStatus helper agree
// on the boundary.
const UnpublishHold = 90 * 24 * time.Hour

// UnpublishResult is the response payload for a successful unpublish, in
// the shape `docs/plans/marketplace-mvp.md` §4 documents. The handler
// renders this verbatim — the service owns the contract.
type UnpublishResult struct {
	Visibility    model.KBVisibility `json:"visibility"`
	SlugHeldUntil time.Time          `json:"slug_held_until"`
}

// UnpublishService runs the transactional unpublish path: lifecycle-gate
// check, atomic flip of visibility + published_at, and 90-day slug-hold
// registration in `kb_slug_holds`.
//
// The service deliberately owns the entire unpublish boundary so the
// canonical "what does unpublishing do" answer lives in one file
// (ADR-0007). Existing import rows in other Orgs are unaffected: the
// `source_public_kb_id` FK on `knowledge_bases` does not reference the
// row's visibility column, so flipping visibility back to `private` does
// not touch downstream forks — verified in unpublish_test.go.
type UnpublishService struct {
	pool *pgxpool.Pool
	gate PublishGateFunc
}

// NewUnpublishService constructs an UnpublishService. The gate callback
// is reused from the publish path — both transitions share the same
// KBStatusGate.CanPublish semantics (DMCA-locked / archived KBs cannot
// be unpublished either; the publisher must contact admin). Pass nil to
// disable the freeze check in unit-test wiring that only exercises the
// SQL paths.
func NewUnpublishService(pool *pgxpool.Pool, gate PublishGateFunc) *UnpublishService {
	return &UnpublishService{pool: pool, gate: gate}
}

// UnpublishKB flips a KB from public back to private and registers a
// 90-day slug hold so external Marketplace links resolve to a clean
// 410 Gone for the duration. Returns the typed UnpublishResult on
// success or an *apierror.AppError already shaped for the handler:
//
//   - 404 — KB does not exist for this org OR is not currently public.
//   - 409 — KBStatusGate rejects the action (kb_frozen / kb_dmca_locked).
//
// orgID is the publishing Org; kbID the KB. No userID is recorded — the
// publish metadata (published_by_user_id) is left in place as historical
// audit; the unpublish actor is captured by the route's audit log layer,
// not by mutating the KB row.
//
// Runs inside one db.WithOrgID transaction so a crash mid-unpublish
// cannot leave the row visibility flipped without a hold registered
// (and vice versa). The hold's `held_until` is computed inside the
// transaction (`NOW() + UnpublishHold`) so the value the handler
// returns to the client matches what landed in the database to the
// microsecond.
func (s *UnpublishService) UnpublishKB(
	ctx context.Context,
	orgID, workspaceID, kbID string,
) (*UnpublishResult, error) {
	var result UnpublishResult

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Establish workspace-scoped ownership BEFORE the lifecycle gate.
		//
		// The SELECT ... FOR UPDATE filter `visibility = 'public'` is
		// load-bearing: unpublishing an already-private KB is a no-op
		// semantically, but the API contract returns 404 to distinguish
		// "you can't take down what's already down" from a successful
		// idempotent repeat. Catching it here keeps the slug-hold insert
		// from running with a stale (org_id, slug) pair.
		//
		// SECURITY: scope the predicate by workspace_id as well as
		// org_id. The route guard runs RequireWorkspaceRole on :ws_id,
		// so the caller is authorized for THAT workspace — but without
		// `workspace_id = $3` here a workspace-A admin could pass a
		// kb_id belonging to workspace B of the same Org and we'd
		// happily unpublish it. The filter turns that into a 404 so
		// the authorization scope matches the route's intent.
		//
		// The ownership check runs first so a caller scoped to one
		// workspace cannot probe lifecycle state of a KB in another
		// workspace of the same Org: an out-of-scope kb_id resolves to
		// 404 here before the gate can leak a 409 (kb_frozen /
		// kb_dmca_locked). FOR UPDATE locks the row so the visibility
		// flip below stays atomic with the gate decision.
		var slug string
		err := tx.QueryRow(ctx,
			`SELECT slug
			   FROM knowledge_bases
			  WHERE id           = $1
			    AND org_id       = $2
			    AND workspace_id = $3
			    AND visibility   = 'public'
			  FOR UPDATE`,
			kbID, orgID, workspaceID,
		).Scan(&slug)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierror.NewNotFound("knowledge base not found or not currently public")
			}
			return apierror.NewInternal("failed to unpublish knowledge base: " + err.Error())
		}

		// Lifecycle gate, run only after workspace ownership is proven.
		// CanPublish covers the symmetric case: a DMCA-locked or archived
		// KB cannot toggle visibility either direction — the publisher
		// must contact admin / restore.
		if s.gate != nil {
			if gateErr := s.gate(ctx, tx, orgID, kbID); gateErr != nil {
				return gateErr
			}
		}

		// Flip visibility. The row is already locked and proven to be
		// public + in-scope, so the bare id predicate is sufficient.
		if _, err := tx.Exec(ctx,
			`UPDATE knowledge_bases
			   SET visibility   = 'private',
			       published_at = NULL
			 WHERE id = $1`,
			kbID,
		); err != nil {
			return apierror.NewInternal("failed to unpublish knowledge base: " + err.Error())
		}

		// Register the 90-day hold. ON CONFLICT updates the row when an
		// older hold for the same (org_id, slug) is still present —
		// this happens when a KB is unpublished, re-published under the
		// same slug, then unpublished again inside the original window.
		// We refresh both the kb_id pointer (it may have changed if the
		// slug was reassigned to a different KB) and the held_until
		// timestamp so the second 90-day window runs from the second
		// unpublish, not the first.
		var heldUntil time.Time
		if err := tx.QueryRow(ctx,
			`INSERT INTO kb_slug_holds (org_id, slug, kb_id, held_until)
			 VALUES ($1, $2, $3, NOW() + $4::interval)
			 ON CONFLICT (org_id, slug) DO UPDATE
			   SET kb_id      = EXCLUDED.kb_id,
			       held_until = EXCLUDED.held_until
			 RETURNING held_until`,
			orgID, slug, kbID, fmt.Sprintf("%d seconds", int(UnpublishHold.Seconds())),
		).Scan(&heldUntil); err != nil {
			return apierror.NewInternal("failed to register slug hold: " + err.Error())
		}

		result = UnpublishResult{
			Visibility:    model.KBVisibilityPrivate,
			SlugHeldUntil: heldUntil,
		}
		return nil
	})

	if err != nil {
		// AppErrors from the guard / not-found path are already shaped;
		// pass them through untouched so the handler renders the right
		// status code + machine-readable error code.
		var appErr *apierror.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		// db.WithOrgID can wrap "no rows" coming out of an RLS context
		// preflight into a generic error string. Map that to 404 rather
		// than 500 so callers see the canonical contract.
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("knowledge base not found")
		}
		return nil, apierror.NewInternal("failed to unpublish knowledge base: " + err.Error())
	}

	return &result, nil
}
