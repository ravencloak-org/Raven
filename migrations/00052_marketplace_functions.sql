-- +goose Up
--
-- Marketplace cross-tenant read functions (issue #728, M2).
--
-- ADR-0001 names the Marketplace listing function as the ONLY cross-tenant
-- read path in Raven outside the moderation surface. Both functions are
-- `SECURITY DEFINER` so the application role (raven_app) can read public
-- KBs across Orgs without weakening RLS; ownership is `raven_admin`, whose
-- `admin_bypass` policies (migration 00015) allow the function body to see
-- every row regardless of the caller's `app.current_org_id` setting.
--
-- Defence in depth:
--   * `SET search_path = pg_catalog, public` — prevents schema-injection on
--     unqualified identifier resolution inside the function body.
--   * Preview cap (≤3 chunks) is enforced inside the function so a caller
--     cannot ask for more by tampering with arguments.
--   * Listing `page_limit` is clamped to MAX_PAGE_LIMIT internally; callers
--     cannot exfiltrate the entire Marketplace in one round trip.
--   * Non-public preview targets raise `insufficient_privilege` so the Go
--     layer maps the error to HTTP 404 without leaking existence.
--
-- References:
--   * docs/plans/marketplace-mvp.md §2 (00052_marketplace_functions.sql)
--   * docs/adr/0001-marketplace-fork-on-import.md
--   * docs/adr/0005-org-as-marketplace-publisher.md
--   * docs/adr/0008-marketplace-discovery-and-operations.md

-- ---------------------------------------------------------------------------
-- 1. marketplace_list_public_kbs — discovery listing.
-- ---------------------------------------------------------------------------
--
-- Returns the row shape declared in ADR-0005 (`kb_id`, `org_slug`,
-- `org_display_name`, `kb_slug`, `kb_name`, `description`, `last_modified_at`,
-- `source_public_kb_id`, `source_org_slug`, `source_org_display_name`) plus
-- `license_spdx_id` and `import_count` per ADR-0008.
--
-- `organizations.name` is the public-facing display name today (ADR-0005
-- §"Trade-offs"). It is returned as `org_display_name` so the API and
-- frontend can rename the column without a schema migration when/if a
-- separate `display_name` lands.
--
-- We use plpgsql (not plain SQL) because the sort branch needs a CASE-in-
-- ORDER-BY, which plain SQL functions can't inline as efficiently when
-- combined with the optional FTS predicate, and because we want to raise
-- an exception on an unknown `sort` value rather than silently degrading.
--
-- Join shape: a single LEFT JOIN to organizations for the publisher slug +
-- name, then a LEFT JOIN through `source_public_kb_id` to the parent KB
-- and parent organisation for the one-hop derivative lineage in ADR-0005.
-- LEFT JOINs (not LATERAL) because every row needs the publisher and the
-- parent columns are NULL for non-derivative KBs; the planner produces a
-- flatter hash-join plan than a correlated LATERAL would.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION marketplace_list_public_kbs(
    q             TEXT,
    sort          TEXT,
    licenses      TEXT[],
    page_limit    INT,
    page_offset   INT
)
RETURNS TABLE (
    kb_id                   UUID,
    org_slug                TEXT,
    org_display_name        TEXT,
    kb_slug                 TEXT,
    kb_name                 TEXT,
    description             TEXT,
    license_spdx_id         TEXT,
    last_modified_at        TIMESTAMPTZ,
    import_count            INT,
    source_public_kb_id     UUID,
    source_org_slug         TEXT,
    source_org_display_name TEXT
)
LANGUAGE plpgsql
STABLE
PARALLEL SAFE
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
    -- Hard cap so a misconfigured client (or malicious one) cannot ask for
    -- the entire Marketplace in a single round trip. 50 matches the listing
    -- page size the frontend infinite-scroll requests.
    max_page_limit CONSTANT INT := 50;
    effective_limit  INT;
    effective_offset INT;
    tsq              tsquery;
BEGIN
    -- Pagination clamps. NULLs and out-of-range values fall back to safe
    -- defaults so the function never returns a misleading empty page from
    -- a bad arg.
    effective_limit  := LEAST(COALESCE(page_limit, max_page_limit), max_page_limit);
    IF effective_limit < 1 THEN
        effective_limit := max_page_limit;
    END IF;
    effective_offset := GREATEST(COALESCE(page_offset, 0), 0);

    -- websearch_to_tsquery accepts the kind of free-form input users type
    -- into a search box (quoted phrases, OR/AND, leading minus) and never
    -- raises on malformed input — it just produces an empty tsquery. This
    -- gets us safe FTS without manual sanitisation.
    IF q IS NOT NULL AND length(btrim(q)) > 0 THEN
        tsq := websearch_to_tsquery('english', q);
    END IF;

    -- Sort whitelist. Unknown values raise so the Go layer surfaces a 400
    -- to the caller instead of silently returning the default order.
    IF sort IS NULL OR sort NOT IN ('newest', 'most_imported', 'recently_updated', 'alphabetic') THEN
        RAISE EXCEPTION 'unknown_sort: %', sort
            USING ERRCODE = 'invalid_parameter_value';
    END IF;

    RETURN QUERY
    SELECT
        kb.id                       AS kb_id,
        o.slug::TEXT                AS org_slug,
        o.name::TEXT                AS org_display_name,
        kb.slug::TEXT               AS kb_slug,
        kb.name::TEXT               AS kb_name,
        kb.description              AS description,
        kb.license_spdx_id          AS license_spdx_id,
        kb.last_modified_at         AS last_modified_at,
        kb.import_count             AS import_count,
        kb.source_public_kb_id      AS source_public_kb_id,
        src_o.slug::TEXT            AS source_org_slug,
        src_o.name::TEXT            AS source_org_display_name
    FROM knowledge_bases kb
    JOIN organizations  o     ON o.id = kb.org_id
    LEFT JOIN knowledge_bases src_kb ON src_kb.id  = kb.source_public_kb_id
    LEFT JOIN organizations   src_o  ON src_o.id   = src_kb.org_id
    WHERE kb.visibility = 'public'
      AND (tsq IS NULL OR kb.search_tsv @@ tsq)
      AND (licenses IS NULL OR cardinality(licenses) = 0
           OR kb.license_spdx_id = ANY (licenses))
    ORDER BY
        -- ORDER BY uses one CASE per sort key so each branch produces a
        -- single deterministic ordering. A NULL secondary key (`id`) is
        -- appended as a tiebreaker so pagination is stable when timestamps
        -- or names collide.
        CASE WHEN sort = 'newest'           THEN kb.published_at        END DESC NULLS LAST,
        CASE WHEN sort = 'most_imported'    THEN kb.import_count        END DESC NULLS LAST,
        CASE WHEN sort = 'recently_updated' THEN kb.last_modified_at    END DESC NULLS LAST,
        CASE WHEN sort = 'alphabetic'       THEN kb.name                END ASC  NULLS LAST,
        kb.id ASC
    LIMIT effective_limit
    OFFSET effective_offset;
END;
$$;
-- +goose StatementEnd

-- STABLE — read-only, deterministic within a statement. Set inline above.
ALTER FUNCTION marketplace_list_public_kbs(TEXT, TEXT, TEXT[], INT, INT)
    OWNER TO raven_admin;

REVOKE ALL ON FUNCTION marketplace_list_public_kbs(TEXT, TEXT, TEXT[], INT, INT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION marketplace_list_public_kbs(TEXT, TEXT, TEXT[], INT, INT) TO raven_app;

-- ---------------------------------------------------------------------------
-- 2. marketplace_preview_kb — first three chunks of a Public KB.
-- ---------------------------------------------------------------------------
--
-- The ≤3 cap is enforced in the function body (LIMIT 3) so callers cannot
-- ask for more by tampering with arguments. Non-public targets raise
-- `insufficient_privilege` so the Go layer can map the error to HTTP 404
-- (or 403) without leaking the existence of a private KB.
--
-- `preview_count` is bumped at the end so every successful preview lights
-- up the ADR-0008 discovery counter. The UPDATE runs as `raven_admin`
-- under SECURITY DEFINER, so RLS does not block the cross-tenant write
-- that this same-row counter requires (the bump is the entire point of
-- the function — there is no other way to attribute the preview to the
-- publisher Org).

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION marketplace_preview_kb(p_public_kb_id UUID)
RETURNS TABLE (
    chunk_id UUID,
    ordinal  INT,
    text     TEXT
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
    target_visibility kb_visibility;
BEGIN
    IF p_public_kb_id IS NULL THEN
        RAISE EXCEPTION 'kb_not_public: id is null'
            USING ERRCODE = 'insufficient_privilege';
    END IF;

    -- Existence + visibility check in one round trip. We treat "not found"
    -- and "found but private" identically — both raise
    -- `insufficient_privilege` so the API surface cannot be used to probe
    -- for the existence of private KBs.
    SELECT kb.visibility
    INTO target_visibility
    FROM knowledge_bases kb
    WHERE kb.id = p_public_kb_id;

    IF NOT FOUND OR target_visibility <> 'public' THEN
        RAISE EXCEPTION 'kb_not_public: %', p_public_kb_id
            USING ERRCODE = 'insufficient_privilege';
    END IF;

    -- Bump preview_count before returning chunks. Doing this BEFORE the
    -- RETURN QUERY means the increment is committed with the same
    -- transaction the caller sees, and any error in the chunk read does
    -- not double-count. ADR-0008 calls this "eventually consistent" but
    -- in practice we get exact counts because the function is the single
    -- write path.
    UPDATE knowledge_bases
       SET preview_count = preview_count + 1
     WHERE id = p_public_kb_id;

    RETURN QUERY
    SELECT
        c.id                       AS chunk_id,
        c.chunk_index              AS ordinal,
        c.content                  AS text
    FROM chunks c
    WHERE c.knowledge_base_id = p_public_kb_id
    ORDER BY c.chunk_index ASC, c.id ASC
    LIMIT 3;
END;
$$;
-- +goose StatementEnd

-- Cannot be STABLE — bumps preview_count. VOLATILE is the default for
-- plpgsql; we leave it explicit here to document the choice.
ALTER FUNCTION marketplace_preview_kb(UUID) OWNER TO raven_admin;

REVOKE ALL ON FUNCTION marketplace_preview_kb(UUID) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION marketplace_preview_kb(UUID) TO raven_app;

-- +goose Down
--
-- Reverse in dependency order. Function drops cascade-revoke their grants.
DROP FUNCTION IF EXISTS marketplace_preview_kb(UUID);
DROP FUNCTION IF EXISTS marketplace_list_public_kbs(TEXT, TEXT, TEXT[], INT, INT);
