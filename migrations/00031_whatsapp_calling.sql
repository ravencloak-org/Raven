-- +goose Up
-- Issue #65 — WhatsApp Business Calling API Integration
--
-- whatsapp_phone_numbers stores provisioned WhatsApp Business phone numbers
-- scoped to an org, along with their WABA (WhatsApp Business Account) ID.
-- whatsapp_calls tracks individual call records with Meta's call ID, direction,
-- state, and duration.

CREATE TYPE whatsapp_call_direction AS ENUM ('inbound', 'outbound');
CREATE TYPE whatsapp_call_state AS ENUM ('ringing', 'connected', 'ended');

CREATE TABLE whatsapp_phone_numbers (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    phone_number    VARCHAR(20) NOT NULL,
    display_name    VARCHAR(255) NOT NULL DEFAULT '',
    waba_id         VARCHAR(64) NOT NULL,
    verified        BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_whatsapp_phone_org UNIQUE (org_id, phone_number)
);

CREATE INDEX idx_whatsapp_phones_org ON whatsapp_phone_numbers(org_id);
CREATE INDEX idx_whatsapp_phones_waba ON whatsapp_phone_numbers(waba_id);

ALTER TABLE whatsapp_phone_numbers ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON whatsapp_phone_numbers FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON whatsapp_phone_numbers FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_whatsapp_phone_numbers_updated_at
    BEFORE UPDATE ON whatsapp_phone_numbers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TABLE whatsapp_calls (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    call_id             VARCHAR(128) NOT NULL,
    phone_number_id     UUID NOT NULL REFERENCES whatsapp_phone_numbers(id) ON DELETE CASCADE,
    direction           whatsapp_call_direction NOT NULL,
    state               whatsapp_call_state NOT NULL DEFAULT 'ringing',
    caller              VARCHAR(20) NOT NULL,
    callee              VARCHAR(20) NOT NULL,
    started_at          TIMESTAMPTZ,
    ended_at            TIMESTAMPTZ,
    duration_seconds    INTEGER GENERATED ALWAYS AS (
        CASE
            WHEN started_at IS NOT NULL AND ended_at IS NOT NULL
            THEN EXTRACT(EPOCH FROM (ended_at - started_at))::INTEGER
            ELSE NULL
        END
    ) STORED,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_whatsapp_call_id UNIQUE (call_id),
    CONSTRAINT chk_whatsapp_call_times CHECK (
        started_at IS NULL OR ended_at IS NULL OR ended_at >= started_at
    )
);

CREATE INDEX idx_whatsapp_calls_org ON whatsapp_calls(org_id);
CREATE INDEX idx_whatsapp_calls_phone ON whatsapp_calls(phone_number_id);
CREATE INDEX idx_whatsapp_calls_org_state ON whatsapp_calls(org_id, state);
CREATE INDEX idx_whatsapp_calls_org_created ON whatsapp_calls(org_id, created_at DESC);
CREATE INDEX idx_whatsapp_calls_call_id ON whatsapp_calls(call_id);

ALTER TABLE whatsapp_calls ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON whatsapp_calls FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON whatsapp_calls FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_whatsapp_calls_updated_at
    BEFORE UPDATE ON whatsapp_calls
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down

DROP TRIGGER IF EXISTS trg_whatsapp_calls_updated_at ON whatsapp_calls;
DROP POLICY IF EXISTS admin_bypass ON whatsapp_calls;
DROP POLICY IF EXISTS tenant_isolation ON whatsapp_calls;
ALTER TABLE whatsapp_calls DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS whatsapp_calls;

DROP TRIGGER IF EXISTS trg_whatsapp_phone_numbers_updated_at ON whatsapp_phone_numbers;
DROP POLICY IF EXISTS admin_bypass ON whatsapp_phone_numbers;
DROP POLICY IF EXISTS tenant_isolation ON whatsapp_phone_numbers;
ALTER TABLE whatsapp_phone_numbers DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS whatsapp_phone_numbers;

DROP TYPE IF EXISTS whatsapp_call_state;
DROP TYPE IF EXISTS whatsapp_call_direction;
