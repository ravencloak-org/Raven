#!/usr/bin/env bash
# install.sh — Raven Edge Deployment setup script.
#
# Checks prerequisites, creates data directories, copies example env,
# and optionally pulls container images.
#
# Usage:
#   chmod +x deploy/edge/install.sh
#   ./deploy/edge/install.sh

set -euo pipefail

# ─── Colors ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# ─── Resolve project root (where docker-compose.edge.yml lives) ─────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

info "Raven Edge Deployment Installer"
info "Project root: ${PROJECT_ROOT}"
echo ""

# ─── Check prerequisites ────────────────────────────────────────────────────
MISSING=0

check_cmd() {
    if ! command -v "$1" &>/dev/null; then
        error "Required command not found: $1"
        MISSING=1
    else
        info "Found $1: $(command -v "$1")"
    fi
}

info "Checking prerequisites..."
check_cmd docker

# Check for docker compose (v2 plugin) or docker-compose (v1 standalone)
if docker compose version &>/dev/null; then
    info "Found docker compose (v2 plugin)"
    COMPOSE_CMD="docker compose"
elif command -v docker-compose &>/dev/null; then
    info "Found docker-compose (standalone)"
    COMPOSE_CMD="docker-compose"
else
    error "Neither 'docker compose' nor 'docker-compose' found."
    MISSING=1
fi

if [ "$MISSING" -ne 0 ]; then
    echo ""
    error "Missing prerequisites. Please install them and re-run this script."
    error "Docker install guide: https://docs.docker.com/engine/install/debian/"
    exit 1
fi

# ─── Check architecture ─────────────────────────────────────────────────────
ARCH="$(uname -m)"
info "Detected architecture: ${ARCH}"

if [[ "$ARCH" != "aarch64" && "$ARCH" != "arm64" && "$ARCH" != "x86_64" ]]; then
    warn "Unsupported architecture: ${ARCH}. Raven edge supports arm64 and amd64."
fi

# ─── Check Docker is running ────────────────────────────────────────────────
if ! docker info &>/dev/null; then
    error "Docker daemon is not running. Please start Docker and re-run."
    exit 1
fi
info "Docker daemon is running."

# ─── Check available memory ─────────────────────────────────────────────────
if [ -f /proc/meminfo ]; then
    TOTAL_MEM_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    TOTAL_MEM_MB=$((TOTAL_MEM_KB / 1024))
    info "Available RAM: ${TOTAL_MEM_MB} MB"
    if [ "$TOTAL_MEM_MB" -lt 1024 ]; then
        warn "Less than 1 GB RAM detected. Raven edge recommends at least 2 GB."
    fi
fi

# ─── Create data directories ────────────────────────────────────────────────
echo ""
info "Creating data directories..."
mkdir -p "${PROJECT_ROOT}/data/postgres"
info "Created ${PROJECT_ROOT}/data/postgres"

# ─── Copy environment file ──────────────────────────────────────────────────
ENV_FILE="${PROJECT_ROOT}/.env.edge"
if [ -f "$ENV_FILE" ]; then
    warn ".env.edge already exists — skipping copy. Review it manually."
else
    cp "${SCRIPT_DIR}/.env.example" "$ENV_FILE"
    info "Copied .env.example to ${ENV_FILE}"
    warn "Edit .env.edge with your actual values before starting Raven."
fi

# ─── Pull container images ──────────────────────────────────────────────────
echo ""
read -rp "Pull container images now? [Y/n] " PULL_IMAGES
PULL_IMAGES="${PULL_IMAGES:-Y}"

if [[ "$PULL_IMAGES" =~ ^[Yy]$ ]]; then
    info "Pulling images (this may take a few minutes on first run)..."
    cd "$PROJECT_ROOT"
    $COMPOSE_CMD -f docker-compose.edge.yml --env-file .env.edge pull || {
        warn "Image pull failed — you can retry later with:"
        warn "  $COMPOSE_CMD -f docker-compose.edge.yml --env-file .env.edge pull"
    }
    info "Images pulled successfully."
else
    info "Skipping image pull. Run manually when ready:"
    info "  $COMPOSE_CMD -f docker-compose.edge.yml --env-file .env.edge pull"
fi

# ─── Done ────────────────────────────────────────────────────────────────────
echo ""
info "Setup complete. Next steps:"
echo ""
echo "  1. Edit .env.edge with your configuration:"
echo "     - Set POSTGRES_PASSWORD to a strong password"
echo "     - Set GRPC_AI_WORKER_ADDR to your remote AI worker address"
echo "     - Update DATABASE_URL to match your password"
echo ""
echo "  2. Start Raven Edge:"
echo "     cd ${PROJECT_ROOT}"
echo "     $COMPOSE_CMD -f docker-compose.edge.yml --env-file .env.edge up -d"
echo ""
echo "  3. Check status:"
echo "     $COMPOSE_CMD -f docker-compose.edge.yml --env-file .env.edge ps"
echo ""
echo "  4. View logs:"
echo "     $COMPOSE_CMD -f docker-compose.edge.yml --env-file .env.edge logs -f"
echo ""
info "Done."
