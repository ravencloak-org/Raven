-- +goose Up
-- Intentionally violates ban-drop-table. Used by
-- .github/workflows/sql-review-selftest.yml to verify the rule fires.
DROP TABLE IF EXISTS chunks;
