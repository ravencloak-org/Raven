package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

const (
	securityRulesCachePrefix = "raven:security:rules:"
	securityRulesCacheTTL    = 5 * time.Minute
)

// SecurityAction is the result of evaluating security rules against a request.
type SecurityAction struct {
	Block    bool
	Throttle bool
	RuleID   string
	RuleName string
	Action   model.SecurityActionType
}

// SecurityService contains business logic for security rules management.
type SecurityService struct {
	repo         *repository.SecurityRepository
	pool         *pgxpool.Pool
	valkeyClient redis.Cmdable
	logger       *slog.Logger
}

// NewSecurityService creates a new SecurityService.
func NewSecurityService(repo *repository.SecurityRepository, pool *pgxpool.Pool, valkeyClient redis.Cmdable) *SecurityService {
	return &SecurityService{
		repo:         repo,
		pool:         pool,
		valkeyClient: valkeyClient,
		logger:       slog.Default(),
	}
}

// ValidateCreateRequest validates a CreateSecurityRuleRequest based on rule type.
func ValidateCreateRequest(req *model.CreateSecurityRuleRequest) error {
	// Validate rule_type enum
	switch req.RuleType {
	case model.SecurityRuleIPAllowlist, model.SecurityRuleIPDenylist,
		model.SecurityRuleGeoBlock, model.SecurityRulePatternMatch,
		model.SecurityRuleRateOverride:
		// valid
	default:
		return fmt.Errorf("invalid rule_type: %s", req.RuleType)
	}

	// Validate action enum
	switch req.Action {
	case model.SecurityActionAllow, model.SecurityActionBlock,
		model.SecurityActionThrottle, model.SecurityActionLog,
		model.SecurityActionAlert:
		// valid
	default:
		return fmt.Errorf("invalid action: %s", req.Action)
	}

	switch req.RuleType {
	case model.SecurityRuleIPAllowlist, model.SecurityRuleIPDenylist:
		if len(req.IPCIDRs) == 0 {
			return fmt.Errorf("ip_cidrs is required for %s rules", req.RuleType)
		}
		for _, cidr := range req.IPCIDRs {
			if err := validateCIDR(cidr); err != nil {
				return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
			}
		}
	case model.SecurityRuleGeoBlock:
		if len(req.CountryCodes) == 0 {
			return fmt.Errorf("country_codes is required for geo_block rules")
		}
		for _, cc := range req.CountryCodes {
			if len(cc) != 2 {
				return fmt.Errorf("country code must be ISO 3166-1 alpha-2 (2 letters), got: %q", cc)
			}
		}
	case model.SecurityRulePatternMatch:
		if req.PatternConfig == nil || (len(req.PatternConfig.PathPatterns) == 0 && len(req.PatternConfig.HeaderPatterns) == 0) {
			return fmt.Errorf("pattern_config with at least one pattern is required for pattern_match rules")
		}
		for _, p := range req.PatternConfig.PathPatterns {
			if _, err := regexp.Compile(p); err != nil {
				return fmt.Errorf("invalid path regex %q: %w", p, err)
			}
		}
		for _, p := range req.PatternConfig.HeaderPatterns {
			if _, err := regexp.Compile(p); err != nil {
				return fmt.Errorf("invalid header regex %q: %w", p, err)
			}
		}
	case model.SecurityRuleRateOverride:
		if req.RateLimit == nil || *req.RateLimit <= 0 {
			return fmt.Errorf("rate_limit must be > 0 for rate_override rules")
		}
		if req.RateWindowSeconds == nil || *req.RateWindowSeconds <= 0 {
			return fmt.Errorf("rate_window_seconds must be > 0 for rate_override rules")
		}
	}

	return nil
}

// validateCIDR checks that a string is valid CIDR or a single IP address.
func validateCIDR(cidr string) error {
	// Try parsing as CIDR first
	_, _, err := net.ParseCIDR(cidr)
	if err == nil {
		return nil
	}
	// Fall back to plain IP (single host)
	if net.ParseIP(cidr) != nil {
		return nil
	}
	return fmt.Errorf("not a valid CIDR or IP address")
}

