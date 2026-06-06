// This file ships the thin Go wrappers over the two cross-tenant SQL
// functions added by migration 00052 (issue #728): the Marketplace listing
// and the Public-KB preview.
//
// The wrappers are intentionally minimal — they translate between Go types
// and the SECURITY DEFINER functions in the database. All policy lives in
// SQL: pagination caps, sort whitelisting, license filtering, the 3-chunk
// preview ceiling, and the public-only visibility check. Adding policy
// logic up here would split the security perimeter and create two places a
// future change would have to be made; ADR-0001 keeps that perimeter in
// one place on purpose.
//
// HTTP handler wiring lives in issue #731 and depends on these wrappers
// (`Queries.ListPublicKBs`, `Queries.PreviewKB`).

package marketplace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgInsufficientPrivilege is the SQLSTATE class for
// `insufficient_privilege`. The preview SQL function raises this code when
// the target KB is not public; we map it to a typed error so handlers can
// translate it to HTTP 404 without leaking existence.
const pgInsufficientPrivilege = "42501"

// pgInvalidParameterValue is the SQLSTATE class for
// `invalid_parameter_value`. The listing function raises this when `sort`
// is not one of the four accepted values. Handlers map it to HTTP 400.
const pgInvalidParameterValue = "22023"

// ListSort is one of the four sort orders accepted by
// `marketplace_list_public_kbs`. ADR-0008 §"Discovery" pins this set.
type ListSort string

// ListSort values mirror the SQL whitelist exactly. Keep these in lock-step
// with the `sort NOT IN (...)` clause in migration 00052.
const (
	SortNewest          ListSort = "newest"
	SortMostImported    ListSort = "most_imported"
	SortRecentlyUpdated ListSort = "recently_updated"
	SortAlphabetic      ListSort = "alphabetic"
)

// ListFilters is the bag of parameters callers pass to ListPublicKBs.
// A struct (not a 5-arg method) so callers cannot transpose `q` and the
// license filter at the call site — a positional-argument trap that would
// silently return wrong results.
type ListFilters struct {
	// Query is the free-form search text (websearch_to_tsquery syntax).
	// Empty string means "no FTS filter".
	Query string
	// Sort selects the result order. Empty defaults to SortNewest so the
	// listing endpoint is callable without explicit ordering — the
	// Marketplace homepage shows newest-published first.
	Sort ListSort
	// Licenses is the optional SPDX allow-list filter. Empty / nil means
	// "no license filter".
	Licenses []string
	// Limit is the page size; the SQL function clamps to a hard cap of 50
	// regardless of what we pass.
	Limit int
	// Offset is the page start; the SQL function clamps to >= 0.
	Offset int
}

// ListItem is one row returned by `marketplace_list_public_kbs`.
// Field order mirrors the SQL `RETURNS TABLE (...)` declaration; any
// reorder must update both sides together.
type ListItem struct {
	KBID                 uuid.UUID
	OrgSlug              string
	OrgDisplayName       string
	KBSlug               string
	KBName               string
	Description          *string
	LicenseSPDXID        *string
	LastModifiedAt       time.Time
	ImportCount          int
	SourcePublicKBID     *uuid.UUID
	SourceOrgSlug        *string
	SourceOrgDisplayName *string
}

// PreviewChunk is one of the (at most three) chunks returned by
// `marketplace_preview_kb`. The 3-chunk cap is enforced inside the SQL
// function; callers cannot ask for more.
type PreviewChunk struct {
	ChunkID uuid.UUID
	Ordinal int
	Text    string
}

// Queries owns the read-only SECURITY DEFINER call surface for the
// Marketplace. It is the seam between handler code (#731) and the SQL
// boundary; keeping the pool here (vs. on each method) lets future
// instrumentation hang per-Queries (metrics, tracing) without retrofitting
// every call site.
type Queries struct {
	pool *pgxpool.Pool
}

// NewQueries constructs a Queries wrapper around an existing pool.
func NewQueries(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool}
}

// ErrKBNotPublic is returned by PreviewKB when the target KB does not
// exist or is not `visibility='public'`. The two cases are deliberately
// indistinguishable at the SQL boundary so the API can't be used to probe
// for private-KB existence (ADR-0001 §"Why").
var ErrKBNotPublic = errors.New("marketplace: kb not public")

