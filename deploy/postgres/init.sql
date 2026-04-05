-- Raven PostgreSQL initialisation
-- Runs once when the data volume is first created.

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
