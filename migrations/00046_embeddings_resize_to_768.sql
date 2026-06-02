-- +goose Up
-- Resize embeddings.embedding from vector(1536) to vector(768) to match
-- the dimension of nomic-embed-text — Ollama's de-facto default
-- embedding model and the only ~free local option for the Raven demo.
-- The table is currently empty so a drop+readd is safe; preserving the
-- HNSW index requires recreating it because pgvector indexes are bound
-- to a specific dimension.
--
-- Other supported embedding models and their dimensions, for the day
-- someone needs to retune:
--   text-embedding-3-small        1536
--   text-embedding-3-large        3072
--   mxbai-embed-large (Ollama)    1024
--   nomic-embed-text (Ollama)      768  ← we're standardising here
--
-- Going forward the same constant lives in:
--   ai-worker/raven_worker/providers/ollama_provider.py:_DEFAULT_DIMENSIONS
--   ai-worker/raven_worker/providers/registry.py:_DEFAULT_EMBEDDING_MODELS
--   internal/jobs/document_process.go (when the embed call lands)

DROP INDEX IF EXISTS idx_embeddings_hnsw;

ALTER TABLE embeddings DROP COLUMN embedding;
ALTER TABLE embeddings ADD COLUMN embedding vector(768) NOT NULL;

-- Recreate the HNSW cosine index at the new dimension. The m/ef_construction
-- params mirror the original (m=16, ef_construction=64) which are the
-- pgvector defaults for "good recall, modest build time".
CREATE INDEX idx_embeddings_hnsw ON embeddings
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- +goose Down
DROP INDEX IF EXISTS idx_embeddings_hnsw;
ALTER TABLE embeddings DROP COLUMN embedding;
ALTER TABLE embeddings ADD COLUMN embedding vector(1536) NOT NULL;
CREATE INDEX idx_embeddings_hnsw ON embeddings
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
