package marketplace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// PublishGateFunc is the callback shape the publish service uses to ask
// the lifecycle freeze gate "may this KB transition private → public
// right now?". Defined as a function (rather than an interface that
// references internal/service.KBAction) so this package stays a leaf
// import-graph node: internal/repository already imports
// internal/marketplace for org-slug helpers, and internal/service imports
// internal/repository, so a direct dependency on internal/service from
// this package would create an import cycle.
//
// The callback returns nil when the KB may publish, an *apierror.AppError
// when the gate refuses, and any other error type for unexpected failures.
// The wiring site in cmd/api/main.go closes over service.KBStatusGuard
// and service.KBActionPublish to satisfy this shape with one line.
type PublishGateFunc func(ctx context.Context, tx pgx.Tx, orgID, kbID string) error

// marketplaceURLHost is the canonical host segment of the public Marketplace
// URL contract per ADR-0005. Kept as a code constant — the host is part of
// the documented user-facing surface and must not drift between deployments.
const marketplaceURLHost = "https://raven.ravencloak.org/marketplace"

// URL builds the public Marketplace URL for a Public KB given the
// publishing Org's slug and the KB's slug. Single source of truth so the
// handler response, future emails, and any audit log row produce
// byte-identical URLs. Named URL rather than MarketplaceURL because
// marketplace.MarketplaceURL would stutter at call sites; the package
// name already conveys the domain.
func URL(orgSlug, kbSlug string) string {
	return fmt.Sprintf("%s/%s/%s", marketplaceURLHost, orgSlug, kbSlug)
}

// PublishResult is the response payload for a successful publish, in the
// exact shape ADR-0002 / docs/plans/marketplace-mvp.md §4 require the API
// to return. Kept as a typed struct so the handler stays a thin renderer
// and the service owns the contract.
type PublishResult struct {
	Visibility     model.KBVisibility `json:"visibility"`
	PublishedAt    time.Time          `json:"published_at"`
	MarketplaceURL string             `json:"marketplace_url"`
}

// PublishService runs the transactional publish path: status-gate check,
// license allow-list check, atomic flip of visibility + published_at +
// published_by_user_id + license_spdx_id, and assembly of the typed
// response.
//
// The service deliberately owns the entire publish boundary — handler
// validates payload shape, then defers every business rule to here. That
// keeps the canonical "what does it mean to publish a KB" answer in one
// file (ADR-0002).
type PublishService struct {
	pool *pgxpool.Pool
	gate PublishGateFunc
}

// NewPublishService constructs a PublishService. The gate callback is
// invoked inside the publish transaction before any UPDATE; pass nil to
// disable the freeze check in unit-test wiring that only exercises the
// license-validation / SQL paths.
func NewPublishService(pool *pgxpool.Pool, gate PublishGateFunc) *PublishService {
	return &PublishService{pool: pool, gate: gate}
}

// PublishKB flips a KB from private to public and stamps the publish
// metadata. Returns the typed PublishResult on success or an *apierror.AppError
// already shaped for the handler:
//
//   - 404 — KB does not exist for this org.
//   - 409 — KBStatusGate rejects the action (kb_frozen / kb_dmca_locked).
//   - 422 — license_spdx_id is not in AllowedSPDXLicenses.
//
// userID is the actor's internal user UUID (set by the JWT middleware on
// the handler context). It is recorded on published_by_user_id for audit.
//
// The whole thing runs inside one db.WithOrgID transaction so the row read
// for org-slug resolution, the status check, and the UPDATE all share the
// same snapshot — no chance of a concurrent rename racing the URL we
// return.
func (s *PublishService) PublishKB(
	ctx context.Context,
	orgID, kbID, userID, licenseSPDXID string,
) (*PublishResult, error) {
	// License validation runs before we touch the database. There is no
	// reason to pay a round trip for a payload the allow-list will
	// reject anyway.
	if !IsAllowedLicense(licenseSPDXID) {
		return nil, apierror.NewUnprocessableEntity(
			fmt.Sprintf("license_spdx_id %q is not in the allow-list", licenseSPDXID),
		)
	}

	var result PublishResult

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Lifecycle gate (ADR-0004 / issue #725). Catches archived, frozen,
		// and DMCA-locked KBs with the canonical machine-readable error
		// codes the SPA already branches on. Runs before any UPDATE so a
		// frozen KB never sees its row touched.
		if s.gate != nil {
			if gateErr := s.gate(ctx, tx, orgID, kbID); gateErr != nil {
				return gateErr
			}
		}

		// The UPDATE + RETURNING does double duty:
		//   1. Verifies the KB exists for this org (RowsAffected == 0 →
		//      404). Bypassing GetByID keeps the path one round trip.
		//   2. Reads back the Org slug via JOIN in a CTE so the response
		//      URL is computed inside the same transaction.
		//
		// Status filter on the UPDATE keeps the freeze check (above) and
		// the SQL write defence-in-depth: even if the guard were ever
		// disabled, an archived row would be untouched.
		var publishedAt time.Time
		var orgSlug, kbSlug string
		err := tx.QueryRow(ctx,
			`WITH updated AS (
				UPDATE knowledge_bases
				   SET visibility           = 'public',
				       published_at         = NOW(),
				       published_by_user_id = $3,
				       license_spdx_id      = $4
				 WHERE id     = $1
				   AND org_id = $2
				   AND status = 'active'
				 RETURNING published_at, slug, org_id
			 )
			 SELECT u.published_at, o.slug, u.slug
			   FROM updated u
			   JOIN organizations o ON o.id = u.org_id`,
			kbID, orgID, userID, licenseSPDXID,
		).Scan(&publishedAt, &orgSlug, &kbSlug)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierror.NewNotFound("knowledge base not found")
			}
			// The CHECK constraint knowledge_bases_public_requires_license
			// or the partial UNIQUE idx_kb_public_org_slug_uniq are the
			// only realistic write-time failures here. The former should
			// be unreachable (we always set license_spdx_id), so a
			// constraint hit means a slug collision against another
			// already-public KB in this org — surface 409 so the SPA can
			// prompt for a rename.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
				return apierror.NewConflict(
					"slug collision: another public knowledge base in this org already owns this slug",
				)
			}
			return apierror.NewInternal("failed to publish knowledge base: " + err.Error())
		}

		result = PublishResult{
			Visibility:     model.KBVisibilityPublic,
			PublishedAt:    publishedAt,
			MarketplaceURL: URL(orgSlug, kbSlug),
		}
		return nil
	})

	if err != nil {
		// AppErrors from the guard / CHECK paths are already shaped; pass
		// them through untouched so the handler renders the right status
		// code + machine-readable error code.
		var appErr *apierror.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		// db.WithOrgID can wrap "no rows" coming out of LoadStatus into a
		// generic error string. Map that to a 404 rather than a 500.
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("knowledge base not found")
		}
		return nil, apierror.NewInternal("failed to publish knowledge base: " + err.Error())
	}

	return &result, nil
}
