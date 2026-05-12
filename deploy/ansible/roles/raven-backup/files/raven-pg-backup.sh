#!/bin/bash
set -euo pipefail

S3_BUCKET="${1:-raven-demo-backups}"
DATE=$(date -u +%Y-%m-%dT%H-%M-%SZ)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

DUMP="$TMP/postgres-$DATE.dump"

docker exec raven-postgres-1 pg_dump -U raven -d raven -Fc > "$DUMP"

if [ ! -s "$DUMP" ]; then
  echo "ERROR: pg_dump produced an empty file" >&2
  exit 1
fi

aws s3 cp "$DUMP" "s3://$S3_BUCKET/postgres/postgres-$DATE.dump"
echo "OK: uploaded postgres/postgres-$DATE.dump"
