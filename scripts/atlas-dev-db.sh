#!/usr/bin/env bash
# Spin an ephemeral pgvector-capable Postgres for Atlas schema tooling and
# print two connection URLs on stdout (one per line):
#
#   line 1:  GOOSE_URL  — empty DB into which the caller applies goose migrations
#   line 2:  ATLAS_DEV  — a separate CLEAN DB (extensions + roles, no tables)
#                         that Atlas uses as its dev-url scratch space
#
# Atlas needs a CLEAN dev database to materialise/compare schema states. It
# does NOT drop pre-existing extensions or cluster-global roles, so we
# pre-create uuid-ossp / vector / pg_trgm and raven_app / raven_admin there —
# otherwise the inspected schema (which uses uuid_generate_v4(), vector(768),
# and `FOR ALL TO raven_admin` policies) cannot be replayed on the dev DB.
#
# Atlas's built-in `docker://postgres/N` dev images are STOCK Postgres with no
# pgvector, so we cannot use them; this script uses pgvector/pgvector:pg18.
#
# Caller is responsible for tearing the container down (the Makefile traps it).
# All diagnostics go to stderr so stdout carries only the two URLs.
set -euo pipefail

CONTAINER="${ATLAS_DEV_CONTAINER:-raven-atlas-dev}"
IMAGE="${ATLAS_DEV_IMAGE:-pgvector/pgvector:pg18}"
HOST_PORT="${ATLAS_DEV_PORT:-5456}"
PGPASS="${ATLAS_DEV_PASSWORD:-pass}"
RUNTIME="${ATLAS_DEV_RUNTIME:-podman}"

log() { echo "atlas-dev-db: $*" >&2; }

# Resolve a runtime that exists (podman locally, docker in CI).
if ! command -v "$RUNTIME" >/dev/null 2>&1; then
  if command -v docker >/dev/null 2>&1; then RUNTIME=docker
  elif command -v podman >/dev/null 2>&1; then RUNTIME=podman
  else log "no container runtime (podman/docker) found"; exit 1; fi
fi
log "using runtime: $RUNTIME"

# Podman on this Mac wires a broken credential helper for docker.io; an empty
# authfile forces an anonymous pull of the public pgvector image. Harmless on
# docker / in CI where credentials already work.
AUTHFILE_ARGS=()
if [ "$RUNTIME" = "podman" ]; then
  AUTH_DIR="$(mktemp -d)"; echo '{}' > "$AUTH_DIR/auth.json"
  AUTHFILE_ARGS=(--authfile "$AUTH_DIR/auth.json")
fi

"$RUNTIME" rm -f "$CONTAINER" >/dev/null 2>&1 || true
log "starting $IMAGE on 127.0.0.1:$HOST_PORT ..."
"$RUNTIME" run -d --name "$CONTAINER" "${AUTHFILE_ARGS[@]}" \
  -e POSTGRES_PASSWORD="$PGPASS" \
  -p "127.0.0.1:${HOST_PORT}:5432" \
  "$IMAGE" >/dev/null

BASE="postgres://postgres:${PGPASS}@127.0.0.1:${HOST_PORT}"
export PGPASSWORD="$PGPASS"
for i in $(seq 1 60); do
  if psql -h 127.0.0.1 -p "$HOST_PORT" -U postgres -tc 'SELECT 1' >/dev/null 2>&1; then
    log "ready after ${i}s"; break
  fi
  if [ "$i" = 60 ]; then log "postgres never became ready"; exit 1; fi
  sleep 1
done

# Clean scratch DB for Atlas, pre-seeded with the extensions + roles the
# inspected schema references. Run each statement separately so a pre-existing
# cluster-global role does not abort the extension creation.
psql -h 127.0.0.1 -p "$HOST_PORT" -U postgres -c 'CREATE DATABASE atlas_dev;' >/dev/null
psql -h 127.0.0.1 -p "$HOST_PORT" -U postgres -d atlas_dev -c 'CREATE EXTENSION IF NOT EXISTS "uuid-ossp";' >/dev/null
psql -h 127.0.0.1 -p "$HOST_PORT" -U postgres -d atlas_dev -c 'CREATE EXTENSION IF NOT EXISTS "vector";'    >/dev/null
psql -h 127.0.0.1 -p "$HOST_PORT" -U postgres -d atlas_dev -c 'CREATE EXTENSION IF NOT EXISTS "pg_trgm";'   >/dev/null
psql -h 127.0.0.1 -p "$HOST_PORT" -U postgres -d atlas_dev \
  -c "DO \$\$ BEGIN
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='raven_app')   THEN CREATE ROLE raven_app;   END IF;
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='raven_admin') THEN CREATE ROLE raven_admin; END IF;
      END \$\$;" >/dev/null

# Stdout: the two URLs the Makefile consumes.
echo "${BASE}/postgres?sslmode=disable&search_path=public"
echo "${BASE}/atlas_dev?sslmode=disable&search_path=public"
