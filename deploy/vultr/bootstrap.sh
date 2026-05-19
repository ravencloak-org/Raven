#!/usr/bin/env bash
# bootstrap.sh — one-shot provisioning for a fresh Ubuntu Vultr instance.
#
# Brings the box from "bare Ubuntu" to "ready to run docker compose":
#   - Docker Engine + Compose plugin (official get.docker.com script)
#   - /opt/raven repo clone (the working tree the deploy steps live in)
#   - cloudflared systemd unit pointing at /etc/cloudflared/config.yml
#
# Idempotent — re-runnable. Skips work that's already done.
#
# Manual prerequisites (NOT done by this script — they need secrets):
#   1. /opt/raven/.env.server — populate from .env.server.example
#   2. /etc/cloudflared/<tunnel_id>.json — extract via fetch-tunnel-creds.sh
#      from AWS SSM, scp to box, chmod 600.
#   3. /etc/cloudflared/config.yml — written by this script using
#      CLOUDFLARE_TUNNEL_ID env (must be set before running).
#
# Usage:
#   sudo CLOUDFLARE_TUNNEL_ID=<uuid> bash bootstrap.sh
#
set -euo pipefail

if [ "${EUID:-0}" -ne 0 ]; then
  echo "Run as root (this script installs system packages)." >&2
  exit 1
fi

REPO_URL="${REPO_URL:-https://github.com/ravencloak-org/Raven.git}"
REPO_DIR="${REPO_DIR:-/opt/raven}"
CF_TUNNEL_ID="${CLOUDFLARE_TUNNEL_ID:-}"

# ─── Docker Engine + Compose ──────────────────────────────────────────────────
if ! command -v docker >/dev/null 2>&1; then
  echo "[bootstrap] installing docker via get.docker.com..."
  curl -fsSL https://get.docker.com | sh
else
  echo "[bootstrap] docker present: $(docker --version)"
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "[bootstrap] installing docker compose plugin..."
  apt-get update -y
  apt-get install -y docker-compose-plugin
else
  echo "[bootstrap] docker compose plugin present: $(docker compose version --short)"
fi

systemctl enable --now docker

# ─── Repo clone ───────────────────────────────────────────────────────────────
if [ ! -d "$REPO_DIR/.git" ]; then
  echo "[bootstrap] cloning $REPO_URL into $REPO_DIR..."
  git clone "$REPO_URL" "$REPO_DIR"
else
  echo "[bootstrap] $REPO_DIR already a git checkout; fetching latest..."
  git -C "$REPO_DIR" fetch --tags --prune
fi

# ─── Cloudflared systemd unit ─────────────────────────────────────────────────
if [ -z "$CF_TUNNEL_ID" ]; then
  echo "[bootstrap] CLOUDFLARE_TUNNEL_ID not set; skipping cloudflared config." >&2
  echo "[bootstrap] re-run with CLOUDFLARE_TUNNEL_ID=<uuid> to wire the tunnel." >&2
else
  install -d -m 700 /etc/cloudflared
  cat > /etc/cloudflared/config.yml <<EOF
tunnel: ${CF_TUNNEL_ID}
credentials-file: /etc/cloudflared/${CF_TUNNEL_ID}.json
# Ingress rules are managed remotely via Cloudflare's API (terraform-controlled
# in deploy/terraform/demo/cloudflare.tf). No local ingress config needed.
EOF
  if [ ! -f "/etc/cloudflared/${CF_TUNNEL_ID}.json" ]; then
    echo "[bootstrap] WARN: /etc/cloudflared/${CF_TUNNEL_ID}.json missing." >&2
    echo "[bootstrap] Extract it from AWS SSM via deploy/vultr/fetch-tunnel-creds.sh" >&2
    echo "[bootstrap] then scp it to this box (chmod 600) before starting cloudflared." >&2
  fi
  cat > /etc/systemd/system/cloudflared.service <<'EOF'
[Unit]
Description=Cloudflare Tunnel
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/cloudflared --config /etc/cloudflared/config.yml --no-autoupdate tunnel run
Restart=on-failure
User=root

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable cloudflared
  if [ -f "/etc/cloudflared/${CF_TUNNEL_ID}.json" ]; then
    systemctl restart cloudflared
    echo "[bootstrap] cloudflared started; check: systemctl status cloudflared"
  else
    echo "[bootstrap] cloudflared NOT started (waiting for credentials JSON)."
  fi
fi

# ─── Final reminders ──────────────────────────────────────────────────────────
ENV_FILE="$REPO_DIR/.env.server"
if [ ! -f "$ENV_FILE" ]; then
  cp "$REPO_DIR/deploy/vultr/.env.server.example" "$ENV_FILE"
  echo "[bootstrap] WROTE template $ENV_FILE — populate secrets before deploying."
fi

echo
echo "[bootstrap] DONE. Next steps:"
echo "  1. populate $ENV_FILE with real secrets (POSTGRES_PASSWORD, SUPERTOKENS_API_KEY, etc.)"
echo "  2. place /etc/cloudflared/<tunnel_id>.json (chmod 600) if not already there"
echo "  3. trigger the demo-deploy workflow OR run: bash $REPO_DIR/deploy/vultr/update.sh --tag <ver>"
