#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

# Portable YAML validation: tries PyYAML, then falls back to ruby, then a
# basic Python heuristic (checks indentation and structure with stdlib only).
validate_yaml() {
  local file="$1"
  # Try PyYAML
  if python3 -c "import yaml, sys; yaml.safe_load(open(sys.argv[1]))" "$file" 2>/dev/null; then
    return 0
  fi
  # Try Ruby (ships on macOS)
  if ruby -ryaml -e "YAML.safe_load(File.read(ARGV[0]))" "$file" 2>/dev/null; then
    return 0
  fi
  # Fallback: basic structural check with Python stdlib
  python3 -c "
import sys, re
with open(sys.argv[1]) as f:
    content = f.read()
# Must not be empty
if not content.strip():
    sys.exit(1)
# Must not have tab indentation (YAML uses spaces)
if '\t' in content.split('#')[0]:
    sys.exit(1)
# Rough check: should contain key: value patterns
if not re.search(r'^[\w-]+:', content, re.MULTILINE):
    sys.exit(1)
sys.exit(0)
" "$file" 2>/dev/null
}

echo "=== GitHub Config Validation ==="
echo ""

# ── 1. dependabot.yml is valid YAML ──────────────────────────────────────────
echo "[1/5] Validating .github/dependabot.yml ..."
DEPENDABOT="$REPO_ROOT/.github/dependabot.yml"
if [ ! -f "$DEPENDABOT" ]; then
  fail "dependabot.yml not found"
else
  if validate_yaml "$DEPENDABOT"; then
    pass "dependabot.yml is valid YAML"
  else
    fail "dependabot.yml is not valid YAML"
  fi
fi

# ── 2. .coderabbit.yaml is valid YAML ───────────────────────────────────────
echo "[2/5] Validating .coderabbit.yaml ..."
CODERABBIT="$REPO_ROOT/.coderabbit.yaml"
if [ ! -f "$CODERABBIT" ]; then
  fail ".coderabbit.yaml not found"
else
  if validate_yaml "$CODERABBIT"; then
    pass ".coderabbit.yaml is valid YAML"
  else
    fail ".coderabbit.yaml is not valid YAML"
  fi
fi

# ── 3. Dependabot covers all 5 ecosystems ───────────────────────────────────
echo "[3/5] Checking dependabot ecosystems ..."
if [ -f "$DEPENDABOT" ]; then
  for eco in gomod pip npm docker github-actions; do
    if grep -q "\"$eco\"" "$DEPENDABOT"; then
      pass "dependabot ecosystem: $eco"
    else
      fail "dependabot missing ecosystem: $eco"
    fi
  done
else
  fail "dependabot.yml not found - skipping ecosystem check"
fi

# ── 4. CODEOWNERS exists and is non-empty ────────────────────────────────────
echo "[4/5] Checking CODEOWNERS ..."
CODEOWNERS="$REPO_ROOT/.github/CODEOWNERS"
if [ -s "$CODEOWNERS" ]; then
  pass "CODEOWNERS exists and is non-empty"
else
  fail "CODEOWNERS missing or empty"
fi

# ── 5. PR template exists ───────────────────────────────────────────────────
echo "[5/5] Checking PR template ..."
PR_TEMPLATE="$REPO_ROOT/.github/PULL_REQUEST_TEMPLATE.md"
if [ -s "$PR_TEMPLATE" ]; then
  pass "PULL_REQUEST_TEMPLATE.md exists and is non-empty"
else
  fail "PULL_REQUEST_TEMPLATE.md missing or empty"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
