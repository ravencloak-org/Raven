package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// KBRepository handles database operations for knowledge bases.
// All operations use a pgx.Tx with org_id set for RLS enforcement.
type KBRepository struct {
	pool *pgxpool.Pool
}

// NewKBRepository creates a new KBRepository.
func NewKBRepository(pool *pgxpool.Pool) *KBRepository {
	return &KBRepository{pool: pool}
}

// kbColumns lists every column returned by KB reads.
//
// History:
//   - M9 semantic cache (#256) added cache_enabled, cache_similarity_threshold.
//   - Marketplace MVP (#723, migration 00047) added the visibility / publish /
//     lineage / freshness / license / discovery block.
//
// search_tsv is intentionally omitted: callers never need the raw tsvector,
// and Marketplace search reads it server-side via dedicated SQL functions
// (added in migration 00052).
const kbColumns = `id, org_id, workspace_id, name, slug,
	COALESCE(description, '') AS description, settings, status,
	cache_enabled, cache_similarity_threshold,
	visibility, published_at, published_by_user_id,
	source_public_kb_id, imported_from_revision_at, last_modified_at,
	license_spdx_id, import_count, preview_count,
	created_at, updated_at`

func scanKB(row pgx.Row) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	if err := scanKBInto(row, &kb); err != nil {
		return nil, err
	}
	return &kb, nil
}

// scanKBInto is shared by single-row scans and the iterator loop in
// ListByWorkspace so the column order stays defined in one place.
func scanKBInto(row pgx.Row, kb *model.KnowledgeBase) error {
	return row.Scan(
		&kb.ID,
		&kb.OrgID,
		&kb.WorkspaceID,
		&kb.Name,
		&kb.Slug,
		&kb.Description,
		&kb.Settings,
		&kb.Status,
		&kb.CacheEnabled,
		&kb.CacheSimilarityThreshold,
		&kb.Visibility,
		&kb.PublishedAt,
		&kb.PublishedByUserID,
		&kb.SourcePublicKBID,
		&kb.ImportedFromRevisionAt,
		&kb.LastModifiedAt,
		&kb.LicenseSPDXID,
		&kb.ImportCount,
		&kb.PreviewCount,
		&kb.CreatedAt,
		&kb.UpdatedAt,
	)
}

