-- +goose Up

-- Chunks table
CREATE TABLE chunks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    source_id UUID REFERENCES sources(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    token_count INTEGER,
    page_number INTEGER,
    heading VARCHAR(500),
    chunk_type chunk_type NOT NULL DEFAULT 'text',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (document_id IS NOT NULL OR source_id IS NOT NULL)
);

CREATE INDEX idx_chunks_org_id ON chunks(org_id);
CREATE INDEX idx_chunks_knowledge_base_id ON chunks(knowledge_base_id);
CREATE INDEX idx_chunks_document_id ON chunks(document_id);
CREATE INDEX idx_chunks_source_id ON chunks(source_id);
CREATE INDEX idx_chunks_content_fts ON chunks USING gin(to_tsvector('english', content));

-- Enable RLS on chunks
ALTER TABLE chunks ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON chunks
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON chunks
    FOR ALL TO raven_admin
    USING (true);

-- Embeddings table
CREATE TABLE embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    embedding vector(1536) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    model_version VARCHAR(50),
    dimensions INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chunk_id, model_name)
);

CREATE INDEX idx_embeddings_org_id ON embeddings(org_id);
CREATE INDEX idx_embeddings_chunk_id ON embeddings(chunk_id);
CREATE INDEX idx_embeddings_hnsw ON embeddings USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- Enable RLS on embeddings
ALTER TABLE embeddings ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON embeddings
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON embeddings
    FOR ALL TO raven_admin
    USING (true);

-- +goose Down
DROP POLICY IF EXISTS admin_bypass ON embeddings;
DROP POLICY IF EXISTS tenant_isolation ON embeddings;
ALTER TABLE embeddings DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS embeddings;

DROP POLICY IF EXISTS admin_bypass ON chunks;
DROP POLICY IF EXISTS tenant_isolation ON chunks;
ALTER TABLE chunks DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS chunks;
