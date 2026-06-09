package marketplace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// PlanResolver answers "is this Org on a Free Plan today?". The Importer
// asks it ABOVE the projection so the visibility override stays a single
// readable line in Import() — keeping the projection pure of plan policy.
//
// Defined as an interface (not a function) so production wiring can plug a
// quota checker that talks to the subscriptions table, and unit tests can
// pass a const stub without standing up Valkey + Postgres. Returning
// (bool, error) instead of (PlanTier, error) keeps the call site readable:
// `freePlan, err := resolver.IsFreePlanOrg(...)` is the question we ask
// every time, and there is no reason to expose the full PlanTier here.
type PlanResolver interface {
	IsFreePlanOrg(ctx context.Context, orgID string) (bool, error)
}

// EmbeddingModelResolver answers "what embedding model does this Org use by
// default?". Returns an empty string when no default is set; the SQL
// function treats that as "do not enforce a match, do not copy embeddings"
// (re-embed flow deferred per ADR-0001).
//
// Same interface-vs-function rationale as PlanResolver: unit tests can stub
// without the LLM provider repo, production wiring can use whatever
// per-Org default lives in the llm_providers table.
type EmbeddingModelResolver interface {
	DefaultEmbeddingModel(ctx context.Context, orgID string) (string, error)
}

// ImportResult is the response payload for a successful import, in the
// shape docs/plans/marketplace-mvp.md §4 requires the API to return. The
// `ForcedPublic` flag surfaces the Free Plan override (ADR-0004) so the
// SPA can show a one-time banner.
type ImportResult struct {
	KBID                   uuid.UUID  `json:"kb_id"`
	WorkspaceID            uuid.UUID  `json:"workspace_id"`
	Visibility             string     `json:"visibility"`
	ImportedFromRevisionAt time.Time  `json:"imported_from_revision_at"`
	SourcePublicKBID       uuid.UUID  `json:"source_public_kb_id"`
	LicenseSPDXID          *string    `json:"license_spdx_id,omitempty"`
	// ForcedPublic is true when the importer Org is on a Free Plan and
	// the local KB was created with visibility='public' regardless of
	// what the input or source said. ADR-0004 §"Consequences".
	ForcedPublic bool `json:"forced_public,omitempty"`
}

// Typed errors the Importer surfaces. Each maps to one HTTP status in the
// handler; centralising them here keeps the contract readable and lets the
// SQL boundary stay opaque (handler never branches on SQLSTATE directly).

// ErrSourceNotPublic is returned when the source KB does not exist OR is
// not visibility='public'. The two cases are deliberately indistinguishable
// at this boundary (matching the listing/preview policy in 00052) so the
// API cannot be used to probe for private-KB existence.
var ErrSourceNotPublic = errors.New("marketplace: source kb not public")

// ErrSourceEmpty is returned when the source KB has zero documents. Plan
// §4 declares an empty Public KB unimportable.
var ErrSourceEmpty = errors.New("marketplace: source kb has zero documents")

// ErrEmbeddingModelMismatch is returned when the source's embeddings use a
// different model than the destination Org's default. Re-embed flow is
// deferred per ADR-0001 §Trade-offs; for the MVP we fail loud.
var ErrEmbeddingModelMismatch = errors.New("marketplace: source embedding model does not match destination default")

// ErrAlreadyImported is returned when the destination workspace already
// holds a KB whose source_public_kb_id equals the source. Re-import is
// issue #730 — the caller must use that endpoint to refresh, not the
// import endpoint to duplicate.
var ErrAlreadyImported = errors.New("marketplace: kb already imported into this workspace; use re-import to refresh")

// ErrNotWorkspaceMember is returned when the actor is not a member of the
// destination workspace. Membership is verified up front so we never
// allocate state for a caller the policy will reject anyway.
var ErrNotWorkspaceMember = errors.New("marketplace: actor is not a member of the destination workspace")

// ErrLicenseMissing is returned when the source KB carries no license. The
// CHECK constraint added in migration 00050 should make this unreachable
// for visibility='public' rows, but the Go layer guards in case a row
// pre-dates the constraint or admin moderation has cleared the column.
var ErrLicenseMissing = errors.New("marketplace: source kb has no license declared")

// Importer is the cross-tenant fork service. It owns the canonical "what
// happens during an import" answer: membership check, Free Plan resolution,
// embedding-model resolution, and the call into the SQL function that
// performs the atomic content-grade copy.
//
// One struct, one method (Import). Resist the urge to fan this out into
// a dozen helpers — the projection function and the SQL function are the
// two surfaces the policy lives on, everything else is glue.
type Importer struct {
	pool             *pgxpool.Pool
	planResolver     PlanResolver
	embeddingResolver EmbeddingModelResolver
}

