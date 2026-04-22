-- +goose Up
-- Issue #257 — Post-session conversation email summaries (M9)
--
-- conversation_sessions is a unified, channel-agnostic record of a user's
-- conversation with a knowledge base. It captures the last N messages in
-- JSONB so that the email summary worker (#257) and the conversation history
-- read API (#258) can operate off a single source of truth.
--
-- Although voice_sessions (00029) and chat_sessions (00013) already exist for
-- channel-specific operational data (LiveKit room, transcripts, per-turn
-- metadata), conversation_sessions stores the *conversation* in a shape that
-- is easy to summarise and replay to the end user.
--
-- NOTE: This migration is owned by #257 but is also consumed by #258
-- (conversation history read API). #258 intentionally does NOT re-create this
-- table; it only adds its own service/handler code.

CREATE TABLE conversation_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    kb_id       UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL, -- external identity (SuperTokens sub or stranger token)
    channel     TEXT NOT NULL CHECK (channel IN ('chat', 'voice', 'webrtc')),
    messages    JSONB NOT NULL DEFAULT '[]'::jsonb,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at    TIMESTAMPTZ,
    summary     TEXT,
    CONSTRAINT chk_conversation_sessions_times CHECK (
        ended_at IS NULL OR ended_at >= started_at
    ),
    -- Enforce that messages is always a JSON array. Without this, a bug in
    -- the Go side could persist an object/scalar and silently break the
    -- email-summary and conversation-history readers.
    CONSTRAINT chk_conversation_sessions_messages_is_array CHECK (
        jsonb_typeof(messages) = 'array'
    )
);

-- Lookup index required by both #257 (find sessions to summarise for a user)
-- and #258 (list a user's recent conversations within a KB).
CREATE INDEX idx_conversation_sessions_kb_user_started
    ON conversation_sessions (kb_id, user_id, started_at DESC);

-- Tenant scoping index for RLS-assisted planner decisions.
CREATE INDEX idx_conversation_sessions_org
    ON conversation_sessions (org_id);

ALTER TABLE conversation_sessions ENABLE ROW LEVEL SECURITY;

-- Enforces hard tenant isolation: every row read/written must match the
-- current org_id set by the request middleware via:
--   SET LOCAL app.current_org_id = '<uuid>'
CREATE POLICY tenant_isolation ON conversation_sessions FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON conversation_sessions FOR ALL TO raven_admin USING (true) WITH CHECK (true);

-- Per-user email-summary preferences.
-- A user opts in to receiving a recap email after each conversation ends.
-- Default is FALSE (explicit opt-in) to stay CAN-SPAM compliant.
CREATE TABLE user_notification_preferences (
    user_id                 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id            UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    org_id                  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email_summaries_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, workspace_id)
);

CREATE INDEX idx_user_notification_preferences_org
    ON user_notification_preferences (org_id);

ALTER TABLE user_notification_preferences ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON user_notification_preferences FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON user_notification_preferences FOR ALL TO raven_admin USING (true) WITH CHECK (true);

-- Workspace-admin master switch. When FALSE the workspace opts out of summary
-- emails for every member regardless of their personal preference.
ALTER TABLE workspaces
    ADD COLUMN email_summaries_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN IF EXISTS email_summaries_enabled;

DROP POLICY IF EXISTS admin_bypass ON user_notification_preferences;
DROP POLICY IF EXISTS tenant_isolation ON user_notification_preferences;
ALTER TABLE user_notification_preferences DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS user_notification_preferences;

DROP POLICY IF EXISTS admin_bypass ON conversation_sessions;
DROP POLICY IF EXISTS tenant_isolation ON conversation_sessions;
ALTER TABLE conversation_sessions DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS conversation_sessions;
