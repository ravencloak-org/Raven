-- +goose Up
-- Issue #61 — Voice Session Management
--
-- voice_sessions tracks a LiveKit-backed voice call scoped to an org and a user
-- (either a registered user or an anonymous stranger).
-- voice_turns stores individual transcribed turns within a session.

CREATE TYPE voice_session_state AS ENUM ('created', 'active', 'ended');
CREATE TYPE voice_speaker AS ENUM ('agent', 'user');

CREATE TABLE voice_sessions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    stranger_id     UUID REFERENCES stranger_users(id) ON DELETE SET NULL,
    livekit_room    VARCHAR(255) NOT NULL,
    state           voice_session_state NOT NULL DEFAULT 'created',
    started_at      TIMESTAMPTZ,
    ended_at        TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- call_duration_seconds is derived (ended_at - started_at) but stored for fast reads
    call_duration_seconds INTEGER GENERATED ALWAYS AS (
        CASE
            WHEN started_at IS NOT NULL AND ended_at IS NOT NULL
            THEN EXTRACT(EPOCH FROM (ended_at - started_at))::INTEGER
            ELSE NULL
        END
    ) STORED,
    CONSTRAINT chk_voice_session_actor CHECK (
        (user_id IS NOT NULL AND stranger_id IS NULL) OR
        (user_id IS NULL AND stranger_id IS NOT NULL) OR
        (user_id IS NULL AND stranger_id IS NULL)
    ),
    CONSTRAINT chk_voice_session_times CHECK (
        started_at IS NULL OR ended_at IS NULL OR ended_at >= started_at
    )
);

CREATE INDEX idx_voice_sessions_org ON voice_sessions(org_id);
CREATE INDEX idx_voice_sessions_org_state ON voice_sessions(org_id, state);
CREATE INDEX idx_voice_sessions_org_created ON voice_sessions(org_id, created_at DESC);
CREATE INDEX idx_voice_sessions_user ON voice_sessions(org_id, user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_voice_sessions_stranger ON voice_sessions(org_id, stranger_id) WHERE stranger_id IS NOT NULL;

ALTER TABLE voice_sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON voice_sessions FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON voice_sessions FOR ALL TO raven_admin USING (true);

CREATE TRIGGER trg_voice_sessions_updated_at
    BEFORE UPDATE ON voice_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- voice_turns stores individual transcribed turns within a voice session.
CREATE TABLE voice_turns (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id  UUID NOT NULL REFERENCES voice_sessions(id) ON DELETE CASCADE,
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    speaker     voice_speaker NOT NULL,
    transcript  TEXT NOT NULL,
    started_at  TIMESTAMPTZ NOT NULL,
    ended_at    TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_voice_turn_times CHECK (
        ended_at IS NULL OR ended_at >= started_at
    )
);

CREATE INDEX idx_voice_turns_session ON voice_turns(session_id);
CREATE INDEX idx_voice_turns_org ON voice_turns(org_id);
CREATE INDEX idx_voice_turns_session_started ON voice_turns(session_id, started_at ASC);

ALTER TABLE voice_turns ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON voice_turns FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON voice_turns FOR ALL TO raven_admin USING (true);

-- +goose Down

DROP POLICY IF EXISTS admin_bypass ON voice_turns;
DROP POLICY IF EXISTS tenant_isolation ON voice_turns;
ALTER TABLE voice_turns DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS voice_turns;

DROP TRIGGER IF EXISTS trg_voice_sessions_updated_at ON voice_sessions;
DROP POLICY IF EXISTS admin_bypass ON voice_sessions;
DROP POLICY IF EXISTS tenant_isolation ON voice_sessions;
ALTER TABLE voice_sessions DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS voice_sessions;

DROP TYPE IF EXISTS voice_speaker;
DROP TYPE IF EXISTS voice_session_state;
