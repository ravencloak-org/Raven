# Raven Edge Deployment

Deploy Raven on a Raspberry Pi or other ARM64 edge node with a minimal footprint.
The edge configuration runs only the Go API, PostgreSQL, and Traefik -- the Python
AI worker runs remotely in the cloud and is accessed over gRPC.

## Architecture

```
  Edge Node (Raspberry Pi)              Cloud
  ┌─────────────────────────┐          ┌────────────────────┐
  │  Traefik  :80/:443      │          │  Python AI Worker  │
  │    │                     │  gRPC    │  :50051            │
  │  Go API   :8080     ────────────►  │                    │
  │    │                     │          └────────────────────┘
  │  PostgreSQL :5432        │
  │  (pgvector/pg18)         │
  └─────────────────────────┘
```

## Requirements

| Component      | Minimum            | Recommended         |
|----------------|--------------------|---------------------|
| Hardware       | Raspberry Pi 4     | Raspberry Pi 5      |
| RAM            | 2 GB               | 4 GB                |
| Storage        | 16 GB SD / SSD     | 32 GB+ SSD          |
| OS             | Raspberry Pi OS 64-bit (Bookworm) | Ubuntu 24.04 LTS arm64 |
| Docker         | 24.0+              | 27.0+               |
| Docker Compose | v2 plugin          | v2 plugin           |
| Network        | Outbound to cloud AI worker (gRPC port) | Static IP or Tailscale |

## Quick Start

### 1. Install Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# Log out and back in, then verify:
docker run --rm hello-world
```

### 2. Clone the repository

```bash
git clone https://github.com/ravencloak-org/Raven.git
cd Raven
```

### 3. Run the setup script

```bash
chmod +x deploy/edge/install.sh
./deploy/edge/install.sh
```

This checks prerequisites, creates data directories, and copies the example
environment file.

### 4. Configure environment

```bash
# Edit the generated .env.edge file:
nano .env.edge
```

Key variables to set:

| Variable              | Description                                    | Example                              |
|-----------------------|------------------------------------------------|--------------------------------------|
| `POSTGRES_PASSWORD`   | PostgreSQL password                            | `a-strong-random-password`           |
| `DATABASE_URL`        | Full Postgres connection string                | `postgresql://raven:PASS@postgres:5432/raven?sslmode=disable` |
| `GRPC_AI_WORKER_ADDR` | Remote AI worker gRPC address (host:port)     | `ai-worker.example.com:50051`        |
| `RAVEN_SERVER_PORT`   | API listen port (inside container)             | `8080`                               |

### 5. Start the stack

```bash
docker compose -f docker-compose.edge.yml --env-file .env.edge up -d
```

### 6. Verify

```bash
# Check all services are running:
docker compose -f docker-compose.edge.yml --env-file .env.edge ps

# Health check the API:
curl http://localhost/api/healthz

# View logs:
docker compose -f docker-compose.edge.yml --env-file .env.edge logs -f go-api
```

## Cross-Compilation (Building from Source)

If you want to build the Go binary locally instead of using a pre-built Docker image:

```bash
# Build for ARM64 (Raspberry Pi):
make -f Makefile.edge build-arm64

# Build for AMD64:
make -f Makefile.edge build-amd64

# Build both:
make -f Makefile.edge build-all

# Build Docker image for ARM64:
make -f Makefile.edge docker-arm64
```

All binaries are compiled with `CGO_ENABLED=0` producing fully static binaries
with no external dependencies.

Output binaries are placed in `bin/edge/`.

## Updating

```bash
cd Raven

# Pull latest images:
docker compose -f docker-compose.edge.yml --env-file .env.edge pull

# Restart with new images:
docker compose -f docker-compose.edge.yml --env-file .env.edge up -d
```

## Resource Usage

The edge stack is tuned for constrained hardware:

| Service    | Memory Limit | Typical Usage |
|------------|-------------|---------------|
| Go API     | 256 MB      | ~50-80 MB     |
| PostgreSQL | 512 MB      | ~100-200 MB   |
| Traefik    | 128 MB      | ~20-30 MB     |
| **Total**  | **896 MB**  | **~200-350 MB** |

## Troubleshooting

**API cannot reach the AI worker:**
- Verify `GRPC_AI_WORKER_ADDR` is reachable from the edge node:
  `nc -zv ai-worker.example.com 50051`
- Check firewall rules allow outbound traffic on the gRPC port.

**PostgreSQL fails to start:**
- Ensure the `pg-data` volume has enough disk space.
- On first run, initialization may take a minute on slower SD cards.

**Out of memory:**
- Check `docker stats` for per-container usage.
- Consider adding swap: `sudo fallocate -l 2G /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile`

**ARM64 image not found:**
- Ensure you are using the `latest-arm64` tag or building locally with `make -f Makefile.edge docker-arm64`.
