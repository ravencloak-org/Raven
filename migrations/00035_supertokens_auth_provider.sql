-- migrations/00035_supertokens_auth_provider.sql
-- +goose Up
-- +goose StatementBegin

-- Update auth_provider default and existing rows from 'zitadel' to 'supertokens'.
ALTER TABLE users ALTER COLUMN auth_provider SET DEFAULT 'supertokens';
UPDATE users SET auth_provider = 'supertokens' WHERE auth_provider = 'zitadel';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users ALTER COLUMN auth_provider SET DEFAULT 'zitadel';
UPDATE users SET auth_provider = 'zitadel' WHERE auth_provider = 'supertokens';

-- +goose StatementEnd
