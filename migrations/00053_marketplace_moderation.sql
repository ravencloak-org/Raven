-- +goose Up
--
-- Marketplace MVP (issue #733, M3).
--
-- Stand up the moderation backbone for Public KBs:
--
--   - marketplace_reports — user-submitted abuse reports against a Public KB.
--     A reporter sees only their own rows; raven_admin bypasses RLS for the
--     review queue. Status is a finite state machine:
--         open -> reviewing -> { resolved | dismissed }
--     enforced both by a CHECK constraint (data integrity backstop) and by
--     the Go repository's Transition helper (structured diagnostics — see
--     internal/marketplace/moderation.go).
--
--   - marketplace_takedowns — append-only audit log for KB removals. The
--     "source" column attributes the removal to a publisher (self-takedown),
--     an admin action (report-driven), or a DMCA notice. Admin-only read and
--     write — non-admin sessions cannot even see this table.
--
-- Out of scope for this migration (deliberately):
--   - organizations.takedown_strikes — landed by issue #726 / migration 00050.
--   - knowledge_bases.license_spdx_id CHECK constraint — landed by #726 / 00050.
--   - kb_status enum values 'read_only_private' / 'dmca_pending' — already
--     in place from migration 00049.
--
-- See:
--   docs/plans/marketplace-mvp.md §2 (00053_marketplace_moderation.sql)
--   docs/adr/0006-licence-and-moderation.md (report / takedown semantics)
--   docs/adr/0008-marketplace-discovery-and-operations.md (admin queue)
--   migrations/00015_rls_policies.sql (tenant_isolation + admin_bypass pattern)

-- ─── marketplace_reports ─────────────────────────────────────────────────────
--
-- One row per report submission. The reporter is referenced as a nullable
-- FK with ON DELETE SET NULL so a user-deletion cascade does not erase
-- the audit trail — the report row survives, its reporter is anonymised.
-- The reported KB cascades on delete: if the underlying KB is dropped,
-- the report has no target and is moot.
--
-- reason length is capped to keep abusive payloads from blowing up rows;
-- the lower bound of 1 forces callers to send a non-empty reason rather
-- than silently bypassing the CHECK with an empty string.

CREATE TABLE marketplace_reports (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reported_kb_id   UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    reporter_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    reason           TEXT NOT NULL
                     CHECK (length(reason) BETWEEN 1 AND 4000),
    status           TEXT NOT NULL DEFAULT 'open'
                     CHECK (status IN ('open', 'reviewing', 'resolved', 'dismissed')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Status filter index: the admin review queue's primary query is
-- `WHERE status = 'open' ORDER BY created_at`, with occasional sweeps for
-- `reviewing`. A full b-tree on status is cheap on a low-cardinality
-- column and lets the planner pick it for both queries.
CREATE INDEX idx_marketplace_reports_status
    ON marketplace_reports (status);

-- KB-target lookup: when admins approve a report, they look up "are there
-- other open reports against this same KB?" (de-duplicate strike events)
-- via reported_kb_id; the rate-limit helper in moderation.go uses
-- reporter_user_id but a single-column status-filter index covers that.
CREATE INDEX idx_marketplace_reports_kb
    ON marketplace_reports (reported_kb_id);

ALTER TABLE marketplace_reports ENABLE ROW LEVEL SECURITY;

-- Reporter sees own rows. We use a NULL-tolerant cast — if the session
-- variable is unset, current_setting returns the empty string (with the
-- `true` missing_ok flag), nullif maps that to NULL, and the cast yields
-- NULL — which never equals reporter_user_id, so the policy denies.
CREATE POLICY reporter_self_read ON marketplace_reports
    FOR ALL
    USING (
        reporter_user_id = nullif(current_setting('app.current_user_id', true), '')::uuid
    )
    WITH CHECK (
        reporter_user_id = nullif(current_setting('app.current_user_id', true), '')::uuid
    );

-- raven_admin bypass — the review queue handler runs as raven_admin and
-- needs to see every report regardless of reporter.
CREATE POLICY admin_bypass ON marketplace_reports
    FOR ALL TO raven_admin
    USING (true);

-- ─── marketplace_takedowns ───────────────────────────────────────────────────
--
-- Append-only audit log. There is no "update a takedown" path — corrections
-- are made by inserting a new row referencing the same target KB with a
-- note explaining the correction. This keeps the log forensically clean.
--
-- target_kb_id cascades on delete: when a KB row is hard-deleted, its
-- takedown log entries go with it (the publisher Org's strike counter on
-- the organizations row survives — that is the durable record).

CREATE TABLE marketplace_takedowns (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    target_kb_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    source       TEXT NOT NULL
                 CHECK (source IN ('publisher', 'admin', 'dmca')),
    notes        TEXT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-KB lookup: "show me the takedown history for this KB". The
-- admin/takedowns view filters by target_kb_id; no other access pattern
-- is in-scope.
CREATE INDEX idx_marketplace_takedowns_target_kb
    ON marketplace_takedowns (target_kb_id);

ALTER TABLE marketplace_takedowns ENABLE ROW LEVEL SECURITY;

-- Admin-only read+write. No tenant policy is defined — non-admin sessions
-- see zero rows because no USING clause matches. This is the correct
-- shape for an internal-only audit log per ADR-0008 §Consequences.
CREATE POLICY admin_bypass ON marketplace_takedowns
    FOR ALL TO raven_admin
    USING (true);

-- +goose Down
--
-- Reverse in dependency order: policies → indexes → tables.
DROP POLICY IF EXISTS admin_bypass ON marketplace_takedowns;
DROP INDEX IF EXISTS idx_marketplace_takedowns_target_kb;
DROP TABLE IF EXISTS marketplace_takedowns;

DROP POLICY IF EXISTS admin_bypass ON marketplace_reports;
DROP POLICY IF EXISTS reporter_self_read ON marketplace_reports;
DROP INDEX IF EXISTS idx_marketplace_reports_kb;
DROP INDEX IF EXISTS idx_marketplace_reports_status;
DROP TABLE IF EXISTS marketplace_reports;
