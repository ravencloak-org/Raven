-- +goose Up
CREATE TYPE routing_mode AS ENUM ('static', 'column_based', 'auto');

CREATE TABLE routing_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    source_type VARCHAR(100) NOT NULL,          -- 'airbyte', 'upload', 'url', 'api'
    source_identifier VARCHAR(500),              -- connector ID, table name, etc.
    routing_mode routing_mode NOT NULL DEFAULT 'static',
    -- Static mode: all data → target_kb_id
    target_kb_id UUID REFERENCES knowledge_bases(id) ON DELETE SET NULL,
    -- Column-based mode: route by discriminator column
    discriminator_column VARCHAR(255),
    column_mappings JSONB DEFAULT '{}',          -- {"value1": "kb-uuid-1", "value2": "kb-uuid-2"}
    -- Auto mode: LLM classification config
    classification_prompt TEXT,
    classification_model VARCHAR(100),
    classification_provider VARCHAR(50),
    -- Metadata
    priority INTEGER NOT NULL DEFAULT 0,         -- higher = evaluated first
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routing_rules_org ON routing_rules(org_id);
CREATE INDEX idx_routing_rules_source ON routing_rules(org_id, source_type, source_identifier);

ALTER TABLE routing_rules ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON routing_rules FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON routing_rules FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_routing_rules_updated_at
    BEFORE UPDATE ON routing_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Catalog metadata cache for external data catalogs
CREATE TABLE catalog_metadata (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    catalog_type VARCHAR(100) NOT NULL,          -- 'snowflake_tags', 'dbt', 'datahub', 'glue'
    resource_path VARCHAR(1000) NOT NULL,        -- schema.table or catalog path
    labels JSONB DEFAULT '{}',
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, catalog_type, resource_path)
);

ALTER TABLE catalog_metadata ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON catalog_metadata FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON catalog_metadata FOR ALL TO raven_admin USING (true);

-- +goose Down
DROP TABLE IF EXISTS catalog_metadata;
DROP TABLE IF EXISTS routing_rules;
DROP TYPE IF EXISTS routing_mode;
