package model

import "time"

// RoutingMode represents the strategy for routing data to knowledge bases.
type RoutingMode string

// RoutingModeStatic, RoutingModeColumnBased, and RoutingModeAuto are the
// valid routing mode values.
const (
	RoutingModeStatic      RoutingMode = "static"
	RoutingModeColumnBased RoutingMode = "column_based"
	RoutingModeAuto        RoutingMode = "auto"
)

// RoutingRule defines how ingested data is classified and routed to the
// correct knowledge base.
type RoutingRule struct {
	ID                     string            `json:"id"`
	OrgID                  string            `json:"org_id"`
	Name                   string            `json:"name"`
	Description            *string           `json:"description,omitempty"`
	SourceType             string            `json:"source_type"`
	SourceIdentifier       *string           `json:"source_identifier,omitempty"`
	RoutingMode            RoutingMode       `json:"routing_mode"`
	TargetKBID             *string           `json:"target_kb_id,omitempty"`
	DiscriminatorColumn    *string           `json:"discriminator_column,omitempty"`
	ColumnMappings         map[string]string `json:"column_mappings,omitempty"`
	ClassificationPrompt   *string           `json:"classification_prompt,omitempty"`
	ClassificationModel    *string           `json:"classification_model,omitempty"`
	ClassificationProvider *string           `json:"classification_provider,omitempty"`
	Priority               int               `json:"priority"`
	IsActive               bool              `json:"is_active"`
	CreatedBy              *string           `json:"created_by,omitempty"`
	CreatedAt              time.Time         `json:"created_at"`
	UpdatedAt              time.Time         `json:"updated_at"`
}

// CreateRoutingRuleRequest is the payload for POST .../routing-rules.
type CreateRoutingRuleRequest struct {
	Name                   string            `json:"name" binding:"required,min=1,max=255"`
	Description            *string           `json:"description,omitempty"`
	SourceType             string            `json:"source_type" binding:"required"`
	SourceIdentifier       *string           `json:"source_identifier,omitempty"`
	RoutingMode            RoutingMode       `json:"routing_mode" binding:"required"`
	TargetKBID             *string           `json:"target_kb_id,omitempty"`
	DiscriminatorColumn    *string           `json:"discriminator_column,omitempty"`
	ColumnMappings         map[string]string `json:"column_mappings,omitempty"`
	ClassificationPrompt   *string           `json:"classification_prompt,omitempty"`
	ClassificationModel    *string           `json:"classification_model,omitempty"`
	ClassificationProvider *string           `json:"classification_provider,omitempty"`
	Priority               int               `json:"priority"`
}

// UpdateRoutingRuleRequest is the payload for PUT .../routing-rules/:rule_id.
type UpdateRoutingRuleRequest struct {
	Name                   *string           `json:"name,omitempty" binding:"omitempty,min=1,max=255"`
	Description            *string           `json:"description,omitempty"`
	SourceType             *string           `json:"source_type,omitempty"`
	SourceIdentifier       *string           `json:"source_identifier,omitempty"`
	RoutingMode            *RoutingMode      `json:"routing_mode,omitempty"`
	TargetKBID             *string           `json:"target_kb_id,omitempty"`
	DiscriminatorColumn    *string           `json:"discriminator_column,omitempty"`
	ColumnMappings         map[string]string `json:"column_mappings,omitempty"`
	ClassificationPrompt   *string           `json:"classification_prompt,omitempty"`
	ClassificationModel    *string           `json:"classification_model,omitempty"`
	ClassificationProvider *string           `json:"classification_provider,omitempty"`
	Priority               *int              `json:"priority,omitempty"`
	IsActive               *bool             `json:"is_active,omitempty"`
}

// RoutingRuleListResponse wraps a paginated list of routing rules.
type RoutingRuleListResponse struct {
	Data     []RoutingRule `json:"data"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// ResolveRoutingRequest is the payload for POST .../routing-rules/resolve.
type ResolveRoutingRequest struct {
	SourceType       string         `json:"source_type" binding:"required"`
	SourceIdentifier string         `json:"source_identifier"`
	Metadata         map[string]any `json:"metadata"`
}

// ResolveRoutingResponse is the response for the routing resolution endpoint.
type ResolveRoutingResponse struct {
	KnowledgeBaseID string `json:"knowledge_base_id"`
	RuleName        string `json:"rule_name"`
	RuleID          string `json:"rule_id"`
}

// CatalogMetadata holds cached metadata from external data catalogs.
type CatalogMetadata struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"org_id"`
	CatalogType  string         `json:"catalog_type"`
	ResourcePath string         `json:"resource_path"`
	Labels       map[string]any `json:"labels"`
	DiscoveredAt time.Time      `json:"discovered_at"`
}