// Create validates and creates a new security rule.
func (s *SecurityService) Create(ctx context.Context, orgID, userID string, req model.CreateSecurityRuleRequest) (*model.SecurityRule, error) {
	if err := ValidateCreateRequest(&req); err != nil {
		return nil, apierror.NewBadRequest(err.Error())
	}

	rule := &model.SecurityRule{
		OrgID:             orgID,
		Name:              req.Name,
		Description:       req.Description,
		RuleType:          req.RuleType,
		Action:            req.Action,
		IPCIDRs:           req.IPCIDRs,
		CountryCodes:      req.CountryCodes,
		PatternConfig:     req.PatternConfig,
		RateLimit:         req.RateLimit,
		RateWindowSeconds: req.RateWindowSeconds,
		Priority:          0,
		IsActive:          true,
		CreatedBy:         userID,
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.IsActive != nil {
		rule.IsActive = *req.IsActive
	}

	var created *model.SecurityRule
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		created, e = s.repo.Create(ctx, tx, rule)
		return e
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apierror.NewBadRequest("a security rule with this name already exists")
		}
		return nil, apierror.NewInternal("failed to create security rule: " + err.Error())
	}

	// Invalidate cache for this org
	s.invalidateCache(ctx, orgID)

	return created, nil
}

// GetByID retrieves a security rule by ID.
func (s *SecurityService) GetByID(ctx context.Context, orgID, ruleID string) (*model.SecurityRule, error) {
	var rule *model.SecurityRule
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		rule, e = s.repo.GetByID(ctx, tx, orgID, ruleID)
		return e
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("security rule not found")
		}
		return nil, apierror.NewInternal("failed to fetch security rule: " + err.Error())
	}
	return rule, nil
}

// List returns all security rules for an org.
func (s *SecurityService) List(ctx context.Context, orgID string) ([]model.SecurityRule, error) {
	var rules []model.SecurityRule
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		rules, e = s.repo.List(ctx, tx, orgID)
		return e
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list security rules: " + err.Error())
	}
	return rules, nil
}

// Update applies partial updates to a security rule.
func (s *SecurityService) Update(ctx context.Context, orgID, ruleID string, req model.UpdateSecurityRuleRequest) (*model.SecurityRule, error) {
	var rule *model.SecurityRule
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		rule, e = s.repo.Update(ctx, tx, orgID, ruleID, &req)
		return e
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("security rule not found")
		}
		return nil, apierror.NewInternal("failed to update security rule: " + err.Error())
	}

	// Invalidate cache for this org
	s.invalidateCache(ctx, orgID)

	return rule, nil
}

// Delete removes a security rule.
func (s *SecurityService) Delete(ctx context.Context, orgID, ruleID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, orgID, ruleID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("security rule not found")
		}
		return apierror.NewInternal("failed to delete security rule: " + err.Error())
	}

	// Invalidate cache for this org
	s.invalidateCache(ctx, orgID)

	return nil
}

// ListEvents returns security events for an org.
func (s *SecurityService) ListEvents(ctx context.Context, orgID string, limit, offset int) (*model.SecurityEventResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var events []model.SecurityEvent
	var total int
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		events, total, e = s.repo.ListSecurityEvents(ctx, tx, orgID, limit, offset)
		return e
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list security events: " + err.Error())
	}
	if events == nil {
		events = []model.SecurityEvent{}
	}
	return &model.SecurityEventResponse{Events: events, Total: total}, nil
}

// InvalidateCache deletes the cached security rules for an org.
func (s *SecurityService) InvalidateCache(ctx context.Context, orgID string) {
	s.invalidateCache(ctx, orgID)
}

func (s *SecurityService) invalidateCache(ctx context.Context, orgID string) {
	key := securityRulesCachePrefix + orgID
	callCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if err := s.valkeyClient.Del(callCtx, key).Err(); err != nil {
		s.logger.WarnContext(ctx, "security: failed to invalidate rules cache",
			slog.String("org_id", orgID),
			slog.String("error", err.Error()),
		)
	}
}

// LoadRulesForOrg loads active security rules, trying cache first, then DB.
func (s *SecurityService) LoadRulesForOrg(ctx context.Context, orgID string) ([]model.SecurityRule, error) {
	key := securityRulesCachePrefix + orgID

	// Try cache first
	callCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	cached, err := s.valkeyClient.Get(callCtx, key).Bytes()
	if err == nil && len(cached) > 0 {
		var rules []model.SecurityRule
		if e := json.Unmarshal(cached, &rules); e == nil {
			return rules, nil
		}
	}

	// Cache miss — load from DB
	var rules []model.SecurityRule
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		rules, e = s.repo.ListActiveRules(ctx, tx, orgID)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("load active rules: %w", err)
	}
	if rules == nil {
		rules = []model.SecurityRule{}
	}

	// Cache the result
	data, err := json.Marshal(rules)
	if err == nil {
		cacheCtx, cacheCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cacheCancel()
		if e := s.valkeyClient.Set(cacheCtx, key, data, securityRulesCacheTTL).Err(); e != nil {
			s.logger.WarnContext(ctx, "security: failed to cache rules",
				slog.String("org_id", orgID),
				slog.String("error", e.Error()),
			)
		}
	}

	return rules, nil
}

