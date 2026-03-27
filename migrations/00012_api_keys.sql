-- +goose Up
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    key_hash VARCHAR(128) NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL,
    allowed_domains TEXT[],
    rate_limit INTEGER DEFAULT 60,
    status api_key_status NOT NULL DEFAULT 'active',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_org_id ON api_keys(org_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_knowledge_base_id ON api_keys(knowledge_base_id);

-- Enable RLS
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON api_keys
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON api_keys
    FOR ALL TO raven_admin
    USING (true);

-- +goose Down
DROP POLICY IF EXISTS admin_bypass ON api_keys;
DROP POLICY IF EXISTS tenant_isolation ON api_keys;
ALTER TABLE api_keys DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS api_keys;
