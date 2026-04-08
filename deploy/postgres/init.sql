-- Raven PostgreSQL initialisation
-- Runs once when the data volume is first created.

-- Keycloak needs its own database
SELECT 'CREATE DATABASE keycloak'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'keycloak')\gexec

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Voice session enums (added in migration 00029)
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'voice_session_state') THEN
        CREATE TYPE voice_session_state AS ENUM ('created', 'active', 'ended');
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'voice_speaker') THEN
        CREATE TYPE voice_speaker AS ENUM ('agent', 'user');
    END IF;
END $$;

-- WhatsApp-LiveKit bridge table (added for issue #67)
CREATE TABLE IF NOT EXISTS whatsapp_bridges (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id           UUID NOT NULL,
    call_id          TEXT NOT NULL,
    livekit_room     TEXT NOT NULL,
    bridge_state     TEXT NOT NULL DEFAULT 'initializing',
    voice_session_id UUID,
    sdp_offer        TEXT NOT NULL DEFAULT '',
    sdp_answer       TEXT NOT NULL DEFAULT '',
    metadata         TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at        TIMESTAMPTZ,
    UNIQUE (org_id, call_id)
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_bridges_org_state
    ON whatsapp_bridges (org_id, bridge_state)
    WHERE bridge_state IN ('initializing', 'active');
