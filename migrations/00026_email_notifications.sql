-- +goose Up

CREATE TYPE notification_type AS ENUM ('conversation_summary', 'admin_digest', 'custom');
CREATE TYPE notification_status AS ENUM ('pending', 'sent', 'failed');

CREATE TABLE notification_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    notification_type notification_type NOT NULL,
    recipients TEXT[] NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uniq_notification_configs_org_type UNIQUE (org_id, notification_type)
);

CREATE INDEX idx_notification_configs_org ON notification_configs(org_id);
CREATE INDEX idx_notification_configs_type ON notification_configs(org_id, notification_type) WHERE enabled = true;

ALTER TABLE notification_configs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification_configs FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON notification_configs FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_notification_configs_updated_at
    BEFORE UPDATE ON notification_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TABLE notification_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    config_id UUID REFERENCES notification_configs(id) ON DELETE SET NULL,
    notification_type notification_type NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    subject VARCHAR(500) NOT NULL,
    status notification_status NOT NULL DEFAULT 'pending',
    error_message TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_log_org ON notification_log(org_id);
CREATE INDEX idx_notification_log_status ON notification_log(org_id, status);
CREATE INDEX idx_notification_log_time ON notification_log(org_id, created_at DESC);

ALTER TABLE notification_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification_log FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON notification_log FOR ALL TO raven_admin USING (true);

-- +goose Down
DROP TABLE IF EXISTS notification_log;
DROP TABLE IF EXISTS notification_configs;
DROP TYPE IF EXISTS notification_status;
DROP TYPE IF EXISTS notification_type;
