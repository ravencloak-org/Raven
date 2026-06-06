-- +goose Up
--
-- Marketplace content-grade fork on import (issue #729, M2).
--
-- ADR-0001 makes the Marketplace import a fork: each import creates a brand
-- new local KB under the importer Org's workspace by applying the content-
-- grade publish projection (ADR-0002). This migration adds the cross-tenant
-- SECURITY DEFINER function that performs the entire copy atomically.
--
-- ADR-0001 pins the listing function as one of the only cross-tenant read
-- paths; the import function is its cross-tenant write twin. Both are owned
-- by `raven_admin` and run as SECURITY DEFINER so application-role callers
-- (raven_app) can transit the tenant boundary for this single, audited
-- operation without weakening RLS on the underlying tables.
--
-- The function is the canonical structural enforcement of ADR-0002:
--
--   * The INSERT column lists are hard-coded and explicitly omit the
--     never-projected tables (`api_keys`, `chat_sessions`, `routing_rules`,
--     `webhook_configs`, `response_cache`, `airbyte_connectors`) and the
--     never-projected per-row fields on the projected tables (e.g. the
--     SeaweedFS blob path on `documents.storage_path`). Adding a new
--     never-projected column to one of the projected tables means
--     deliberately deciding NOT to add it to this function — silent
--     widening is impossible.
--
--   * The `settings` JSONB scrub uses an explicit jsonb allow-list
--     (`p_settings_allowlist TEXT[]`) supplied by the Go layer (the
--     authoritative `internal/marketplace.settingsAllowList` constant).
--     Keys not in the list are dropped. Default deny.
--
--   * The embedding model match check fails loud with a typed SQLSTATE so
--     the Go layer maps it to ErrEmbeddingModelMismatch without paying for
--     a write. Re-embedding is deferred (see ADR-0001 §Trade-offs).
--
--   * Idempotency: the function refuses (unique_violation SQLSTATE) when a
--     KB with the same `(workspace_id, source_public_kb_id)` already
--     exists. This matches the package-manager UX in ADR-0001 — re-import
--     is an explicit user action (#730), not a silent duplicate fork.
--
-- The Go-side projection struct in `internal/marketplace/projection.go`
-- mirrors this function's column lists field-for-field; the test
-- `TestProjectionStructMatchesSQLFunction` pins them together so drift on
-- either side fails the build loudly.
--
-- References:
--   * docs/plans/marketplace-mvp.md §3 (`import.go`), §4 (import endpoint)
--   * docs/adr/0001-marketplace-fork-on-import.md
--   * docs/adr/0002-content-grade-publish-boundary.md
--   * docs/adr/0004-free-tier-public-only-rule.md

SET lock_timeout = '5s';
SET statement_timeout = '30s';

-- ---------------------------------------------------------------------------
-- marketplace_import_kb — content-grade cross-tenant fork.
-- ---------------------------------------------------------------------------
--
-- Parameters:
--   p_src_kb_id              — UUID of the source Public KB.
--   p_dst_org_id             — UUID of the importer Org.
--   p_dst_workspace_id       — UUID of the importer Workspace.
--   p_dst_user_id            — UUID of the importer User (audit only).
--   p_force_public           — TRUE iff the importer Org is Free Plan and
--                              must inherit visibility='public'. Resolved
--                              by the Go layer above the projection
--                              (ADR-0004); this function only obeys.
--   p_required_embedding_model — Destination Org's default embedding model
--                              name. NULL means "destination has no
--                              default; copy embeddings if any". Empty
--                              string is treated identically to NULL.
--   p_settings_allowlist     — TEXT[] of `settings` JSONB top-level keys
--                              that may cross the publish boundary. Keys
--                              prefixed with `public:` are also allowed
--                              regardless of explicit list membership.
--
-- Returns the new local KB row with the columns the Go service needs to
-- render the API response (id, workspace_id, visibility,
-- imported_from_revision_at, source_public_kb_id, license_spdx_id).
--
-- Error SQLSTATEs the Go layer maps to typed errors:
--
--   insufficient_privilege (42501) — source KB not visibility='public'.
--                                    Mapped to ErrSourceNotPublic / HTTP 403.
--   data_exception        (22000) — source KB has zero documents (an
--                                    empty Public KB is unimportable per
--                                    plan §4).
--   restrict_violation    (2BP01) — embedding model mismatch. The Go layer
--                                    maps this to ErrEmbeddingModelMismatch
--                                    and surfaces HTTP 422 / 409 per spec.
--   unique_violation       (23505) — a KB already exists for this
--                                    (workspace_id, source_public_kb_id).
--                                    Mapped to ErrAlreadyImported / HTTP 409.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION marketplace_import_kb(
    p_src_kb_id                UUID,
    p_dst_org_id               UUID,
    p_dst_workspace_id         UUID,
    p_dst_user_id              UUID,
    p_force_public             BOOLEAN,
    p_required_embedding_model TEXT,
    p_settings_allowlist       TEXT[]
)
RETURNS TABLE (
    kb_id                     UUID,
    workspace_id              UUID,
    visibility                kb_visibility,
    imported_from_revision_at TIMESTAMPTZ,
    source_public_kb_id       UUID,
    license_spdx_id           TEXT
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
    src             RECORD;
    src_doc_count   INT;
    src_emb_models  TEXT[];
    new_kb_id       UUID;
    base_slug       TEXT;
    final_slug      TEXT;
    new_settings    JSONB;
    new_visibility  kb_visibility;
BEGIN
    -- 1. Resolve and gate the source KB. Treating "missing" and "private"
    -- identically (both raise insufficient_privilege) matches the listing /
    -- preview policy in 00052 so the API surface cannot probe for the
    -- existence of private KBs.
    SELECT
        kb.id, kb.org_id, kb.name, kb.slug, kb.description,
        kb.settings, kb.visibility, kb.last_modified_at, kb.license_spdx_id
    INTO src
    FROM knowledge_bases kb
    WHERE kb.id = p_src_kb_id;

    IF NOT FOUND OR src.visibility <> 'public' THEN
        RAISE EXCEPTION 'kb_not_public: %', p_src_kb_id
            USING ERRCODE = 'insufficient_privilege';
    END IF;

    -- 2. Plan §4: a source with zero documents is unimportable. Checked
    -- before any write so we never half-create a KB whose chunks join was
    -- always going to be empty.
    SELECT COUNT(*) INTO src_doc_count
    FROM documents d
    WHERE d.knowledge_base_id = p_src_kb_id;

    IF src_doc_count = 0 THEN
        RAISE EXCEPTION 'source_kb_empty: %', p_src_kb_id
            USING ERRCODE = 'data_exception';
    END IF;

    -- 3. Embedding model match. We collect the DISTINCT set of model_name
    -- values present on the source's embeddings; if the destination has a
    -- required model and the source's set is non-empty and does not
    -- contain it, we fail loud. A source with no embeddings is allowed
    -- through (re-embed flow deferred, no embeddings to copy).
    SELECT COALESCE(array_agg(DISTINCT e.model_name), ARRAY[]::TEXT[])
    INTO src_emb_models
    FROM embeddings e
    JOIN chunks c ON c.id = e.chunk_id
    WHERE c.knowledge_base_id = p_src_kb_id;

    IF p_required_embedding_model IS NOT NULL
       AND length(btrim(p_required_embedding_model)) > 0
       AND cardinality(src_emb_models) > 0
       AND NOT (p_required_embedding_model = ANY (src_emb_models)) THEN
        RAISE EXCEPTION 'embedding_model_mismatch: source=% dest=%',
            src_emb_models, p_required_embedding_model
            USING ERRCODE = 'restrict_violation';
    END IF;

    -- 4. Idempotency: refuse to fork the same source into the same
    -- workspace twice. The Go layer maps unique_violation onto a 409 with
    -- a Re-import hint pointing at #730.
    IF EXISTS (
        SELECT 1 FROM knowledge_bases dup
        WHERE dup.workspace_id        = p_dst_workspace_id
          AND dup.source_public_kb_id = p_src_kb_id
          AND dup.status              <> 'archived'
    ) THEN
        RAISE EXCEPTION 'duplicate_import: workspace=% source=%',
            p_dst_workspace_id, p_src_kb_id
            USING ERRCODE = 'unique_violation';
    END IF;

    -- 5. Settings projection: keep only allow-listed top-level keys plus
    -- anything explicitly tagged with the `public:` prefix. Default deny.
    -- jsonb_object_agg of the surviving keys preserves the JSONB shape
    -- the column expects.
    SELECT COALESCE(
        jsonb_object_agg(s.key, s.value),
        '{}'::jsonb
    ) INTO new_settings
    FROM jsonb_each(COALESCE(src.settings, '{}'::jsonb)) AS s(key, value)
    WHERE s.key = ANY (COALESCE(p_settings_allowlist, ARRAY[]::TEXT[]))
       OR s.key LIKE 'public:%';

    -- 6. Free Plan override (ADR-0004). Above the projection in the Go
    -- layer; here we just apply the flag the caller resolved.
    IF p_force_public THEN
        new_visibility := 'public';
    ELSE
        new_visibility := 'private';
    END IF;

    -- 7. Slug collision avoidance inside the destination workspace. The
    -- unique constraint is (workspace_id, slug); we append -2, -3, … to
    -- the source slug until we find a free one. Bounded loop so a hostile
    -- workspace can't make us spin forever.
    base_slug  := src.slug;
    final_slug := base_slug;
    FOR i IN 2..100 LOOP
        EXIT WHEN NOT EXISTS (
            SELECT 1 FROM knowledge_bases dup
            WHERE dup.workspace_id = p_dst_workspace_id
              AND dup.slug         = final_slug
        );
        final_slug := base_slug || '-' || i::text;
    END LOOP;

    -- 8. Insert the KB row. Column list is the projection contract:
    -- explicitly excludes api_keys, chat_sessions, routing_rules,
    -- webhook_configs, response_cache, airbyte_connectors (none of those
    -- live on knowledge_bases, but the principle holds for the per-row
    -- settings scrub above). Lineage is stamped with source's
    -- last_modified_at so the "stale relative to source" check from
    -- ADR-0003 is computable without a Marketplace round-trip.
    INSERT INTO knowledge_bases (
        org_id, workspace_id, name, slug, description, settings,
        status,
        visibility,
        published_at,
        published_by_user_id,
        source_public_kb_id,
        imported_from_revision_at,
        license_spdx_id
    )
    VALUES (
        p_dst_org_id, p_dst_workspace_id, src.name, final_slug,
        NULLIF(src.description, ''),
        new_settings,
        'active',
        new_visibility,
        -- A forced-public Free Plan import lights up as published at
        -- import time (the Marketplace card needs a published_at to
        -- order by). Private imports leave published_at NULL.
        CASE WHEN new_visibility = 'public' THEN NOW() ELSE NULL END,
        CASE WHEN new_visibility = 'public' THEN p_dst_user_id ELSE NULL END,
        p_src_kb_id,
        src.last_modified_at,
        -- A forced-public import inherits the source's license so the
        -- public_requires_license CHECK is satisfied. Private imports
        -- copy the license too (it's still part of the projection — a
        -- subsequent publish from Free→Paid can rely on it).
        src.license_spdx_id
    )
    RETURNING knowledge_bases.id INTO new_kb_id;

    -- 9. Sources — metadata only, never file blobs (none on this table to
    -- begin with; sources are URLs/sitemaps/RSS). created_by stamps the
    -- importer User so the audit trail attributes the row to the actor
    -- who ran the fork, not the original publisher.
    INSERT INTO sources (
        org_id, knowledge_base_id, source_type, url, crawl_depth,
        crawl_frequency, processing_status, processing_error, title,
        pages_crawled, metadata, created_by
    )
    SELECT
        p_dst_org_id, new_kb_id, s.source_type, s.url, s.crawl_depth,
        s.crawl_frequency, 'ready', NULL, s.title,
        s.pages_crawled, COALESCE(s.metadata, '{}'::jsonb), p_dst_user_id
    FROM sources s
    WHERE s.knowledge_base_id = p_src_kb_id;

    -- 10. Documents — metadata only. file_hash, file_name, file_type, and
    -- title cross the boundary; storage_path is deliberately NULL on the
    -- import (the SeaweedFS blob is publisher-private per ADR-0002).
    -- processing_status is forced to 'ready' so the importer's UI does
    -- not show a stale "queued" badge — the content (chunks/embeddings)
    -- is already complete.
    INSERT INTO documents (
        org_id, knowledge_base_id, file_name, file_type, file_size_bytes,
        file_hash, storage_path, processing_status, processing_error,
        title, page_count, metadata, uploaded_by
    )
    SELECT
        p_dst_org_id, new_kb_id, d.file_name, d.file_type, d.file_size_bytes,
        d.file_hash,
        NULL,              -- ADR-0002 §"Never crosses": SeaweedFS blob ref
        'ready', NULL,
        d.title, d.page_count, COALESCE(d.metadata, '{}'::jsonb),
        p_dst_user_id
    FROM documents d
    WHERE d.knowledge_base_id = p_src_kb_id;

    -- 11. Chunks. Re-keyed to the new KB and to the new documents/sources
    -- by matching on file_hash (documents) or url (sources). Both keys
    -- are stable within the projection: file_hash is content-addressed
    -- and url is the source identity. The dual lookup uses LEFT JOIN +
    -- COALESCE so a chunk whose parent didn't survive (shouldn't happen
    -- given we copy them above, but defensive) still inserts with a NULL
    -- pointer to the missing parent and the CHECK on chunks fires.
    INSERT INTO chunks (
        org_id, knowledge_base_id, document_id, source_id,
        content, chunk_index, token_count, page_number, heading,
        chunk_type, metadata
    )
    SELECT
        p_dst_org_id, new_kb_id,
        new_d.id AS document_id,
        new_s.id AS source_id,
        c.content, c.chunk_index, c.token_count, c.page_number, c.heading,
        c.chunk_type, COALESCE(c.metadata, '{}'::jsonb)
    FROM chunks c
    LEFT JOIN documents src_d ON src_d.id = c.document_id
    LEFT JOIN documents new_d ON new_d.knowledge_base_id = new_kb_id
                              AND new_d.file_hash = src_d.file_hash
                              AND new_d.file_hash IS NOT NULL
    LEFT JOIN sources   src_s ON src_s.id = c.source_id
    LEFT JOIN sources   new_s ON new_s.knowledge_base_id = new_kb_id
                              AND new_s.url = src_s.url
    WHERE c.knowledge_base_id = p_src_kb_id;

    -- 12. Embeddings — only when the model matches the destination's
    -- required model. We re-link to the new chunks via the content
    -- identity (`(knowledge_base_id, content, chunk_index)`), which is
    -- stable across the projection because chunks are content-identical
    -- by construction.
    INSERT INTO embeddings (
        org_id, chunk_id, embedding, model_name, model_version, dimensions
    )
    SELECT
        p_dst_org_id, new_c.id, e.embedding, e.model_name, e.model_version,
        e.dimensions
    FROM embeddings e
    JOIN chunks src_c ON src_c.id = e.chunk_id
    JOIN chunks new_c ON new_c.knowledge_base_id = new_kb_id
                     AND new_c.content     = src_c.content
                     AND new_c.chunk_index = src_c.chunk_index
    WHERE src_c.knowledge_base_id = p_src_kb_id
      AND (p_required_embedding_model IS NULL
           OR length(btrim(p_required_embedding_model)) = 0
           OR e.model_name = p_required_embedding_model);

    -- 13. Bump import_count on the source KB (ADR-0008 discovery counter).
    -- Single-statement atomic increment — the lineage row is the source
    -- of truth.
    UPDATE knowledge_bases
       SET import_count = import_count + 1
     WHERE id = p_src_kb_id;

    -- 14. Return the new KB row's identity for the API response.
    RETURN QUERY
    SELECT
        new_kb.id                        AS kb_id,
        new_kb.workspace_id              AS workspace_id,
        new_kb.visibility                AS visibility,
        new_kb.imported_from_revision_at AS imported_from_revision_at,
        new_kb.source_public_kb_id       AS source_public_kb_id,
        new_kb.license_spdx_id           AS license_spdx_id
    FROM knowledge_bases new_kb
    WHERE new_kb.id = new_kb_id;
END;
$$;
-- +goose StatementEnd

ALTER FUNCTION marketplace_import_kb(UUID, UUID, UUID, UUID, BOOLEAN, TEXT, TEXT[])
    OWNER TO raven_admin;

REVOKE ALL ON FUNCTION marketplace_import_kb(UUID, UUID, UUID, UUID, BOOLEAN, TEXT, TEXT[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION marketplace_import_kb(UUID, UUID, UUID, UUID, BOOLEAN, TEXT, TEXT[]) TO raven_app;

-- +goose Down
DROP FUNCTION IF EXISTS marketplace_import_kb(UUID, UUID, UUID, UUID, BOOLEAN, TEXT, TEXT[]);
