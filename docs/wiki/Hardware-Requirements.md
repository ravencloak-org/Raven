# Hardware Requirements

## Tier 1: Edge / Raspberry Pi (Go API only)

| Spec | Minimum | Recommended |
|------|---------|-------------|
| Device | Raspberry Pi 4 (2GB) | Raspberry Pi 4 (4GB) |
| RAM used | ~400 MB | ~400 MB + headroom |
| Disk | 8 GB SD card | 32 GB SD card |
| Cost | $0-6/mo (ARM VPS) | ~$35 one-time (Pi) |

Everything else runs on a remote server.

## Tier 2: Self-Hosted Single Server

### Per-Service RAM Breakdown

| Service | Idle | Light Load | Heavy Load |
|---------|------|------------|------------|
| Go API | 10 MB | 50 MB | 200 MB |
| Python Worker | 120 MB | 500 MB | 2+ GB |
| PostgreSQL 18 | 256 MB | 512 MB | 2+ GB |
| Valkey | 20 MB | 50 MB | 200 MB |
| Keycloak | 512 MB | 768 MB | 1 GB |
| Strapi | 150 MB | 250 MB | 400 MB |
| SeaweedFS | 30 MB | 100 MB | 300 MB |
| Traefik | 30 MB | 50 MB | 100 MB |
| LiveKit (Phase 2) | 40 MB | 200 MB | 500 MB |

| Spec | Minimum | Recommended |
|------|---------|-------------|
| CPU | 4 cores | 8 cores |
| RAM | 4 GB (tight) | 16 GB |
| Disk | 40 GB SSD | 200 GB NVMe |
| Cost | $28-55/mo (Hetzner) | $55-140/mo (Hetzner) |

## Tier 3: Production Cloud (AWS)

| Scale | Tenants | Users | Embeddings | Monthly Cost |
|-------|---------|-------|------------|-------------|
| Small | 10 | 50 | 500K | ~$700/mo |
| Medium | 100 | 500 | 5M | ~$2,825/mo |
| Large | 1,000 | 5,000 | 50M | ~$11,930/mo |

For detailed analysis including HNSW index sizing and quantization options, see `docs/research/hardware-requirements.md`.
