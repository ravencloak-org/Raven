-- +goose Up
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    file_name VARCHAR(500) NOT NULL,
    file_type VARCHAR(50),
    file_size_bytes BIGINT,
    file_hash VARCHAR(128),
    storage_path TEXT,
    processing_status processing_status NOT NULL DEFAULT 'queued',
    processing_error TEXT,
    title VARCHAR(500),
    page_count INTEGER,
    metadata JSONB DEFAULT '{}',
    uploaded_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documents_org_id ON documents(org_id);
CREATE INDEX idx_documents_knowledge_base_id ON documents(knowledge_base_id);
CREATE INDEX idx_documents_processing_status ON documents(processing_status);
CREATE INDEX idx_documents_file_hash ON documents(file_hash);

-- Enable RLS
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON documents
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON documents
    FOR ALL TO raven_admin
    USING (true);

-- Updated_at trigger
CREATE TRIGGER trg_documents_updated_at
    BEFORE UPDATE ON documents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_documents_updated_at ON documents;
DROP POLICY IF EXISTS admin_bypass ON documents;
DROP POLICY IF EXISTS tenant_isolation ON documents;
ALTER TABLE documents DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS documents;
