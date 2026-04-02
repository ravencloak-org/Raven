package service

import (
	"net"
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
)

func TestMatchIPCIDRs_SingleIP(t *testing.T) {
	ip := net.ParseIP("192.168.1.100")
	cidrs := []string{"192.168.1.100"}
	if !matchIPCIDRs(ip, cidrs) {
		t.Error("expected IP to match exact address")
	}
}

func TestMatchIPCIDRs_CIDRRange(t *testing.T) {
	ip := net.ParseIP("10.0.0.55")
	cidrs := []string{"10.0.0.0/24"}
	if !matchIPCIDRs(ip, cidrs) {
		t.Error("expected IP to match CIDR range")
	}
}

func TestMatchIPCIDRs_NoMatch(t *testing.T) {
	ip := net.ParseIP("172.16.0.1")
	cidrs := []string{"10.0.0.0/24", "192.168.1.0/24"}
	if matchIPCIDRs(ip, cidrs) {
		t.Error("expected IP not to match any CIDR")
	}
}

func TestMatchIPCIDRs_IPv6(t *testing.T) {
	ip := net.ParseIP("2001:db8::1")
	cidrs := []string{"2001:db8::/32"}
	if !matchIPCIDRs(ip, cidrs) {
		t.Error("expected IPv6 address to match CIDR")
	}
}

func TestMatchIPCIDRs_MultipleCIDRs(t *testing.T) {
	ip := net.ParseIP("192.168.2.50")
	cidrs := []string{"10.0.0.0/8", "192.168.2.0/24", "172.16.0.0/12"}
	if !matchIPCIDRs(ip, cidrs) {
		t.Error("expected IP to match second CIDR in list")
	}
}

func TestMatchIPCIDRs_InvalidCIDR(t *testing.T) {
	ip := net.ParseIP("10.0.0.1")
	cidrs := []string{"not-a-cidr", "10.0.0.1"}
	if !matchIPCIDRs(ip, cidrs) {
		t.Error("expected IP to match plain IP after skipping invalid CIDR")
	}
}

func TestMatchPatterns_PathMatch(t *testing.T) {
	config := &model.PatternConfig{
		PathPatterns: []string{`/admin/.*`, `/api/v1/internal`},
	}
	if !matchPatterns(config, "/admin/users", "") {
		t.Error("expected path to match /admin/.*")
	}
}

func TestMatchPatterns_PathNoMatch(t *testing.T) {
	config := &model.PatternConfig{
		PathPatterns: []string{`/admin/.*`},
	}
	if matchPatterns(config, "/api/v1/chat", "") {
		t.Error("expected path not to match /admin/.*")
	}
}

func TestMatchPatterns_HeaderMatch(t *testing.T) {
	config := &model.PatternConfig{
		HeaderPatterns: []string{`(?i)sqlmap`, `(?i)nikto`},
	}
	if !matchPatterns(config, "/api/v1/chat", "sqlmap/1.5") {
		t.Error("expected user-agent to match sqlmap pattern")
	}
}

func TestMatchPatterns_HeaderNoMatch(t *testing.T) {
	config := &model.PatternConfig{
		HeaderPatterns: []string{`(?i)sqlmap`},
	}
	if matchPatterns(config, "/api/v1/chat", "Mozilla/5.0") {
		t.Error("expected user-agent not to match sqlmap pattern")
	}
}

func TestMatchPatterns_InvalidRegex(t *testing.T) {
	config := &model.PatternConfig{
		PathPatterns: []string{`[invalid`},
	}
	// Invalid regex should not cause a panic, and should not match
	if matchPatterns(config, "/anything", "") {
		t.Error("invalid regex should not produce a match")
	}
}

func TestMatchPatterns_EmptyConfig(t *testing.T) {
	config := &model.PatternConfig{}
	if matchPatterns(config, "/anything", "anything") {
		t.Error("empty pattern config should not match")
	}
}

func TestValidateCreateRequest_IPAllowlist_Valid(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:     "Allow office",
		RuleType: model.SecurityRuleIPAllowlist,
		Action:   model.SecurityActionAllow,
		IPCIDRs:  []string{"10.0.0.0/24"},
	}
	if err := ValidateCreateRequest(req); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateCreateRequest_IPDenylist_NoCIDRs(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:     "Block range",
		RuleType: model.SecurityRuleIPDenylist,
		Action:   model.SecurityActionBlock,
		IPCIDRs:  []string{},
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for empty ip_cidrs on denylist rule")
	}
}

