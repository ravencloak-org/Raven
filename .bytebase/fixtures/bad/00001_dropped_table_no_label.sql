-- +goose Up
-- Intentionally violates statement.disallow-rm-tbl-cascade.
-- Used by .github/workflows/sql-review-selftest.yml to verify the policy fires.
DROP TABLE IF EXISTS chunks CASCADE;
