-- +goose Up
-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";       -- pgvector
CREATE EXTENSION IF NOT EXISTS "pg_trgm";      -- trigram similarity

-- Create custom ENUM types
CREATE TYPE org_status AS ENUM ('active', 'suspended', 'deactivated');
CREATE TYPE user_status AS ENUM ('active', 'disabled');
CREATE TYPE workspace_role AS ENUM ('owner', 'admin', 'member', 'viewer');
CREATE TYPE kb_status AS ENUM ('active', 'archived');
CREATE TYPE processing_status AS ENUM ('queued', 'crawling', 'parsing', 'chunking', 'embedding', 'ready', 'failed', 'reprocessing');
CREATE TYPE source_type AS ENUM ('url', 'sitemap', 'rss_feed');
CREATE TYPE crawl_frequency AS ENUM ('manual', 'daily', 'weekly', 'monthly');
CREATE TYPE chunk_type AS ENUM ('text', 'table', 'image_caption', 'code');
CREATE TYPE llm_provider AS ENUM ('openai', 'anthropic', 'cohere', 'google', 'azure_openai', 'custom');
CREATE TYPE provider_status AS ENUM ('active', 'revoked', 'expired');
CREATE TYPE api_key_status AS ENUM ('active', 'revoked');
CREATE TYPE message_role AS ENUM ('user', 'assistant', 'system');

-- +goose Down
DROP TYPE IF EXISTS message_role;
DROP TYPE IF EXISTS api_key_status;
DROP TYPE IF EXISTS provider_status;
DROP TYPE IF EXISTS llm_provider;
DROP TYPE IF EXISTS chunk_type;
DROP TYPE IF EXISTS crawl_frequency;
DROP TYPE IF EXISTS source_type;
DROP TYPE IF EXISTS processing_status;
DROP TYPE IF EXISTS kb_status;
DROP TYPE IF EXISTS workspace_role;
DROP TYPE IF EXISTS user_status;
DROP TYPE IF EXISTS org_status;
DROP EXTENSION IF EXISTS "pg_trgm";
DROP EXTENSION IF EXISTS "vector";
DROP EXTENSION IF EXISTS "uuid-ossp";
