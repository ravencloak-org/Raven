#!/usr/bin/env bash
# update.sh — pull latest images and restart the Raven stack on a Vultr box.
#
# Differences from deploy/ec2/update.sh:
#   - Uses the default docker daemon socket (Vultr runs a single daemon;
#     EC2 used a hidden secondary daemon at /run/raven/docker.sock).
#   - Targets deploy/vultr/docker-compose.server.yml.
#
# Usage:
#   ./deploy/vultr/update.sh
#   ./deploy/vultr/update.sh --tag v0.4.1   # pin to a release tag
#   ./deploy/vultr/update.sh --sha abc1234  # pin to a commit SHA
#
set -euo pipefail

COMPOSE="docker compose"
ENV_FILE=".env.server"
# The compose file in deploy/ec2/ is platform-agnostic — the EC2-vs-Vultr
# distinction was the secondary docker socket (an env var, not in the YAML).
# Reusing it here avoids drift between deploys.
COMPOSE_FILE="deploy/ec2/docker-compose.server.yml"

REF=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --sha)
      REF="sha-${2::7}"
      shift 2
      ;;
    --tag)
      REF="$2"
      shift 2
      ;;
    *)
      echo "Unknown arg: $1" >&2
      exit 2
      ;;
  esac
done

if [ -n "${REF}" ]; then
  GO_API_IMAGE="ghcr.io/ravencloak-org/go-api:${REF}"
  PYTHON_WORKER_IMAGE="ghcr.io/ravencloak-org/python-worker:${REF}"
  FRONTEND_IMAGE="ghcr.io/ravencloak-org/frontend:${REF}"

  set_or_append() {
    local key="$1" value="$2"
    if grep -q "^${key}=" "$ENV_FILE"; then
      sed -i "s|^${key}=.*|${key}=${value}|" "$ENV_FILE"
    else
      echo "${key}=${value}" >> "$ENV_FILE"
    fi
  }
  set_or_append GO_API_IMAGE "$GO_API_IMAGE"
  set_or_append PYTHON_WORKER_IMAGE "$PYTHON_WORKER_IMAGE"
  set_or_append FRONTEND_IMAGE "$FRONTEND_IMAGE"
  echo "Pinned to ref: ${REF}"
fi

echo "Pulling images..."
$COMPOSE -f "$COMPOSE_FILE" --env-file "$ENV_FILE" pull

echo "Restarting stack..."
$COMPOSE -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --remove-orphans

echo "Stack status:"
$COMPOSE -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps
