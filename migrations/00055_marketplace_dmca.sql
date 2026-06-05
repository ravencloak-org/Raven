-- +goose Up
--
-- Marketplace MVP (issue #736, launch blocker per ADR-0006).
--
-- Stand up the DMCA inbox + two-stage counter-notice workflow:
--
--   - dmca_notices — one row per incoming DMCA notice received at
--     dmca@ravencloak.org. Pending notices freeze the target KB into
--     `kb_status='dmca_pending'` for the 14-day statutory counter-notice
--     window (17 U.S.C. § 512(g)(2)(B)). If the publisher files a
--     counter-notice through the admin (MVP simplification — admin
--     acts on behalf of publisher), the row records the counter-notice
--     text and pivots to `counter_filed`; the KB stays in
--     `dmca_pending` until the admin issues a final keep-up / take-down
--     decision. Notices with no counter-notice when the window expires
--     are auto-resolved by the daily sweeper (see
--     internal/jobs/marketplace_dmca_sweeper.go), which flips the KB
--     to `visibility='private'` and writes a `source='dmca'` row to
--     marketplace_takedowns.
--
-- RLS: admin-only read+write (same shape as marketplace_takedowns from
-- 00053 — non-admin sessions cannot even see the table). The sweeper
-- runs under `SET LOCAL ROLE raven_admin` inside its sweep transaction.
--
-- Out of scope (deliberately):
--   - kb_status 'dmca_pending' enum value — already in place from
--     migration 00049.
--   - marketplace_takedowns table — already in place from migration
--     00053. The sweep auto-resolve writes into it via Takedowns.Create.
--
-- See:
--   docs/plans/marketplace-mvp.md §4 (admin DMCA endpoints), §6 (DMCA inbox).
--   docs/adr/0006-licence-and-moderation.md (14-day counter-notice window).
--   docs/adr/0008-marketplace-discovery-and-operations.md (admin queue).
--   migrations/00053_marketplace_moderation.sql (admin-bypass policy pattern).

-- Bound long-running locks: the table is brand new, so the only risk
-- is the CREATE TABLE itself blocking behind an unrelated long
-- transaction. 5s is the project convention (see e.g. 00050).
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- ─── dmca_notices ────────────────────────────────────────────────────────────
--
-- The `notice_text` body is bounded between 1 and 8192 chars — empty
-- notices are rejected, and 8 KiB comfortably accommodates a 17 U.S.C.
-- § 512(c)(3) compliant notice including a full URL list and
-- good-faith / accuracy declarations without becoming an unbounded
-- write target.
--
-- claimant_email / claimant_name are NOT NULL so every notice has a
-- responsible party on file (statutory requirement). They are stored
-- verbatim — no normalisation, no FK to users (claimants are external
-- third parties, not Raven accounts).
--
-- counter_notice_text / counter_notice_submitted_at are nullable: they
-- stay NULL until / unless the publisher files a counter-notice via
-- the admin endpoint. We do not duplicate the publisher's email here —
-- the publisher Org is reachable via target_kb_id → knowledge_bases →
-- organizations and lookups happen at admin-action time.
--
-- counter_notice_window_ends materialises the 14-day clock at notice
-- submission time so the sweep query can run a simple `< now()` filter
-- without recomputing `created_at + interval '14 days'` on every row.
-- This also lets the admin extend the window with a single UPDATE if
-- the statute or our policy ever calls for it (out of scope for MVP).

CREATE TABLE dmca_notices (
    id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    target_kb_id                UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    notice_text                 TEXT NOT NULL
                                CHECK (length(notice_text) BETWEEN 1 AND 8192),
    claimant_email              TEXT NOT NULL,
    claimant_name               TEXT NOT NULL,
    counter_notice_text         TEXT NULL,
    counter_notice_submitted_at TIMESTAMPTZ NULL,
    counter_notice_window_ends  TIMESTAMPTZ NOT NULL,
    status                      TEXT NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending', 'counter_filed', 'resolved_take_down', 'resolved_keep_up', 'withdrawn')),
    resolved_at                 TIMESTAMPTZ NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sweep-query covering index: the daily sweeper's primary query is
--   WHERE status = 'pending' AND counter_notice_window_ends < now()
-- A composite b-tree on (status, counter_notice_window_ends) lets the
-- planner pick it directly without consulting the heap until the
-- window-end filter has eliminated most rows.
CREATE INDEX idx_dmca_notices_sweep
    ON dmca_notices (status, counter_notice_window_ends);

-- Per-KB lookup: admin UI lists "is there an active DMCA hold on this
-- KB?" via target_kb_id. The CASCADE on target_kb_id means hard-
-- deleting a KB cleans up its notices; the takedown audit log (from
-- 00053) preserves the long-term record.
CREATE INDEX idx_dmca_notices_target_kb
    ON dmca_notices (target_kb_id);

ALTER TABLE dmca_notices ENABLE ROW LEVEL SECURITY;

-- Admin-only read+write. No tenant policy is defined — non-admin
-- sessions see zero rows because no USING clause matches. Mirrors the
-- marketplace_takedowns shape from 00053 (internal-only legal record).
CREATE POLICY admin_bypass ON dmca_notices
    FOR ALL TO raven_admin
    USING (true);

-- +goose Down
--
-- Reverse in dependency order: policy → indexes → table. No reversal
-- needed for the lock_timeout / statement_timeout SETs — they are
-- scoped to the migration transaction.
DROP POLICY IF EXISTS admin_bypass ON dmca_notices;
DROP INDEX IF EXISTS idx_dmca_notices_target_kb;
DROP INDEX IF EXISTS idx_dmca_notices_sweep;
DROP TABLE IF EXISTS dmca_notices;
