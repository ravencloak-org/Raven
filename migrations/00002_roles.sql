-- +goose Up
-- Application roles

-- +goose StatementBegin
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'raven_app') THEN
    CREATE ROLE raven_app;
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'raven_admin') THEN
    CREATE ROLE raven_admin;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP ROLE IF EXISTS raven_admin;
DROP ROLE IF EXISTS raven_app;
