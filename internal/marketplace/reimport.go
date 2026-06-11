// Package marketplace — reimport.go is the Re-import service: a destructive
// content overwrite of an Imported KB that preserves the destination KB id
// and every runtime artefact that FKs to it (chats, widgets, API keys,
// routing rules, webhooks, response cache). See:
//
//   - ADR-0001 — fork-on-import contract.
//   - ADR-0007 §7b — what is preserved vs overwritten.
//   - Issue #730 — endpoint definition.
//
// The service is structured as a single transaction wrapping four steps:
//
//  1. SELECT…FOR UPDATE the destination KB to lock it and read lineage.
//  2. Validate (a) the row is an import, (b) the source is still public.
//  3. DELETE every Source, Document, Chunk row scoped by knowledge_base_id.
//     Embeddings are removed by ON DELETE CASCADE from chunks.
//  4. Call the projection to refill Sources / Documents / Chunks from the
//     publisher's current state.
//  5. Bump imported_from_revision_at to the publisher's last_modified_at.
//
// Step 1 is FOR UPDATE so a concurrent Re-import or content edit on the same
// KB serialises behind us — there is no "half re-imported" state visible to
// reads from another connection because the destructive deletes and the
// projection both live inside the same transaction.
package marketplace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
)

// ErrNotAnImport is returned by ReImporter.ReImport when the destination KB
// has no source_public_kb_id — i.e. it was authored locally, not imported.
// Handlers turn this into HTTP 409 Conflict because the request is well-formed
// but conflicts with the resource's lineage state.
var ErrNotAnImport = errors.New("marketplace: target KB is not an import")

// ErrSourceUnpublished is returned when the upstream Public KB referenced by
// source_public_kb_id either no longer exists or is no longer visibility=
// 'public'. Handlers turn this into HTTP 410 Gone so the Importer's UI can
// distinguish "publisher took it down" from a generic 404.
var ErrSourceUnpublished = errors.New("marketplace: source public KB has been unpublished")

// ErrKBNotFound is returned when the destination KB id does not exist within
// the actor's org. Distinct from pgx.ErrNoRows so callers can map to 404
// without reaching into pgx error types.
var ErrKBNotFound = errors.New("marketplace: knowledge base not found")

// ReImporter performs Re-import operations. It is constructed once at boot
// and reused across requests; all dependencies are immutable after
// construction.
//
// The Projection field is injected so tests can substitute a deterministic
// in-memory projection without touching the production embedding pipeline.
// In production it is wired to marketplace.ProjectFromPublicKB at main.go.
type ReImporter struct {
	pool *pgxpool.Pool
	// project is the publish-boundary projection (ADR-0002). See ProjectionFunc.
	project ProjectionFunc
	// now lets tests freeze the clock for assertions on
	// imported_from_revision_at. Production uses time.Now.
	now func() time.Time
}

// NewReImporter constructs a ReImporter wired to the production projection.
// Override the projection in tests via WithProjection.
func NewReImporter(pool *pgxpool.Pool) *ReImporter {
	return &ReImporter{
		pool:    pool,
		project: ProjectFromPublicKB,
		now:     time.Now,
	}
}

// WithProjection replaces the projection function. The chainable form lets
// tests build a ReImporter inline:
//
//	r := marketplace.NewReImporter(pool).WithProjection(fakeProjector)
func (r *ReImporter) WithProjection(p ProjectionFunc) *ReImporter {
	r.project = p
	return r
}

// WithClock replaces the clock used to stamp imported_from_revision_at when
// the source's last_modified_at is unavailable. Tests use it for golden
// assertions; production callers never need it.
func (r *ReImporter) WithClock(now func() time.Time) *ReImporter {
	r.now = now
	return r
}

// ReImportResult is what callers (handler, tests) receive on success. It is
// a small typed shape rather than (kb, time) so future fields — number of
// chunks projected, kept-artefacts summary — can be added without breaking
// the handler signature.
type ReImportResult struct {
	// KBID is the destination KB id. Guaranteed equal to the input id —
	// included in the shape so the handler can mirror the issue-defined
	// response contract `{ kb_id, imported_from_revision_at }` without
	// re-fetching the row.
	KBID string
	// ImportedFromRevisionAt is the timestamp written into knowledge_bases.
	// imported_from_revision_at on success — i.e. the publisher's
	// last_modified_at at the moment we read it (per ADR-0007 §7b).
	ImportedFromRevisionAt time.Time
	// ChunksProjected is the count returned by the projection. Useful for
	// telemetry; the handler is free to drop it.
	ChunksProjected int
}

