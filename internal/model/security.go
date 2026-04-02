package model

import "time"

// SecurityRuleType enumerates the supported rule types.
type SecurityRuleType string

const (
	SecurityRuleIPAllowlist  SecurityRuleType = "ip_allowlist"
	SecurityRuleIPDenylist   SecurityRuleType = "ip_denylist"
	SecurityRuleGeoBlock     SecurityRuleType = "geo_block"
	SecurityRulePatternMatch SecurityRuleType = "pattern_match"
	SecurityRuleRateOverride SecurityRuleType = "rate_override"
)

// SecurityActionType enumerates the possible actions for a matched rule.
type SecurityActionType string

const (
	SecurityActionAllow    SecurityActionType = "allow"
	SecurityActionBlock    SecurityActionType = "block"
	SecurityActionThrottle SecurityActionType = "throttle"
	SecurityActionLog      SecurityActionType = "log"
	SecurityActionAlert    SecurityActionType = "alert"
)

// PatternConfig holds the regex configuration for pattern_match rules.
type PatternConfig struct {
	PathPatterns   []string `json:"path_patterns,omitempty"`
	HeaderPatterns []string `json:"header_patterns,omitempty"`
}

// SecurityRule represents a WAF-style security rule stored in the database.
type SecurityRule struct {
	ID                string             `json:"id"`
	OrgID             string             `json:"org_id"`
	Name              string             `json:"name"`
	Description       string             `json:"description,omitempty"`
	RuleType          SecurityRuleType   `json:"rule_type"`
	Action            SecurityActionType `json:"action"`
	IPCIDRs           []string           `json:"ip_cidrs,omitempty"`
	CountryCodes      []string           `json:"country_codes,omitempty"`
	PatternConfig     *PatternConfig     `json:"pattern_config,omitempty"`
	RateLimit         *int               `json:"rate_limit,omitempty"`
	RateWindowSeconds *int               `json:"rate_window_seconds,omitempty"`
	Priority          int                `json:"priority"`
	IsActive          bool               `json:"is_active"`
	HitsCount         int64              `json:"hits_count"`
	LastHitAt         *time.Time         `json:"last_hit_at,omitempty"`
	CreatedBy         string             `json:"created_by,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// CreateSecurityRuleRequest is the payload for creating a security rule.
type CreateSecurityRuleRequest struct {
	Name              string             `json:"name" binding:"required,min=2,max=255"`
	Description       string             `json:"description,omitempty"`
	RuleType          SecurityRuleType   `json:"rule_type" binding:"required"`
	Action            SecurityActionType `json:"action" binding:"required"`
	IPCIDRs           []string           `json:"ip_cidrs,omitempty"`
	CountryCodes      []string           `json:"country_codes,omitempty"`
	PatternConfig     *PatternConfig     `json:"pattern_config,omitempty"`
	RateLimit         *int               `json:"rate_limit,omitempty"`
	RateWindowSeconds *int               `json:"rate_window_seconds,omitempty"`
	Priority          *int               `json:"priority,omitempty"`
	IsActive          *bool              `json:"is_active,omitempty"`
}

// UpdateSecurityRuleRequest is the payload for updating a security rule.
type UpdateSecurityRuleRequest struct {
	Name              *string             `json:"name,omitempty" binding:"omitempty,min=2,max=255"`
	Description       *string             `json:"description,omitempty"`
	Action            *SecurityActionType `json:"action,omitempty"`
	IPCIDRs           []string            `json:"ip_cidrs,omitempty"`
	CountryCodes      []string            `json:"country_codes,omitempty"`
	PatternConfig     *PatternConfig      `json:"pattern_config,omitempty"`
	RateLimit         *int                `json:"rate_limit,omitempty"`
	RateWindowSeconds *int                `json:"rate_window_seconds,omitempty"`
	Priority          *int                `json:"priority,omitempty"`
	IsActive          *bool               `json:"is_active,omitempty"`
}

// SecurityEvent represents a security event in the audit log.
type SecurityEvent struct {
	ID            string         `json:"id"`
	OrgID         string         `json:"org_id"`
	RuleID        string         `json:"rule_id,omitempty"`
	EventType     string         `json:"event_type"`
	IPAddress     string         `json:"ip_address"`
	CountryCode   string         `json:"country_code,omitempty"`
	RequestPath   string         `json:"request_path,omitempty"`
	RequestMethod string         `json:"request_method,omitempty"`
	UserAgent     string         `json:"user_agent,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

// SecurityEventResponse is a paginated list of security events.
type SecurityEventResponse struct {
	Events []SecurityEvent `json:"events"`
	Total  int             `json:"total"`
}
