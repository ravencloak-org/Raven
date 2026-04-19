#!/usr/bin/env bash
# Generate a merged Go coverage report across unit, integration, and (optionally)
# instrumented-binary runs, using the Go 1.20+ coverage-directory format so
# multiple runs can be merged with `go tool covdata`.
#
# Outputs:
#   .coverdata/unit/          raw coverage from `go test ./...`
#   .coverdata/integration/   raw coverage from `go test -tags=integration`
#   .coverdata/binary/        raw coverage from any instrumented-binary runs
#                             (populated externally; see `make build-cover`)
#   coverage/cov.out          merged legacy textfmt profile
#   coverage/coverage.html    browsable HTML report
#   coverage/summary.txt      per-package percent summary
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COV_ROOT="${COV_ROOT:-$ROOT/.coverdata}"
OUT_DIR="${OUT_DIR:-$ROOT/coverage}"
COV_PKGS="${COV_PKGS:-github.com/ravencloak-org/Raven/internal/...,github.com/ravencloak-org/Raven/pkg/...,github.com/ravencloak-org/Raven/cmd/...}"

SKIP_UNIT="${SKIP_UNIT:-0}"
SKIP_INTEGRATION="${SKIP_INTEGRATION:-0}"

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }

mkdir -p "$COV_ROOT/unit" "$COV_ROOT/integration" "$COV_ROOT/binary" "$OUT_DIR"

if [ "$SKIP_UNIT" != "1" ]; then
  log "Running unit tests with coverage into $COV_ROOT/unit"
  rm -f "$COV_ROOT/unit"/cov*
  if command -v dotenvx >/dev/null 2>&1 && [ -f .env.ci ]; then
    dotenvx run -f .env.ci --quiet -- \
      go test -cover -coverpkg="$COV_PKGS" -timeout 30m ./... \
      -args -test.gocoverdir="$COV_ROOT/unit"
  else
    go test -cover -coverpkg="$COV_PKGS" -timeout 30m ./... \
      -args -test.gocoverdir="$COV_ROOT/unit"
  fi
fi

if [ "$SKIP_INTEGRATION" != "1" ]; then
  log "Running integration tests with coverage into $COV_ROOT/integration"
  rm -f "$COV_ROOT/integration"/cov*
  go test -tags=integration -cover -coverpkg="$COV_PKGS" \
    -timeout 10m ./internal/integration/ \
    -args -test.gocoverdir="$COV_ROOT/integration"
fi

INPUTS=()
for d in unit integration binary; do
  if compgen -G "$COV_ROOT/$d/cov*" >/dev/null; then
    INPUTS+=("$COV_ROOT/$d")
  fi
done

if [ "${#INPUTS[@]}" -eq 0 ]; then
  echo "no coverage data found under $COV_ROOT" >&2
  exit 1
fi

IFS=','; INPUT_CSV="${INPUTS[*]}"; unset IFS

log "Merging coverage from: $INPUT_CSV"
go tool covdata textfmt -i="$INPUT_CSV" -o="$OUT_DIR/cov.out"

log "Writing HTML report to $OUT_DIR/coverage.html"
go tool cover -html="$OUT_DIR/cov.out" -o "$OUT_DIR/coverage.html"

log "Writing per-package summary to $OUT_DIR/summary.txt"
go tool covdata percent -i="$INPUT_CSV" | tee "$OUT_DIR/summary.txt"

TOTAL="$(go tool cover -func="$OUT_DIR/cov.out" | awk '/^total:/ {print $3}')"
log "Total coverage: ${TOTAL:-unknown}"
