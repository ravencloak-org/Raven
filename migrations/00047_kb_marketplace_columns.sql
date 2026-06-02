-- +goose Up
--
-- Marketplace MVP schema groundwork (issue #723, M1).
--
-- Extends knowledge_bases with publish-state, lineage, freshness, license,
-- and discovery columns. See:
--   - docs/plans/marketplace-mvp.md §2 (00047_kb_marketplace_columns.sql)
--   - ADR-0001 (fork on import) — source_public_kb_id lineage pointer
--   - ADR-0002 (content-grade publish boundary) — visibility transitions
--   - ADR-0003 (always-fresh publish lifecycle) — last_modified_at signal
--   - ADR-0005 (org as marketplace publisher) — published_by_user_id audit
--   - ADR-0008 (marketplace discovery and operations) — search_tsv + counters
--
-- This is a foundation slice. Publish/import handlers, the partial UNIQUE
-- on (org_id, slug) WHERE visibility='public' (depends on org_slug from
-- migration 00048), the license CHECK constraint (migration 00050), and
-- the read-only-private kb_status value (migration 00049) all land in
-- later migrations. Column shape + trigger + indexes only here.

-- New enum: KB visibility flag. Marketplace listing reads filter
-- visibility='public'; everything else stays private (the default).
CREATE TYPE kb_visibility AS ENUM ('private', 'public');

ALTER TABLE knowledge_bases
    ADD COLUMN visibility kb_visibility NOT NULL DEFAULT 'private',
    ADD COLUMN published_at TIMESTAMPTZ NULL,
    ADD COLUMN published_by_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN source_public_kb_id UUID NULL REFERENCES knowledge_bases(id) ON DELETE SET NULL,
    ADD COLUMN imported_from_revision_at TIMESTAMPTZ NULL,
    ADD COLUMN last_modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN license_spdx_id TEXT NULL,
    ADD COLUMN import_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN preview_count INTEGER NOT NULL DEFAULT 0,
    -- search_tsv is GENERATED from in-row text only (name + description).
    -- The publishing org's display_name lives on organizations and cannot be
    -- referenced from a GENERATED expression on this table. The Marketplace
    -- listing function (00052) JOINs organizations and concatenates its
    -- display_name into the search expression at query time instead.
    ADD COLUMN search_tsv tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'B')
    ) STORED;

-- Discovery / freshness indexes. Partial indexes keep them tiny while the
-- vast majority of rows stay private.
CREATE INDEX idx_kb_visibility_public
    ON knowledge_bases (visibility)
    WHERE visibility = 'public';

CREATE INDEX idx_kb_source_public_kb_id
    ON knowledge_bases (source_public_kb_id)
    WHERE source_public_kb_id IS NOT NULL;

CREATE INDEX idx_kb_last_modified_at_public
    ON knowledge_bases (last_modified_at DESC)
    WHERE visibility = 'public';

CREATE INDEX idx_kb_search_tsv_public
    ON knowledge_bases USING gin (search_tsv)
    WHERE visibility = 'public';

-- Trigger function: bump the parent KB's last_modified_at on any
-- INSERT/UPDATE/DELETE of documents, sources, or chunks. ADR-0003 makes
-- last_modified_at the canonical "is this Public KB stale relative to
-- the Importer's snapshot?" signal, so it must move whenever content
-- mutates — not just when the KB row itself is updated.
--
-- The trigger reads knowledge_base_id from NEW for INSERT/UPDATE and from
-- OLD for DELETE; both shapes carry the column on all three child tables.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION bump_kb_last_modified()
RETURNS TRIGGER AS $$
DECLARE
    target_kb_id UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_kb_id := OLD.knowledge_base_id;
    ELSE
        target_kb_id := NEW.knowledge_base_id;
    END IF;

    IF target_kb_id IS NOT NULL THEN
        UPDATE knowledge_bases
        SET last_modified_at = NOW()
        WHERE id = target_kb_id;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_kb_last_modified_on_documents
    AFTER INSERT OR UPDATE OR DELETE ON documents
    FOR EACH ROW EXECUTE FUNCTION bump_kb_last_modified();

CREATE TRIGGER trg_kb_last_modified_on_sources
    AFTER INSERT OR UPDATE OR DELETE ON sources
    FOR EACH ROW EXECUTE FUNCTION bump_kb_last_modified();

CREATE TRIGGER trg_kb_last_modified_on_chunks
    AFTER INSERT OR UPDATE OR DELETE ON chunks
    FOR EACH ROW EXECUTE FUNCTION bump_kb_last_modified();

-- +goose Down
--
-- Reverse in dependency order: triggers → function → indexes → columns → enum.
DROP TRIGGER IF EXISTS trg_kb_last_modified_on_chunks ON chunks;
DROP TRIGGER IF EXISTS trg_kb_last_modified_on_sources ON sources;
DROP TRIGGER IF EXISTS trg_kb_last_modified_on_documents ON documents;
DROP FUNCTION IF EXISTS bump_kb_last_modified();

DROP INDEX IF EXISTS idx_kb_search_tsv_public;
DROP INDEX IF EXISTS idx_kb_last_modified_at_public;
DROP INDEX IF EXISTS idx_kb_source_public_kb_id;
DROP INDEX IF EXISTS idx_kb_visibility_public;

ALTER TABLE knowledge_bases
    DROP COLUMN IF EXISTS search_tsv,
    DROP COLUMN IF EXISTS preview_count,
    DROP COLUMN IF EXISTS import_count,
    DROP COLUMN IF EXISTS license_spdx_id,
    DROP COLUMN IF EXISTS last_modified_at,
    DROP COLUMN IF EXISTS imported_from_revision_at,
    DROP COLUMN IF EXISTS source_public_kb_id,
    DROP COLUMN IF EXISTS published_by_user_id,
    DROP COLUMN IF EXISTS published_at,
    DROP COLUMN IF EXISTS visibility;

DROP TYPE IF EXISTS kb_visibility;
