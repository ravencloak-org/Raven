-- +goose Up
-- +goose NO TRANSACTION
--
-- Add 'web_page' and 'web_site' to the source_type enum so URL sources
-- can be created. The Go model (internal/model/source.go) and the
-- frontend (frontend/src/api/knowledge-bases.ts) both send 'web_page'
-- when a user enters a single URL via the add-source UI; without this
-- enum extension the insert fails with:
--
--   ERROR: invalid input value for enum source_type: "web_page"
--   (SQLSTATE 22P02)
--
-- Postgres requires ALTER TYPE ADD VALUE to run outside any transaction
-- (hence the NO TRANSACTION directive above). IF NOT EXISTS makes the
-- migration idempotent so re-running against a partially-migrated DB
-- is safe.
ALTER TYPE source_type ADD VALUE IF NOT EXISTS 'web_page';
ALTER TYPE source_type ADD VALUE IF NOT EXISTS 'web_site';

-- +goose Down
-- +goose NO TRANSACTION
--
-- Postgres has no ALTER TYPE ... REMOVE VALUE; rolling back the enum
-- would require recreating the type and rewriting every column that
-- references it. The risk-reward isn't worth it for an additive change,
-- so the Down is a no-op. If a true rollback is ever needed, drop the
-- sources table or write a bespoke rebuild migration.
SELECT 1;
