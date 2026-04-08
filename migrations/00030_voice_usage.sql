-- +goose Up
-- Issue #69 — Voice Usage Summaries
--
-- Aggregates ended voice sessions per org into hourly summary rows for
-- billing and analytics dashboards.

CREATE TABLE voice_usage_summaries (
    id                     UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id                 UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    period_start           TIMESTAMPTZ NOT NULL,
    total_sessions         INTEGER NOT NULL DEFAULT 0,
    total_duration_seconds INTEGER NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_voice_usage_org_period UNIQUE (org_id, period_start)
);

CREATE INDEX idx_voice_usage_org ON voice_usage_summaries(org_id);
CREATE INDEX idx_voice_usage_period ON voice_usage_summaries(org_id, period_start DESC);

ALTER TABLE voice_usage_summaries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON voice_usage_summaries FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON voice_usage_summaries FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_voice_usage_summaries_updated_at
    BEFORE UPDATE ON voice_usage_summaries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down

DROP TRIGGER IF EXISTS trg_voice_usage_summaries_updated_at ON voice_usage_summaries;
DROP POLICY IF EXISTS admin_bypass ON voice_usage_summaries;
DROP POLICY IF EXISTS tenant_isolation ON voice_usage_summaries;
ALTER TABLE voice_usage_summaries DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS voice_usage_summaries;
