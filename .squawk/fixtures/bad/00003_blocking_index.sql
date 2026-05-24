-- +goose Up
-- Intentionally violates require-concurrent-index-creation: blocking
-- CREATE INDEX on a populated table prevents concurrent writes.
CREATE INDEX idx_sources_workspace ON sources (workspace_id);