// EvaluateRequest evaluates security rules against an incoming request.
// Returns nil if no rule matches (allow by default).
func (s *SecurityService) EvaluateRequest(ctx context.Context, orgID, clientIP, path, method, userAgent string) (*SecurityAction, error) {
	rules, err := s.LoadRulesForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}

	parsedIP := net.ParseIP(clientIP)

	for i := range rules {
		rule := &rules[i]
		if !rule.IsActive {
			continue
		}

		matched := false

		switch rule.RuleType {
		case model.SecurityRuleIPAllowlist:
			if parsedIP != nil && matchIPCIDRs(parsedIP, rule.IPCIDRs) {
				// IP allowlist match means explicitly allow — short circuit
				s.recordHitAsync(ctx, orgID, rule.ID)
				return nil, nil
			}
		case model.SecurityRuleIPDenylist:
			if parsedIP != nil && matchIPCIDRs(parsedIP, rule.IPCIDRs) {
				matched = true
			}
		case model.SecurityRuleGeoBlock:
			// TODO(geo-ip): Requires MaxMind DB integration to resolve IP to country.
			// For now, log the rule existence but do not block.
			continue
		case model.SecurityRulePatternMatch:
			if rule.PatternConfig != nil && matchPatterns(rule.PatternConfig, path, userAgent) {
				matched = true
			}
		case model.SecurityRuleRateOverride:
			// Rate overrides are handled separately by the rate limiter.
			// This rule type flags that a custom rate limit should apply.
			if rule.RateLimit != nil && rule.RateWindowSeconds != nil {
				s.recordHitAsync(ctx, orgID, rule.ID)
				return &SecurityAction{
					Throttle: true,
					RuleID:   rule.ID,
					RuleName: rule.Name,
					Action:   rule.Action,
				}, nil
			}
		}

		if matched {
			s.recordHitAsync(ctx, orgID, rule.ID)
			s.logEventAsync(ctx, orgID, rule.ID, string(rule.Action), clientIP, path, method, userAgent)

			action := &SecurityAction{
				RuleID:   rule.ID,
				RuleName: rule.Name,
				Action:   rule.Action,
			}
			switch rule.Action {
			case model.SecurityActionBlock:
				action.Block = true
			case model.SecurityActionThrottle:
				action.Throttle = true
			case model.SecurityActionLog, model.SecurityActionAlert:
				// Log/alert only — don't block the request
				return nil, nil
			}
			return action, nil
		}
	}

	return nil, nil
}

// matchIPCIDRs checks if an IP matches any of the CIDRs in the list.
func matchIPCIDRs(ip net.IP, cidrs []string) bool {
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			// Try plain IP comparison
			if ruleIP := net.ParseIP(cidr); ruleIP != nil && ruleIP.Equal(ip) {
				return true
			}
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// matchPatterns checks if the request path or user agent matches any pattern.
func matchPatterns(config *model.PatternConfig, path, userAgent string) bool {
	for _, p := range config.PathPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		if re.MatchString(path) {
			return true
		}
	}
	for _, p := range config.HeaderPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		// Match header patterns against user-agent as the primary header
		if re.MatchString(userAgent) {
			return true
		}
	}
	return false
}

// recordHitAsync increments the hit counter in a background goroutine to avoid
// blocking the request path.
func (s *SecurityService) recordHitAsync(ctx context.Context, orgID, ruleID string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := db.WithOrgID(bgCtx, s.pool, orgID, func(tx pgx.Tx) error {
			return s.repo.IncrementHitCount(bgCtx, tx, ruleID)
		})
		if err != nil {
			s.logger.Warn("security: failed to increment hit count",
				slog.String("rule_id", ruleID),
				slog.String("error", err.Error()),
			)
		}
	}()
	_ = ctx // consumed by caller; background goroutine uses its own context
}

// logEventAsync logs a security event in a background goroutine.
func (s *SecurityService) logEventAsync(ctx context.Context, orgID, ruleID, eventType, clientIP, path, method, userAgent string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := db.WithOrgID(bgCtx, s.pool, orgID, func(tx pgx.Tx) error {
			return s.repo.LogSecurityEvent(bgCtx, tx, &model.SecurityEvent{
				OrgID:         orgID,
				RuleID:        ruleID,
				EventType:     eventType,
				IPAddress:     clientIP,
				RequestPath:   path,
				RequestMethod: method,
				UserAgent:     userAgent,
			})
		})
		if err != nil {
			s.logger.Warn("security: failed to log security event",
				slog.String("org_id", orgID),
				slog.String("error", err.Error()),
			)
		}
	}()
	_ = ctx // consumed by caller; background goroutine uses its own context
}
