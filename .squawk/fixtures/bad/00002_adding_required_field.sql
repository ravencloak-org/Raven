-- +goose Up
-- Intentionally violates adding-required-field: adds a NOT NULL column
-- to an existing table with no default value — would break existing rows.
ALTER TABLE users ADD COLUMN email_verified boolean NOT NULL;
