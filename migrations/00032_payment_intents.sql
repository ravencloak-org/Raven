-- +goose Up
-- Issue #192 — Hyperswitch + Razorpay payment integration: payment_intents table + subscriptions RLS.
--
-- Tracks one-off payment intents created via Hyperswitch. The hyperswitch_payment_id
-- column is used as an idempotency key so that duplicate webhook events (replays)
-- cannot double-activate a subscription.

CREATE TABLE IF NOT EXISTS payment_intents (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    amount                BIGINT NOT NULL,
    currency              TEXT NOT NULL DEFAULT 'INR',
    status                TEXT NOT NULL DEFAULT 'requires_payment_method'
                              CHECK (status IN (
                                  'requires_payment_method',
                                  'processing',
                                  'succeeded',
                                  'failed',
                                  'canceled'
                              )),
    hyperswitch_payment_id TEXT,
    client_secret          TEXT,
    idempotency_key        TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Unique index on Hyperswitch payment ID for idempotent webhook processing.
CREATE UNIQUE INDEX idx_payment_intents_hs_payment_id
    ON payment_intents (hyperswitch_payment_id)
    WHERE hyperswitch_payment_id IS NOT NULL;

-- Look-up by org for per-tenant queries under RLS.
CREATE INDEX idx_payment_intents_org_id
    ON payment_intents (org_id);

-- RLS: row-level security mirrors other tables — only the owning org can see its records.
ALTER TABLE payment_intents ENABLE ROW LEVEL SECURITY;

CREATE POLICY payment_intents_org_isolation ON payment_intents
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- Backfill RLS on subscriptions (was missing from 00019).
ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;

CREATE POLICY subscriptions_org_isolation ON subscriptions
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

-- +goose Down
DROP POLICY IF EXISTS subscriptions_org_isolation ON subscriptions;
ALTER TABLE subscriptions DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS payment_intents_org_isolation ON payment_intents;
DROP INDEX IF EXISTS idx_payment_intents_hs_payment_id;
DROP INDEX IF EXISTS idx_payment_intents_org_id;
DROP TABLE IF EXISTS payment_intents;
