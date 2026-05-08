# Raven Local (Desktop)

A self-contained desktop edition of Raven for users who want everything to run locally — no cloud, no telemetry by default, no multi-tenancy. Built as a thin [Tauri](https://tauri.app/) shell over the existing Docker compose, with [Ollama](https://ollama.com/) bundled as a local LLM provider.

> **Status:** Active development under [milestone M11](https://github.com/ravencloak-org/Raven/milestone/14). Track progress on the [project board](https://github.com/orgs/ravencloak-org/projects/2).

## Why

Raven's cloud SaaS is the primary product, but the architecture has always been edge-deployable (the same stack runs on a Raspberry Pi). Raven Local turns that into a one-click experience for:

- Privacy-conscious users who don't want any data leaving their machine
- Regulated industries with on-prem mandates
- Hobbyists and developers who want to experiment locally before committing to cloud
- A clean funnel from "try locally" → "self-hosted prod" → "managed SaaS"

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Tauri shell (Rust + native window)                         │
│  ├─ Compose orchestrator (start / stop / health / logs)     │
│  ├─ System-requirements precheck (RAM, CPU, disk)           │
│  ├─ Tray / menubar status                                   │
│  ├─ Auto-updater                                            │
│  └─ Native window → http://127.0.0.1:<api-port>             │
└────────────────────────┬────────────────────────────────────┘
                         │  manages
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  Docker compose (existing services)                         │
│  ├─ Go API (Gin) ── single-user mode = true                 │
│  ├─ Python AI worker (gRPC)                                 │
│  ├─ PostgreSQL 18 + pgvector                                │
│  ├─ Valkey + Asynq                                          │
│  ├─ Vue.js frontend                                         │
│  └─ Ollama (sidecar — local LLM)                            │
└─────────────────────────────────────────────────────────────┘
```

## Key design decisions

| Decision | Rationale |
|----------|-----------|
| Tauri over Electron | ~10× smaller footprint, native performance, Rust security model |
| Bundle existing compose, don't rewrite services | Same binary as the SaaS; lower maintenance burden |
| Single-user mode = config flag | One env var (`RAVEN_SINGLE_USER=true`) bypasses auth + pins to default workspace |
| Ollama as a BYOK provider | Reuses the existing AI worker provider abstraction |
| Unsigned installers in M11; signing in M12 | Faster ship; signed-and-notarized binaries follow |

## Hardware requirements

| | Minimum | Recommended |
|--|---------|-------------|
| RAM | 8 GB | 16 GB+ |
| Disk (free) | 20 GB | 50 GB |
| CPU cores | 4 | 8+ |
| GPU | optional | recommended for larger Ollama models |

The system-requirements precheck ([#420](https://github.com/ravencloak-org/Raven/issues/420)) warns the user before launch if they're below minimum.

## Upgrade path

A user who outgrows Raven Local — needs multiple users, remote access, scaling — can graduate to:

1. **Self-hosted Raven** — same compose, run it on a server, switch off `RAVEN_SINGLE_USER`, point users at the host's URL.
2. **Raven Cloud (managed SaaS)** — export the workspace from Raven Local, import into the hosted offering.

The data model is identical across all three; the differences are deployment topology and the auth flag.

## Telemetry

Raven Local ships with telemetry **off by default**. The optional opt-in (added in [#424](https://github.com/ravencloak-org/Raven/issues/424)) sends only anonymous usage counters (e.g., "Ollama model downloaded") via the existing OpenObserve pipeline pointed at our public endpoint. No content, no prompts, no PII.

## See also

- [Architecture-Overview](Architecture-Overview.md) — the underlying Raven architecture (cloud + self-hosted + local share this)
- [Hardware-Requirements](Hardware-Requirements.md) — the broader Raven hardware story
- [Roadmap](Roadmap.md) — milestone-level plan
