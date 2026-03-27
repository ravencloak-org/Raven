# API Reference

> **Coming soon.** API documentation will be auto-generated via swaggo/swag and available at `/api/docs` when the server is running.

## Base URL

```
https://your-raven-instance.com/api/v1/
```

## Authentication

- **Admin API:** Bearer token (JWT from Keycloak OIDC)
- **Chatbot Widget:** API key via `X-API-Key` header (scoped to knowledge base)

## Planned Endpoints

### Organizations
- `GET /api/v1/orgs` -- List organizations
- `POST /api/v1/orgs` -- Create organization
- `GET /api/v1/orgs/:id` -- Get organization
- `PUT /api/v1/orgs/:id` -- Update organization
- `DELETE /api/v1/orgs/:id` -- Delete organization

### Workspaces
- `GET /api/v1/orgs/:orgId/workspaces` -- List workspaces
- `POST /api/v1/orgs/:orgId/workspaces` -- Create workspace
- `GET /api/v1/workspaces/:id` -- Get workspace
- `PUT /api/v1/workspaces/:id` -- Update workspace

### Knowledge Bases
- `GET /api/v1/workspaces/:wsId/knowledge-bases` -- List knowledge bases
- `POST /api/v1/workspaces/:wsId/knowledge-bases` -- Create knowledge base
- `GET /api/v1/knowledge-bases/:id` -- Get knowledge base

### Documents
- `POST /api/v1/knowledge-bases/:kbId/documents` -- Upload document (multipart)
- `GET /api/v1/knowledge-bases/:kbId/documents` -- List documents
- `GET /api/v1/documents/:id/status` -- Get processing status

### Sources (URLs)
- `POST /api/v1/knowledge-bases/:kbId/sources` -- Add URL source
- `GET /api/v1/knowledge-bases/:kbId/sources` -- List sources

### Chat (RAG Query)
- `POST /api/v1/chat/:kbId/completions` -- Chat with knowledge base (SSE streaming)

### API Keys
- `POST /api/v1/knowledge-bases/:kbId/keys` -- Generate API key
- `GET /api/v1/knowledge-bases/:kbId/keys` -- List API keys
- `DELETE /api/v1/keys/:id` -- Revoke API key

### LLM Provider Config
- `POST /api/v1/orgs/:orgId/llm-providers` -- Add LLM provider (BYOK)
- `GET /api/v1/orgs/:orgId/llm-providers` -- List providers
