#!/usr/bin/env bash
# setup.sh — Bootstrap Raven on an EC2 instance.
#
# What this does:
#   1. Installs Docker (if missing)
#   2. Creates a secondary Docker daemon on /run/raven/docker.sock
#      → invisible to default `docker ps`
#   3. Installs cloudflared for CF Tunnel ingress (no open ports)
#   4. Copies configs and creates .env.server from the example
#
# Usage:
#   # On EC2 — from the cloned repo root:
#   sudo bash deploy/ec2/setup.sh
#
#   # Or pipe directly:
#   curl -fsSL https://raw.githubusercontent.com/ravencloak-org/Raven/main/deploy/ec2/setup.sh | sudo bash

set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
step()  { echo -e "\n${CYAN}══ $* ══${NC}"; }

[[ $EUID -ne 0 ]] && { error "Run as root: sudo bash $0"; exit 1; }

# Resolve project root (where this script was cloned)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "$PROJECT_ROOT"

info "Project root: $PROJECT_ROOT"

# ─── 1. Install Docker ────────────────────────────────────────────────────────
step "Docker"
if command -v docker &>/dev/null; then
    info "Docker already installed: $(docker --version)"
else
    info "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
    info "Docker installed."
fi

# containerd must be running for the secondary daemon to share it
systemctl enable --now containerd 2>/dev/null || true

# ─── 2. Secondary Docker daemon (hidden from default docker ps) ───────────────
step "Secondary Docker daemon (raven-dockerd)"

# Runtime directory
mkdir -p /run/raven
chmod 750 /run/raven

# Data root (separate from /var/lib/docker)
mkdir -p /var/lib/raven-docker

# Install systemd unit
cp "${SCRIPT_DIR}/raven-dockerd.service" /etc/systemd/system/raven-dockerd.service
systemctl daemon-reload
systemctl enable raven-dockerd
systemctl restart raven-dockerd

# Wait for socket to appear
for i in $(seq 1 20); do
    [[ -S /run/raven/docker.sock ]] && break
    sleep 1
done
[[ -S /run/raven/docker.sock ]] || { error "raven-dockerd socket did not appear — check: journalctl -u raven-dockerd"; exit 1; }
info "raven-dockerd running at /run/raven/docker.sock"

# Convenience alias in /etc/profile.d
cat > /etc/profile.d/raven-docker.sh <<'EOF'
# Access Raven's hidden Docker daemon
alias rdocker='docker -H unix:///run/raven/docker.sock'
alias rdc='docker -H unix:///run/raven/docker.sock compose'
EOF
info "Aliases added: rdocker, rdc"

# ─── 3. GHCR authentication ───────────────────────────────────────────────────
step "GHCR authentication"
if [[ -f /root/.docker/config.json ]] && grep -q "ghcr.io" /root/.docker/config.json 2>/dev/null; then
    info "Already authenticated to ghcr.io"
else
    echo ""
    warn "To pull private GHCR images you need a GitHub Personal Access Token"
    warn "with read:packages scope."
    echo ""
    read -rp "GitHub username: " GHCR_USER
    read -rsp "GitHub PAT (read:packages): " GHCR_TOKEN
    echo ""
    echo "$GHCR_TOKEN" | docker -H unix:///run/raven/docker.sock login ghcr.io -u "$GHCR_USER" --password-stdin
    info "Authenticated to ghcr.io"
fi

# ─── 4. cloudflared ──────────────────────────────────────────────────────────
step "cloudflared"
if command -v cloudflared &>/dev/null; then
    info "cloudflared already installed: $(cloudflared --version)"
else
    info "Installing cloudflared..."
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64)  CF_ARCH="amd64" ;;
        aarch64) CF_ARCH="arm64" ;;
        *)       error "Unsupported arch: $ARCH"; exit 1 ;;
    esac
    curl -fsSL "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${CF_ARCH}" \
        -o /usr/local/bin/cloudflared
    chmod +x /usr/local/bin/cloudflared
    info "cloudflared installed: $(cloudflared --version)"
fi

mkdir -p /etc/cloudflared

