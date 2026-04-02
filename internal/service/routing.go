package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// RoutingService contains business logic for routing rules and catalog metadata.
type RoutingService struct {
	repo   *repository.RoutingRepository
	kbRepo *repository.KBRepository
	pool   *pgxpool.Pool
}

// NewRoutingService creates a new RoutingService.
func NewRoutingService(repo *repository.RoutingRepository, kbRepo *repository.KBRepository, pool *pgxpool.Pool) *RoutingService {
	return &RoutingService{repo: repo, kbRepo: kbRepo, pool: pool}
}

// validRoutingModes is the set of allowed routing mode values.
var validRoutingModes = map[model.RoutingMode]bool{
	model.RoutingModeStatic:      true,
	model.RoutingModeColumnBased: true,
	model.RoutingModeAuto:        true,
}

// validateRoutingRule validates a routing rule on create/update based on its mode.
func validateRoutingRule(mode model.RoutingMode, targetKBID, discriminatorColumn *string, columnMappings map[string]string, classificationPrompt *string) error {
	switch mode {
	case model.RoutingModeStatic:
		if targetKBID == nil || *targetKBID == "" {
			return apierror.NewBadRequest("static routing mode requires target_kb_id")
		}
	case model.RoutingModeColumnBased:
		if discriminatorColumn == nil || *discriminatorColumn == "" {
			return apierror.NewBadRequest("column_based routing mode requires discriminator_column")
		}
		if len(columnMappings) == 0 {
			return apierror.NewBadRequest("column_based routing mode requires non-empty column_mappings")
		}
	case model.RoutingModeAuto:
		if classificationPrompt == nil || *classificationPrompt == "" {
			return apierror.NewBadRequest("auto routing mode requires classification_prompt")
		}
	default:
		return apierror.NewBadRequest("invalid routing_mode: " + string(mode))
	}
	return nil
}

// Create validates and persists a new routing rule.
func (s *RoutingService) Create(ctx context.Context, orgID string, req model.CreateRoutingRuleRequest, createdBy string) (*model.RoutingRule, error) {
	if !validRoutingModes[req.RoutingMode] {
		return nil, apierror.NewBadRequest("invalid routing_mode: " + string(req.RoutingMode))
	}

	if err := validateRoutingRule(req.RoutingMode, req.TargetKBID, req.DiscriminatorColumn, req.ColumnMappings, req.ClassificationPrompt); err != nil {
		return nil, err
	}

	var rule *model.RoutingRule
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Verify target KB exists if specified.
		if req.TargetKBID != nil && *req.TargetKBID != "" {
			if _, err := s.kbRepo.GetByID(ctx, tx, orgID, *req.TargetKBID); err != nil {
				return apierror.NewBadRequest("target knowledge base not found: " + *req.TargetKBID)
			}
		}

		// Verify KB IDs in column mappings exist.
		for colVal, kbID := range req.ColumnMappings {
			if _, err := s.kbRepo.GetByID(ctx, tx, orgID, kbID); err != nil {
				return apierror.NewBadRequest(fmt.Sprintf("knowledge base %q for column value %q not found", kbID, colVal))
			}
		}

		var err error
		rule, err = s.repo.Create(ctx, tx, orgID, req, createdBy)
		return err
	})
	if err != nil {
		if appErr, ok := err.(*apierror.AppError); ok {
			return nil, appErr
		}
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "violates") {
			return nil, apierror.NewBadRequest("invalid reference in routing rule")
		}
		return nil, apierror.NewInternal("failed to create routing rule: " + err.Error())
	}
	return rule, nil
}

// GetByID retrieves a routing rule by ID within an org.
func (s *RoutingService) GetByID(ctx context.Context, orgID, ruleID string) (*model.RoutingRule, error) {
	var rule *model.RoutingRule
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		rule, err = s.repo.GetByID(ctx, tx, orgID, ruleID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("routing rule not found")
		}
		return nil, apierror.NewInternal("failed to fetch routing rule: " + err.Error())
	}
	return rule, nil
}

