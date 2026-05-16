-- +goose Up
-- Issue #592 — Add annual billing cycle with 20% discount.
--
-- Adds seat_count (from #591) and billing_cycle to subscriptions.
-- seat_count tracks the number of licensed seats for per-seat pricing.
-- billing_cycle controls the payment cadence; annual gives 2 months free (≈20% off).

ALTER TABLE subscriptions
    ADD COLUMN seat_count    INT  NOT NULL DEFAULT 1,
    ADD COLUMN billing_cycle TEXT NOT NULL DEFAULT 'monthly'
                                 CHECK (billing_cycle IN ('monthly', 'annual'));

-- +goose Down
ALTER TABLE subscriptions
    DROP COLUMN billing_cycle,
    DROP COLUMN seat_count;