// ErrUnknownSort is returned by ListPublicKBs when the SQL function
// rejects the requested sort. The Go-side enum should prevent this, but
// the typed error exists so callers who synthesise a ListFilters from a
// raw query string can map it to HTTP 400 cleanly.
var ErrUnknownSort = errors.New("marketplace: unknown sort")

// ListPublicKBs returns the Marketplace listing page matching `f`. The
// SQL function clamps `Limit` to 50 and `Offset` to >= 0 — passing
// pathological values is safe, but the caller will see the clamped page.
//
// The `Sort` field defaults to SortNewest so the most common Marketplace
// homepage call can pass a zero-value ListFilters.
func (q *Queries) ListPublicKBs(ctx context.Context, f ListFilters) ([]ListItem, error) {
	sort := f.Sort
	if sort == "" {
		sort = SortNewest
	}

	// The SQL function treats an empty query string (length(btrim(q))=0)
	// and an empty/NULL licenses array (cardinality=0) as "no filter", so
	// we can pass the caller's values through unchanged — no Go-side
	// nil-mapping needed. Keeping the boundary thin avoids two places the
	// "what counts as empty?" rule could drift apart.
	rows, err := q.pool.Query(ctx,
		`SELECT kb_id, org_slug, org_display_name, kb_slug, kb_name,
		        description, license_spdx_id, last_modified_at, import_count,
		        source_public_kb_id, source_org_slug, source_org_display_name
		 FROM marketplace_list_public_kbs($1, $2, $3, $4, $5)`,
		f.Query, string(sort), f.Licenses, f.Limit, f.Offset,
	)
	if err != nil {
		if pgErrCode(err) == pgInvalidParameterValue {
			return nil, fmt.Errorf("%w: %s", ErrUnknownSort, sort)
		}
		return nil, fmt.Errorf("marketplace: list public KBs: %w", err)
	}
	defer rows.Close()

	var items []ListItem
	for rows.Next() {
		var it ListItem
		if err := rows.Scan(
			&it.KBID,
			&it.OrgSlug,
			&it.OrgDisplayName,
			&it.KBSlug,
			&it.KBName,
			&it.Description,
			&it.LicenseSPDXID,
			&it.LastModifiedAt,
			&it.ImportCount,
			&it.SourcePublicKBID,
			&it.SourceOrgSlug,
			&it.SourceOrgDisplayName,
		); err != nil {
			return nil, fmt.Errorf("marketplace: scan listing row: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		if pgErrCode(err) == pgInvalidParameterValue {
			return nil, fmt.Errorf("%w: %s", ErrUnknownSort, sort)
		}
		return nil, fmt.Errorf("marketplace: list public KBs iterate: %w", err)
	}
	return items, nil
}

// PreviewKB returns up to three chunks from the given Public KB and bumps
// `preview_count` on the row. ErrKBNotPublic is returned if the target KB
// does not exist or is not `visibility='public'` — the two cases are
// indistinguishable by design.
func (q *Queries) PreviewKB(ctx context.Context, publicKBID uuid.UUID) ([]PreviewChunk, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT chunk_id, ordinal, text FROM marketplace_preview_kb($1)`,
		publicKBID,
	)
	if err != nil {
		if pgErrCode(err) == pgInsufficientPrivilege {
			return nil, ErrKBNotPublic
		}
		return nil, fmt.Errorf("marketplace: preview kb: %w", err)
	}
	defer rows.Close()

	// Pre-size for the SQL cap. Cheap allocation; the slice never grows
	// beyond three entries.
	chunks := make([]PreviewChunk, 0, 3)
	for rows.Next() {
		var c PreviewChunk
		if err := rows.Scan(&c.ChunkID, &c.Ordinal, &c.Text); err != nil {
			return nil, fmt.Errorf("marketplace: scan preview chunk: %w", err)
		}
		chunks = append(chunks, c)
	}
	if err := rows.Err(); err != nil {
		// The function raises insufficient_privilege via RAISE EXCEPTION,
		// which pgx surfaces here (not on the initial Query call) when the
		// statement is executed lazily. Map that the same way as above.
		if pgErrCode(err) == pgInsufficientPrivilege {
			return nil, ErrKBNotPublic
		}
		return nil, fmt.Errorf("marketplace: preview kb iterate: %w", err)
	}
	return chunks, nil
}

// pgErrCode extracts the SQLSTATE from a pgx error chain, or "" if the
// error is not a *pgconn.PgError.
func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
