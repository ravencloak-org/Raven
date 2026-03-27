# Deployment Guide

> **Status: Coming Soon.** The Docker Compose configuration is in active development. This page outlines the planned deployment architecture.

## Docker Compose Services

| Service | Image | Exposed |
|---------|-------|---------|
| go-api | Custom build | Yes (:8080) |
| python-worker | Custom build | No |
| strapi | Custom build | Yes (:1337) |
| keycloak | quay.io/keycloak/keycloak | Yes (:8443) |
| postgres | pgvector/pgvector:pg18 + ParadeDB | No |
| valkey | valkey/valkey:8-alpine | No |
| seaweedfs | chrislusf/seaweedfs | No |
| traefik | traefik:3 | Yes (:80/:443) |

## Network Topology

```
External (host-exposed):
  go-api :8080, strapi :1337, keycloak :8443, traefik :80/:443

Internal (raven-internal bridge):
  postgres, valkey, python-worker, seaweedfs
```

## Environment Configuration

- `.env` — non-secret defaults (POSTGRES_DB, KEYCLOAK_REALM)
- `.env.secrets` (git-ignored) — credentials (POSTGRES_PASSWORD, JWT_SECRET)
- `raven init` CLI command scaffolds `.env.secrets` interactively

## Edge Deployment Mode

For Raspberry Pi / ARM64 / small VPS:

| What runs on edge | What runs remotely |
|-------------------|--------------------|
| Go API (~10MB binary, 5MB RAM) | Python AI Worker |
| Traefik | PostgreSQL + pgvector |
| | Valkey |
| | SeaweedFS |

The Go API connects to the remote Python worker via gRPC over the network.

**Minimum edge hardware:** Raspberry Pi 4 (2GB RAM), 8GB SD card

## Quick Start (TBD)

```bash
# Coming soon
git clone https://github.com/ravencloak-org/Raven.git
cd Raven
cp .env.example .env
# Edit .env with your settings
docker compose up -d
```