// List returns a paginated list of routing rules for an org.
func (s *RoutingService) List(ctx context.Context, orgID string, page, pageSize int) (*model.RoutingRuleListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var rules []model.RoutingRule
	var total int
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		rules, total, err = s.repo.List(ctx, tx, orgID, page, pageSize)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list routing rules: " + err.Error())
	}
	rules = lo.Ternary(rules == nil, []model.RoutingRule{}, rules)
	return &model.RoutingRuleListResponse{
		Data:     rules,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Update validates and applies partial updates to a routing rule.
func (s *RoutingService) Update(ctx context.Context, orgID, ruleID string, req model.UpdateRoutingRuleRequest) (*model.RoutingRule, error) {
	if req.RoutingMode != nil && !validRoutingModes[*req.RoutingMode] {
		return nil, apierror.NewBadRequest("invalid routing_mode: " + string(*req.RoutingMode))
	}

	var rule *model.RoutingRule
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Fetch existing rule to merge with update for validation.
		existing, err := s.repo.GetByID(ctx, tx, orgID, ruleID)
		if err != nil {
			return err
		}

		// Determine effective mode after update.
		effectiveMode := existing.RoutingMode
		if req.RoutingMode != nil {
			effectiveMode = *req.RoutingMode
		}

		// Determine effective fields.
		effectiveTargetKBID := existing.TargetKBID
		if req.TargetKBID != nil {
			effectiveTargetKBID = req.TargetKBID
		}
		effectiveDiscriminatorColumn := existing.DiscriminatorColumn
		if req.DiscriminatorColumn != nil {
			effectiveDiscriminatorColumn = req.DiscriminatorColumn
		}
		effectiveColumnMappings := existing.ColumnMappings
		if req.ColumnMappings != nil {
			effectiveColumnMappings = req.ColumnMappings
		}
		effectiveClassificationPrompt := existing.ClassificationPrompt
		if req.ClassificationPrompt != nil {
			effectiveClassificationPrompt = req.ClassificationPrompt
		}

		if err := validateRoutingRule(effectiveMode, effectiveTargetKBID, effectiveDiscriminatorColumn, effectiveColumnMappings, effectiveClassificationPrompt); err != nil {
			return err
		}

		// Verify target KB exists if being changed.
		if req.TargetKBID != nil && *req.TargetKBID != "" {
			if _, err := s.kbRepo.GetByID(ctx, tx, orgID, *req.TargetKBID); err != nil {
				return apierror.NewBadRequest("target knowledge base not found: " + *req.TargetKBID)
			}
		}

		// Verify KB IDs in column mappings exist if being changed.
		if req.ColumnMappings != nil {
			for colVal, kbID := range req.ColumnMappings {
				if _, err := s.kbRepo.GetByID(ctx, tx, orgID, kbID); err != nil {
					return apierror.NewBadRequest(fmt.Sprintf("knowledge base %q for column value %q not found", kbID, colVal))
				}
			}
		}

		rule, err = s.repo.Update(ctx, tx, orgID, ruleID, req)
		return err
	})
	if err != nil {
		if appErr, ok := err.(*apierror.AppError); ok {
			return nil, appErr
		}
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("routing rule not found")
		}
		return nil, apierror.NewInternal("failed to update routing rule: " + err.Error())
	}
	return rule, nil
}

// Delete permanently removes a routing rule.
func (s *RoutingService) Delete(ctx context.Context, orgID, ruleID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, orgID, ruleID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("routing rule not found")
		}
		return apierror.NewInternal("failed to delete routing rule: " + err.Error())
	}
	return nil
}

// ResolveKBForDocument determines which knowledge base a document should be
// routed to based on the matching routing rules.
//
// Algorithm:
//  1. Find matching rules (by source_type + source_identifier)
//  2. Evaluate in priority order:
//     - Static: return target_kb_id
//     - Column-based: look up metadata[discriminator_column] in column_mappings
//     - Auto: placeholder — returns error "auto classification not yet implemented"
//  3. If no rule matches, return error "no routing rule found"
func (s *RoutingService) ResolveKBForDocument(ctx context.Context, orgID, sourceType, sourceIdentifier string, metadata map[string]any) (*model.ResolveRoutingResponse, error) {
	var result *model.ResolveRoutingResponse
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		rules, err := s.repo.FindMatchingRules(ctx, tx, orgID, sourceType, sourceIdentifier)
		if err != nil {
			return err
		}
		if len(rules) == 0 {
			return apierror.NewNotFound("no routing rule found for source")
		}

		for i := range rules {
			rule := &rules[i]
			switch rule.RoutingMode {
			case model.RoutingModeStatic:
				if rule.TargetKBID != nil && *rule.TargetKBID != "" {
					result = &model.ResolveRoutingResponse{
						KnowledgeBaseID: *rule.TargetKBID,
						RuleName:        rule.Name,
						RuleID:          rule.ID,
					}
					return nil
				}

			case model.RoutingModeColumnBased:
				if rule.DiscriminatorColumn == nil || rule.ColumnMappings == nil {
					continue
				}
				colValue, ok := metadata[*rule.DiscriminatorColumn]
				if !ok {
					continue
				}
				colValueStr := fmt.Sprintf("%v", colValue)
				kbID, found := rule.ColumnMappings[colValueStr]
				if !found {
					continue
				}
				result = &model.ResolveRoutingResponse{
					KnowledgeBaseID: kbID,
					RuleName:        rule.Name,
					RuleID:          rule.ID,
				}
				return nil

			case model.RoutingModeAuto:
				return apierror.NewBadRequest("auto classification not yet implemented")
			}
		}

		return apierror.NewNotFound("no routing rule matched the given metadata")
	})
	if err != nil {
		if appErr, ok := err.(*apierror.AppError); ok {
			return nil, appErr
		}
		return nil, apierror.NewInternal("failed to resolve routing: " + err.Error())
	}
	return result, nil
}

// ListCatalogMetadata returns catalog metadata for an org.
func (s *RoutingService) ListCatalogMetadata(ctx context.Context, orgID, catalogType string) ([]model.CatalogMetadata, error) {
	var items []model.CatalogMetadata
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		items, err = s.repo.ListCatalogMetadata(ctx, tx, orgID, catalogType)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list catalog metadata: " + err.Error())
	}
	items = lo.Ternary(items == nil, []model.CatalogMetadata{}, items)
	return items, nil
}