// ReImport runs the destructive overwrite. The actorUserID is recorded for
// observability only — it does NOT change row ownership (the KB stays owned
// by destOrgID).
//
// All work happens inside one transaction. Either every change lands (chunks
// refreshed AND imported_from_revision_at bumped) or none does — Postgres
// guarantees this via standard MVCC. We do not need advisory locks: the
// SELECT … FOR UPDATE on the KB row serialises every concurrent Re-import
// against the same KB, and the FK CASCADE on embeddings + the within-tx
// DELETE/INSERT ordering eliminates cross-table races.
func (r *ReImporter) ReImport(ctx context.Context, destOrgID, destKBID, actorUserID string) (ReImportResult, error) {
	var result ReImportResult
	err := db.WithOrgID(ctx, r.pool, destOrgID, func(tx pgx.Tx) error {
		// Step 1 — load + lock the destination KB. FOR UPDATE serialises
		// concurrent re-imports of the same KB and concurrent content edits
		// (the trigger on documents/sources/chunks ultimately UPDATEs this
		// row's last_modified_at, which would deadlock against us if we did
		// not hold the row lock for the full transaction).
		var (
			sourcePublicKBID *string
			currentVisibility model.KBVisibility
		)
		err := tx.QueryRow(ctx,
			`SELECT source_public_kb_id, visibility
			 FROM knowledge_bases
			 WHERE id = $1 AND org_id = $2 AND status != 'archived'
			 FOR UPDATE`,
			destKBID, destOrgID,
		).Scan(&sourcePublicKBID, &currentVisibility)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrKBNotFound
			}
			return fmt.Errorf("ReImport: load dest kb: %w", err)
		}

		// Step 2a — refuse if not an import.
		if sourcePublicKBID == nil || *sourcePublicKBID == "" {
			return ErrNotAnImport
		}

		// Step 2b — verify the source is still public. We read the source
		// outside RLS via a direct UUID lookup; the SECURITY DEFINER
		// projection function (#729) reads chunks the same way. We deliberately
		// do NOT take FOR UPDATE on the source row — a publisher unpublishing
		// mid re-import is exactly the kind of race we want to detect via
		// the visibility check, not block on.
		var (
			srcVisibility    model.KBVisibility
			srcLastModified  time.Time
		)
		err = tx.QueryRow(ctx,
			`SELECT visibility, last_modified_at
			 FROM knowledge_bases
			 WHERE id = $1`,
			*sourcePublicKBID,
		).Scan(&srcVisibility, &srcLastModified)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrSourceUnpublished
			}
			return fmt.Errorf("ReImport: load source kb: %w", err)
		}
		if srcVisibility != model.KBVisibilityPublic {
			return ErrSourceUnpublished
		}

		// Step 3 — drop content. Order: chunks first (FK CASCADE drops
		// embeddings), then documents and sources whose chunks now have no
		// children referring up to them. The ON DELETE CASCADE chains make
		// this a clean wipe with no orphan rows.
		//
		// We pin org_id in every WHERE so a future schema change that loosens
		// RLS on these tables does not turn a re-import into a cross-tenant
		// nuke. Defence in depth — the RLS policies already enforce this.
		for _, stmt := range []string{
			`DELETE FROM chunks    WHERE knowledge_base_id = $1 AND org_id = $2`,
			`DELETE FROM documents WHERE knowledge_base_id = $1 AND org_id = $2`,
			`DELETE FROM sources   WHERE knowledge_base_id = $1 AND org_id = $2`,
		} {
			if _, err := tx.Exec(ctx, stmt, destKBID, destOrgID); err != nil {
				return fmt.Errorf("ReImport: wipe content: %w", err)
			}
		}

		// Step 4 — project the publisher's current content into the
		// destination KB. The projection runs inside the same tx, so a
		// failure here triggers the deferred Rollback in db.WithOrgID and
		// leaves chunks/sources/documents in their pre-call state.
		projected, projErr := r.project(ctx, tx, destOrgID, destKBID, *sourcePublicKBID)
		if projErr != nil {
			return fmt.Errorf("ReImport: projection: %w", projErr)
		}

		// Step 5 — bump imported_from_revision_at to the source's
		// last_modified_at at the moment we read it. Per ADR-0007 §7b this
		// is the canonical "I am pinned to publisher revision X" stamp.
		// We deliberately use the value we read in Step 2b, not now() —
		// a future content edit on the publisher should still register as
		// "stale relative to source" without forcing the Importer through
		// another Re-import immediately.
		if _, err := tx.Exec(ctx,
			`UPDATE knowledge_bases
			 SET imported_from_revision_at = $1
			 WHERE id = $2 AND org_id = $3`,
			srcLastModified, destKBID, destOrgID,
		); err != nil {
			return fmt.Errorf("ReImport: bump revision stamp: %w", err)
		}

		result = ReImportResult{
			KBID:                   destKBID,
			ImportedFromRevisionAt: srcLastModified,
			ChunksProjected:        projected,
		}
		// actorUserID is intentionally unused in the SQL layer — Re-import
		// is not a publish action and does not write to published_by_user_id.
		// It is included in the signature for future structured-log emission.
		_ = actorUserID
		return nil
	})
	if err != nil {
		return ReImportResult{}, err
	}
	return result, nil
}
