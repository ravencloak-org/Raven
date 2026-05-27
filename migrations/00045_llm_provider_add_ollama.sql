-- +goose Up
-- +goose NO TRANSACTION
--
-- Add 'ollama' to the llm_provider enum so users can register a local
-- Ollama daemon as a chat provider. The frontend already speaks
-- 'ollama' (LLM provider dialog, PROVIDER_MODELS map) and the Python
-- AI worker already has an OllamaEmbeddingProvider — only the
-- backend's validator + DB enum were missing the value, so any POST
-- /llm-providers with provider=ollama 400'd at validation.
--
-- Postgres requires ALTER TYPE ADD VALUE outside any transaction.
-- IF NOT EXISTS makes the migration idempotent.
ALTER TYPE llm_provider ADD VALUE IF NOT EXISTS 'ollama';

-- +goose Down
-- +goose NO TRANSACTION
--
-- Postgres has no ALTER TYPE ... REMOVE VALUE. Rolling back would
-- require rebuilding the enum and rewriting every column that
-- references it — not worth it for an additive change. No-op down.
SELECT 1;
