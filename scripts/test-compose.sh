#!/usr/bin/env bash
# ─── test-compose.sh ─────────────────────────────────────────────────────────
# Validates Docker Compose configuration, .env.example completeness,
# and Dockerfile health-check declarations.
# Exit on first failure.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); printf "  \033[32mPASS\033[0m  %s\n" "$1"; }
fail() { FAIL=$((FAIL + 1)); printf "  \033[31mFAIL\033[0m  %s\n" "$1"; }

echo "=== Raven Docker Compose test suite ==="
echo ""

# ─── 1. Validate docker compose config (syntax check) ────────────────────────
echo "--- Compose syntax ---"

# Compose services reference env_file: .env — create a temporary one from the
# example so that `docker compose config` can interpolate variables.
TEMP_ENV=0
if [ ! -f "$REPO_ROOT/.env" ]; then
    cp "$REPO_ROOT/.env.example" "$REPO_ROOT/.env"
    TEMP_ENV=1
fi

cleanup_env() {
    if [ "$TEMP_ENV" -eq 1 ] && [ -f "$REPO_ROOT/.env" ]; then
        rm "$REPO_ROOT/.env"
    fi
}
trap cleanup_env EXIT

if docker compose -f "$REPO_ROOT/docker-compose.yml" config --quiet 2>/dev/null; then
    pass "docker-compose.yml is valid"
else
    fail "docker-compose.yml has syntax errors"
fi

if docker compose -f "$REPO_ROOT/docker-compose.edge.yml" config --quiet 2>/dev/null; then
    pass "docker-compose.edge.yml is valid"
else
    fail "docker-compose.edge.yml has syntax errors"
fi

echo ""

# ─── 2. Validate .env.example has all required vars ──────────────────────────
echo "--- .env.example completeness ---"

ENV_EXAMPLE="$REPO_ROOT/.env.example"
REQUIRED_VARS=(
    POSTGRES_USER
    POSTGRES_PASSWORD
    POSTGRES_DB
    DATABASE_URL
    VALKEY_URL
    KEYCLOAK_ADMIN
    KEYCLOAK_ADMIN_PASSWORD
    KC_DB_URL
    KC_DB_USERNAME
    KC_DB_PASSWORD
    RAVEN_SERVER_PORT
    RAVEN_DATABASE_URL
    RAVEN_VALKEY_URL
    RAVEN_GRPC_WORKER_ADDR
    SEAWEEDFS_FILER_URL
    ZO_ROOT_USER_EMAIL
    ZO_ROOT_USER_PASSWORD
    OTEL_EXPORTER_OTLP_ENDPOINT
)

for var in "${REQUIRED_VARS[@]}"; do
    if grep -q "^${var}=" "$ENV_EXAMPLE" 2>/dev/null; then
        pass ".env.example contains $var"
    else
        fail ".env.example missing $var"
    fi
done

echo ""

# ─── 3. Validate Dockerfiles have HEALTHCHECK or proper CMD ──────────────────
echo "--- Dockerfile health checks ---"

DOCKERFILE_API="$REPO_ROOT/Dockerfile"
DOCKERFILE_WORKER="$REPO_ROOT/ai-worker/Dockerfile"

if grep -q "HEALTHCHECK" "$DOCKERFILE_API" 2>/dev/null; then
    pass "Dockerfile (Go API) has HEALTHCHECK"
else
    fail "Dockerfile (Go API) missing HEALTHCHECK"
fi

if grep -q "CMD\|ENTRYPOINT" "$DOCKERFILE_WORKER" 2>/dev/null; then
    pass "ai-worker/Dockerfile has CMD or ENTRYPOINT"
else
    fail "ai-worker/Dockerfile missing CMD or ENTRYPOINT"
fi

echo ""

# ─── 4. Validate init.sql exists and has required extensions ─────────────────
echo "--- PostgreSQL init script ---"

INIT_SQL="$REPO_ROOT/deploy/postgres/init.sql"

if [ -f "$INIT_SQL" ]; then
    pass "deploy/postgres/init.sql exists"
else
    fail "deploy/postgres/init.sql missing"
fi

for ext in "uuid-ossp" "vector" "pg_trgm"; do
    if grep -q "$ext" "$INIT_SQL" 2>/dev/null; then
        pass "init.sql enables $ext"
    else
        fail "init.sql missing $ext extension"
    fi
done

echo ""

# ─── Summary ─────────────────────────────────────────────────────────────────
echo "=== Results: $PASS passed, $FAIL failed ==="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
