-- +goose Up
-- Intentionally violates statement.disallow-truncate.
TRUNCATE TABLE response_cache;
