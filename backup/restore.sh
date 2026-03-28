#!/usr/bin/env bash
# ─── Raven pgBackRest Restore Script ────────────────────────────────────────
# Usage:
#   ./restore.sh                       # restore latest backup
#   ./restore.sh --target-time "..."   # point-in-time recovery (PITR)
#
# This script MUST be run when PostgreSQL is STOPPED.
# Typical workflow:
#   1. docker compose stop postgres
#   2. docker compose exec pgbackrest /backup/restore.sh [OPTIONS]
#   3. docker compose start postgres
#
# For PITR, pass any pgbackrest-compatible flags after the script name:
#   ./restore.sh --type=time --target="2026-03-28 12:00:00+00"
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

STANZA="raven"
LOG_PREFIX="[raven-restore]"

echo "${LOG_PREFIX} ================================================================"
echo "${LOG_PREFIX}  Raven PostgreSQL Restore"
echo "${LOG_PREFIX} ================================================================"
echo ""

# Safety check — ensure PostgreSQL is not running
if pg_isready -h postgres -p 5432 -q 2>/dev/null; then
  echo "${LOG_PREFIX} ERROR: PostgreSQL is still running!" >&2
  echo "${LOG_PREFIX} Stop PostgreSQL before restoring:" >&2
  echo "${LOG_PREFIX}   docker compose stop postgres" >&2
  exit 1
fi

echo "${LOG_PREFIX} PostgreSQL is stopped. Proceeding with restore..."
echo ""

# Show available backups before restoring
echo "${LOG_PREFIX} Available backups:"
pgbackrest --stanza="${STANZA}" info
echo ""

# Perform the restore — pass through any additional arguments (e.g. PITR flags)
echo "${LOG_PREFIX} Starting restore at $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "${LOG_PREFIX} Extra arguments: ${*:-<none>}"

pgbackrest --stanza="${STANZA}" \
  --delta \
  --link-all \
  "$@" \
  restore

echo ""
echo "${LOG_PREFIX} Restore completed at $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "${LOG_PREFIX} Start PostgreSQL to apply WAL recovery:"
echo "${LOG_PREFIX}   docker compose start postgres"
echo "${LOG_PREFIX} ================================================================"
