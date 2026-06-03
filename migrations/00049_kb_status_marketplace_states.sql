-- +goose Up
--
-- Marketplace MVP (issue #725, M1).
--
-- Adds two new values to the kb_status enum so the lifecycle column on
-- knowledge_bases can express the two Marketplace-specific frozen states:
--
--   - read_only_private — Free Plan downgrade lands previously private KBs
--     here per ADR-0004. Reads stay open; ingestion / mutation is blocked.
--     The user must Publish, Delete, or upgrade back to thaw the KB.
--
--   - dmca_pending — DMCA notice received per ADR-0006 / ADR-0008. The KB
--     is in legal hold during the 14-day counter-notice window: reads
--     allowed (the publisher still owns the row) but chat, ingestion, and
--     all surfaces that look like "active use" are blocked.
--
-- Status-aware behaviour lives behind a single policy helper —
-- internal/model.KBStatusGate — so every read/write call site asks the
-- gate, not the enum literal. See ADR-0004 §Consequences and ADR-0008
-- §Consequences for the surface contracts.
--
-- !! ROLLBACK LIMITATION !!
--
-- Postgres does NOT support `ALTER TYPE … DROP VALUE`. The values added
-- below are permanent for the lifetime of the kb_status type — `goose
-- down` to this migration cannot actually remove them.
--
-- The Down half therefore takes the only sane fallback: rewrite any row
-- still sitting in one of the new states back to 'active'. That restores
-- the pre-migration invariant (every kb_status value the application sees
-- is one the application code knows about) without pretending the enum
-- values themselves can disappear. The enum stays widened; the data shape
-- the previous app version expected is reinstated.
--
-- A real removal of the values requires dropping and recreating the type
-- — out of scope for a rollback and dangerous on a populated table.

ALTER TYPE kb_status ADD VALUE IF NOT EXISTS 'read_only_private';
ALTER TYPE kb_status ADD VALUE IF NOT EXISTS 'dmca_pending';

-- +goose Down
--
-- Cannot drop enum values (Postgres limitation, see header). Instead,
-- normalise any row sitting in one of the new states back to 'active' so
-- the pre-migration application code does not crash on an unknown enum.
UPDATE knowledge_bases
SET status = 'active'
WHERE status IN ('read_only_private', 'dmca_pending');
