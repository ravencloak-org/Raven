-- +goose Up
-- Issue #192 — Hyperswitch + Razorpay payment integration.
--
-- 1. Enable RLS on subscriptions table (created in 00019).
-- 2. Create payment_events table for idempotent webhook processing.

-- ─── RLS on subscriptions ────────────────────────────────────────────────────

ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON subscriptions
    USING (org_id = nullif(current_setting('app.current_org_id', true), '')::uuid);

CREATE POLICY admin_bypass ON subscriptions
    FOR ALL
    USING (current_setting('app.bypass_rls', true) = 'true');

-- ─── Payment events (idempotent webhook processing) ──────────────────────────

CREATE TABLE IF NOT EXISTS payment_events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_type     TEXT NOT NULL,
    payment_id     TEXT NOT NULL,
    status         TEXT NOT NULL,
    raw_payload    JSONB,
    processed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ensure idempotency: one event per payment_id + event_type pair.
CREATE UNIQUE INDEX idx_payment_events_idempotent
    ON payment_events (payment_id, event_type);

-- RLS on payment_events
ALTER TABLE payment_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON payment_events
    USING (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON payment_events
    FOR ALL
    USING (current_setting('app.bypass_rls', true) = 'true');

-- +goose Down
DROP POLICY IF EXISTS admin_bypass ON payment_events;
DROP POLICY IF EXISTS tenant_isolation ON payment_events;
DROP INDEX IF EXISTS idx_payment_events_idempotent;
DROP TABLE IF EXISTS payment_events;
DROP POLICY IF EXISTS admin_bypass ON subscriptions;
DROP POLICY IF EXISTS tenant_isolation ON subscriptions;