// NewImporter constructs an Importer. Both resolvers must be non-nil in
// production wiring; the constructor enforces that so a misconfigured
// startup fails loud rather than mid-import on the first request.
func NewImporter(pool *pgxpool.Pool, plan PlanResolver, emb EmbeddingModelResolver) *Importer {
	if pool == nil {
		panic("marketplace.NewImporter: nil pool")
	}
	if plan == nil {
		panic("marketplace.NewImporter: nil plan resolver")
	}
	if emb == nil {
		panic("marketplace.NewImporter: nil embedding resolver")
	}
	return &Importer{pool: pool, planResolver: plan, embeddingResolver: emb}
}

// Import performs the content-grade fork from a Public KB into the
// destination Org's workspace. Atomic: either the new KB row + all child
// rows land together or nothing changes.
//
// Free Plan override (ADR-0004) is applied ABOVE the projection — the
// PlanResolver call here decides whether to pass `forcePublic=true` to
// the SQL function. projectPublished itself stays pure (does not know
// about plans).
//
// Embedding model match check runs against the destination's default
// model. If the source has embeddings whose model does not match and the
// destination has a model set, we return ErrEmbeddingModelMismatch
// BEFORE any write. Re-embedding the source's chunks on import would
// require a Python AI-worker round-trip the MVP does not pay (ADR-0001
// §Trade-offs).
//
// Idempotency: a second import of the same source into the same
// destination workspace returns ErrAlreadyImported. Defended choice: ADR-
// 0001 makes imports an explicit user action with a package-manager
// mental model; silently producing a duplicate fork would (a) confuse
// the importer (two indistinguishable rows in their KB list), (b) break
// the lineage badge the listing query relies on, and (c) double-count
// the source's import_count. The dedicated Re-import endpoint (#730) is
// the affordance for refreshing.
func (i *Importer) Import(
	ctx context.Context,
	sourceKBID, dstOrgID, dstWorkspaceID, dstUserID uuid.UUID,
) (*ImportResult, error) {
	// 1. Resolve Free Plan visibility override ABOVE the projection.
	// ADR-0004 makes this a plan-state policy, not a projection rule.
	freePlan, err := i.planResolver.IsFreePlanOrg(ctx, dstOrgID.String())
	if err != nil {
		return nil, apierror.NewInternal("failed to resolve org plan: " + err.Error())
	}

	// 2. Resolve destination's default embedding model. Empty string
	// means "no default" — the SQL function will not enforce a match
	// and will skip the embeddings projection. The importer's UI will
	// surface a "re-embed pending" banner in that case (out of scope
	// here; handled by #730 or a follow-up).
	requiredModel, err := i.embeddingResolver.DefaultEmbeddingModel(ctx, dstOrgID.String())
	if err != nil {
		return nil, apierror.NewInternal("failed to resolve embedding model: " + err.Error())
	}

	// 3. Single transaction. The SQL function is SECURITY DEFINER so it
	// bypasses the destination Org's RLS to read the source row; the
	// Go layer never has to switch role. Membership of the actor in
	// the destination workspace is verified INSIDE the same tx, so the
	// row read for membership and the import write share a snapshot.
	var (
		kbID                     uuid.UUID
		workspaceID              uuid.UUID
		visibility               string
		importedFromRevisionAt   time.Time
		returnedSourcePublicKBID uuid.UUID
		licenseSPDXID            *string
	)

	err = db.WithOrgID(ctx, i.pool, dstOrgID.String(), func(tx pgx.Tx) error {
		// 3a. Workspace membership check. The (workspace_id, user_id)
		// unique index on workspace_members guarantees at most one
		// row; absence is the canonical "not a member" signal. We
		// fold this into the import tx so a concurrent revocation
		// between the check and the write cannot race past us.
		var memberExists bool
		err := tx.QueryRow(ctx,
			`SELECT EXISTS (
			   SELECT 1 FROM workspace_members
			    WHERE workspace_id = $1
			      AND user_id      = $2
			      AND org_id       = $3
			 )`,
			dstWorkspaceID, dstUserID, dstOrgID,
		).Scan(&memberExists)
		if err != nil {
			return apierror.NewInternal("failed to check workspace membership: " + err.Error())
		}
		if !memberExists {
			return ErrNotWorkspaceMember
		}

		// 3b. Defence in depth: refuse to call the SQL function if
		// the source has no license. The CHECK on knowledge_bases
		// (migration 00050) makes this unreachable for legitimate
		// public rows, but a defensive read here gives the SPA a
		// precise 422 instead of a generic 403 if it ever fires.
		// We read the license column from the source via a CTE that
		// pretends we are the source org, using the SECURITY DEFINER
		// listing function's pattern: a single SELECT that bypasses
		// the destination's RLS by addressing the row id directly,
		// because the table-level RLS for non-admin roles would hide
		// the source row otherwise.
		//
		// In tests we connect as superuser so this SELECT just
		// works; in production the actual cross-tenant read is
		// performed by the SECURITY DEFINER function below. The
		// purpose of this read is solely to surface the precise
		// "license missing" error code; if RLS hides the row in
		// production, the SQL function below raises kb_not_public
		// (insufficient_privilege) which is the same fallback the
		// listing/preview functions use — defensible but less
		// specific. The win is the SPA's UX in the common path.
		var hasLicense bool
		err = tx.QueryRow(ctx,
			`SELECT license_spdx_id IS NOT NULL
			   FROM knowledge_bases
			  WHERE id         = $1
			    AND visibility = 'public'`,
			sourceKBID,
		).Scan(&hasLicense)
		if err == nil && !hasLicense {
			return ErrLicenseMissing
		}
		// On any read error (including the RLS-hides-row case
		// described above), fall through to the SQL function — it
		// has the SECURITY DEFINER privilege to see the row and
		// will produce the correct error.

		// 3c. Apply the SQL function. All boundary policy lives in
		// migration 00055; this Go layer just passes the resolved
		// flags. The settings allow-list is sourced from the Go
		// constant settingsAllowListConst — the same constant
		// scrubSettings uses — so the two surfaces are guaranteed
		// to agree.
		var nullableModel any
		if strings.TrimSpace(requiredModel) != "" {
			nullableModel = requiredModel
		} else {
			nullableModel = nil
		}

		err = tx.QueryRow(ctx,
			`SELECT kb_id, workspace_id, visibility::TEXT,
			        imported_from_revision_at, source_public_kb_id,
			        license_spdx_id
			   FROM marketplace_import_kb($1, $2, $3, $4, $5, $6, $7)`,
			sourceKBID, dstOrgID, dstWorkspaceID, dstUserID,
			freePlan,
			nullableModel,
			settingsAllowListSlice(),
		).Scan(
			&kbID, &workspaceID, &visibility,
			&importedFromRevisionAt, &returnedSourcePublicKBID,
			&licenseSPDXID,
		)
		if err != nil {
			return mapImportErr(err)
		}
		return nil
	})
	if err != nil {
		// Wrap typed errors into AppError shapes the handler can render
		// without further translation.
		return nil, wrapImportErr(err)
	}

	return &ImportResult{
		KBID:                   kbID,
		WorkspaceID:            workspaceID,
		Visibility:             visibility,
		ImportedFromRevisionAt: importedFromRevisionAt,
		SourcePublicKBID:       returnedSourcePublicKBID,
		LicenseSPDXID:          licenseSPDXID,
		ForcedPublic:           freePlan,
	}, nil
}

