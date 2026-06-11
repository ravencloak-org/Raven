// projection_reimport.go holds the Go-level publish-boundary projection
// contract that the Re-import service (issue #730) is wired against.
//
// History / why this is still a stub:
//
//   - #730 (Re-import) was deliberately shipped as a thin slice: the wiring,
//     route, handler, and single-transaction atomicity contract are reviewed
//     in isolation, with the projection itself behind ProjectFromPublicKB so
//     the slice does not also carry the full 1,200-line projection.
//   - The original plan was for #729 (initial Import) to replace this stub's
//     body with the real Go projection. #729 (PR #794) instead implements the
//     content-grade fork projection in SQL — the SECURITY DEFINER function
//     marketplace_import_kb (migration 00057) — driven by the Importer service.
//     It therefore does NOT expose a Go ProjectFromPublicKB.
//   - So this contract is intentionally still a stub: it fails LOUDLY with
//     ErrProjectionNotImplemented rather than silently producing an empty KB.
//     A follow-up wires Re-import onto the same SQL projection path the
//     Importer uses (preserving the destination KB id). Re-import's own tests
//     inject a deterministic ProjectionFunc via WithProjection, so the stub is
//     never exercised in CI and the slice's behaviour is fully covered.
package marketplace

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrProjectionNotImplemented is returned by the stub ProjectFromPublicKB. It
// is the deliberate sentinel for "the production projection has not been wired
// yet" — callers (and tests) can treat this as a non-retriable signal rather
// than a transient infrastructure error. A deployment that wires the stub
// instead of a real projection fails on the first Re-import call.
var ErrProjectionNotImplemented = errors.New("marketplace: re-import projection not yet wired to the import SQL projection (issue #730 follow-up)")

// ProjectionFunc is the contract Re-import calls to copy the publisher's
// current content into the destination KB. The signature is frozen — ReImporter
// and its tests depend on it.
//
// Contract:
//
//   - The caller has already deleted every Source / Document / Chunk row for
//     destKBID inside tx (Re-import) or is operating on a freshly-inserted KB
//     row with no children. The projection MUST NOT delete anything.
//
//   - tx already has app.current_org_id set to destOrgID so RLS allows writes
//     into the Importer's tenant. The projection must NOT change session
//     state — no SET LOCAL, no advisory locks beyond the caller's scope.
//
//   - srcPublicKBID is guaranteed by the caller to be visibility='public' at
//     the time of the call. The projection reads via a SECURITY DEFINER
//     function (migration 00052) so the cross-tenant read is the only
//     auditable hole in the otherwise single-tenant retrieval path.
//
//   - Returns the number of chunks projected so callers can attach the count
//     to telemetry / response payloads.
type ProjectionFunc func(ctx context.Context, tx pgx.Tx, destOrgID, destKBID, srcPublicKBID string) (chunksProjected int, err error)

// ProjectFromPublicKB is the production projection wiring point. Pending the
// follow-up that bridges Re-import to the marketplace_import_kb SQL projection
// (PR #794), invoking it returns ErrProjectionNotImplemented so a deployment
// that forgets to substitute a real projection fails loudly at first call
// rather than producing an empty KB.
func ProjectFromPublicKB(_ context.Context, _ pgx.Tx, _, _, _ string) (int, error) {
	return 0, ErrProjectionNotImplemented
}
