package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// SecurityRepository handles database operations for security rules and events.
type SecurityRepository struct {
	pool *pgxpool.Pool
}

// NewSecurityRepository creates a new SecurityRepository.
func NewSecurityRepository(pool *pgxpool.Pool) *SecurityRepository {
	return &SecurityRepository{pool: pool}
}

const securityRuleCols = `id, org_id, name, COALESCE(description, '') AS description,
	rule_type, action,
	COALESCE(ip_cidrs, '{}') AS ip_cidrs,
	COALESCE(country_codes, '{}') AS country_codes,
	COALESCE(pattern_config, '{}') AS pattern_config,
	rate_limit, rate_window_seconds,
	priority, is_active, hits_count, last_hit_at,
	COALESCE(created_by::text, '') AS created_by,
	created_at, updated_at`

func scanSecurityRule(row pgx.Row) (*model.SecurityRule, error) {
	var r model.SecurityRule
	var patternConfigBytes []byte
	err := row.Scan(
		&r.ID, &r.OrgID, &r.Name, &r.Description,
		&r.RuleType, &r.Action,
		&r.IPCIDRs, &r.CountryCodes,
		&patternConfigBytes,
		&r.RateLimit, &r.RateWindowSeconds,
		&r.Priority, &r.IsActive, &r.HitsCount, &r.LastHitAt,
		&r.CreatedBy,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if r.IPCIDRs == nil {
		r.IPCIDRs = []string{}
	}
	if r.CountryCodes == nil {
		r.CountryCodes = []string{}
	}
	if len(patternConfigBytes) > 2 { // more than "{}"
		var pc model.PatternConfig
		if e := json.Unmarshal(patternConfigBytes, &pc); e == nil {
			r.PatternConfig = &pc
		}
	}
	return &r, nil
}

// Create inserts a new security rule.
func (r *SecurityRepository) Create(ctx context.Context, tx pgx.Tx, rule *model.SecurityRule) (*model.SecurityRule, error) {
	patternBytes, err := json.Marshal(rule.PatternConfig)
	if err != nil {
		patternBytes = []byte("{}")
	}
	if rule.IPCIDRs == nil {
		rule.IPCIDRs = []string{}
	}
	if rule.CountryCodes == nil {
		rule.CountryCodes = []string{}
	}

	row := tx.QueryRow(ctx,
		`INSERT INTO security_rules (org_id, name, description, rule_type, action,
			ip_cidrs, country_codes, pattern_config,
			rate_limit, rate_window_seconds, priority, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING `+securityRuleCols,
		rule.OrgID, rule.Name, rule.Description, rule.RuleType, rule.Action,
		rule.IPCIDRs, rule.CountryCodes, patternBytes,
		rule.RateLimit, rule.RateWindowSeconds, rule.Priority, rule.IsActive, rule.CreatedBy,
	)
	created, err := scanSecurityRule(row)
	if err != nil {
		return nil, fmt.Errorf("SecurityRepository.Create: %w", err)
	}
	return created, nil
}

// GetByID fetches a security rule by ID within an org.
func (r *SecurityRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, ruleID string) (*model.SecurityRule, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+securityRuleCols+` FROM security_rules WHERE id = $1 AND org_id = $2`,
		ruleID, orgID,
	)
	rule, err := scanSecurityRule(row)
	if err != nil {
		return nil, fmt.Errorf("SecurityRepository.GetByID: %w", err)
	}
	return rule, nil
}

