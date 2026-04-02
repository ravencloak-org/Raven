-- +goose Up

CREATE TYPE security_rule_type AS ENUM ('ip_allowlist', 'ip_denylist', 'geo_block', 'pattern_match', 'rate_override');
CREATE TYPE security_action AS ENUM ('allow', 'block', 'throttle', 'log', 'alert');

CREATE TABLE security_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    rule_type security_rule_type NOT NULL,
    action security_action NOT NULL DEFAULT 'block',
    -- IP rules: CIDR notation (single IP or range)
    ip_cidrs TEXT[],
    -- Geo rules: ISO 3166-1 alpha-2 country codes
    country_codes TEXT[],
    -- Pattern rules: regex patterns for path/header/body matching
    pattern_config JSONB DEFAULT '{}',
    -- Rate override: custom rate limit for matched requests
    rate_limit INTEGER,
    rate_window_seconds INTEGER,
    -- Metadata
    priority INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    hits_count BIGINT NOT NULL DEFAULT 0,
    last_hit_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_security_rules_org ON security_rules(org_id);
CREATE INDEX idx_security_rules_type ON security_rules(org_id, rule_type) WHERE is_active = true;

ALTER TABLE security_rules ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON security_rules FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON security_rules FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_security_rules_updated_at
    BEFORE UPDATE ON security_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Security event log for audit trail
CREATE TABLE security_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    rule_id UUID REFERENCES security_rules(id) ON DELETE SET NULL,
    event_type VARCHAR(50) NOT NULL,  -- 'blocked', 'throttled', 'alert', 'suspicious'
    ip_address INET NOT NULL,
    country_code VARCHAR(2),
    request_path VARCHAR(1000),
    request_method VARCHAR(10),
    user_agent TEXT,
    details JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_security_events_org ON security_events(org_id);
CREATE INDEX idx_security_events_time ON security_events(org_id, created_at DESC);
CREATE INDEX idx_security_events_ip ON security_events(ip_address);

ALTER TABLE security_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON security_events FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON security_events FOR ALL TO raven_admin USING (true);

-- +goose Down
DROP TABLE IF EXISTS security_events;
DROP TABLE IF EXISTS security_rules;
DROP TYPE IF EXISTS security_action;
DROP TYPE IF EXISTS security_rule_type;
