#!/usr/bin/env bash
# update.sh — pull latest images and restart the Raven stack on EC2.
#
# Usage:
#   ./deploy/ec2/update.sh
#   ./deploy/ec2/update.sh --sha abc1234   # pin to a specific commit SHA

set -euo pipefail

RAVEN_SOCK="unix:///run/raven/docker.sock"
COMPOSE="docker -H ${RAVEN_SOCK} compose"
ENV_FILE=".env.server"
COMPOSE_FILE="deploy/ec2/docker-compose.server.yml"

SHA="${1:-}"
if [[ "$SHA" == "--sha" ]]; then
  SHA="$2"
  GO_API_IMAGE="ghcr.io/ravencloak-org/go-api:${SHA}"
  PYTHON_WORKER_IMAGE="ghcr.io/ravencloak-org/python-worker:${SHA}"
  # Patch env file
  sed -i "s|^GO_API_IMAGE=.*|GO_API_IMAGE=${GO_API_IMAGE}|" "$ENV_FILE"
  sed -i "s|^PYTHON_WORKER_IMAGE=.*|PYTHON_WORKER_IMAGE=${PYTHON_WORKER_IMAGE}|" "$ENV_FILE"
  echo "Pinned to SHA: ${SHA}"
fi

echo "Pulling latest images..."
$COMPOSE -f "$COMPOSE_FILE" --env-file "$ENV_FILE" pull

echo "Restarting stack..."
$COMPOSE -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --remove-orphans

echo "Stack status:"
$COMPOSE -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps
