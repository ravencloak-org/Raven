#!/bin/bash
set -euo pipefail

S3_BUCKET="${1:-raven-demo-backups}"
DATE=$(date -u +%Y-%m-%dT%H-%M-%SZ)
BACKUP_NAME="clickhouse-$DATE"

docker exec raven-clickhouse-1 clickhouse-backup create "$BACKUP_NAME"
docker exec raven-clickhouse-1 clickhouse-backup upload "$BACKUP_NAME"
docker exec raven-clickhouse-1 clickhouse-backup delete local "$BACKUP_NAME"

echo "OK: uploaded clickhouse $BACKUP_NAME"
