-- migrations/00034_zitadel_migration.sql
-- +goose Up
-- +goose StatementBegin

-- Rename keycloak_sub to external_id in users table
ALTER TABLE users RENAME COLUMN keycloak_sub TO external_id;
ALTER TABLE users ADD COLUMN auth_provider TEXT NOT NULL DEFAULT 'zitadel';
DROP INDEX IF EXISTS idx_users_keycloak_sub;
CREATE UNIQUE INDEX idx_users_external_id ON users(external_id) WHERE external_id IS NOT NULL;

-- Make org_id nullable for pre-onboarding users
ALTER TABLE users ALTER COLUMN org_id DROP NOT NULL;

-- Drop Keycloak-specific column from organizations
ALTER TABLE organizations DROP COLUMN IF EXISTS keycloak_realm;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE organizations ADD COLUMN keycloak_realm TEXT;
ALTER TABLE users ALTER COLUMN org_id SET NOT NULL;
DROP INDEX IF EXISTS idx_users_external_id;
ALTER TABLE users DROP COLUMN IF EXISTS auth_provider;
ALTER TABLE users RENAME COLUMN external_id TO keycloak_sub;
CREATE INDEX idx_users_keycloak_sub ON users(keycloak_sub) WHERE keycloak_sub IS NOT NULL;

-- +goose StatementEnd
