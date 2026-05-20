#!/usr/bin/env bash
# lockdown.sh — one-shot lockdown of an already-running Vultr demo box.
#
# Rewrites /opt/raven/docker-compose.override.yml on the target box to the
# locked-down baseline (no public port bindings), restarts go-api + frontend,
# and verifies the services are now only listening on 127.0.0.1.
#
# Context:
#   During the AWS→Vultr demo emergency, the override on the box temporarily
#   bound two services to 0.0.0.0 so the demo was reachable via the public
#   IP without going through Cloudflare:
#     - 0.0.0.0:13080:8080  (frontend)
#     - 0.0.0.0:18081:8081  (go-api)
#   Once the Cloudflare cutover (see docs/runbooks/demo-cutover.md, PR #632)
#   is complete, traffic flows through cloudflared again and those public
#   ports are no longer needed. This script removes them.
#
# Prerequisites:
#   - Cloudflare cutover must be DONE (runbook PR #632 merged AND executed).
#     If you run this BEFORE the cutover, the demo will be unreachable from
#     the public internet until cloudflared is in place.
#   - SSH access to the box as a user that can run docker compose (root).
#
# Usage:
#   bash deploy/vultr/lockdown.sh root@64.176.97.248
#
# Rollback:
#   The previous override is preserved as docker-compose.override.yml.bak-<ts>
#   on the box. To restore, on the box:
#     cd /opt/raven
#     mv docker-compose.override.yml.bak-<ts> docker-compose.override.yml
#     docker compose --env-file .env.server \
#       -f deploy/ec2/docker-compose.server.yml \
#       -f docker-compose.demo.yml \
#       -f docker-compose.override.yml \
#       up -d go-api frontend
#
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Usage: $0 <ssh-target>" >&2
  echo "Example: $0 root@64.176.97.248" >&2
  exit 2
fi

SSH_TARGET="$1"
SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10)

echo "[lockdown] target: $SSH_TARGET"
echo "[lockdown] rewriting /opt/raven/docker-compose.override.yml on box..."

# Remote script — runs on the Vultr box. Quoted heredoc so $vars are
# evaluated REMOTELY, not locally.
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" 'bash -se' <<'REMOTE'
set -euo pipefail

REPO_DIR="/opt/raven"
OVERRIDE="$REPO_DIR/docker-compose.override.yml"
TS="$(date -u +%Y%m%dT%H%M%SZ)"

if [ ! -d "$REPO_DIR" ]; then
  echo "[remote] $REPO_DIR missing — is this the right box?" >&2
  exit 1
fi

cd "$REPO_DIR"

# Back up whatever override is currently on disk (may or may not exist).
if [ -f "$OVERRIDE" ]; then
  cp -p "$OVERRIDE" "$OVERRIDE.bak-$TS"
  echo "[remote] backed up existing override to $OVERRIDE.bak-$TS"
else
  echo "[remote] no existing $OVERRIDE; writing fresh."
fi

# Write the locked-down baseline. NO ports: overrides → docker falls back
# to base compose's 127.0.0.1: bindings.
cat > "$OVERRIDE" <<'YAML'
# Locked-down compose override for the Vultr demo box.
# Managed by deploy/vultr/lockdown.sh — see that script for rationale.
# DO NOT add `ports:` overrides here; public ingress goes via cloudflared only.
services:
  go-api:
    entrypoint: ["/app/api"]
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8081/raven/healthz"]
YAML
echo "[remote] wrote locked-down $OVERRIDE"

# Restart only the two services whose port bindings changed. Other services
# (postgres, valkey, etc.) keep running uninterrupted.
ENV_FILE="$REPO_DIR/.env.server"
COMPOSE_ARGS=(
  --env-file "$ENV_FILE"
  -f deploy/ec2/docker-compose.server.yml
  -f docker-compose.demo.yml
  -f docker-compose.override.yml
)

echo "[remote] recreating go-api + frontend with new override..."
docker compose "${COMPOSE_ARGS[@]}" up -d --force-recreate --no-deps go-api frontend

# Give the containers a moment to settle before probing.
sleep 3

echo "[remote] verifying host port bindings (expect 127.0.0.1: only)..."
# `docker compose ps --format json` lists per-service. `ss` is the authoritative
# source for what the kernel is actually listening on.
if command -v ss >/dev/null 2>&1; then
  PROBE="ss -ltnH"
elif command -v netstat >/dev/null 2>&1; then
  PROBE="netstat -ltn"
else
  echo "[remote] neither ss nor netstat available; skipping kernel-level probe." >&2
  PROBE=""
fi

if [ -n "$PROBE" ]; then
  # Look at the ports we care about. Public bindings show as 0.0.0.0:<port>
  # or *:<port>; locked-down bindings show as 127.0.0.1:<port>.
  PORTS_OF_INTEREST="13080|18081|3080|8081"
  echo "[remote] listening sockets matching ($PORTS_OF_INTEREST):"
  $PROBE | grep -E ":($PORTS_OF_INTEREST)\\b" || echo "  (none — services may still be starting)"

  if $PROBE | grep -E "(0\\.0\\.0\\.0|\\*):(13080|18081)\\b" >/dev/null 2>&1; then
    echo "[remote] FAIL: still listening on 0.0.0.0 for 13080/18081." >&2
    echo "[remote] docker may need a full 'compose down/up' cycle to drop the old bindings." >&2
    exit 1
  fi

  if $PROBE | grep -E "(0\\.0\\.0\\.0|\\*):(3080|8081)\\b" >/dev/null 2>&1; then
    echo "[remote] WARN: 3080 or 8081 bound on 0.0.0.0 (expected 127.0.0.1)." >&2
    echo "[remote] Check base compose vs override for unexpected ports: overrides." >&2
  fi
fi

echo "[remote] docker compose ps (go-api, frontend):"
docker compose "${COMPOSE_ARGS[@]}" ps go-api frontend

echo "[remote] lockdown complete on box."
REMOTE

cat <<'OPERATOR'

[lockdown] DONE on box. Operator-side verification (run from your laptop):

  1. Public ports must REJECT/timeout (no inbound exposure):
       nc -zv -w 5 64.176.97.248 13080   # expect: refused / timeout
       nc -zv -w 5 64.176.97.248 18081   # expect: refused / timeout

  2. Cloudflare path must still serve the demo:
       curl -sSf https://demo.ravencloak.org/raven/healthz
       curl -sSf -I https://demo.ravencloak.org/raven/

  3. (Optional) SSH tunnel to hit services locally if you need to debug:
       ssh -N -L 8081:127.0.0.1:8081 root@64.176.97.248
       # then in another shell:
       curl -sSf http://127.0.0.1:8081/raven/healthz

If step (2) fails, cloudflared on the box may not be running.
Check on box: systemctl status cloudflared

To roll back, on the box:
  cd /opt/raven
  ls -1t docker-compose.override.yml.bak-*  # pick the most recent
  mv docker-compose.override.yml.bak-<TS> docker-compose.override.yml
  docker compose --env-file .env.server \
    -f deploy/ec2/docker-compose.server.yml \
    -f docker-compose.demo.yml \
    -f docker-compose.override.yml \
    up -d --force-recreate --no-deps go-api frontend

OPERATOR
