package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// RoutingRepository handles database operations for routing rules and catalog metadata.
// All operations use a pgx.Tx with org_id set for RLS enforcement.
type RoutingRepository struct {
	pool *pgxpool.Pool
}

// NewRoutingRepository creates a new RoutingRepository.
func NewRoutingRepository(pool *pgxpool.Pool) *RoutingRepository {
	return &RoutingRepository{pool: pool}
}

const routingRuleColumns = `id, org_id, name,
	COALESCE(description, '') AS description,
	source_type,
	COALESCE(source_identifier, '') AS source_identifier,
	routing_mode,
	COALESCE(target_kb_id::text, '') AS target_kb_id,
	COALESCE(discriminator_column, '') AS discriminator_column,
	COALESCE(column_mappings, '{}') AS column_mappings,
	COALESCE(classification_prompt, '') AS classification_prompt,
	COALESCE(classification_model, '') AS classification_model,
	COALESCE(classification_provider, '') AS classification_provider,
	priority, is_active,
	COALESCE(created_by::text, '') AS created_by,
	created_at, updated_at`

func scanRoutingRule(row pgx.Row) (*model.RoutingRule, error) {
	var r model.RoutingRule
	var description, sourceIdentifier, targetKBID string
	var discriminatorColumn, classificationPrompt string
	var classificationModel, classificationProvider string
	var createdBy string

	err := row.Scan(
		&r.ID,
		&r.OrgID,
		&r.Name,
		&description,
		&r.SourceType,
		&sourceIdentifier,
		&r.RoutingMode,
		&targetKBID,
		&discriminatorColumn,
		&r.ColumnMappings,
		&classificationPrompt,
		&classificationModel,
		&classificationProvider,
		&r.Priority,
		&r.IsActive,
		&createdBy,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if description != "" {
		r.Description = &description
	}
	if sourceIdentifier != "" {
		r.SourceIdentifier = &sourceIdentifier
	}
	if targetKBID != "" {
		r.TargetKBID = &targetKBID
	}
	if discriminatorColumn != "" {
		r.DiscriminatorColumn = &discriminatorColumn
	}
	if classificationPrompt != "" {
		r.ClassificationPrompt = &classificationPrompt
	}
	if classificationModel != "" {
		r.ClassificationModel = &classificationModel
	}
	if classificationProvider != "" {
		r.ClassificationProvider = &classificationProvider
	}
	if createdBy != "" {
		r.CreatedBy = &createdBy
	}

	return &r, nil
}

// Create inserts a new routing rule within a transaction.
func (r *RoutingRepository) Create(ctx context.Context, tx pgx.Tx, orgID string, req model.CreateRoutingRuleRequest, createdBy string) (*model.RoutingRule, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO routing_rules (
			org_id, name, description, source_type, source_identifier,
			routing_mode, target_kb_id, discriminator_column, column_mappings,
			classification_prompt, classification_model, classification_provider,
			priority, created_by
		) VALUES (
			$1, $2, NULLIF($3, ''), $4, NULLIF($5, ''),
			$6, NULLIF($7, '')::uuid, NULLIF($8, ''), COALESCE($9::jsonb, '{}'),
			NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''),
			$13, NULLIF($14, '')::uuid
		) RETURNING `+routingRuleColumns,
		orgID,
		req.Name,
		ptrToString(req.Description),
		req.SourceType,
		ptrToString(req.SourceIdentifier),
		req.RoutingMode,
		ptrToString(req.TargetKBID),
		ptrToString(req.DiscriminatorColumn),
		req.ColumnMappings,
		ptrToString(req.ClassificationPrompt),
		ptrToString(req.ClassificationModel),
		ptrToString(req.ClassificationProvider),
		req.Priority,
		createdBy,
	)
	rule, err := scanRoutingRule(row)
	if err != nil {
		return nil, fmt.Errorf("RoutingRepository.Create: %w", err)
	}
	return rule, nil
}

// GetByID fetches a routing rule by its primary key within an org.
func (r *RoutingRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, ruleID string) (*model.RoutingRule, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+routingRuleColumns+`
		 FROM routing_rules
		 WHERE id = $1 AND org_id = $2`,
		ruleID, orgID,
	)
	rule, err := scanRoutingRule(row)
	if err != nil {
		return nil, fmt.Errorf("RoutingRepository.GetByID: %w", err)
	}
	return rule, nil
}

// List returns a paginated list of routing rules for an org.
func (r *RoutingRepository) List(ctx context.Context, tx pgx.Tx, orgID string, page, pageSize int) ([]model.RoutingRule, int, error) {
	var total int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM routing_rules WHERE org_id = $1`,
		orgID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("RoutingRepository.List count: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := tx.Query(ctx,
		`SELECT `+routingRuleColumns+`
		 FROM routing_rules
		 WHERE org_id = $1
		 ORDER BY priority DESC, created_at DESC
		 LIMIT $2 OFFSET $3`,
		orgID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("RoutingRepository.List query: %w", err)
	}
	defer rows.Close()

	var rules []model.RoutingRule
	for rows.Next() {
		var rule model.RoutingRule
		var description, sourceIdentifier, targetKBID string
		var discriminatorColumn, classificationPrompt string
		var classificationModel, classificationProvider string
		var createdBy string

		if err := rows.Scan(
			&rule.ID, &rule.OrgID, &rule.Name,
			&description, &rule.SourceType, &sourceIdentifier,
			&rule.RoutingMode, &targetKBID, &discriminatorColumn,
			&rule.ColumnMappings, &classificationPrompt,
			&classificationModel, &classificationProvider,
			&rule.Priority, &rule.IsActive, &createdBy,
			&rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("RoutingRepository.List scan: %w", err)
		}

		if description != "" {
			rule.Description = &description
		}
		if sourceIdentifier != "" {
			rule.SourceIdentifier = &sourceIdentifier
		}
		if targetKBID != "" {
			rule.TargetKBID = &targetKBID
		}
		if discriminatorColumn != "" {
			rule.DiscriminatorColumn = &discriminatorColumn
		}
		if classificationPrompt != "" {
			rule.ClassificationPrompt = &classificationPrompt
		}
		if classificationModel != "" {
			rule.ClassificationModel = &classificationModel
		}
		if classificationProvider != "" {
			rule.ClassificationProvider = &classificationProvider
		}
		if createdBy != "" {
			rule.CreatedBy = &createdBy
		}

		rules = append(rules, rule)
	}
	return rules, total, rows.Err()
}

