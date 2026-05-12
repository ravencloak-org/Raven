---
title: "Deployment Models"
---

# Deployment Models

Raven is designed to run anywhere from a single Raspberry Pi to a multi-host
AWS deployment. The repository ships three canonical topologies, each driven by
a different Compose file, plus two optional overlays for specialised
development tasks.

The frontend is intentionally **not** part of any compose stack — it is
always shipped to Cloudflare Pages independently of the API. See
[Frontend deployment](#frontend-deployment) below.

## Cloud-hosted full stack

**File:** [`docker-compose.yml`](https://github.com/ravencloak-org/Raven/blob/main/docker-compose.yml)
**Services:** 14
**Best for:** production deployments serving more than ~100 GB of knowledge
base content, multi-tenant SaaS use, anything that needs voice (LiveKit) or
the enterprise vector store (ClickHouse).

This is the reference topology — a single host (or a single Compose project
spread across a small swarm) runs the entire platform. The Go API talks to
every dependency over the private `raven-internal` Docker network; only
Traefik exposes ports 80/443 publicly.

| Service | Image | Role |
|---|---|---|
| `go-api` | built from `Dockerfile` | REST API, SSE streaming, gRPC client |
| `python-worker` | built from `ai-worker/Dockerfile` | RAG, embeddings, parsing (gRPC :50051) |
| `python-agent` | built from `ai-worker/Dockerfile` | LiveKit voice agent (STT/LLM/TTS) |
| `livekit-server` | `livekit/livekit-server:latest` | WebRTC SFU for voice |
| `postgres` | `pgvector/pgvector:pg18` | Relational, pgvector, BM25, RLS |
| `clickhouse` | ClickHouse | Enterprise vector tier (optional path) |
| `valkey` | Valkey (Redis fork) | Asynq jobs, cache, rate-limit counters |
| `supertokens` | SuperTokens core | Auth (`:3567`) |
| `seaweedfs-master` | SeaweedFS master | S3-compatible blob store |
| `seaweedfs-volume` | SeaweedFS volume | Object data plane |
| `seaweedfs-filer` | SeaweedFS filer + S3 gateway | S3 API on `:8888` |
| `traefik` | `traefik:v3.3` | Auto-TLS via ACME DNS-01, routing |
| `openobserve` | OpenObserve | Traces, metrics, logs |
| `pgbackrest` | `pgbackrest/pgbackrest:latest` | WAL archiving + on-demand backup |

The Go API requires `RAVEN_ENCRYPTION_AES_KEY` and connects to SuperTokens at
`http://supertokens:3567` and OpenTelemetry at `openobserve:5081`. The
production deployment on AWS is automated by an Ansible playbook in
[`deploy/ansible/`](https://github.com/ravencloak-org/Raven/tree/main/deploy/ansible)
(roles: `base`, `docker`, `nodejs`, `cloudflared`, `admin-tools`,
`raven-stack`); a one-shot EC2 bootstrap script lives in
[`deploy/ec2/setup.sh`](https://github.com/ravencloak-org/Raven/blob/main/deploy/ec2/setup.sh)
and uses Cloudflare Tunnel (`cloudflared`) instead of opening ports 80/443.

**Minimum host:** 4 vCPU / 8 GB RAM / 50 GB SSD. **Typical host:** AWS
`t4g.xlarge` or `c7g.xlarge` (ARM64) — Raven is built and tested on ARM64.

## Edge / Single-node

**File:** [`docker-compose.edge.yml`](https://github.com/ravencloak-org/Raven/blob/main/docker-compose.edge.yml)
**Services:** 3 (`go-api`, `postgres`, `traefik`)
**Best for:** branch offices, in-store kiosks, on-prem appliances, dev boxes,
anywhere a full Compose stack is overkill or impossible.

The edge variant is intentionally minimal. The Go API binary is statically
linked (`CGO_ENABLED=0`, `-ldflags="-s -w"` — see
[`Makefile.edge`](https://github.com/ravencloak-org/Raven/blob/main/Makefile.edge))
and compiles to roughly **25 MB** for `linux/arm64`. There is no Python
ai-worker, no LiveKit, no SeaweedFS, no SuperTokens, no OpenObserve — the API
talks to a remote ai-worker over gRPC and writes RLS-isolated data to a local
Postgres.

Per-service memory caps declared in the compose file:

| Service | Image | `deploy.resources.limits.memory` |
|---|---|---|
| `go-api` | `ghcr.io/ravencloak-org/raven-api:latest` (ARM64) | 256 MB |
| `postgres` | `pgvector/pgvector:pg18` (ARM64) | 512 MB |
| `traefik` | `traefik:v3.3` (ARM64) | 128 MB |

Total reserved memory ≈ 900 MB — the stack fits comfortably on a Raspberry
Pi 4 (4 GB RAM) or an AWS `t4g.small` (2 GB). The build targets are emitted
by `make -f Makefile.edge build-arm64` (or `build-amd64`); cross-compilation
uses `aarch64-linux-musl-gcc` only when eBPF is enabled.

Bring-up:

```bash
cp deploy/edge/.env.example .env.edge
# edit .env.edge — set DATABASE_URL, GRPC_AI_WORKER_ADDR, POSTGRES_PASSWORD
docker compose -f docker-compose.edge.yml --env-file .env.edge up -d
```

The edge build sets `RAVEN_EDGE_MODE: "true"` and listens on port 8080 (the
full-stack build uses 8081 to coexist with other dev servers).

## Hybrid

The hybrid model is the edge stack pointed at a cloud ai-worker. It is the
recommended topology for users who want sub-100 ms first-token latency on
reads but do not want to host the Python ML runtime themselves.

```
┌─── EDGE (e.g. Raspberry Pi, in-VPC EC2) ───┐    ┌── CLOUD (managed host) ──┐
│  go-api  ──┐                               │    │   python-worker (gRPC)   │
│  postgres ─┤ pgvector + BM25 (local)       │    │   livekit-server         │
│  traefik   │                               │◀──▶│   clickhouse (optional)  │
└────────────┴───────────────────────────────┘    │   openobserve            │
                ▲                                 └──────────────────────────┘
                │ gRPC, mTLS recommended
                └── RAVEN_GRPC_WORKER_ADDR=worker.example.com:50051
```

The edge node points at the cloud ai-worker by setting one environment
variable, sourced from `GRPC_AI_WORKER_ADDR` in the edge env file and exported
to the Go process as **`RAVEN_GRPC_WORKER_ADDR`**:

```yaml
# docker-compose.edge.yml — go-api environment
RAVEN_GRPC_WORKER_ADDR: ${GRPC_AI_WORKER_ADDR:?GRPC_AI_WORKER_ADDR is required}
```

The Go API uses the worker for embeddings, document parsing, and chat
completions only — every other read path (search, KB lookups, auth session
validation) stays on the local Postgres, so a transient cloud outage does not
take the edge offline for already-ingested documents.

Voice is currently cloud-only: the LiveKit SFU and the Python voice agent both
need to be reachable from the user's browser, and Raven does not ship an edge
LiveKit topology.

## When to pick which

| Concern | Cloud | Edge | Hybrid |
|---|---|---|---|
| Latency-sensitive (single-region users) | Good | **Best** | **Best** |
| Cost-sensitive (small org, < 10 GB KB) | Overkill | **Best** | Good |
| Privacy-sensitive (data must stay on-prem) | Bad | **Best** | Partial — embeddings leave the box |
| Compliance (SOC 2, EU data residency) | Good — single region | **Best** — full physical control | Good — pick the worker region |
| Multi-region / globally distributed users | **Best** | Bad — one node | Good — many edges, one worker |
| Voice / WhatsApp (LiveKit, public webhooks) | **Best** | Not supported | Good — voice in cloud, chat at edge |

## Frontend deployment

The Vue 3 SPA is **never** packaged inside the Compose stack — not in
`docker-compose.yml`, not in `docker-compose.edge.yml`. It is built with Vite
and uploaded to Cloudflare Pages by `wrangler pages deploy`. The deployment
config lives at
[`deploy/cloudflare-pages.json`](https://github.com/ravencloak-org/Raven/blob/main/deploy/cloudflare-pages.json):

```json
{
  "project_name": "raven-frontend",
  "build": { "command": "npm run build", "output_directory": "dist", "root_directory": "frontend" },
  "deployment": { "production_branch": "main" }
}
```

The frontend is configured with `VITE_API_URL`, `VITE_API_BASE_URL`,
`VITE_API_DOMAIN`, and `VITE_LIVEKIT_URL` so it can be pointed at any
backend — the same SPA build can serve a cloud install and an edge install
simultaneously.

The marketing site and this documentation site (VitePress, in
[`docs-site/`](https://github.com/ravencloak-org/Raven/tree/main/docs-site))
are also Cloudflare Pages projects on their own build pipelines.

## Specialised variants

These two files are overlays, not standalone topologies — combine them with
either `docker-compose.yml` or `docker-compose.edge.yml`.

### `docker-compose.ebpf.yml`

Adds `CAP_BPF` and `CAP_NET_ADMIN` to the `go-api` container and sets
`RAVEN_EBPF_ENABLED=true`. Required for the eBPF tenancy-observation probe
(kernel ≥ 5.8). Use case: developing or debugging the eBPF data path.

```bash
docker compose -f docker-compose.yml -f docker-compose.ebpf.yml up
# or, with the edge stack:
docker compose -f docker-compose.edge.yml -f docker-compose.ebpf.yml up
```

eBPF builds require `CGO_ENABLED=1` and a musl cross-compiler
(`aarch64-linux-musl-gcc` / `x86_64-linux-musl-gcc`); see the `*-ebpf` targets
in `Makefile.edge`.

### `docker-compose.chartdb.yml`

Adds a single `chartdb` container (image `ghcr.io/chartdb/chartdb:latest`)
bound to `127.0.0.1:3000`. Used to visualise the Postgres schema. Not for
production.

```bash
docker compose -f docker-compose.yml -f docker-compose.chartdb.yml up -d chartdb
# open http://localhost:3000 and paste the output of scripts/chartdb-query.sh
```

## Resource footprint summary

| Variant | Services | Minimum host | Typical host |
|---|---|---|---|
| Cloud full stack | 14 | 4 vCPU / 8 GB / 50 GB SSD | AWS `t4g.xlarge` / `c7g.xlarge` (ARM64) |
| Edge / single-node | 3 | 2 vCPU / 2 GB / 16 GB | Raspberry Pi 4 (4 GB) or AWS `t4g.small` |
| Hybrid (edge half) | 3 at edge + remote worker | 2 vCPU / 2 GB at edge | Pi 4 at edge + `t4g.medium` worker |
| eBPF overlay | +0 (modifies `go-api`) | Linux kernel ≥ 5.8 | Same as base variant |
| ChartDB overlay | +1 | Negligible | Dev laptop |

## See also

- [`/guides/self-hosting/docker-compose`](/guides/self-hosting/docker-compose) — full-stack bring-up walkthrough
- [`/guides/self-hosting/edge-and-raspberry-pi`](/guides/self-hosting/edge-and-raspberry-pi) — Raspberry Pi specifics, ARM64 image pulls, eBPF caveats
- [`/reference/configuration`](/reference/configuration) — every `RAVEN_*` environment variable
