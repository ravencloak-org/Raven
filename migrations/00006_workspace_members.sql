-- +goose Up
CREATE TABLE workspace_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role workspace_role NOT NULL DEFAULT 'member',
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, user_id)
);

CREATE INDEX idx_workspace_members_workspace_id ON workspace_members(workspace_id);
CREATE INDEX idx_workspace_members_user_id ON workspace_members(user_id);
CREATE INDEX idx_workspace_members_org_id ON workspace_members(org_id);

-- Enable RLS
ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON workspace_members
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON workspace_members
    FOR ALL TO raven_admin
    USING (true);

-- +goose Down
DROP POLICY IF EXISTS admin_bypass ON workspace_members;
DROP POLICY IF EXISTS tenant_isolation ON workspace_members;
ALTER TABLE workspace_members DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS workspace_members;