// Update applies partial updates to a routing rule.
func (r *RoutingRepository) Update(ctx context.Context, tx pgx.Tx, orgID, ruleID string, req model.UpdateRoutingRuleRequest) (*model.RoutingRule, error) {
	row := tx.QueryRow(ctx,
		`UPDATE routing_rules
		 SET
		   name                    = COALESCE($3, name),
		   description             = COALESCE($4, description),
		   source_type             = COALESCE($5, source_type),
		   source_identifier       = COALESCE($6, source_identifier),
		   routing_mode            = COALESCE($7, routing_mode),
		   target_kb_id            = CASE WHEN $8::text IS NOT NULL THEN NULLIF($8, '')::uuid ELSE target_kb_id END,
		   discriminator_column    = COALESCE($9, discriminator_column),
		   column_mappings         = CASE WHEN $10::jsonb IS NOT NULL THEN $10::jsonb ELSE column_mappings END,
		   classification_prompt   = COALESCE($11, classification_prompt),
		   classification_model    = COALESCE($12, classification_model),
		   classification_provider = COALESCE($13, classification_provider),
		   priority                = COALESCE($14, priority),
		   is_active               = COALESCE($15, is_active),
		   updated_at              = NOW()
		 WHERE id = $1 AND org_id = $2
		 RETURNING `+routingRuleColumns,
		ruleID, orgID,
		req.Name, req.Description, req.SourceType, req.SourceIdentifier,
		req.RoutingMode, req.TargetKBID, req.DiscriminatorColumn,
		req.ColumnMappings, req.ClassificationPrompt,
		req.ClassificationModel, req.ClassificationProvider,
		req.Priority, req.IsActive,
	)
	rule, err := scanRoutingRule(row)
	if err != nil {
		return nil, fmt.Errorf("RoutingRepository.Update: %w", err)
	}
	return rule, nil
}

