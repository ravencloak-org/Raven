---
title: "Edge & Raspberry Pi"
---

# Edge & Raspberry Pi

This guide is the operator's how-to for running Raven on a Raspberry Pi 4 / 5,
an AWS `t4g.small`, or any small ARM64 or amd64 single-node host. The edge
deployment is intentionally minimal — three containers, no Python ai-worker
in-process, no LiveKit, no object store. If you are wondering *whether* this
topology suits your use case, read
[Deployment Models](/concepts/deployment-models) first; this page assumes
you have already chosen the edge variant and want to bring it up.

The stack that lands on the Pi is just the Go API, a PostgreSQL with
pgvector, and a Traefik reverse proxy. The Python ai-worker is offloaded to
a remote endpoint over gRPC — typically a separate worker host, or a BYOK
LLM provider fronted by a small worker shim. That offload is what keeps the
edge footprint under a gigabyte of RAM.

## Prereqs

- Raspberry Pi 4 or 5 with **at least 4 GB RAM** (the bootstrap script
  warns below 1 GB, recommends 2 GB minimum, and the Compose limits add up
  to ~900 MB before the OS).
- 32 GB SD card minimum, **SSD strongly preferred** for Postgres durability
  and SD-wear reasons (see [Backups on a Pi](#backups-on-a-pi)).
- 64-bit Linux (Raspberry Pi OS 64-bit, Ubuntu Server 24.04 LTS, or
  Debian 12). The compose file pins `platform: linux/arm64` for every
  service.
- **Docker Engine with Compose v2** (`docker compose`, not `docker-compose`).
  The bootstrap script accepts the v1 standalone binary as a fallback but
  v2 is preferred.
- A reachable remote ai-worker — either your own deployment of
  `ai-worker/` on a beefier host, or a BYOK LLM you proxy from one. The
  Go API will refuse to start without `GRPC_AI_WORKER_ADDR`.
- Roughly **one hour** end-to-end, including image pulls.

## Hardware notes

| Concern | Pi 4 (4 GB) | Pi 5 (8 GB) | `t4g.small` |
|---|---|---|---|
| Sustained ingest of medium PDFs | Tight — close to the 256 MB API cap. | Comfortable. | Comfortable. |
| pgvector HNSW build | Slow on SD card; minutes per 10k vectors. | Faster, especially on NVMe HAT. | Fastest. |
| Idle thermal | OK with passive heatsink. | Needs active cooling under load. | N/A. |
| Power | PoE+ HAT works; USB-C PD 5 V/3 A also fine. | Use the official 27 W USB-C PD. | N/A. |
| Storage failure mode | SD wear within ~12 months at heavy write rates. | Same SD risk; NVMe HAT eliminates it. | EBS gp3, durable. |

If you have any choice, run from an SSD or NVMe HAT, not the SD card.
Postgres on an SD card is a known-bad pattern — the WAL beats the flash
endurance into the ground. The Pi 5 with an M.2 HAT is materially better
than a Pi 4 here.

## Bootstrap

Clone the repo, copy the edge env file, and bring the stack up. The
project ships a bootstrap helper at
[`deploy/edge/install.sh`](https://github.com/ravencloak-org/Raven/blob/main/deploy/edge/install.sh)
which does prerequisite checks (Docker present, Compose v2, ARM64
architecture detected, ≥ 1 GB RAM warning) and creates `data/postgres`
before you start.

```bash
git clone https://github.com/ravencloak-org/Raven.git
cd Raven

# Prereq checks + create data dirs + copy env example
chmod +x deploy/edge/install.sh
./deploy/edge/install.sh

# Edit .env.edge — set DATABASE_URL, GRPC_AI_WORKER_ADDR, POSTGRES_PASSWORD
$EDITOR .env.edge

# Bring it up
docker compose -f docker-compose.edge.yml --env-file .env.edge up -d
docker compose -f docker-compose.edge.yml --env-file .env.edge ps
```

Verify the API is healthy:

```bash
curl http://localhost/api/healthz
docker compose -f docker-compose.edge.yml --env-file .env.edge logs -f go-api
```

The Go API runs database migrations automatically at start — there is no
separate migrate step on the edge variant.

## The 3-service stack

The full file is at
[`docker-compose.edge.yml`](https://github.com/ravencloak-org/Raven/blob/main/docker-compose.edge.yml).
It declares exactly three services. Memory limits below are the values
declared in the compose file:

| Service | Image | Memory limit | Role |
|---|---|---|---|
| `go-api` | `ghcr.io/ravencloak-org/raven-api:latest` (linux/arm64) | **256 MB** | HTTP/JSON API, auth, orchestration; talks to remote ai-worker over gRPC. Sets `RAVEN_EDGE_MODE: "true"`. |
| `postgres` | `pgvector/pgvector:pg18` (linux/arm64) | **512 MB** | Primary store. pgvector for embeddings; tenant isolation via RLS. |
| `traefik` | `traefik:v3.3` (linux/arm64) | **128 MB** | Reverse proxy on ports 80/443. Routes `/api` to `go-api`. |

Total reserved ≈ **896 MB**. Typical resident usage in steady state is
~200–350 MB across the three services (the API floats around 50–80 MB,
Postgres 100–200 MB, Traefik 20–30 MB).

The Postgres health check uses `pg_isready` and the API waits on
`postgres: service_healthy` before starting — a clean cold-boot
sequence is part of the design.

## Remote ai-worker

The defining feature of the edge variant is that the heavy Python service
runs *somewhere else*. The Go API connects to it as a gRPC client:

```env
# in .env.edge
GRPC_AI_WORKER_ADDR=ai-worker.example.com:50051
```

This is wired into the `go-api` service as `RAVEN_GRPC_WORKER_ADDR` (the
Compose file re-maps `GRPC_AI_WORKER_ADDR` → `RAVEN_GRPC_WORKER_ADDR`).
Format is `host:port` with **no scheme prefix** — the Go API speaks plain
gRPC, TLS is a separate config bit on the worker side.

Typical hybrid topology:

```
┌──────────────────────┐    gRPC :50051     ┌────────────────────────┐
│  Edge node (Pi 4)    │ ─────────────────▶ │  Remote ai-worker      │
│  ┌────────────────┐  │                    │  (cloud VM or GPU box) │
│  │ traefik :443   │  │                    │  ┌──────────────────┐  │
│  │ go-api  :8080  │  │                    │  │ python ai-worker │  │
│  │ postgres :5432 │  │                    │  └──────────────────┘  │
│  └────────────────┘  │                    │  + BYOK LLM provider   │
└──────────────────────┘                    └────────────────────────┘
```

Three rules:

1. The worker endpoint must be reachable from the Pi over the network.
2. The worker holds the BYOK LLM credentials, not the edge node.
3. If the worker is on the public internet, terminate TLS for gRPC on
   the worker side (or front it with an ingress that does). The Pi has no
   business holding LLM API keys.

## ARM64 binaries

Pre-built ARM64 images are pulled from GHCR by default
(`ghcr.io/ravencloak-org/raven-api:latest`). If you want to build the
binary yourself — e.g. for an air-gapped Pi or a custom version string —
use [`Makefile.edge`](https://github.com/ravencloak-org/Raven/blob/main/Makefile.edge):

```bash
make -f Makefile.edge build-arm64
# Output: build/raven-api-linux-arm64
```

That target runs:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags="-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)" \
    -o build/raven-api-linux-arm64 ./cmd/api
```

`CGO_ENABLED=0` makes the binary statically linked (no glibc surprises on
Alpine or musl-based hosts). The `-s -w` flags strip the symbol table and
DWARF debug info — the resulting ARM64 binary is roughly **25 MB**, the
number the README quotes. eBPF builds (`build-arm64-ebpf`) require an
`aarch64-linux-musl-gcc` cross-compiler and are not relevant for the
default edge stack.

`make -f Makefile.edge build-all` produces both architectures;
`docker-arm64` / `docker-amd64` / `docker-all` build the OCI images and
push a multi-arch manifest.

## Cloudflare Tunnel

Most Pis live behind a home NAT, a corporate firewall, or a carrier-grade
NAT that you can't reasonably punch through. The standard answer is
Cloudflare Tunnel — `cloudflared` runs as a daemon on the Pi, dials out
to Cloudflare's edge, and Cloudflare proxies inbound traffic in.

The Ansible role at
[`deploy/ansible/roles/cloudflared/`](https://github.com/ravencloak-org/Raven/tree/main/deploy/ansible/roles/cloudflared)
is the canonical pattern. The role:

- Detects `x86_64` vs `aarch64` and downloads the matching `cloudflared`
  release binary.
- Renders `/etc/cloudflared/config.yml` from
  [`templates/config.yml.j2`](https://github.com/ravencloak-org/Raven/blob/main/deploy/ansible/roles/cloudflared/templates/config.yml.j2)
  with your `cloudflare_tunnel_id` and ingress rules.
- Installs a hardened systemd unit (`NoNewPrivileges`, `ProtectSystem`,
  `ProtectHome`) that runs `cloudflared tunnel run`.

For an edge node you typically only need one ingress rule:

```yaml
ingress:
  - hostname: pi.your-domain.example
    service: http://localhost:80   # Traefik
  - service: http_status:404
```

The tunnel credentials JSON (`/etc/cloudflared/<tunnel-id>.json`) must be
copied to the Pi manually — the role warns if it's missing and won't
start the service.

## TLS at the edge

You have two choices:

1. **Pi is internet-routable** (you have a static IP or a working
   dynamic DNS, ports 80/443 forwarded). Add the standard Traefik
   Let's Encrypt resolver and let it provision certs over HTTP-01. The
   edge compose file leaves the resolver unconfigured — wire it through
   the `traefik` service `command:` block. See
   [Traefik & TLS](/guides/self-hosting/traefik-and-tls).

2. **Pi is behind NAT (most likely)**. Use a Cloudflare Tunnel and a
   Cloudflare Origin Certificate. The Pi never holds a public cert — the
   public TLS terminates at Cloudflare's edge, and the tunnel between the
   Pi and Cloudflare is encrypted by the `cloudflared` daemon. Set
   `DOMAIN=pi.your-domain.example` and `ACME_EMAIL=...` in `.env.edge` for
   reference but you don't strictly need a Traefik resolver in this mode.

Option 2 is what we recommend for almost every Pi deployment. It removes
the need for any inbound port at all.

## Backups on a Pi

Two hard rules:

- **The volume *is* the SD card** unless you've moved the Docker data root
  to an SSD. `pg-data:` is a named volume that lives under
  `/var/lib/docker/volumes/` by default — i.e., on the SD card. A card
  failure loses the database.
- **Always back up off the device.** pgBackRest (the same tool the
  full-stack deployment uses for Postgres) works fine on a Pi, but the
  repo must live somewhere that isn't the Pi.

Practical recipe:

1. Move Docker's data root to your SSD or NVMe (`/etc/docker/daemon.json`
   → `"data-root": "/mnt/ssd/docker"`, restart Docker, then bring the
   stack back up). The compose file's `pg-data` named volume will land
   on the SSD.
2. Configure pgBackRest to push diffs to S3 (or R2, B2, MinIO — anything
   S3-compatible). See [Backups](/guides/self-hosting/backups) for the
   stanza layout the full stack uses.
3. Add a nightly cron to `pg_dump` as a belt-and-braces safety net while
   you settle into pgBackRest.

Off-site is not optional. A house fire, theft, or a kid pulling the
power during a write all end the same way.

## Voice agent on edge

**Probably not.** LiveKit needs UDP, decent uplink bandwidth, and enough
CPU to terminate WebRTC for every concurrent caller. A Pi 4 will do one,
maybe two simultaneous voice sessions before audio quality degrades.

If you genuinely want voice on the edge, you have two paths:

1. **Self-host alongside.** Deploy `python-agent/` and `livekit-server`
   on the same Pi or, more sensibly, on a second host on the same LAN.
   This is not part of `docker-compose.edge.yml`; it's a manual
   composition.
2. **Offload to LiveKit Cloud.** Point your `python-agent/` config at
   LiveKit Cloud and let them handle the SFU. You still need a `python-agent`
   process somewhere, but that's the cheap part.

Either way, the edge variant *itself* does not ship voice. If voice is a
day-one requirement, the [Cloud-hosted full stack](/concepts/deployment-models)
is the right topology, not edge.

## Update workflow

```bash
cd Raven
git pull
docker compose -f docker-compose.edge.yml --env-file .env.edge pull
docker compose -f docker-compose.edge.yml --env-file .env.edge up -d
```

Migrations run automatically on `go-api` start — there is no separate
migrate step on the edge variant. If you've pinned `RAVEN_API_IMAGE` to a
specific tag (recommended for production), bump the tag in `.env.edge`
before `pull`. See [Upgrades](/guides/self-hosting/upgrades) for the
broader upgrade story.

## Performance

Honest answer: it varies. Some rough numbers from a Pi 4 (4 GB, USB SSD)
and a Pi 5 (8 GB, NVMe HAT), both with a remote ai-worker on a beefier
amd64 host:

| Operation | Pi 4 + SSD | Pi 5 + NVMe |
|---|---|---|
| Cold start (compose up to `healthz`) | ~20 s | ~10 s |
| Ingest a 10-page PDF (parse + embed remotely) | 3–5 s | 2–3 s |
| Hybrid retrieval query, 50k-chunk KB | 80–200 ms | 40–120 ms |
| pgvector HNSW build, 10k vectors | ~30 s | ~12 s |

Most of the latency budget is in the network hop to the remote
ai-worker and in pgvector. The Go API itself is rarely the bottleneck on
either platform. **Benchmark on your own hardware** before you make
capacity promises.

## Troubleshooting

**OOM kills.** The Go API has a 256 MB cap. If you see `OOMKilled` under
load, the usual cause is large embedding payloads buffered in memory.
Raise the limit, or move the ai-worker closer to the Pi. Check with
`docker compose -f docker-compose.edge.yml ps` and `docker stats`.

**Slow pgvector queries.** HNSW indexes are memory-resident. On a Pi 4
the 512 MB Postgres cap is tight for any KB above ~100k chunks. Either
raise the cap (and accept the OS pressure), or use a flat index for
small KBs.

**SD card wear / Postgres corruption.** `PANIC: could not write to file
"pg_wal/..."` after months of uptime is the classic SD-failure mode.
Move to SSD, restore from backup, never look back.

**`GRPC_AI_WORKER_ADDR is required` on `up`.** The compose file enforces
this with `${GRPC_AI_WORKER_ADDR:?...}` syntax — you have to set it in
`.env.edge` or the stack refuses to start. There is no "local fallback".

**Architecture mismatch.** All three services pin `platform: linux/arm64`.
If you run the file on an amd64 host without removing the `platform:`
lines, Docker will pull the ARM64 image and fail to run it. The bootstrap
script flags this with an architecture check.

**Cloudflare Tunnel won't start.** Check
`/etc/cloudflared/<tunnel-id>.json` exists — the Ansible role warns when
it doesn't, and the systemd unit will sit in a restart loop until it's
present.

## See also

- [Deployment Models](/concepts/deployment-models) — the full
  three-topology overview, including when the edge variant is the right
  call.
- [Docker Compose](/guides/self-hosting/docker-compose) — full-stack
  bring-up; useful for the worker host side of a hybrid edge deployment.
- [Traefik & TLS](/guides/self-hosting/traefik-and-tls) — ACME and
  DNS-01 details if you do choose Let's Encrypt over Cloudflare Tunnel.
- [Backups](/guides/self-hosting/backups) — pgBackRest stanza layout.
- [Upgrades](/guides/self-hosting/upgrades) — the broader update story.
- [Configuration reference](/reference/configuration) — every `RAVEN_*`
  environment variable.
