# Raven Quick Start

Get a local Raven instance running in under five minutes.

## Prerequisites

- Docker and Docker Compose v2
- Git

## Steps

### 1. Clone the repository

```bash
git clone https://github.com/ravencloak-org/Raven.git
cd Raven
```

### 2. Configure the environment

Copy the example environment file and fill in the required values:

```bash
cp .env.example .env
```

At minimum, set:

| Variable | Description |
|---|---|
| `RAVEN_DATABASE_URL` | PostgreSQL connection URL |
| `RAVEN_KEYCLOAK_ISSUER_URL` | Keycloak issuer URL (e.g. `http://localhost:8080/realms/raven`) |
| `RAVEN_ENCRYPTION_AES_KEY` | 32-byte AES key for encrypting LLM API keys |

### 3. Start all services

```bash
docker compose up -d
```

This starts the Go API, Python AI worker, PostgreSQL, Keycloak, Valkey, and SeaweedFS.

Database migrations are applied automatically on first start.

### 4. Access the platform

Open [http://localhost:3000](http://localhost:3000) in your browser.

On first login, Raven automatically provisions your Keycloak realm and walks you through the onboarding wizard to:
1. Set up your first workspace and knowledge base
2. Connect an LLM provider (OpenAI, Anthropic, etc.)
3. Test your chatbot in the sandbox

### 5. Upload your first document

From the dashboard, navigate to your workspace knowledge base and click **Upload** to add a PDF, Word document, or plain text file.

## Useful commands

```bash
# View logs
docker compose logs -f api

# Stop all services
docker compose down

# Reset the database (destroys all data)
docker compose down -v
```
