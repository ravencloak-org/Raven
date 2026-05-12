#!/usr/bin/env bash
# update.sh — pull latest images and restart the Raven stack on EC2.
#
# Usage:
#   ./deploy/ec2/update.sh
#   ./deploy/ec2/update.sh --sha abc1234     # pin to a specific commit SHA
#   ./deploy/ec2/update.sh --tag v0.3.0      # pin to a release tag

set -euo pipefail

RAVEN_SOCK="unix:///run/raven/docker.sock"
COMPOSE="docker -H ${RAVEN_SOCK} compose"
ENV_FILE=".env.server"
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

  sed -i "s|^GO_API_IMAGE=.*|GO_API_IMAGE=${GO_API_IMAGE}|" "$ENV_FILE"
  sed -i "s|^PYTHON_WORKER_IMAGE=.*|PYTHON_WORKER_IMAGE=${PYTHON_WORKER_IMAGE}|" "$ENV_FILE"
  if grep -q '^FRONTEND_IMAGE=' "$ENV_FILE"; then
    sed -i "s|^FRONTEND_IMAGE=.*|FRONTEND_IMAGE=${FRONTEND_IMAGE}|" "$ENV_FILE"
  else
    echo "FRONTEND_IMAGE=${FRONTEND_IMAGE}" >> "$ENV_FILE"
  fi
  echo "Pinned to ref: ${REF}"
fi

echo "Pulling latest images..."
$COMPOSE -f "$COMPOSE_FILE" --env-file "$ENV_FILE" pull

echo "Restarting stack..."
$COMPOSE -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --remove-orphans

echo "Stack status:"
$COMPOSE -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps
