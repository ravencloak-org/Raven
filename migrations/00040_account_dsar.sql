-- +goose Up
-- DSAR scheduled deletes — POST /account/delete inserts a row here
-- with a 24h grace window. A separate purge worker (out of scope for
-- this migration) drains the table and applies the cascade decided in
-- docs/superpowers/specs/2026-05-12-public-demo-deployment-design.md §6.
CREATE TABLE IF NOT EXISTS scheduled_deletes (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    run_at      TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '24 hours'
);

CREATE INDEX IF NOT EXISTS scheduled_deletes_run_at_idx
    ON scheduled_deletes (run_at);

-- Retention purge warnings — set by the nightly cron when an account
-- crosses the 23-day inactive threshold. The row's presence (plus the
-- last_login_at on users) tells the cron whether a warning email was
-- already sent, preventing duplicates.
CREATE TABLE IF NOT EXISTS account_purge_warnings (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    warned_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS account_purge_warnings;
DROP TABLE IF EXISTS scheduled_deletes;
