-- +goose Up
CREATE TABLE llm_provider_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider llm_provider NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    api_key_encrypted BYTEA,
    api_key_iv BYTEA,
    api_key_hint VARCHAR(20),
    base_url TEXT,
    config JSONB DEFAULT '{}',
    is_default BOOLEAN NOT NULL DEFAULT false,
    status provider_status NOT NULL DEFAULT 'active',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_llm_provider_configs_org_id ON llm_provider_configs(org_id);

-- Enable RLS
ALTER TABLE llm_provider_configs ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON llm_provider_configs
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON llm_provider_configs
    FOR ALL TO raven_admin
    USING (true);

-- Updated_at trigger
CREATE TRIGGER trg_llm_provider_configs_updated_at
    BEFORE UPDATE ON llm_provider_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_llm_provider_configs_updated_at ON llm_provider_configs;
DROP POLICY IF EXISTS admin_bypass ON llm_provider_configs;
DROP POLICY IF EXISTS tenant_isolation ON llm_provider_configs;
ALTER TABLE llm_provider_configs DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS llm_provider_configs;
