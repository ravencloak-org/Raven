-- +goose Up

CREATE TABLE response_cache (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    kb_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    query_text TEXT NOT NULL,
    query_embedding vector(1536) NOT NULL,
    response_text TEXT NOT NULL,
    sources JSONB DEFAULT '[]',
    model_name VARCHAR(100),
    hit_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '1 hour'
);

CREATE INDEX idx_response_cache_org_kb ON response_cache(org_id, kb_id);
CREATE INDEX CONCURRENTLY idx_response_cache_embedding ON response_cache USING hnsw (query_embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX idx_response_cache_expires ON response_cache(expires_at);

ALTER TABLE response_cache ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON response_cache FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON response_cache FOR ALL TO raven_admin USING (true);

-- +goose Down

DROP POLICY IF EXISTS admin_bypass ON response_cache;
DROP POLICY IF EXISTS tenant_isolation ON response_cache;
DROP INDEX IF EXISTS idx_response_cache_expires;
DROP INDEX IF EXISTS idx_response_cache_embedding;
DROP INDEX IF EXISTS idx_response_cache_org_kb;
DROP TABLE IF EXISTS response_cache;
