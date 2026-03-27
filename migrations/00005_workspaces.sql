-- +goose Up
CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

CREATE INDEX idx_workspaces_org_id ON workspaces(org_id);

-- Enable RLS
ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON workspaces
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON workspaces
    FOR ALL TO raven_admin
    USING (true);

-- Updated_at trigger
CREATE TRIGGER trg_workspaces_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_workspaces_updated_at ON workspaces;
DROP POLICY IF EXISTS admin_bypass ON workspaces;
DROP POLICY IF EXISTS tenant_isolation ON workspaces;
ALTER TABLE workspaces DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS workspaces;
