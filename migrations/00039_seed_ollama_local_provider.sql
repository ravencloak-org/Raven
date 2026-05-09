-- migrations/00039_seed_ollama_local_provider.sql
-- +goose Up
-- +goose NO TRANSACTION

-- ALTER TYPE ... ADD VALUE cannot run inside a transaction; that's why this
-- whole file is annotated NO TRANSACTION above. Each statement auto-commits.

-- 1. Extend the llm_provider enum to include 'ollama'. IF NOT EXISTS makes
-- this safe to re-run on databases that already have it.
ALTER TYPE llm_provider ADD VALUE IF NOT EXISTS 'ollama';

-- 2. Defensive UNIQUE index covering (org_id, provider) so the seed below
-- (and any future seed migrations) can use ON CONFLICT cleanly. CREATE
-- INDEX IF NOT EXISTS is idempotent.
CREATE UNIQUE INDEX IF NOT EXISTS llm_provider_configs_org_provider_uniq
    ON llm_provider_configs (org_id, provider);

-- 3. Seed an 'ollama' provider row for the local org (created in 00038).
-- The WHERE EXISTS clause filters this out on multi-user deployments where
-- the local org doesn't exist, so this migration is safe to apply on the
-- cloud SaaS too.
INSERT INTO llm_provider_configs (
    org_id, provider, display_name, api_key_encrypted, api_key_iv,
    api_key_hint, base_url, config, is_default, status, created_by
)
SELECT
    '00000000-0000-0000-0000-000000000001'::uuid,  -- the local org from 00038
    'ollama'::llm_provider,
    'Local Ollama',
    NULL,                                          -- no API key needed
    NULL,
    'local',
    'http://ollama:11434/v1',
    '{"is_local": true}'::jsonb,
    true,
    'active',
    '00000000-0000-0000-0000-000000000002'::uuid   -- the local user from 00038
WHERE EXISTS (
    SELECT 1 FROM organizations
    WHERE id = '00000000-0000-0000-0000-000000000001'::uuid
)
ON CONFLICT (org_id, provider) DO NOTHING;

-- +goose Down

-- Intentionally empty. Removing 'ollama' from the enum would require
-- rewriting the column type and migrating any existing 'ollama' rows;
-- removing the seed row alone would orphan any chat history that
-- referenced it as the default provider.
