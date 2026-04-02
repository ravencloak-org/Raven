-- +goose Up

CREATE TYPE stranger_status AS ENUM ('active', 'throttled', 'blocked', 'banned');

CREATE TABLE stranger_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    session_id VARCHAR(255) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    status stranger_status NOT NULL DEFAULT 'active',
    block_reason TEXT,
    message_count INTEGER NOT NULL DEFAULT 0,
    rate_limit_rpm INTEGER,
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    blocked_at TIMESTAMPTZ,
    blocked_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, session_id)
);

CREATE INDEX idx_stranger_users_org ON stranger_users(org_id);
CREATE INDEX idx_stranger_users_org_status ON stranger_users(org_id, status);
CREATE INDEX idx_stranger_users_ip ON stranger_users(org_id, ip_address);
CREATE INDEX idx_stranger_users_last_active ON stranger_users(org_id, last_active_at DESC);

ALTER TABLE stranger_users ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON stranger_users FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON stranger_users FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_stranger_users_updated_at
    BEFORE UPDATE ON stranger_users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down

DROP TRIGGER IF EXISTS trg_stranger_users_updated_at ON stranger_users;
DROP POLICY IF EXISTS admin_bypass ON stranger_users;
DROP POLICY IF EXISTS tenant_isolation ON stranger_users;
DROP TABLE IF EXISTS stranger_users;
DROP TYPE IF EXISTS stranger_status;
