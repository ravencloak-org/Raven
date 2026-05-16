-- +goose Up
-- Issue #596 — Prorated refund on annual subscription cancellation.
--
-- Stores the Hyperswitch refund ID created when an annual subscription is
-- cancelled mid-year, allowing idempotent refund tracking and audit.

ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS refund_id TEXT;

-- +goose Down
ALTER TABLE subscriptions DROP COLUMN IF EXISTS refund_id;
