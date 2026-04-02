-- +goose Up

CREATE TABLE lead_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id UUID REFERENCES knowledge_bases(id) ON DELETE SET NULL,
    session_ids UUID[] DEFAULT '{}',
    email VARCHAR(255),
    name VARCHAR(255),
    phone VARCHAR(50),
    company VARCHAR(255),
    metadata JSONB DEFAULT '{}',
    engagement_score REAL NOT NULL DEFAULT 0.0,
    total_messages INTEGER NOT NULL DEFAULT 0,
    total_sessions INTEGER NOT NULL DEFAULT 1,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partial unique index acts as the ON CONFLICT arbiter for (org_id, email) upserts.
-- PostgreSQL resolves ON CONFLICT (org_id, email) WHERE email IS NOT NULL against this index.
CREATE UNIQUE INDEX uq_lead_profiles_org_email ON lead_profiles(org_id, email) WHERE email IS NOT NULL;
CREATE INDEX idx_lead_profiles_org_score ON lead_profiles(org_id, engagement_score DESC);

ALTER TABLE lead_profiles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON lead_profiles FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON lead_profiles FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_lead_profiles_updated_at
    BEFORE UPDATE ON lead_profiles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_lead_profiles_updated_at ON lead_profiles;
DROP POLICY IF EXISTS admin_bypass ON lead_profiles;
DROP POLICY IF EXISTS tenant_isolation ON lead_profiles;
DROP INDEX IF EXISTS uq_lead_profiles_org_email;
DROP TABLE IF EXISTS lead_profiles;
