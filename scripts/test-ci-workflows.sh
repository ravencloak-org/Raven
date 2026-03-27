#!/usr/bin/env bash
# test-ci-workflows.sh - Validate GitHub Actions workflow files
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORKFLOWS_DIR="$REPO_ROOT/.github/workflows"
PASS=0
FAIL=0

green()  { printf '\033[0;32m%s\033[0m\n' "$1"; }
red()    { printf '\033[0;31m%s\033[0m\n' "$1"; }
yellow() { printf '\033[0;33m%s\033[0m\n' "$1"; }

pass() { PASS=$((PASS + 1)); green "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); red   "  FAIL: $1"; }

echo "============================================"
echo " CI Workflow Validation"
echo "============================================"
echo ""

# ── 1. Check all .yml files are valid YAML ───────────────────────────────────
echo "--- Checking YAML validity ---"
for f in "$WORKFLOWS_DIR"/*.yml; do
  name="$(basename "$f")"
  if python3 -c "import yaml, sys; yaml.safe_load(open(sys.argv[1]))" "$f" 2>/dev/null; then
    pass "$name is valid YAML"
  else
    fail "$name is NOT valid YAML"
  fi
done
echo ""

# ── 2. Each workflow has on: trigger ──────────────────────────────────────────
echo "--- Checking for 'on:' triggers ---"
for f in "$WORKFLOWS_DIR"/*.yml; do
  name="$(basename "$f")"
  if python3 -c "
import yaml, sys
data = yaml.safe_load(open(sys.argv[1]))
assert data is not None and True in data or 'on' in data
" "$f" 2>/dev/null; then
    pass "$name has 'on:' trigger"
  else
    fail "$name missing 'on:' trigger"
  fi
done
echo ""

# ── 3. Each workflow has at least one jobs: section ───────────────────────────
echo "--- Checking for 'jobs:' section ---"
for f in "$WORKFLOWS_DIR"/*.yml; do
  name="$(basename "$f")"
  if python3 -c "
import yaml, sys
data = yaml.safe_load(open(sys.argv[1]))
assert data is not None and 'jobs' in data and len(data['jobs']) >= 1
" "$f" 2>/dev/null; then
    pass "$name has 'jobs:' with at least one job"
  else
    fail "$name missing 'jobs:' section or has no jobs"
  fi
done
echo ""

# ── 4. Go workflow uses go-version '1.26' ────────────────────────────────────
echo "--- Checking Go version ---"
GO_WF="$WORKFLOWS_DIR/go.yml"
if [ -f "$GO_WF" ]; then
  if python3 -c "
import yaml, sys
data = yaml.safe_load(open(sys.argv[1]))
found = False
for job in data.get('jobs', {}).values():
    for step in job.get('steps', []):
        w = step.get('with', {})
        if w.get('go-version') == '1.26':
            found = True
assert found
" "$GO_WF" 2>/dev/null; then
    pass "go.yml uses go-version '1.26'"
  else
    fail "go.yml does NOT use go-version '1.26'"
  fi
else
  fail "go.yml not found"
fi
echo ""

# ── 5. Python workflow uses python-version '3.12' ────────────────────────────
echo "--- Checking Python version ---"
PY_WF="$WORKFLOWS_DIR/python.yml"
if [ -f "$PY_WF" ]; then
  if python3 -c "
import yaml, sys
data = yaml.safe_load(open(sys.argv[1]))
found = False
for job in data.get('jobs', {}).values():
    for step in job.get('steps', []):
        w = step.get('with', {})
        if w.get('python-version') == '3.12':
            found = True
assert found
" "$PY_WF" 2>/dev/null; then
    pass "python.yml uses python-version '3.12'"
  else
    fail "python.yml does NOT use python-version '3.12'"
  fi
else
  fail "python.yml not found"
fi
echo ""

# ── 6. Frontend workflow uses node-version '22' ──────────────────────────────
echo "--- Checking Node version ---"
FE_WF="$WORKFLOWS_DIR/frontend.yml"
if [ -f "$FE_WF" ]; then
  if python3 -c "
import yaml, sys
data = yaml.safe_load(open(sys.argv[1]))
found = False
for job in data.get('jobs', {}).values():
    for step in job.get('steps', []):
        w = step.get('with', {})
        if str(w.get('node-version')) == '22':
            found = True
assert found
" "$FE_WF" 2>/dev/null; then
    pass "frontend.yml uses node-version '22'"
  else
    fail "frontend.yml does NOT use node-version '22'"
  fi
else
  fail "frontend.yml not found"
fi
echo ""

# ── 7. No hardcoded secrets ──────────────────────────────────────────────────
echo "--- Checking for hardcoded secrets ---"
SECRET_PATTERNS='(ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{82}|AKIA[0-9A-Z]{16}|sk-[A-Za-z0-9]{48}|password\s*[:=]\s*["\x27][^"\x27]{8,}|token\s*[:=]\s*["\x27][^"\x27]{8,}|secret\s*[:=]\s*["\x27][^"\x27]{8,})'
SECRETS_FOUND=0
for f in "$WORKFLOWS_DIR"/*.yml; do
  name="$(basename "$f")"
  if grep -qEi "$SECRET_PATTERNS" "$f" 2>/dev/null; then
    fail "$name contains potential hardcoded secrets"
    SECRETS_FOUND=1
  fi
done
if [ "$SECRETS_FOUND" -eq 0 ]; then
  pass "No hardcoded secrets found in workflow files"
fi
echo ""

# ── Summary ──────────────────────────────────────────────────────────────────
echo "============================================"
echo " Results: $PASS passed, $FAIL failed"
echo "============================================"

if [ "$FAIL" -gt 0 ]; then
  red "Some checks failed!"
  exit 1
else
  green "All checks passed!"
  exit 0
fi