func TestValidateCreateRequest_IPDenylist_InvalidCIDR(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:     "Block range",
		RuleType: model.SecurityRuleIPDenylist,
		Action:   model.SecurityActionBlock,
		IPCIDRs:  []string{"not-an-ip"},
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

func TestValidateCreateRequest_GeoBlock_NoCodes(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:         "Block countries",
		RuleType:     model.SecurityRuleGeoBlock,
		Action:       model.SecurityActionBlock,
		CountryCodes: []string{},
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for empty country_codes")
	}
}

func TestValidateCreateRequest_GeoBlock_InvalidCode(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:         "Block countries",
		RuleType:     model.SecurityRuleGeoBlock,
		Action:       model.SecurityActionBlock,
		CountryCodes: []string{"USA"}, // must be 2 chars
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for 3-letter country code")
	}
}

func TestValidateCreateRequest_PatternMatch_NoConfig(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:     "Block pattern",
		RuleType: model.SecurityRulePatternMatch,
		Action:   model.SecurityActionBlock,
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for missing pattern_config")
	}
}

func TestValidateCreateRequest_PatternMatch_InvalidRegex(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:     "Block pattern",
		RuleType: model.SecurityRulePatternMatch,
		Action:   model.SecurityActionBlock,
		PatternConfig: &model.PatternConfig{
			PathPatterns: []string{`[invalid`},
		},
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestValidateCreateRequest_RateOverride_NoLimit(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:     "Rate override",
		RuleType: model.SecurityRuleRateOverride,
		Action:   model.SecurityActionThrottle,
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for missing rate_limit")
	}
}

func TestValidateCreateRequest_RateOverride_NoWindow(t *testing.T) {
	limit := 100
	req := &model.CreateSecurityRuleRequest{
		Name:      "Rate override",
		RuleType:  model.SecurityRuleRateOverride,
		Action:    model.SecurityActionThrottle,
		RateLimit: &limit,
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for missing rate_window_seconds")
	}
}

func TestValidateCreateRequest_RateOverride_Valid(t *testing.T) {
	limit := 100
	window := 60
	req := &model.CreateSecurityRuleRequest{
		Name:              "Rate override",
		RuleType:          model.SecurityRuleRateOverride,
		Action:            model.SecurityActionThrottle,
		RateLimit:         &limit,
		RateWindowSeconds: &window,
	}
	if err := ValidateCreateRequest(req); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateCreateRequest_InvalidRuleType(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:     "Bad type",
		RuleType: "nonexistent",
		Action:   model.SecurityActionBlock,
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for invalid rule_type")
	}
}

func TestValidateCreateRequest_InvalidAction(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:     "Bad action",
		RuleType: model.SecurityRuleIPDenylist,
		Action:   "nonexistent",
		IPCIDRs:  []string{"10.0.0.0/8"},
	}
	if err := ValidateCreateRequest(req); err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestValidateCIDR_ValidCIDR(t *testing.T) {
	if err := validateCIDR("10.0.0.0/8"); err != nil {
		t.Errorf("expected valid CIDR, got: %v", err)
	}
}

func TestValidateCIDR_ValidIP(t *testing.T) {
	if err := validateCIDR("192.168.1.1"); err != nil {
		t.Errorf("expected valid IP, got: %v", err)
	}
}

func TestValidateCIDR_Invalid(t *testing.T) {
	if err := validateCIDR("not-valid"); err == nil {
		t.Error("expected error for invalid CIDR/IP")
	}
}

func TestValidateCreateRequest_GeoBlock_Valid(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:         "Block CN",
		RuleType:     model.SecurityRuleGeoBlock,
		Action:       model.SecurityActionBlock,
		CountryCodes: []string{"CN", "RU"},
	}
	if err := ValidateCreateRequest(req); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateCreateRequest_PatternMatch_Valid(t *testing.T) {
	req := &model.CreateSecurityRuleRequest{
		Name:     "Block scanners",
		RuleType: model.SecurityRulePatternMatch,
		Action:   model.SecurityActionBlock,
		PatternConfig: &model.PatternConfig{
			PathPatterns:   []string{`/admin/.*`},
			HeaderPatterns: []string{`(?i)sqlmap`},
		},
	}
	if err := ValidateCreateRequest(req); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}
