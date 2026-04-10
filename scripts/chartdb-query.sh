#!/usr/bin/env bash
# Emit the ChartDB PostgreSQL smart-query result as JSON.
#
# Usage:
#   ./scripts/chartdb-query.sh
#   ./scripts/chartdb-query.sh | pbcopy     # copy to clipboard (macOS)
#
# The script reads DATABASE_URL from the environment (or .env if present).
# Then paste the JSON into the ChartDB UI at http://localhost:3000.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# Load .env if DATABASE_URL is not already set
if [[ -z "${DATABASE_URL:-}" && -f "$ROOT_DIR/.env" ]]; then
  # shellcheck disable=SC1090
  set -o allexport
  source "$ROOT_DIR/.env"
  set +o allexport
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "Error: DATABASE_URL is not set." >&2
  echo "Either export it or create a .env file with DATABASE_URL=postgresql://..." >&2
  exit 1
fi

SQL_FILE="$SCRIPT_DIR/chartdb-query.sql"

psql "$DATABASE_URL" \
  --no-psqlrc \
  --tuples-only \
  --no-align \
  --quiet \
  --file="$SQL_FILE"
