#!/usr/bin/env bash
# ─── Raven pgBackRest Backup Script ──────────────────────────────────────────
# Usage:
#   ./backup.sh              # incremental backup (default)
#   ./backup.sh full         # full backup
#   ./backup.sh diff         # differential backup
#   ./backup.sh incr         # incremental backup
#
# This script is designed to run inside the pgbackrest sidecar container
# or be invoked via:
#   docker compose exec pgbackrest /backup/backup.sh [full|diff|incr]
#
# Cron example (host crontab, daily full at 02:00, hourly incremental):
#   0  2 * * * docker compose -f /path/to/docker-compose.yml exec -T pgbackrest /backup/backup.sh full
#   0  * * * * docker compose -f /path/to/docker-compose.yml exec -T pgbackrest /backup/backup.sh incr
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

STANZA="raven"
BACKUP_TYPE="${1:-incr}"
LOG_PREFIX="[raven-backup]"

# Validate backup type
case "${BACKUP_TYPE}" in
  full|diff|incr) ;;
  *)
    echo "${LOG_PREFIX} ERROR: Invalid backup type '${BACKUP_TYPE}'. Use: full, diff, incr" >&2
    exit 1
    ;;
esac

echo "${LOG_PREFIX} Starting ${BACKUP_TYPE} backup for stanza '${STANZA}' at $(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Ensure the stanza is initialized (idempotent — safe to call every time)
pgbackrest --stanza="${STANZA}" stanza-create 2>/dev/null || true

# Run the backup
pgbackrest --stanza="${STANZA}" --type="${BACKUP_TYPE}" backup

echo "${LOG_PREFIX} Backup completed successfully at $(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Show backup info
echo "${LOG_PREFIX} Current backup inventory:"
pgbackrest --stanza="${STANZA}" info

# Expire old backups based on retention policy in pgbackrest.conf
pgbackrest --stanza="${STANZA}" expire

echo "${LOG_PREFIX} Expiry complete. Done."
