-- migrations/00036_query_cache.sql
-- Issue #256 — Semantic response cache (M9).
--
-- Extends response_cache (00027) with a JSONB metadata slot and a 7-day
-- default TTL, plus adds per-KB cache knobs.
--
-- NO TRANSACTION: the new composite index uses CONCURRENTLY so production
-- tables aren't write-locked during migration. That rules out goose's
-- default BEGIN wrapper. Every DDL below is idempotent (IF NOT EXISTS /
-- IF EXISTS), so a partial failure can be re-run safely.

-- +goose NO TRANSACTION

-- +goose Up
ALTER TABLE response_cache
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';

ALTER TABLE response_cache
    ALTER COLUMN expires_at SET DEFAULT (NOW() + INTERVAL '7 days');

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_response_cache_kb_expires
    ON response_cache (kb_id, expires_at);

ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS cache_enabled BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS cache_similarity_threshold REAL NOT NULL DEFAULT 0.92
    CHECK (cache_similarity_threshold >= 0.80 AND cache_similarity_threshold <= 0.99);

-- +goose Down
ALTER TABLE knowledge_bases
    DROP COLUMN IF EXISTS cache_similarity_threshold;
ALTER TABLE knowledge_bases
    DROP COLUMN IF EXISTS cache_enabled;

DROP INDEX CONCURRENTLY IF EXISTS idx_response_cache_kb_expires;

ALTER TABLE response_cache
    ALTER COLUMN expires_at SET DEFAULT (NOW() + INTERVAL '1 hour');

ALTER TABLE response_cache
    DROP COLUMN IF EXISTS metadata;
