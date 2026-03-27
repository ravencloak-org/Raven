-- +goose Up

-- Chat sessions
CREATE TABLE chat_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    session_token VARCHAR(255) NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

CREATE INDEX idx_chat_sessions_org_id ON chat_sessions(org_id);
CREATE INDEX idx_chat_sessions_knowledge_base_id ON chat_sessions(knowledge_base_id);
CREATE INDEX idx_chat_sessions_session_token ON chat_sessions(session_token);

-- Enable RLS on chat_sessions
ALTER TABLE chat_sessions ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON chat_sessions
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON chat_sessions
    FOR ALL TO raven_admin
    USING (true);

-- Chat messages
CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role message_role NOT NULL,
    content TEXT NOT NULL,
    token_count INTEGER,
    chunk_ids UUID[],
    model_name VARCHAR(100),
    latency_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_messages_session_id ON chat_messages(session_id);
CREATE INDEX idx_chat_messages_org_id ON chat_messages(org_id);

-- Enable RLS on chat_messages
ALTER TABLE chat_messages ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON chat_messages
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON chat_messages
    FOR ALL TO raven_admin
    USING (true);

-- +goose Down
DROP POLICY IF EXISTS admin_bypass ON chat_messages;
DROP POLICY IF EXISTS tenant_isolation ON chat_messages;
ALTER TABLE chat_messages DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS chat_messages;

DROP POLICY IF EXISTS admin_bypass ON chat_sessions;
DROP POLICY IF EXISTS tenant_isolation ON chat_sessions;
ALTER TABLE chat_sessions DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS chat_sessions;
