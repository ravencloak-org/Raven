-- +goose Up
--
-- Promote `organizations.slug` to a first-class, URL-safe, globally-unique
-- column (issue #724, M1). The free-form VARCHAR(100) shape from migration
-- 00003 is tightened to VARCHAR(64) plus a strict URL-safe CHECK constraint
-- so Marketplace URLs of shape `/marketplace/{org_slug}/{kb_slug}` per
-- ADR-0005 and ADR-0007 (Q7e) are guaranteed safe to route.
--
-- This migration also:
--   * Backfills any existing slug rows that do not match the new regex by
--     re-slugifying `name` deterministically (lowercase, dash-separated,
--     alphanumeric-only) with `-2`, `-3`, ... suffixes on collisions. The
--     backfill is idempotent — re-running against already-conforming data
--     is a no-op.
--   * Adds `org_slug_holds` — the 30-day soft-redirect table that preserves
--     Marketplace URLs across an Org slug rename (ADR-0007 Q7e).
--   * Adds the partial UNIQUE index on `knowledge_bases (org_id, slug)
--     WHERE visibility='public'` that 00047 deferred until `org_slug`
--     existed; the Marketplace URL handler needs at-most-one public KB per
--     (org, slug) pair to route deterministically.
--
-- References:
--   - docs/plans/marketplace-mvp.md §2 (00048_org_slug.sql)
--   - docs/adr/0005-org-as-marketplace-publisher.md
--   - docs/adr/0007-marketplace-lifecycle-behaviours.md (Q7e)

-- ---------------------------------------------------------------------------
-- 1. Tighten `organizations.slug` to VARCHAR(64).
-- ---------------------------------------------------------------------------
-- The existing column is VARCHAR(100) NOT NULL UNIQUE (migration 00003). The
-- new shape is VARCHAR(64) NOT NULL UNIQUE with a CHECK constraint enforcing
-- the URL-safe regex. We narrow the type first so the backfill sees the
-- final shape, then backfill, then add the CHECK.
ALTER TABLE organizations
    ALTER COLUMN slug TYPE VARCHAR(64);

-- ---------------------------------------------------------------------------
-- 2. Slug normaliser. PL/pgSQL function used only inside this migration.
-- ---------------------------------------------------------------------------
-- Mirrors `internal/marketplace.Slugify`. Kept in lock-step with the Go
-- helper so the migration-time backfill produces the same slugs the
-- application would produce at create-time.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pg_temp.slugify(input TEXT)
RETURNS TEXT AS $$
DECLARE
    s TEXT;
BEGIN
    IF input IS NULL THEN
        RETURN '';
    END IF;
    -- Lowercase, replace non-alphanumeric runs with a single dash, trim
    -- leading/trailing dashes, then clip to 64 characters.
    s := lower(input);
    s := regexp_replace(s, '[^a-z0-9]+', '-', 'g');
    s := regexp_replace(s, '^-+', '');
    s := regexp_replace(s, '-+$', '');
    s := substring(s FROM 1 FOR 64);
    -- After clipping we may have a trailing dash — strip again.
    s := regexp_replace(s, '-+$', '');
    RETURN s;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- 3. Backfill. Idempotent: rows whose existing slug already matches the
--    URL-safe regex are skipped. Rows that violate the regex are
--    re-slugified from `name`, with `-2`, `-3`, ... suffixes on collisions
--    against both the live `organizations.slug` set AND slugs already
--    assigned in this pass (deterministic order by `created_at`, `id`).
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
DO $$
DECLARE
    rec RECORD;
    base TEXT;
    candidate TEXT;
    suffix INT;
BEGIN
    FOR rec IN
        SELECT id, name, slug
        FROM organizations
        WHERE slug !~ '^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$'
        ORDER BY created_at, id
    LOOP
        base := pg_temp.slugify(rec.name);
        IF base = '' THEN
            -- Fallback: slugified name was empty (pure-symbol name). Use the
            -- org id's first dash-segment so the result remains URL-safe.
            base := substring(replace(rec.id::text, '-', '') FROM 1 FOR 12);
        END IF;
        -- Reserve room for a `-N` suffix while staying inside 64 chars.
        IF length(base) > 62 THEN
            base := substring(base FROM 1 FOR 62);
            base := regexp_replace(base, '-+$', '');
        END IF;
        candidate := base;
        suffix := 2;
        WHILE EXISTS (
            SELECT 1 FROM organizations WHERE slug = candidate AND id <> rec.id
        ) LOOP
            candidate := base || '-' || suffix::text;
            suffix := suffix + 1;
        END LOOP;
        UPDATE organizations SET slug = candidate WHERE id = rec.id;
    END LOOP;
END;
$$;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- 4. Enforce the URL-safe shape going forward.
-- ---------------------------------------------------------------------------
ALTER TABLE organizations
    ADD CONSTRAINT organizations_slug_format_check
    CHECK (slug ~ '^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$');

-- The UNIQUE constraint from 00003 is preserved by ALTER TYPE; no action
-- needed. The `idx_organizations_slug` btree index likewise remains.

-- ---------------------------------------------------------------------------
-- 5. `org_slug_holds` — 30-day soft-redirect on rename. Public-readable
--    lookup table so the Marketplace URL handler can resolve a stale slug
--    cross-tenant without engaging RLS. Writes happen only via the
--    `RenameSlug` repository method (single transaction) — no RLS policy
--    is added because the table has no `org_id` row-isolation axis;
--    permissions are managed at the role grant level by ops.
-- ---------------------------------------------------------------------------
CREATE TABLE org_slug_holds (
    slug VARCHAR(64) PRIMARY KEY,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    held_until TIMESTAMPTZ NOT NULL,
    CONSTRAINT org_slug_holds_slug_format_check
        CHECK (slug ~ '^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$')
);

-- Lookup by org_id is needed when listing an org's previously-held slugs
-- (admin UI) and when expiring holds in a sweeper job.
CREATE INDEX idx_org_slug_holds_org_id ON org_slug_holds(org_id);

-- ---------------------------------------------------------------------------
-- 6. Partial UNIQUE on `knowledge_bases (org_id, slug) WHERE visibility =
--    'public'`. 00047 deferred this until `org_slug` was first-class; with
--    the tightened slug shape in place we can now guarantee Marketplace
--    URL `(org_slug, kb_slug)` resolves to at most one public KB.
-- ---------------------------------------------------------------------------
CREATE UNIQUE INDEX idx_kb_public_org_slug_uniq
    ON knowledge_bases (org_id, slug)
    WHERE visibility = 'public';

-- +goose Down
--
-- Reverse in dependency order: KB partial unique → holds table → CHECK on
-- organizations.slug → widen column back to VARCHAR(100). We do NOT
-- attempt to restore the pre-backfill slug values: that data is lost by
-- design because the original values violated the new URL-safe shape.
DROP INDEX IF EXISTS idx_kb_public_org_slug_uniq;

DROP INDEX IF EXISTS idx_org_slug_holds_org_id;
DROP TABLE IF EXISTS org_slug_holds;

ALTER TABLE organizations
    DROP CONSTRAINT IF EXISTS organizations_slug_format_check;

ALTER TABLE organizations
    ALTER COLUMN slug TYPE VARCHAR(100);
