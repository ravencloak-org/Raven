-- +goose Up
CREATE TABLE processing_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    source_id UUID REFERENCES sources(id) ON DELETE CASCADE,
    from_status VARCHAR(20),
    to_status VARCHAR(20) NOT NULL,
    error_message TEXT,
    duration_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_processing_events_org_id ON processing_events(org_id);
CREATE INDEX idx_processing_events_document_id ON processing_events(document_id);
CREATE INDEX idx_processing_events_source_id ON processing_events(source_id);

-- Enable RLS
ALTER TABLE processing_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON processing_events
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON processing_events
    FOR ALL TO raven_admin
    USING (true);

-- +goose Down
DROP POLICY IF EXISTS admin_bypass ON processing_events;
DROP POLICY IF EXISTS tenant_isolation ON processing_events;
ALTER TABLE processing_events DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS processing_events;
