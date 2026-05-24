-- +goose Up
-- Intentionally violates naming.column (PascalCase).
CREATE TABLE bad_naming (
    id            uuid PRIMARY KEY,
    UserEmail     text NOT NULL,
    CreatedAt     timestamptz NOT NULL DEFAULT now()
);
