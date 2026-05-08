-- migrations/00038_add_local_singleuser_seed.sql
-- +goose Up
-- +goose StatementBegin

-- Idempotent seed for single-user (Raven Local / desktop) mode.
-- Safe to apply on multi-user deployments — only inserts if rows do not exist.
-- The fixed UUIDs are reserved for the local persona and never created by the
-- application in multi-user mode.

INSERT INTO organizations (id, name, slug, status, settings, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Local',
    'local',
    'active',
    '{}',
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, org_id, email, display_name, external_id, auth_provider, status, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    '00000000-0000-0000-0000-000000000001',
    'local@raven.localhost',
    'Local User',
    'local',
    'single_user',
    'active',
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- Intentionally empty — removing the local user/org could break in-flight data
-- and is unnecessary even when rolling back to multi-user mode.