// Create inserts a new knowledge base within a workspace transaction.
func (r *KBRepository) Create(ctx context.Context, tx pgx.Tx, orgID, wsID, name, slug, description string) (*model.KnowledgeBase, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO knowledge_bases (org_id, workspace_id, name, slug, description)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		 RETURNING `+kbColumns,
		orgID, wsID, name, slug, description,
	)
	kb, err := scanKB(row)
	if err != nil {
		return nil, fmt.Errorf("KBRepository.Create: %w", err)
	}
	return kb, nil
}

// kbReadableStatuses is the SQL fragment listing every kb_status value
// whose rows should be visible to user-facing read paths. Archived rows
// stay hidden (soft-delete); read_only_private and dmca_pending stay
// visible so chat / metadata reads keep working during a freeze (see
// model.KBStatusGate.CanRead). Centralising the list here keeps every
// read query consistent — adding a new lifecycle state means updating
// the gate, the capability table, and this fragment.
const kbReadableStatuses = `('active', 'read_only_private', 'dmca_pending')`

// GetByID fetches a KB by its primary key, including the two frozen
// Marketplace states. Service-layer policy (model.KBStatusGate) decides
// whether the caller may act on the returned row.
func (r *KBRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, kbID string) (*model.KnowledgeBase, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+kbColumns+`
		 FROM knowledge_bases
		 WHERE id = $1 AND org_id = $2 AND status IN `+kbReadableStatuses,
		kbID, orgID,
	)
	kb, err := scanKB(row)
	if err != nil {
		return nil, fmt.Errorf("KBRepository.GetByID: %w", err)
	}
	return kb, nil
}

// LoadStatus returns the lifecycle status for a KB. It bypasses the
// read-visibility filter so the service layer can distinguish "row missing"
// from "row frozen" without two round-trips, and can present a 409 /
// 423 instead of the generic 404 the read path would surface.
//
// Returns pgx.ErrNoRows when the KB id is unknown for this org.
func (r *KBRepository) LoadStatus(ctx context.Context, tx pgx.Tx, orgID, kbID string) (model.KBStatus, error) {
	var status model.KBStatus
	err := tx.QueryRow(ctx,
		`SELECT status FROM knowledge_bases WHERE id = $1 AND org_id = $2`,
		kbID, orgID,
	).Scan(&status)
	if err != nil {
		return "", fmt.Errorf("KBRepository.LoadStatus: %w", err)
	}
	return status, nil
}

// ListByWorkspace returns every KB in a workspace that is visible to user-
// facing surfaces — i.e. excluding archived (soft-deleted) rows but
// including frozen Marketplace states.
func (r *KBRepository) ListByWorkspace(ctx context.Context, tx pgx.Tx, orgID, wsID string) ([]model.KnowledgeBase, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+kbColumns+`
		 FROM knowledge_bases
		 WHERE org_id = $1 AND workspace_id = $2 AND status IN `+kbReadableStatuses+`
		 ORDER BY created_at`,
		orgID, wsID,
	)
	if err != nil {
		return nil, fmt.Errorf("KBRepository.ListByWorkspace: %w", err)
	}
	defer rows.Close()

	var kbs []model.KnowledgeBase
	for rows.Next() {
		var kb model.KnowledgeBase
		if err := scanKBInto(rows, &kb); err != nil {
			return nil, fmt.Errorf("KBRepository.ListByWorkspace scan: %w", err)
		}
		kbs = append(kbs, kb)
	}
	return kbs, rows.Err()
}

// Update applies partial updates to a knowledge base. The cacheEnabled and
// cacheThreshold parameters are the M9 semantic-cache knobs (#256); when nil
// the existing value is preserved via COALESCE.
func (r *KBRepository) Update(
	ctx context.Context,
	tx pgx.Tx,
	orgID, kbID string,
	name, description *string,
	settings map[string]any,
	cacheEnabled *bool,
	cacheThreshold *float32,
) (*model.KnowledgeBase, error) {
	// Update stays bounded to non-archived rows so a soft-deleted KB cannot
	// be revived via a settings PATCH. The read-only-private / dmca_pending
	// freeze is enforced at the service layer (model.KBStatusGate) so the
	// caller gets a precise 409 / 423 instead of a generic "not found".
	row := tx.QueryRow(ctx,
		`UPDATE knowledge_bases
		 SET
		   name                       = COALESCE($3, name),
		   description                = COALESCE($4, description),
		   settings                   = CASE WHEN $5::jsonb IS NOT NULL THEN $5::jsonb ELSE settings END,
		   cache_enabled              = COALESCE($6, cache_enabled),
		   cache_similarity_threshold = COALESCE($7, cache_similarity_threshold)
		 WHERE id = $1 AND org_id = $2 AND status IN `+kbReadableStatuses+`
		 RETURNING `+kbColumns,
		kbID, orgID, name, description, settings, cacheEnabled, cacheThreshold,
	)
	kb, err := scanKB(row)
	if err != nil {
		return nil, fmt.Errorf("KBRepository.Update: %w", err)
	}
	return kb, nil
}

// Archive sets a knowledge base status to 'archived' (soft delete).
func (r *KBRepository) Archive(ctx context.Context, tx pgx.Tx, orgID, kbID string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE knowledge_bases SET status = 'archived' WHERE id = $1 AND org_id = $2`,
		kbID, orgID,
	)
	if err != nil {
		return fmt.Errorf("KBRepository.Archive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("KBRepository.Archive: knowledge base %s not found", kbID)
	}
	return nil
}
