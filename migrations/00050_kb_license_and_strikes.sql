-- +goose Up
--
-- Marketplace MVP (issue #726, M1).
--
-- License + moderation schema groundwork (ADR-0006).
--
-- 1. Partial CHECK on knowledge_bases enforcing the publish-time invariant
--    "every Public KB carries an SPDX license". The license_spdx_id column
--    itself was added in migration 00047 (issue #723); this migration only
--    constrains its nullability for visibility='public' rows so existing
--    private rows stay valid and an attempt to publish without selecting a
--    license is rejected at the database layer as the last line of defence
--    behind the application-level allow-list check in
--    internal/marketplace/license.go.
--
-- 2. organizations.takedown_strikes — running tally of confirmed copyright /
--    DMCA strikes against the publishing Org per ADR-0006. The MVP increment
--    path is the admin "approve report" handler (issue #730); the column
--    lands here so the publish lifecycle work and the moderation work can
--    proceed independently against the same schema.
--
-- References:
--   - docs/plans/marketplace-mvp.md §2 (00050_kb_license_and_strikes.sql)
--   - docs/adr/0002-content-grade-publish-boundary.md (publish boundary)
--   - docs/adr/0006-licence-and-moderation.md (SPDX allow-list + strikes)

-- Bound the locks taken by the DDL below so a long-running query cannot
-- block production indefinitely. 5s is enough for the operations here
-- (NOT VALID add-constraint + small column add with constant default).
SET lock_timeout = '5s';
SET statement_timeout = '30s';

-- ---------------------------------------------------------------------------
-- 1. Partial CHECK: public KBs must carry a license.
-- ---------------------------------------------------------------------------
-- Phrased as "private OR licensed" rather than "public IMPLIES licensed"
-- so every pre-existing private row (license_spdx_id IS NULL) passes
-- without backfill. The expression is partial in spirit but unconditional
-- in SQL — the predicate evaluates true for any private row regardless of
-- license_spdx_id.
--
-- NOT VALID skips the full-table scan at add time; the immediately-following
-- VALIDATE CONSTRAINT runs the check without blocking concurrent writes
-- (it only takes SHARE UPDATE EXCLUSIVE, not ACCESS EXCLUSIVE).
ALTER TABLE knowledge_bases
    ADD CONSTRAINT knowledge_bases_public_requires_license
    CHECK (visibility = 'private' OR license_spdx_id IS NOT NULL)
    NOT VALID;

ALTER TABLE knowledge_bases
    VALIDATE CONSTRAINT knowledge_bases_public_requires_license;

-- ---------------------------------------------------------------------------
-- 2. organizations.takedown_strikes.
-- ---------------------------------------------------------------------------
-- BIGINT (not INTEGER) so we never have to widen later if abuse-volume
-- analytics start incrementing a per-Org counter at unexpected rates.
-- Default 0 so the moderation handler can assume the column is non-null on
-- every row.
ALTER TABLE organizations
    ADD COLUMN takedown_strikes BIGINT NOT NULL DEFAULT 0;

-- +goose Down
--
-- Reverse in dependency order: organizations column first, then the
-- knowledge_bases CHECK. Neither has dependent objects (no triggers,
-- views, or RLS policies reference them yet) so straight DROP works.
-- The DROP COLUMN below is destructive in production but the Down half
-- only ever runs in development / test rollback flows; the trade-off is
-- accepted in exchange for a clean reversible migration.
SET lock_timeout = '5s';
SET statement_timeout = '30s';

ALTER TABLE organizations
    DROP COLUMN IF EXISTS takedown_strikes;

ALTER TABLE knowledge_bases
    DROP CONSTRAINT IF EXISTS knowledge_bases_public_requires_license;
