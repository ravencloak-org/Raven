-- +goose Up
CREATE TABLE knowledge_bases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    settings JSONB DEFAULT '{}',
    status kb_status NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, slug)
);

CREATE INDEX idx_knowledge_bases_org_id ON knowledge_bases(org_id);
CREATE INDEX idx_knowledge_bases_workspace_id ON knowledge_bases(workspace_id);

-- Enable RLS
ALTER TABLE knowledge_bases ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON knowledge_bases
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON knowledge_bases
    FOR ALL TO raven_admin
    USING (true);

-- Updated_at trigger
CREATE TRIGGER trg_knowledge_bases_updated_at
    BEFORE UPDATE ON knowledge_bases
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_knowledge_bases_updated_at ON knowledge_bases;
DROP POLICY IF EXISTS admin_bypass ON knowledge_bases;
DROP POLICY IF EXISTS tenant_isolation ON knowledge_bases;
ALTER TABLE knowledge_bases DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS knowledge_bases;