// Delete permanently removes a routing rule.
func (r *RoutingRepository) Delete(ctx context.Context, tx pgx.Tx, orgID, ruleID string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM routing_rules WHERE id = $1 AND org_id = $2`,
		ruleID, orgID,
	)
	if err != nil {
		return fmt.Errorf("RoutingRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("RoutingRepository.Delete: routing rule %s not found", ruleID)
	}
	return nil
}

// FindMatchingRules finds active routing rules matching a source, ordered by priority DESC.
func (r *RoutingRepository) FindMatchingRules(ctx context.Context, tx pgx.Tx, orgID, sourceType, sourceIdentifier string) ([]model.RoutingRule, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+routingRuleColumns+`
		 FROM routing_rules
		 WHERE org_id = $1
		   AND source_type = $2
		   AND (source_identifier IS NULL OR source_identifier = $3)
		   AND is_active = true
		 ORDER BY priority DESC, created_at DESC`,
		orgID, sourceType, sourceIdentifier,
	)
	if err != nil {
		return nil, fmt.Errorf("RoutingRepository.FindMatchingRules query: %w", err)
	}
	defer rows.Close()

	var rules []model.RoutingRule
	for rows.Next() {
		var rule model.RoutingRule
		var description, srcIdent, targetKBID string
		var discriminatorColumn, classificationPrompt string
		var classificationModel, classificationProvider string
		var createdBy string

		if err := rows.Scan(
			&rule.ID, &rule.OrgID, &rule.Name,
			&description, &rule.SourceType, &srcIdent,
			&rule.RoutingMode, &targetKBID, &discriminatorColumn,
			&rule.ColumnMappings, &classificationPrompt,
			&classificationModel, &classificationProvider,
			&rule.Priority, &rule.IsActive, &createdBy,
			&rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("RoutingRepository.FindMatchingRules scan: %w", err)
		}

		if description != "" {
			rule.Description = &description
		}
		if srcIdent != "" {
			rule.SourceIdentifier = &srcIdent
		}
		if targetKBID != "" {
			rule.TargetKBID = &targetKBID
		}
		if discriminatorColumn != "" {
			rule.DiscriminatorColumn = &discriminatorColumn
		}
		if classificationPrompt != "" {
			rule.ClassificationPrompt = &classificationPrompt
		}
		if classificationModel != "" {
			rule.ClassificationModel = &classificationModel
		}
		if classificationProvider != "" {
			rule.ClassificationProvider = &classificationProvider
		}
		if createdBy != "" {
			rule.CreatedBy = &createdBy
		}

		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// UpsertCatalogMetadata inserts or updates a catalog metadata entry.
func (r *RoutingRepository) UpsertCatalogMetadata(ctx context.Context, tx pgx.Tx, metadata *model.CatalogMetadata) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO catalog_metadata (org_id, catalog_type, resource_path, labels)
		 VALUES ($1, $2, $3, COALESCE($4::jsonb, '{}'))
		 ON CONFLICT (org_id, catalog_type, resource_path)
		 DO UPDATE SET labels = COALESCE($4::jsonb, '{}'), discovered_at = NOW()`,
		metadata.OrgID, metadata.CatalogType, metadata.ResourcePath, metadata.Labels,
	)
	if err != nil {
		return fmt.Errorf("RoutingRepository.UpsertCatalogMetadata: %w", err)
	}
	return nil
}

// ListCatalogMetadata returns all catalog metadata for an org, optionally filtered by catalog type.
func (r *RoutingRepository) ListCatalogMetadata(ctx context.Context, tx pgx.Tx, orgID, catalogType string) ([]model.CatalogMetadata, error) {
	var rows pgx.Rows
	var err error

	if catalogType != "" {
		rows, err = tx.Query(ctx,
			`SELECT id, org_id, catalog_type, resource_path, COALESCE(labels, '{}') AS labels, discovered_at
			 FROM catalog_metadata
			 WHERE org_id = $1 AND catalog_type = $2
			 ORDER BY discovered_at DESC`,
			orgID, catalogType,
		)
	} else {
		rows, err = tx.Query(ctx,
			`SELECT id, org_id, catalog_type, resource_path, COALESCE(labels, '{}') AS labels, discovered_at
			 FROM catalog_metadata
			 WHERE org_id = $1
			 ORDER BY discovered_at DESC`,
			orgID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("RoutingRepository.ListCatalogMetadata: %w", err)
	}
	defer rows.Close()

	var items []model.CatalogMetadata
	for rows.Next() {
		var m model.CatalogMetadata
		if err := rows.Scan(&m.ID, &m.OrgID, &m.CatalogType, &m.ResourcePath, &m.Labels, &m.DiscoveredAt); err != nil {
			return nil, fmt.Errorf("RoutingRepository.ListCatalogMetadata scan: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// ptrToString safely dereferences a *string, returning "" for nil.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
