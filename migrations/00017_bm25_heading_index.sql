-- +goose Up
-- Issue #19 — Full-text search index enhancements for BM25-style ranking.
--
-- Migration 00010 already created a GIN index on chunks.content:
--   idx_chunks_content_fts ON chunks USING gin(to_tsvector('english', content))
--
-- This migration adds:
--   1. GIN index on heading alone (for heading-only searches).
--   2. Combined content+heading GIN index (for ranked full-text search).

CREATE INDEX idx_chunks_heading_fts
    ON chunks USING gin(to_tsvector('english', heading))
    WHERE heading IS NOT NULL;

CREATE INDEX idx_chunks_content_heading_fts
    ON chunks USING gin(to_tsvector('english', coalesce(heading, '') || ' ' || content));

-- +goose Down
DROP INDEX IF EXISTS idx_chunks_content_heading_fts;
DROP INDEX IF EXISTS idx_chunks_heading_fts;
