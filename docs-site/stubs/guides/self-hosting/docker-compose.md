---
title: "Docker Compose"
---

# Docker Compose

Raven ships four Compose files. The main file (`docker-compose.yml`) brings up
the full self-hosted stack — Go API, Python AI worker, voice agent, Postgres,
Valkey, ClickHouse, SeaweedFS, SuperTokens, LiveKit, Traefik, OpenObserve and
the pgBackRest sidecar. Two overlays opt into eBPF and a schema viewer; a
separate, smaller file targets ARM64 edge nodes such as a Raspberry Pi.

The frontend is **not** a Compose service — it deploys to Cloudflare Pages.
Compose serves the API and the platform; the SPA points at the API over
Traefik.

For the canonical first-run path, see
[Installation](/get-started/installation). This page is the deeper reference
once you know what each container is doing.

## Variants

| File | Services | Use case | Footprint |
|------|----------|----------|-----------|
| [`docker-compose.yml`](https://github.com/ravencloak-org/Raven/blob/main/docker-compose.yml) | 14 | Cloud / full self-hosted | ~3 GB RAM, ~6 GB disk after seeding |
| [`docker-compose.edge.yml`](https://github.com/ravencloak-org/Raven/blob/main/docker-compose.edge.yml) | 3 (Go API + Postgres + Traefik) | Raspberry Pi / ARM64 edge | ~1 GB RAM (memory limits enforced via `deploy.resources`) |
| [`docker-compose.ebpf.yml`](https://github.com/ravencloak-org/Raven/blob/main/docker-compose.ebpf.yml) | overlay (`go-api` only) | Adds `CAP_BPF` + `CAP_NET_ADMIN` for eBPF audit/XDP | negligible |
| [`docker-compose.chartdb.yml`](https://github.com/ravencloak-org/Raven/blob/main/docker-compose.chartdb.yml) | overlay (1 service: `chartdb`) | Browse the live Postgres schema at `127.0.0.1:3000` | ~150 MB |

Overlays compose in the usual way:

```bash
docker compose -f docker-compose.yml -f docker-compose.ebpf.yml up -d
docker compose -f docker-compose.yml -f docker-compose.chartdb.yml up -d chartdb
```

## Prerequisites

- **Docker** 24+ with **Compose v2** (the bundled `docker compose` plugin, not
  the legacy `docker-compose` Python script).
- **4 GB RAM** minimum for the full stack. ClickHouse, Postgres and the AI
  worker dominate; Valkey is capped at `--maxmemory 256mb` in the compose file.
- **An x86_64 host** for the main file. `docker-compose.edge.yml` pins
  `platform: linux/arm64` for Raspberry Pi 4/5 class devices.
- A **domain name** if you want real TLS via Traefik's ACME resolver (see
  [Traefik & TLS](/guides/self-hosting/traefik-and-tls)). For local
  development, leave `RAVEN_DOMAIN` unset and Traefik routes on `localhost`.

The build layer pins exact base images by digest:

| Component | Base image |
|-----------|------------|
| Go API ([`Dockerfile`](https://github.com/ravencloak-org/Raven/blob/main/Dockerfile)) | `golang:1.26.3-alpine` (build) → `alpine` (runtime) |
| AI worker ([`ai-worker/Dockerfile`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/Dockerfile)) | `python:3.14-slim` |
| Frontend ([`frontend/Dockerfile`](https://github.com/ravencloak-org/Raven/blob/main/frontend/Dockerfile)) | `node:22-alpine` (build) → `nginxinc/nginx-unprivileged:1.27-alpine` (serve). Used in CI; production deploys to Cloudflare Pages. |

Secrets are decrypted at container start by `dotenvx` (currently `v1.65.0`,
hash-pinned across all four install sites). The Go and Python images both run
as non-root UID 1000.

## Initial setup

```bash
git clone https://github.com/ravencloak-org/Raven.git
cd Raven
cp .env.example .env
```

Open `.env` and fill in, at minimum:

| Variable | Source / purpose |
|----------|------------------|
| `POSTGRES_PASSWORD` | Required — Postgres rejects start without it (`${POSTGRES_PASSWORD:?...}` in compose). |
| `CLICKHOUSE_PASSWORD` | Required — same enforcement on the ClickHouse service. |
| `RAVEN_ENCRYPTION_AES_KEY` | 32-byte hex key for at-rest encryption. The Go API refuses to start without it. |
| `SUPERTOKENS_API_KEY` | Shared secret between Go API and SuperTokens core; defaults to a placeholder you must replace. |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | OAuth credentials for SuperTokens social login. |
| `ZO_ROOT_USER_PASSWORD` | OpenObserve admin password — required (`${ZO_ROOT_USER_PASSWORD:?...}`). |
| One of `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `COHERE_API_KEY` | At least one BYOK LLM key for the AI worker. |
| `RAVEN_HYPERSWITCH_API_KEY`, `RAVEN_HYPERSWITCH_WEBHOOK_SECRET`, `RAVEN_HYPERSWITCH_RAZORPAY_KEY_ID` | Payments orchestrator keys (UPI / cards via Razorpay). Leave blank to disable billing. |

Then start the stack:

```bash
docker compose up -d
docker compose ps
docker compose logs -f go-api
```

Postgres runs `deploy/postgres/init.sql` on first boot. ClickHouse runs
`deploy/clickhouse/init.sql`. The Go API waits on Postgres, Valkey,
SuperTokens and OpenObserve health before serving traffic; you can watch the
gating with `docker compose ps`.

The full env-var contract is parsed in
[`internal/config/config.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/config/config.go)
(Viper, `RAVEN_` prefix, dot-to-underscore key replacement). See the
[Configuration reference](/reference/configuration) for the resolved list.

## Service map

All services attach to the `raven-internal` bridge network. Only Traefik,
Postgres-via-pgBackRest, OpenObserve, the SeaweedFS S3 filer and LiveKit
publish ports on the host.

### API & worker tier

| Service | Image | Host port | Role |
|---------|-------|-----------|------|
| `go-api` | built from `./Dockerfile` | `8081` | Gin HTTP API; runs gRPC client to AI worker; serves `/healthz`, `/api/*` (Traefik path-prefix routing). |
| `python-worker` | built from `./ai-worker/Dockerfile` | (internal) | gRPC service on port `50051`; parsing, embeddings, RAG, summaries. Mounts `raven-memories` volume at `/var/raven/memories`. |
| `python-agent` | built from `./ai-worker/Dockerfile` | (internal) | LiveKit voice agent; same image, command `python -m raven_worker.agent`. Connects to `livekit-server:7880`. |

### Auth tier

| Service | Image | Host port | Role |
|---------|-------|-----------|------|
| `supertokens` | `registry.supertokens.io/supertokens/supertokens-postgresql` | (internal `3567`) | Auth core. Persists to the same Postgres instance. Telemetry disabled. |

### Data tier

| Service | Image | Host port | Role |
|---------|-------|-----------|------|
| `postgres` | `pgvector/pgvector:pg18` | (internal `5432`) | Primary OLTP + pgvector + BM25. Started with `archive_mode=on` and `archive_command=pgbackrest --stanza=raven archive-push`. |
| `clickhouse` | `clickhouse/clickhouse-server:25` | `127.0.0.1:9000`, `127.0.0.1:8123` | Enterprise-tier vector + analytics store. Bound to loopback only. |
| `valkey` | `valkey/valkey:8.1-alpine` | (internal `6379`) | Redis-compatible cache; `--maxmemory 256mb --maxmemory-policy allkeys-lru`. |
| `seaweedfs-master` | `chrislusf/seaweedfs` | (internal) | SeaweedFS master. |
| `seaweedfs-volume` | `chrislusf/seaweedfs` | (internal) | Object storage volumes (`seaweedfs-data` volume). |
| `seaweedfs-filer` | `chrislusf/seaweedfs` | `8888` | S3-compatible filer for blob uploads. |
| `pgbackrest` | `pgbackrest/pgbackrest:latest` | (internal) | Sidecar; `command: ["sleep", "infinity"]` so backups run via `docker compose exec`. |

### Voice tier

| Service | Image | Host port | Role |
|---------|-------|-----------|------|
| `livekit-server` | `livekit/livekit-server:latest` | `7882/tcp`, `50000-60000/udp` | WebRTC SFU. Config mounted from `deploy/livekit/livekit.yaml`. |

### Edge / infra tier

| Service | Image | Host port | Role |
|---------|-------|-----------|------|
| `traefik` | `traefik:v3.3` | `80`, `443`, `8082` (dashboard) | Reverse proxy; ACME via DNS-01 (default provider Cloudflare). Discovers services via `traefik.enable=true` labels. |
| `openobserve` | `public.ecr.aws/zinclabs/openobserve:latest` | `5080` (UI), `5081` (OTLP gRPC) | Logs / metrics / traces. The Go API ships OTLP to `openobserve:5081`. |

The Vue frontend is served from Cloudflare Pages and points at
`https://api.<RAVEN_DOMAIN>` over Traefik. There is no `frontend` service in
`docker-compose.yml`; the Dockerfile under `frontend/` is used by CI and
self-hosters who want to run the SPA themselves.

## Health checks

Compose-defined health is wired into `depends_on: condition: service_healthy`,
so `go-api` only starts once its dependencies report healthy.

| Service | Probe | Interval / timeout / retries |
|---------|-------|------------------------------|
| `go-api` | `wget --spider http://localhost:8081/healthz` | 10s / 5s / 5 (start_period 10s) |
| `postgres` | `pg_isready -U $POSTGRES_USER -d $POSTGRES_DB` | 5s / 5s / 5 |
| `valkey` | `valkey-cli ping` | 5s / 3s / 5 |
| `clickhouse` | `clickhouse-client --query 'SELECT 1'` | 10s / 5s / 5 (start_period 10s) |
| `supertokens` | `curl -sf http://localhost:3567/hello` | 10s / 5s / 5 (start_period 10s) |
| `livekit-server` | `wget --spider http://localhost:7880` | 10s / 5s / 5 (start_period 5s) |

The Go runtime image also declares its own `HEALTHCHECK` in `Dockerfile` so
orchestrators that don't read the compose file (e.g. plain `docker run`) still
get a probe.

`go-api`'s `depends_on` block:

```yaml
depends_on:
  postgres:
    condition: service_healthy
  valkey:
    condition: service_healthy
  supertokens:
    condition: service_healthy
  openobserve:
    condition: service_started
```

OpenObserve uses `service_started` (not `service_healthy`) so a misconfigured
observability backend can't block the API from booting.

## Common workflows

```bash
# Tail the API
docker compose logs -f go-api

# Tail several services at once
docker compose logs -f go-api python-worker

# Open a psql shell on the live database
docker compose exec postgres psql -U raven -d raven

# Run an on-demand pgBackRest backup
docker compose exec pgbackrest pgbackrest --stanza=raven backup

# Rebuild after a code change
docker compose up -d --build go-api

# Stop the stack but keep volumes
docker compose down

# DESTRUCTIVE — drops all volumes (Postgres, ClickHouse, SeaweedFS, OpenObserve, ACME certs)
docker compose down -v
```

`docker compose down -v` deletes `pg-data`, `clickhouse-data`,
`seaweedfs-data`, `valkey-data`, `openobserve-data`, `acme-data`,
`pgbackrest-repo`, `pgbackrest-log` and `raven-memories`. Run a backup first.

## Edge deployment

For Raspberry Pi / ARM64 single-node deployments, use `docker-compose.edge.yml`.
It strips the stack down to three services — Go API, Postgres, Traefik — and
expects the AI worker to run remotely (via gRPC at `GRPC_AI_WORKER_ADDR`):

```bash
cp deploy/edge/.env.example .env.edge
# edit .env.edge — at minimum set DATABASE_URL, GRPC_AI_WORKER_ADDR,
#                   POSTGRES_PASSWORD, RAVEN_ENCRYPTION_AES_KEY
docker compose -f docker-compose.edge.yml --env-file .env.edge up -d
```

Differences from the main file:

- `platform: linux/arm64` pinned on every service.
- `deploy.resources.limits.memory` set per service (Go API 256M, Postgres
  512M, Traefik 128M).
- Network is `raven-edge` (not `raven-internal`).
- `RAVEN_EDGE_MODE=true` is exported into the Go API; behaviour switches
  documented in `internal/config/config.go`.
- No SuperTokens, ClickHouse, Valkey, SeaweedFS, OpenObserve, LiveKit or
  pgBackRest — those run elsewhere or are disabled.

To enable eBPF audit on edge, layer the eBPF overlay:

```bash
docker compose -f docker-compose.edge.yml -f docker-compose.ebpf.yml up -d
```

The overlay adds `CAP_BPF` and `CAP_NET_ADMIN` to `go-api` and sets
`RAVEN_EBPF_ENABLED=true`. Requires kernel ≥ 5.8. Drop `CAP_NET_ADMIN` if
`RAVEN_EBPF_XDP_ENABLED=false`.

See [Edge & Raspberry Pi](/guides/self-hosting/edge-and-raspberry-pi) for the
hardware sizing and provisioning walkthrough.

## Troubleshooting

**`postgres` won't start: `POSTGRES_PASSWORD is required`** — the value uses
the strict `${POSTGRES_PASSWORD:?...}` form. Fill `.env` before
`docker compose up`. Same applies to `CLICKHOUSE_PASSWORD`,
`RAVEN_ENCRYPTION_AES_KEY` and `ZO_ROOT_USER_PASSWORD`.

**`pgbackrest` exits immediately or write errors** — the sidecar mounts
`pgbackrest-repo` at `/var/lib/pgbackrest` and `pgbackrest-log` at
`/var/log/pgbackrest`. Both are named volumes; if you bind-mount over them
make sure the path is writable by the `pgbackrest` user. The Postgres archive
command pipes WAL through `pgbackrest --stanza=raven archive-push %p`, so
broken volumes will start failing WAL archival within minutes — watch
`docker compose logs postgres`.

**LiveKit voice doesn't connect** — the SFU needs `7882/tcp` and the full
`50000-60000/udp` range reachable from clients. NAT and firewalls drop UDP
silently; verify with `docker compose port livekit-server 7882` and a UDP
probe. The agent also needs a real `LIVEKIT_API_KEY` / `LIVEKIT_API_SECRET`
pair — the compose default `devkey:devsecret` is for local dev only.

**Traefik returns 404 for `/api`** — the router rule is
``Host(`${RAVEN_DOMAIN}`) && PathPrefix(`/api`)``. If `RAVEN_DOMAIN` is unset
it falls back to `localhost`, so `curl https://example.com/api/...` will miss
the router. Set `RAVEN_DOMAIN` in `.env` and recreate Traefik.

**OpenTelemetry exporter errors at boot** — the Go API ships OTLP to
`openobserve:5081`. If OpenObserve is slow to start, the API logs warnings
but continues (because of `condition: service_started`). Persistent failures
usually mean `ZO_GRPC_PORT` was overridden away from `5081` in `.env`.

**ChartDB "external network not found"** — `docker-compose.chartdb.yml`
attaches to `raven_raven-internal`, the auto-named network from the main
file. Bring the main stack up first, then add the overlay:

```bash
docker compose up -d
docker compose -f docker-compose.yml -f docker-compose.chartdb.yml up -d chartdb
```

Then visit `http://127.0.0.1:3000`, choose PostgreSQL, and paste the SQL
output of `scripts/chartdb-query.sh`.

## See also

- [Installation](/get-started/installation) — first-run path
- [Configuration reference](/reference/configuration) — every `RAVEN_*` env
  var
- [Traefik & TLS](/guides/self-hosting/traefik-and-tls) — ACME, DNS-01,
  Cloudflare provider
- [Edge & Raspberry Pi](/guides/self-hosting/edge-and-raspberry-pi) — ARM64
  hardware notes
- [Backups](/guides/self-hosting/backups) — pgBackRest stanza, restore drill
- [Observability](/guides/self-hosting/observability) — OpenObserve + Beszel