echo ""
echo "─────────────────────────────────────────────────────────"
warn "Choose your Cloudflare Tunnel setup method:"
echo "  A) Zero Trust token (paste a token from the CF dashboard)"
echo "  B) cloudflared tunnel login (opens a browser auth flow)"
echo "─────────────────────────────────────────────────────────"
read -rp "Method [A/b]: " CF_METHOD
CF_METHOD="${CF_METHOD:-A}"

if [[ "${CF_METHOD,,}" == "a" ]]; then
    echo ""
    warn "Get your tunnel token from:"
    warn "  Cloudflare Zero Trust → Access → Tunnels → Create tunnel → copy token"
    echo ""
    read -rsp "Cloudflare Tunnel token: " CF_TOKEN
    echo ""

    # Write the token-based systemd service
    cat > /etc/systemd/system/cloudflared.service <<EOF
[Unit]
Description=Cloudflare Tunnel for Raven
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/cloudflared tunnel --no-autoupdate run --token ${CF_TOKEN}
Restart=on-failure
RestartSec=5s
KillMode=process

[Install]
WantedBy=multi-user.target
EOF
    info "Token-based cloudflared service written."
else
    echo ""
    info "Running: cloudflared tunnel login"
    info "A browser window will open — log in and authorise the tunnel."
    cloudflared tunnel login

    echo ""
    read -rp "Tunnel name (e.g. raven): " TUNNEL_NAME
    cloudflared tunnel create "$TUNNEL_NAME"

    # Find the tunnel ID
    TUNNEL_ID=$(cloudflared tunnel list --output json | python3 -c \
        "import json,sys; tunnels=json.load(sys.stdin); print(next(t['id'] for t in tunnels if t['name']=='${TUNNEL_NAME}'))")

    read -rp "Domain (e.g. raven.jobin.wtf): " RAVEN_DOMAIN
    cloudflared tunnel route dns "$TUNNEL_NAME" "$RAVEN_DOMAIN"
    cloudflared tunnel route dns "$TUNNEL_NAME" "auth.${RAVEN_DOMAIN}" 2>/dev/null || true

    # Write config file
    sed "s/TUNNEL_ID/${TUNNEL_ID}/g; s/\${RAVEN_DOMAIN}/${RAVEN_DOMAIN}/g" \
        "${SCRIPT_DIR}/cloudflared-config.yml" > /etc/cloudflared/config.yml

    cat > /etc/systemd/system/cloudflared.service <<EOF
[Unit]
Description=Cloudflare Tunnel for Raven
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/cloudflared tunnel --config /etc/cloudflared/config.yml run
Restart=on-failure
RestartSec=5s
KillMode=process

[Install]
WantedBy=multi-user.target
EOF
    info "Config-based cloudflared service written."
fi

systemctl daemon-reload
systemctl enable cloudflared
systemctl restart cloudflared
info "cloudflared started."

# ─── 5. Environment file ─────────────────────────────────────────────────────
step "Environment file"
if [[ -f "${PROJECT_ROOT}/.env.server" ]]; then
    warn ".env.server already exists — leaving it untouched."
else
    cp "${SCRIPT_DIR}/.env.example" "${PROJECT_ROOT}/.env.server"
    info "Created .env.server — edit it now before starting the stack."
fi
chmod 600 "${PROJECT_ROOT}/.env.server"

# ─── 6. Make helper scripts executable ───────────────────────────────────────
chmod +x "${SCRIPT_DIR}/update.sh"

# ─── Done ─────────────────────────────────────────────────────────────────────
step "Setup complete"
cat <<DONE

${GREEN}Next steps:${NC}

  1. Edit .env.server with real secrets:
       nano ${PROJECT_ROOT}/.env.server

  2. Start the Raven stack (on the hidden daemon):
       DOCKER_HOST=unix:///run/raven/docker.sock \\
       docker compose -f deploy/ec2/docker-compose.server.yml \\
                      --env-file .env.server up -d

     Or use the alias (reload shell first):
       rdc -f deploy/ec2/docker-compose.server.yml --env-file .env.server up -d

  3. Check it's invisible:
       docker ps          # shows nothing from raven
       rdocker ps         # shows raven containers

  4. Check logs:
       rdocker compose -f deploy/ec2/docker-compose.server.yml logs -f go-api

  5. Update later:
       ./deploy/ec2/update.sh

DONE
