#!/usr/bin/env bash
# validate-migrations.sh
# Validates goose SQL migration files in the migrations/ directory.
#   - Every file has both +goose Up and +goose Down sections
#   - File numbering is sequential (00001, 00002, ...)
#   - No duplicate migration numbers

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIGRATIONS_DIR="${SCRIPT_DIR}/../migrations"

if [ ! -d "$MIGRATIONS_DIR" ]; then
  echo "ERROR: migrations directory not found at $MIGRATIONS_DIR"
  exit 1
fi

errors=0

# Collect all .sql files sorted
mapfile -t files < <(find "$MIGRATIONS_DIR" -maxdepth 1 -name '*.sql' -type f | sort)

if [ ${#files[@]} -eq 0 ]; then
  echo "ERROR: no SQL migration files found in $MIGRATIONS_DIR"
  exit 1
fi

echo "=== Validating ${#files[@]} migration files ==="
echo ""

# 1. Check each file has both +goose Up and +goose Down
echo "--- Checking goose directives ---"
for f in "${files[@]}"; do
  base="$(basename "$f")"
  has_up=$(grep -c '^\-\- +goose Up' "$f" || true)
  has_down=$(grep -c '^\-\- +goose Down' "$f" || true)

  if [ "$has_up" -eq 0 ]; then
    echo "ERROR: $base is missing '-- +goose Up'"
    errors=$((errors + 1))
  fi
  if [ "$has_down" -eq 0 ]; then
    echo "ERROR: $base is missing '-- +goose Down'"
    errors=$((errors + 1))
  fi
  if [ "$has_up" -gt 0 ] && [ "$has_down" -gt 0 ]; then
    echo "  OK: $base"
  fi
done
echo ""

# 2. Extract migration numbers and check for sequential ordering
echo "--- Checking sequential numbering ---"
numbers=()
for f in "${files[@]}"; do
  base="$(basename "$f")"
  # Extract leading digits from filename (e.g. 00001 from 00001_extensions_and_types.sql)
  num="${base%%_*}"
  numbers+=("$num")
done

# 3. Check for duplicate numbers
echo "--- Checking for duplicate numbers ---"
dupes=$(printf '%s\n' "${numbers[@]}" | sort | uniq -d)
if [ -n "$dupes" ]; then
  echo "ERROR: duplicate migration numbers found: $dupes"
  errors=$((errors + 1))
else
  echo "  OK: no duplicate numbers"
fi
echo ""

# 4. Check sequential (no gaps)
echo "--- Checking for gaps ---"
prev=0
for num in "${numbers[@]}"; do
  # Strip leading zeros for arithmetic
  n=$((10#$num))
  expected=$((prev + 1))
  if [ "$n" -ne "$expected" ]; then
    echo "ERROR: expected migration $(printf '%05d' $expected) but found $(printf '%05d' $n) — gap detected"
    errors=$((errors + 1))
  fi
  prev=$n
done
if [ "$errors" -eq 0 ]; then
  echo "  OK: numbering is sequential (1 through $prev)"
fi
echo ""

# Summary
echo "=== Validation complete ==="
if [ "$errors" -gt 0 ]; then
  echo "FAILED: $errors error(s) found"
  exit 1
else
  echo "PASSED: all checks OK"
  exit 0
fi
