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

const kbColumns = `id, org_id, workspace_id, name, slug,
	COALESCE(description, '') AS description, settings, status, created_at, updated_at`

func scanKB(row pgx.Row) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := row.Scan(
		&kb.ID,
		&kb.OrgID,
		&kb.WorkspaceID,
		&kb.Name,
		&kb.Slug,
		&kb.Description,
		&kb.Settings,
		&kb.Status,
		&kb.CreatedAt,
		&kb.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &kb, nil
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

// GetByID fetches an active KB by its primary key.
func (r *KBRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, kbID string) (*model.KnowledgeBase, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+kbColumns+`
		 FROM knowledge_bases
		 WHERE id = $1 AND org_id = $2 AND status = 'active'`,
		kbID, orgID,
	)
	kb, err := scanKB(row)
	if err != nil {
		return nil, fmt.Errorf("KBRepository.GetByID: %w", err)
	}
	return kb, nil
}

// ListByWorkspace returns all active KBs for a workspace.
func (r *KBRepository) ListByWorkspace(ctx context.Context, tx pgx.Tx, orgID, wsID string) ([]model.KnowledgeBase, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+kbColumns+`
		 FROM knowledge_bases
		 WHERE org_id = $1 AND workspace_id = $2 AND status = 'active'
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
		if err := rows.Scan(&kb.ID, &kb.OrgID, &kb.WorkspaceID, &kb.Name, &kb.Slug,
			&kb.Description, &kb.Settings, &kb.Status, &kb.CreatedAt, &kb.UpdatedAt); err != nil {
			return nil, fmt.Errorf("KBRepository.ListByWorkspace scan: %w", err)
		}
		kbs = append(kbs, kb)
	}
	return kbs, rows.Err()
}

// Update applies partial updates to a knowledge base.
func (r *KBRepository) Update(ctx context.Context, tx pgx.Tx, orgID, kbID string, name, description *string, settings map[string]any) (*model.KnowledgeBase, error) {
	row := tx.QueryRow(ctx,
		`UPDATE knowledge_bases
		 SET
		   name        = COALESCE($3, name),
		   description = COALESCE($4, description),
		   settings    = CASE WHEN $5::jsonb IS NOT NULL THEN $5::jsonb ELSE settings END
		 WHERE id = $1 AND org_id = $2 AND status = 'active'
		 RETURNING `+kbColumns,
		kbID, orgID, name, description, settings,
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
