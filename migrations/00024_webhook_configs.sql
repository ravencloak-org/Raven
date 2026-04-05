-- +goose Up

CREATE TYPE webhook_status AS ENUM ('active', 'paused', 'failed');

CREATE TABLE webhook_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL,
    events TEXT[] NOT NULL,
    headers JSONB DEFAULT '{}',
    status webhook_status NOT NULL DEFAULT 'active',
    max_retries INTEGER NOT NULL DEFAULT 5,
    last_triggered_at TIMESTAMPTZ,
    failure_count INTEGER NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_webhook_configs_id_org ON webhook_configs(id, org_id);
CREATE INDEX idx_webhook_configs_org ON webhook_configs(org_id);
CREATE INDEX idx_webhook_configs_events ON webhook_configs USING GIN(events);
CREATE INDEX idx_webhook_configs_status ON webhook_configs(org_id, status) WHERE status = 'active';

ALTER TABLE webhook_configs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON webhook_configs FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON webhook_configs FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_webhook_configs_updated_at
    BEFORE UPDATE ON webhook_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    webhook_id UUID NOT NULL,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_webhook_deliveries_config
        FOREIGN KEY (webhook_id, org_id)
        REFERENCES webhook_configs(id, org_id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    attempt INTEGER NOT NULL DEFAULT 0,
    response_status INTEGER,
    response_body TEXT,
    next_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_org ON webhook_deliveries(org_id);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(webhook_id, status);
CREATE INDEX idx_webhook_deliveries_retry ON webhook_deliveries(next_retry_at) WHERE status = 'pending';

ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON webhook_deliveries FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON webhook_deliveries FOR ALL TO raven_admin USING (true);

-- +goose Down
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_configs;
DROP TYPE IF EXISTS webhook_status;