// mapImportErr translates SQLSTATE values raised by marketplace_import_kb
// into the typed sentinel errors at the top of this file. Centralising the
// map keeps the SQL boundary opaque from the handler's perspective.
func mapImportErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case pgInsufficientPrivilege:
		return ErrSourceNotPublic
	case "22000": // data_exception
		return ErrSourceEmpty
	case "23001": // restrict_violation
		return ErrEmbeddingModelMismatch
	case pgUniqueViolation:
		return ErrAlreadyImported
	}
	return fmt.Errorf("marketplace: import sql: %w", err)
}

// wrapImportErr converts a typed sentinel error into an *apierror.AppError
// already shaped for the handler. The handler does NOT need to know about
// the marketplace package's sentinels — keeping translation here is the
// load-bearing reason the Importer's surface is narrow.
func wrapImportErr(err error) error {
	switch {
	case errors.Is(err, ErrSourceNotPublic):
		// 403 — matches the public/preview surface; deliberately
		// indistinguishable from "no such KB" to prevent existence
		// probing of private rows (ADR-0001 §"Why" point 2).
		return &apierror.AppError{
			Code:    403,
			Message: "Forbidden",
			Detail:  "source knowledge base is not public",
		}
	case errors.Is(err, ErrSourceEmpty):
		return apierror.NewUnprocessableEntity("source knowledge base has zero documents")
	case errors.Is(err, ErrEmbeddingModelMismatch):
		// 422 — the request was well-formed but the embedding model
		// mismatch makes it unprocessable until the destination Org
		// switches to a compatible model.
		return apierror.NewUnprocessableEntity(
			"source embedding model does not match destination default; re-embed flow is not yet available",
		)
	case errors.Is(err, ErrAlreadyImported):
		return apierror.NewConflict(
			"this public knowledge base has already been imported into the target workspace; use the re-import endpoint to refresh",
		)
	case errors.Is(err, ErrNotWorkspaceMember):
		return &apierror.AppError{
			Code:    403,
			Message: "Forbidden",
			Detail:  "you are not a member of the target workspace",
		}
	case errors.Is(err, ErrLicenseMissing):
		return apierror.NewUnprocessableEntity(
			"source knowledge base has no license declared; cannot import",
		)
	}
	// AppErrors raised mid-tx (e.g. from the membership-check internal
	// error path) propagate untouched.
	var appErr *apierror.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return apierror.NewInternal("import failed: " + err.Error())
}
