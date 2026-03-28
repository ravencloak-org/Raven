-- +goose Up
-- Issue #20 — Document Processing State Machine
--
-- The processing_events table (created in 00014) records state transitions for
-- documents and sources. This migration adds a composite index to support
-- efficient timeline queries by document and chronological order.
--
-- Valid state machine transitions for documents:
--   queued       -> crawling | failed
--   crawling     -> parsing  | failed
--   parsing      -> chunking | failed
--   chunking     -> embedding | failed
--   embedding    -> ready    | failed
--   failed       -> queued   | reprocessing
--   ready        -> reprocessing
--   reprocessing -> crawling | failed

CREATE INDEX IF NOT EXISTS idx_processing_events_doc_timeline
    ON processing_events (document_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_processing_events_source_timeline
    ON processing_events (source_id, created_at ASC);

-- +goose Down
DROP INDEX IF EXISTS idx_processing_events_source_timeline;
DROP INDEX IF EXISTS idx_processing_events_doc_timeline;
