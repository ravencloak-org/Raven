-- +goose Up
CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    source_type source_type NOT NULL,
    url TEXT NOT NULL,
    crawl_depth INTEGER DEFAULT 1,
    crawl_frequency crawl_frequency NOT NULL DEFAULT 'manual',
    processing_status processing_status NOT NULL DEFAULT 'queued',
    processing_error TEXT,
    title VARCHAR(500),
    pages_crawled INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sources_org_id ON sources(org_id);
CREATE INDEX idx_sources_knowledge_base_id ON sources(knowledge_base_id);
CREATE INDEX idx_sources_processing_status ON sources(processing_status);

-- Enable RLS
ALTER TABLE sources ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON sources
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON sources
    FOR ALL TO raven_admin
    USING (true);

-- Updated_at trigger
CREATE TRIGGER trg_sources_updated_at
    BEFORE UPDATE ON sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_sources_updated_at ON sources;
DROP POLICY IF EXISTS admin_bypass ON sources;
DROP POLICY IF EXISTS tenant_isolation ON sources;
ALTER TABLE sources DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS sources;
