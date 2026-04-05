-- Raven ClickHouse initialisation
-- Creates the embeddings table with QBit index for enterprise vector search.
-- Runs once when the ClickHouse data volume is first created.

CREATE TABLE IF NOT EXISTS embeddings (
    org_id       UUID,
    kb_id        UUID,
    chunk_id     UUID,
    embedding    Array(Float32),
    model_name   String DEFAULT '',
    created_at   DateTime64(3) DEFAULT now64(3),
    INDEX idx_embedding embedding TYPE qbit GRANULARITY 1
) ENGINE = MergeTree()
PARTITION BY org_id
ORDER BY (org_id, kb_id, chunk_id);
