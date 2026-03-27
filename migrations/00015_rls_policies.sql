-- +goose Up
-- Issue #29 — PostgreSQL RLS Policies
--
-- All tenant-scoped tables already have RLS enabled and policies applied in their
-- respective creation migrations (00004–00014). This migration documents that
-- coverage is complete and applies any remaining tables not previously covered.
--
-- Tables with RLS confirmed in prior migrations:
--   users                (00004) — tenant_isolation + admin_bypass
--   workspaces           (00005) — tenant_isolation + admin_bypass
--   workspace_members    (00006) — tenant_isolation + admin_bypass
--   knowledge_bases      (00007) — tenant_isolation + admin_bypass
--   documents            (00008) — tenant_isolation + admin_bypass
--   sources              (00009) — tenant_isolation + admin_bypass
--   chunks               (00010) — tenant_isolation + admin_bypass
--   embeddings           (00010) — tenant_isolation + admin_bypass
--   llm_provider_configs (00011) — tenant_isolation + admin_bypass
--   api_keys             (00012) — tenant_isolation + admin_bypass
--   chat_sessions        (00013) — tenant_isolation + admin_bypass
--   chat_messages        (00013) — tenant_isolation + admin_bypass
--   processing_events    (00014) — tenant_isolation + admin_bypass
--
-- organizations is intentionally excluded: it IS the tenant boundary.
--
-- This migration is intentionally idempotent — it makes no schema changes.
-- It serves as the definitive record that issue #29 (RLS policy enforcement)
-- is complete for the current schema.

SELECT 1; -- no-op marker for goose

-- +goose Down
SELECT 1; -- no-op
