-- +goose NO TRANSACTION
-- +goose Up
-- Resize response_cache.query_embedding from vector(1536) to vector(768) to
-- match the active embedding model (nomic-embed-text via Ollama). The Python
-- worker logs `cache_store_error: 'expected 1536 dimensions, not 768'` on
-- every semantic-cache write until this lines up, so the RAG cache never
-- materialises a single hit on the demo despite the read path being wired.
--
-- The table holds operational data only — the semantic cache for RAG
-- responses — so dropping the column (and the cached rows that depend
-- on its non-null shape) is fine. The next query a user runs rebuilds
-- the row. pgvector does not support ALTER COLUMN TYPE between vector
-- dimensions, so a drop+readd is the canonical path.
--
-- Postgres rejects adding a NOT NULL column to a populated table without
-- a default, and a vector default makes no semantic sense for a cache
-- key. TRUNCATE first so the ADD succeeds on any DB state (empty on the
-- demo today, possibly populated on a self-hosted instance the day this
-- ships). Running outside a transaction lets us use DROP/CREATE INDEX
-- CONCURRENTLY so the migration is safe to replay on a live table.

-- Bound the locks taken by the DDL below so a long-running query cannot
-- block production indefinitely.
SET lock_timeout = '5s';
SET statement_timeout = '5min';

DROP INDEX CONCURRENTLY IF EXISTS idx_response_cache_embedding;

TRUNCATE response_cache;

-- The table is truncated above; the drop+add is the simplest path to
-- change a pgvector dimension. If this migration is ever back-ported to
-- a populated cluster without truncation, replace with an additive
-- column + backfill (see ban-drop-column note from the migration linter).
-- squawk-ignore ban-drop-column
ALTER TABLE response_cache DROP COLUMN query_embedding;
-- squawk-ignore adding-required-field
ALTER TABLE response_cache ADD COLUMN query_embedding vector(768) NOT NULL;

-- Recreate the HNSW cosine index at the new dimension. Same m/ef_construction
-- parameters as the original (pgvector defaults — good recall, modest build).
CREATE INDEX CONCURRENTLY idx_response_cache_embedding ON response_cache
USING hnsw (query_embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- +goose Down
SET lock_timeout = '5s';
SET statement_timeout = '5min';
DROP INDEX CONCURRENTLY IF EXISTS idx_response_cache_embedding;
TRUNCATE response_cache;
-- squawk-ignore ban-drop-column
ALTER TABLE response_cache DROP COLUMN query_embedding;
-- squawk-ignore adding-required-field
ALTER TABLE response_cache ADD COLUMN query_embedding vector(1536) NOT NULL;
CREATE INDEX CONCURRENTLY idx_response_cache_embedding ON response_cache
USING hnsw (query_embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
