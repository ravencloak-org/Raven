-- +goose Up
-- +goose NO TRANSACTION
--
-- Passkey labels (issue #771, M14).
--
-- Stores user-supplied labels and timestamps for WebAuthn credentials managed
-- by the SuperTokens core. The core itself owns the credential (public key,
-- attestation, counter, transports); Raven only stores the human-readable
-- label and the last-used timestamp so the Settings → Authentication tab can
-- render meaningful rows like "MacBook Pro Touch ID" instead of opaque IDs.
--
-- credential_id is the SuperTokens credential identifier and is the natural
-- primary key — there is exactly one label row per credential. user_id is
-- duplicated here (rather than joining via the core) so RLS can scope reads
-- by tenant without a cross-service round trip on every request.
--
-- Concurrent index on user_id supports the GET /api/v1/me/passkeys query
-- (which lists every label for the calling user in one shot). Built
-- CONCURRENTLY so the deploy does not hold ACCESS EXCLUSIVE on the table
-- while the index is built — this is why the migration is wrapped in the
-- goose "NO TRANSACTION" annotation above (CREATE INDEX CONCURRENTLY
-- cannot run inside a transaction block).
--
-- References:
--   - docs/superpowers/specs/2026-06-04-passkey-auth-design.md §Architecture
--   - migrations/00015_rls_policies.sql (tenant_isolation + admin_bypass)
--   - migrations/00004_users.sql (users.id FK target, UUID PK)

-- Bound the locks taken by the DDL below so a long-running query cannot
-- block production indefinitely. The CREATE TABLE + ADD CONSTRAINT take
-- ACCESS EXCLUSIVE briefly; CREATE INDEX CONCURRENTLY takes a weaker lock
-- but a generous statement_timeout protects against runaway builds on a
-- large users table.
SET lock_timeout = '5s';
SET statement_timeout = '30s';

CREATE TABLE user_passkey_labels (
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id TEXT        PRIMARY KEY,
    label         TEXT        NOT NULL CHECK (length(label) BETWEEN 1 AND 255),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at  TIMESTAMPTZ NULL
);

-- Enable RLS so a stray query without app.current_user_id set sees zero
-- rows by default. Pattern mirrors migrations/00015 + the per-user variant
-- used by marketplace_reports (00053_marketplace_moderation.sql).
ALTER TABLE user_passkey_labels ENABLE ROW LEVEL SECURITY;

-- Self-read/write: callers see only rows whose user_id matches the session
-- variable. We use a NULL-tolerant cast so an unset variable yields NULL
-- (not a parse error) and the policy denies rather than crashes.
CREATE POLICY user_self_access ON user_passkey_labels
    FOR ALL
    USING (
        user_id = nullif(current_setting('app.current_user_id', true), '')::uuid
    )
    WITH CHECK (
        user_id = nullif(current_setting('app.current_user_id', true), '')::uuid
    );

-- Admin bypass — operational tooling (e.g. DSAR exports, support read-only
-- queries) runs as raven_admin and needs to see every row.
CREATE POLICY admin_bypass ON user_passkey_labels
    FOR ALL TO raven_admin
    USING (true);

-- Concurrent secondary index supporting GET /api/v1/me/passkeys. The PK is
-- credential_id (text), so a per-user list would otherwise sequentially
-- scan; this index keeps the list query O(log n) per user. Squawk warns
-- about CONCURRENTLY inside a transaction; the migration is wrapped in
-- the goose annotation at the top so it runs outside one.
-- squawk-ignore ban-concurrent-index-creation-in-transaction
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_passkey_labels_user_id
    ON user_passkey_labels (user_id);

-- +goose Down
-- +goose NO TRANSACTION
--
-- Reverse order: drop the index first (also CONCURRENTLY to avoid blocking),
-- then policies, then the table itself. IF EXISTS keeps the down idempotent
-- across partial-failure rollbacks.
SET lock_timeout = '5s';
SET statement_timeout = '30s';

DROP INDEX CONCURRENTLY IF EXISTS idx_user_passkey_labels_user_id;
DROP POLICY IF EXISTS admin_bypass ON user_passkey_labels;
DROP POLICY IF EXISTS user_self_access ON user_passkey_labels;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS user_passkey_labels;
