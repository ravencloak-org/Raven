-- +goose Up
ALTER TABLE subscriptions
  ADD COLUMN trial_ends_at        TIMESTAMPTZ,
  ADD COLUMN grace_period_ends_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE subscriptions
  DROP COLUMN trial_ends_at,
  DROP COLUMN grace_period_ends_at;
