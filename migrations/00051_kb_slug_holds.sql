-- +goose Up
--
-- `kb_slug_holds` — 90-day lifecycle lock on a KB slug after a public KB is
-- unpublished (issue #727, M1). When a Public KB transitions back to
-- `private` the publishing Org's (org_slug, kb_slug) pair must keep
-- routing to a clean `410 Gone` for 90 days (ADR-0007) rather than
-- collapsing into a 404 or being recycled by a freshly-created KB.
--
-- The shape mirrors `org_slug_holds` from migration 00048: a thin
-- (parent_id, slug, held_until) row with the lookup keyed on the
-- "what URL did the visitor type" pair, so the Marketplace URL handler
-- (#731) can resolve a held slug to a typed result in one index probe.
--
-- A row keeps a soft FK to `knowledge_bases(id)` via `ON DELETE SET NULL`
-- so the hold survives a hard-delete of the KB itself (which is rare;
-- archive is the normal path). The (org_id, slug) pair is the primary
-- key — the Marketplace URL space is per-Org, not global, so two
-- different orgs can hold the same slug at the same time.
--
-- RLS: this table is intentionally NOT row-isolated. The 410-Gone
-- handler is a public Marketplace surface that must resolve a held
-- slug without engaging session-bound RLS. Permissions are managed at
-- the role grant level by ops, identical to how `org_slug_holds`
-- (migration 00048) is exposed. Writes happen only through the
-- `UnpublishService` in `internal/marketplace`, which runs inside a
-- single transaction so the visibility flip and the hold insert can't
-- be observed half-applied.
--
-- References:
--   - docs/plans/marketplace-mvp.md §2 (`00051_kb_slug_holds.sql`), §4
--     (unpublish endpoint row)
--   - docs/adr/0001-marketplace-fork-on-import.md
--   - docs/adr/0007-marketplace-lifecycle-behaviours.md (90-day hold,
--     410 Gone, existing imports unaffected)

-- Bound DDL lock acquisition + statement runtime so a long-running
-- transaction on `knowledge_bases` cannot wedge the migration applier.
-- Matches Squawk's safe-migration defaults; the table is fresh so
-- there is no existing-row scan to budget for here.
SET lock_timeout = '5s';
SET statement_timeout = '30s';

-- ---------------------------------------------------------------------------
-- 1. Table.
-- ---------------------------------------------------------------------------
CREATE TABLE kb_slug_holds (
    org_id     UUID         NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    -- TEXT not VARCHAR(N): per Squawk's prefer-text-field rule, VARCHAR with
    -- an explicit length forces an ACCESS EXCLUSIVE lock if the bound ever
    -- needs to change. The length cap is enforced by the application layer
    -- (the slug validator in internal/marketplace) so the DB type is the
    -- safer-to-evolve one. Same shape as `org_slug_holds` in 00048.
    slug       TEXT         NOT NULL,
    -- Soft pointer to the KB that vacated the slug. Carried for audit
    -- and admin tooling. SET NULL on KB hard-delete so the hold row
    -- survives — the 410 still needs to fire even after the row is
    -- gone, because the visitor's link does not know that.
    kb_id      UUID         REFERENCES knowledge_bases(id) ON DELETE SET NULL,
    held_until TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (org_id, slug)
);

-- ---------------------------------------------------------------------------
-- 2. Sweep index. Daily cron deletes WHERE held_until < now(); without an
--    index on `held_until` that scan would degrade as the table grows.
-- ---------------------------------------------------------------------------
CREATE INDEX idx_kb_slug_holds_held_until ON kb_slug_holds(held_until);

-- +goose Down
-- +goose NO TRANSACTION
--
-- Drop in reverse order. The table is freshly introduced by this
-- migration, so DROP TABLE here is safe — no pre-existing data to
-- preserve. (Squawk's "no DROP COLUMN on existing tables" rule does
-- not apply to a table this migration itself created.)
--
-- DROP INDEX CONCURRENTLY takes only SHARE UPDATE EXCLUSIVE on the
-- table rather than ACCESS EXCLUSIVE, so concurrent reads of the
-- already-doomed table during rollback don't get blocked. The
-- enclosing "+goose NO TRANSACTION" directive is required because
-- CONCURRENTLY cannot run inside a transaction block.
SET lock_timeout = '5s';
SET statement_timeout = '30s';
-- squawk-ignore prefer-robust-stmts
DROP INDEX CONCURRENTLY IF EXISTS idx_kb_slug_holds_held_until;
-- squawk-ignore ban-drop-table — this table is created by this
-- migration's Up half; the Down half necessarily drops it.
DROP TABLE IF EXISTS kb_slug_holds;
