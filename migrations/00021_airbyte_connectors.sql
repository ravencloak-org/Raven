-- +goose Up
-- Issue #111 — Airbyte connector integration: enterprise data movement.
--
-- Tracks Airbyte connector configurations per org/knowledge-base and sync
-- history for individual sync runs.

CREATE TYPE connector_status AS ENUM ('active', 'paused', 'error', 'deleted');
CREATE TYPE sync_mode AS ENUM ('full_refresh', 'incremental', 'cdc');

CREATE TABLE airbyte_connectors (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    connector_type VARCHAR(100) NOT NULL,  -- e.g., 'source-postgres', 'source-salesforce'
    config JSONB NOT NULL DEFAULT '{}',     -- TODO(#111): encrypt like llm_provider_configs in a follow-up
    sync_mode sync_mode NOT NULL DEFAULT 'full_refresh',
    schedule_cron VARCHAR(100),             -- cron expression for scheduled syncs
    status connector_status NOT NULL DEFAULT 'active',
    last_sync_at TIMESTAMPTZ,
    last_sync_status VARCHAR(50),
    last_sync_records INTEGER DEFAULT 0,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_airbyte_connectors_org ON airbyte_connectors(org_id);
CREATE INDEX idx_airbyte_connectors_kb ON airbyte_connectors(knowledge_base_id);

ALTER TABLE airbyte_connectors ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON airbyte_connectors FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON airbyte_connectors FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_airbyte_connectors_updated_at
    BEFORE UPDATE ON airbyte_connectors
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Sync history for tracking individual sync runs
CREATE TABLE airbyte_sync_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    connector_id UUID NOT NULL REFERENCES airbyte_connectors(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'running',
    records_synced INTEGER DEFAULT 0,
    records_failed INTEGER DEFAULT 0,
    bytes_synced BIGINT DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT
);

CREATE INDEX idx_sync_history_connector ON airbyte_sync_history(connector_id);
ALTER TABLE airbyte_sync_history ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON airbyte_sync_history FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON airbyte_sync_history FOR ALL TO raven_admin USING (true);

-- Dedup tracking: prevent re-processing same content
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS chunk_hash VARCHAR(64);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chunks_dedup ON chunks(org_id, knowledge_base_id, source_id, chunk_hash)
    WHERE chunk_hash IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_chunks_dedup;
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_hash;

DROP POLICY IF EXISTS admin_bypass ON airbyte_sync_history;
DROP POLICY IF EXISTS tenant_isolation ON airbyte_sync_history;
ALTER TABLE airbyte_sync_history DISABLE ROW LEVEL SECURITY;
DROP INDEX IF EXISTS idx_sync_history_connector;
DROP TABLE IF EXISTS airbyte_sync_history;

DROP TRIGGER IF EXISTS trg_airbyte_connectors_updated_at ON airbyte_connectors;
DROP POLICY IF EXISTS admin_bypass ON airbyte_connectors;
DROP POLICY IF EXISTS tenant_isolation ON airbyte_connectors;
ALTER TABLE airbyte_connectors DISABLE ROW LEVEL SECURITY;
DROP INDEX IF EXISTS idx_airbyte_connectors_kb;
DROP INDEX IF EXISTS idx_airbyte_connectors_org;
DROP TABLE IF EXISTS airbyte_connectors;

DROP TYPE IF EXISTS sync_mode;
DROP TYPE IF EXISTS connector_status;
