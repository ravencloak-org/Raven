# Raven — Project Overview

## Purpose
Open-source multi-tenant knowledge base platform with AI-powered chat, voice, and WhatsApp integration. Enables organizations to build and deploy RAG-based chatbots over their documents.

## Architecture
Two-process architecture:
- **Go API server** (`cmd/api/main.go`): HTTP routing, JWT auth, tenant routing, SSE streaming
- **Python AI worker** (`ai-worker/`): RAG queries, embeddings, document parsing, web scraping (communicates via gRPC)
- PostgreSQL as single source of truth (relational + pgvector + BM25 FTS)
- Vue.js SPA admin dashboard
- Keycloak for identity management

## Tech Stack
| Layer | Technology |
|-------|-----------|
| API Server | Go + Gin |
| AI Worker | Python + gRPC |
| Database | PostgreSQL 18 + pgvector |
| Frontend | Vue.js 3 + Tailwind CSS |
| Chatbot Widget | Web Component (`<raven-chat>`) |
| Auth | Keycloak (OIDC/OAuth2, multi-tenant realms) |
| Job Queue | Valkey (Redis fork) |
| Object Storage | SeaweedFS (S3-compatible) |
| Voice | LiveKit Server + Agents (WebRTC) |
| Reverse Proxy | Traefik (Auto-TLS) |
| Payments | Hyperswitch (Razorpay/UPI compatible) |

## Repository Structure
```text
cmd/api/        — Go API entrypoint
cmd/worker/     — Go worker entrypoint
internal/       — Go internal packages
pkg/            — Go shared packages
migrations/     — DB migrations (up + down required)
ai-worker/      — Python AI worker (gRPC server)
frontend/       — Vue.js 3 SPA
deploy/
  ec2/          — EC2 Docker Compose deployment
  edge/         — Raspberry Pi / ARM64 edge deployment
  ansible/      — Ansible roles for server setup
  hyperswitch/  — Payment orchestration local dev
docs/
  swagger/      — OpenAPI specs
  wiki/         — Architecture, API reference, data model
  superpowers/  — Plans and specs
contracts/      — OpenAPI stub
```

## Deployment Targets
- **EC2**: Secondary Docker daemon, Cloudflare Tunnel (no exposed ports)
- **Edge**: Raspberry Pi / ARM64, minimal stack (Go API + PostgreSQL + Traefik)
- **Frontend**: Cloudflare Pages
