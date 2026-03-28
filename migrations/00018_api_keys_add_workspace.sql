-- +goose Up
-- Add workspace_id column to api_keys table (issue #36).
-- Existing rows (if any) will have NULL workspace_id; new rows should always
-- provide a workspace_id through the application layer.
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_api_keys_workspace_id ON api_keys(workspace_id);

-- +goose Down
DROP INDEX IF EXISTS idx_api_keys_workspace_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS workspace_id;
