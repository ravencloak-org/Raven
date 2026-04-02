-- +goose Up

CREATE TABLE user_identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    anonymous_id VARCHAR(255) NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    posthog_distinct_id VARCHAR(255),
    channel VARCHAR(50) NOT NULL DEFAULT 'chat',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    session_count INTEGER NOT NULL DEFAULT 1,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, anonymous_id, channel)
);

CREATE INDEX idx_user_identities_org ON user_identities(org_id);
CREATE INDEX idx_user_identities_anonymous ON user_identities(org_id, anonymous_id);
CREATE INDEX idx_user_identities_user ON user_identities(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_user_identities_channel ON user_identities(org_id, channel);
CREATE INDEX idx_user_identities_last_seen ON user_identities(org_id, last_seen_at DESC);

ALTER TABLE user_identities ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_identities FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON user_identities FOR ALL TO raven_admin USING (true);

-- +goose Down
DROP TABLE IF EXISTS user_identities;