// List returns all security rules for an org, ordered by priority descending.
func (r *SecurityRepository) List(ctx context.Context, tx pgx.Tx, orgID string) ([]model.SecurityRule, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+securityRuleCols+` FROM security_rules WHERE org_id = $1 ORDER BY priority DESC, created_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("SecurityRepository.List: %w", err)
	}
	defer rows.Close()

	var rules []model.SecurityRule
	for rows.Next() {
		var rule model.SecurityRule
		var patternConfigBytes []byte
		if err := rows.Scan(
			&rule.ID, &rule.OrgID, &rule.Name, &rule.Description,
			&rule.RuleType, &rule.Action,
			&rule.IPCIDRs, &rule.CountryCodes,
			&patternConfigBytes,
			&rule.RateLimit, &rule.RateWindowSeconds,
			&rule.Priority, &rule.IsActive, &rule.HitsCount, &rule.LastHitAt,
			&rule.CreatedBy,
			&rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("SecurityRepository.List scan: %w", err)
		}
		if rule.IPCIDRs == nil {
			rule.IPCIDRs = []string{}
		}
		if rule.CountryCodes == nil {
			rule.CountryCodes = []string{}
		}
		if len(patternConfigBytes) > 2 {
			var pc model.PatternConfig
			if e := json.Unmarshal(patternConfigBytes, &pc); e == nil {
				rule.PatternConfig = &pc
			}
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// ListActiveRules returns all active security rules for an org, ordered by priority descending.
// Used by the middleware cache loader.
func (r *SecurityRepository) ListActiveRules(ctx context.Context, tx pgx.Tx, orgID string) ([]model.SecurityRule, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+securityRuleCols+` FROM security_rules WHERE org_id = $1 AND is_active = true ORDER BY priority DESC, created_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("SecurityRepository.ListActiveRules: %w", err)
	}
	defer rows.Close()

	var rules []model.SecurityRule
	for rows.Next() {
		var rule model.SecurityRule
		var patternConfigBytes []byte
		if err := rows.Scan(
			&rule.ID, &rule.OrgID, &rule.Name, &rule.Description,
			&rule.RuleType, &rule.Action,
			&rule.IPCIDRs, &rule.CountryCodes,
			&patternConfigBytes,
			&rule.RateLimit, &rule.RateWindowSeconds,
			&rule.Priority, &rule.IsActive, &rule.HitsCount, &rule.LastHitAt,
			&rule.CreatedBy,
			&rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("SecurityRepository.ListActiveRules scan: %w", err)
		}
		if rule.IPCIDRs == nil {
			rule.IPCIDRs = []string{}
		}
		if rule.CountryCodes == nil {
			rule.CountryCodes = []string{}
		}
		if len(patternConfigBytes) > 2 {
			var pc model.PatternConfig
			if e := json.Unmarshal(patternConfigBytes, &pc); e == nil {
				rule.PatternConfig = &pc
			}
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// Update applies partial updates to a security rule.
func (r *SecurityRepository) Update(ctx context.Context, tx pgx.Tx, orgID, ruleID string, req *model.UpdateSecurityRuleRequest) (*model.SecurityRule, error) {
	patternBytes, err := json.Marshal(req.PatternConfig)
	if err != nil {
		patternBytes = nil
	}
	if req.PatternConfig == nil {
		patternBytes = nil
	}

	row := tx.QueryRow(ctx,
		`UPDATE security_rules SET
			name = COALESCE($3, name),
			description = COALESCE($4, description),
			action = COALESCE($5, action),
			ip_cidrs = COALESCE($6, ip_cidrs),
			country_codes = COALESCE($7, country_codes),
			pattern_config = COALESCE($8, pattern_config),
			rate_limit = COALESCE($9, rate_limit),
			rate_window_seconds = COALESCE($10, rate_window_seconds),
			priority = COALESCE($11, priority),
			is_active = COALESCE($12, is_active)
		WHERE id = $1 AND org_id = $2
		RETURNING `+securityRuleCols,
		ruleID, orgID,
		req.Name, req.Description, req.Action,
		req.IPCIDRs, req.CountryCodes, patternBytes,
		req.RateLimit, req.RateWindowSeconds, req.Priority, req.IsActive,
	)
	rule, err := scanSecurityRule(row)
	if err != nil {
		return nil, fmt.Errorf("SecurityRepository.Update: %w", err)
	}
	return rule, nil
}

// Delete removes a security rule by ID.
func (r *SecurityRepository) Delete(ctx context.Context, tx pgx.Tx, orgID, ruleID string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM security_rules WHERE id = $1 AND org_id = $2`,
		ruleID, orgID,
	)
	if err != nil {
		return fmt.Errorf("SecurityRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("SecurityRepository.Delete: rule %s not found", ruleID)
	}
	return nil
}

// IncrementHitCount atomically increments the hit counter and updates last_hit_at.
func (r *SecurityRepository) IncrementHitCount(ctx context.Context, tx pgx.Tx, ruleID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE security_rules SET hits_count = hits_count + 1, last_hit_at = NOW() WHERE id = $1`,
		ruleID,
	)
	if err != nil {
		return fmt.Errorf("SecurityRepository.IncrementHitCount: %w", err)
	}
	return nil
}

// LogSecurityEvent inserts a security event into the audit log.
func (r *SecurityRepository) LogSecurityEvent(ctx context.Context, tx pgx.Tx, event *model.SecurityEvent) error {
	detailsBytes, err := json.Marshal(event.Details)
	if err != nil {
		detailsBytes = []byte("{}")
	}

	var ruleID any
	if event.RuleID != "" {
		ruleID = event.RuleID
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO security_events (org_id, rule_id, event_type, ip_address,
			country_code, request_path, request_method, user_agent, details)
		VALUES ($1, $2, $3, $4::inet, $5, $6, $7, $8, $9)`,
		event.OrgID, ruleID, event.EventType, event.IPAddress,
		event.CountryCode, event.RequestPath, event.RequestMethod, event.UserAgent, detailsBytes,
	)
	if err != nil {
		return fmt.Errorf("SecurityRepository.LogSecurityEvent: %w", err)
	}
	return nil
}

// ListSecurityEvents returns security events for an org, ordered by most recent first.
func (r *SecurityRepository) ListSecurityEvents(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.SecurityEvent, int, error) {
	// Get total count
	var total int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM security_events WHERE org_id = $1`,
		orgID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("SecurityRepository.ListSecurityEvents count: %w", err)
	}

	rows, err := tx.Query(ctx,
		`SELECT id, org_id, COALESCE(rule_id::text, '') AS rule_id, event_type,
			ip_address::text, COALESCE(country_code, '') AS country_code,
			COALESCE(request_path, '') AS request_path,
			COALESCE(request_method, '') AS request_method,
			COALESCE(user_agent, '') AS user_agent,
			COALESCE(details, '{}') AS details, created_at
		FROM security_events WHERE org_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		orgID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("SecurityRepository.ListSecurityEvents: %w", err)
	}
	defer rows.Close()

	var events []model.SecurityEvent
	for rows.Next() {
		var ev model.SecurityEvent
		var detailsBytes []byte
		if err := rows.Scan(
			&ev.ID, &ev.OrgID, &ev.RuleID, &ev.EventType,
			&ev.IPAddress, &ev.CountryCode,
			&ev.RequestPath, &ev.RequestMethod, &ev.UserAgent,
			&detailsBytes, &ev.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("SecurityRepository.ListSecurityEvents scan: %w", err)
		}
		if len(detailsBytes) > 2 {
			_ = json.Unmarshal(detailsBytes, &ev.Details)
		}
		events = append(events, ev)
	}
	return events, total, rows.Err()
}
