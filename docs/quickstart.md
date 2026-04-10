# Raven Quickstart

## Prerequisites

- Docker ≥ 24 and Docker Compose v2
- A domain name (for production TLS) — or use `localhost` for local development

## Steps

1. **Clone the repository**

   ```bash
   git clone https://github.com/ravencloak-org/Raven.git
   cd Raven
   ```

2. **Copy the example environment file and fill in your secrets**

   ```bash
   cp .env.example .env
   ```

   Open `.env` and set at minimum:
   - `POSTGRES_PASSWORD` — strong password for PostgreSQL
   - `KEYCLOAK_ADMIN_PASSWORD` — Keycloak admin console password
   - `RAVEN_KEYCLOAK_ADMIN_CLIENT_SECRET` — secret for the `admin-cli` client
   - `RAVEN_ENCRYPTION_AES_KEY` — 32-byte hex key for at-rest encryption
   - `RAVEN_HYPERSWITCH_API_KEY` — payment orchestrator key (or leave blank to skip billing)
   - At least one LLM provider key (`ANTHROPIC_API_KEY` / `OPENAI_API_KEY`)

3. **Start all services**

   ```bash
   docker compose up -d
   ```

   Docker Compose will pull images, run database migrations, and start Keycloak,
   the Go API, the Python AI worker, SeaweedFS, and the Vue frontend.

4. **Open Raven in your browser**

   Navigate to `http://localhost:3000` (or your configured domain).

5. **Sign in**

   Click **Sign in** to be redirected to Keycloak. Register a new account or log in
   with the credentials you set in `.env`.

   On first login the **Onboarding Wizard** will guide you through:
   - Naming your organisation
   - Creating your first Knowledge Base
   - Configuring your LLM provider (BYOK)
   - Sending a test message to verify the chatbot

6. **(Optional) Auto-provision a Keycloak realm**

   If you need to programmatically create a new Keycloak realm (e.g. for
   multi-tenant deployments), call the internal provision endpoint from inside
   the Docker network:

   ```bash
   curl -s -X POST http://raven-api:8080/internal/provision-realm \
     -H 'Content-Type: application/json' \
     -d '{"realm_name": "my-tenant"}'
   ```

   The endpoint returns `200` with the created realm name, or `409` if the realm
   already exists.
