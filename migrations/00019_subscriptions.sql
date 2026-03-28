-- +goose Up
-- Issue #50 — Hyperswitch payment orchestration: subscriptions table.
--
-- Tracks organisation billing subscriptions managed via Hyperswitch.

CREATE TABLE IF NOT EXISTS subscriptions (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id                     TEXT NOT NULL,
    status                      TEXT NOT NULL DEFAULT 'active'
                                    CHECK (status IN ('active','canceled','past_due','trialing','paused','expired')),
    hyperswitch_subscription_id TEXT,
    current_period_start        TIMESTAMPTZ NOT NULL DEFAULT now(),
    current_period_end          TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '1 month'),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One active subscription per organisation.
CREATE UNIQUE INDEX idx_subscriptions_org_active
    ON subscriptions (org_id)
    WHERE status IN ('active', 'trialing', 'past_due');

-- Look-up by Hyperswitch ID for webhook processing.
CREATE INDEX idx_subscriptions_hyperswitch_id
    ON subscriptions (hyperswitch_subscription_id)
    WHERE hyperswitch_subscription_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_subscriptions_hyperswitch_id;
DROP INDEX IF EXISTS idx_subscriptions_org_active;
DROP TABLE IF EXISTS subscriptions;
