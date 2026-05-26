# Bytebase Tiered Adoption — Plan A: PR-time SQL Review + Edge Readiness

> **POST-EXECUTION PIVOT NOTE (2026-05-25):** Tasks 3–7 below were written assuming `bytebase/sql-review-action` existed as a server-less PR linter. It does not — `bytebase-action check` requires a running Bytebase server. During execution we pivoted to [`sbdchd/squawk-action@v2`](https://github.com/sbdchd/squawk-action) (a Postgres-specific server-less linter) and re-did Tasks 3–7. The high-level goal and `VerifyMigrationsState`/cmd-api wiring (Tasks 1 & 2) are unchanged. For the as-shipped configuration and tool details, read the updated design spec at `docs/superpowers/specs/2026-05-23-bytebase-tiered-design.md` and the runbook at `docs/runbooks/migrations.md`. The Task 3–7 text below is preserved as historical record of what was originally proposed.
>
> **Key as-shipped facts:**
> - PR-time linter: `sbdchd/squawk-action@v2` (not Bytebase). Config: `.squawk.toml` (TOML, not JSON). Fixtures: `.squawk/fixtures/bad/` (not `.bytebase/fixtures/bad/`).
> - PR workflow is **diff-based** (`git diff origin/$BASE...origin/$HEAD`), not full-tree — 271 pre-existing findings on the 43 shipped migrations are tracked in the spec as backlog, not blocking new PRs. Same convention as `golangci-lint --new-from-rev=HEAD` in this repo.
> - Escape hatch is inline `-- squawk-ignore <rule>` comments, not a `migration:approved-destructive` PR label (squawk has no concept of labels).
> - Makefile target is `make sql-lint-local` (not `sql-review-local`), uses `npx squawk-cli`.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the ship-anywhere half of the tiered adoption — PR-time SQL review against `migrations/*.sql`, plus a `VerifyMigrationsState` startup check that gates `cmd/api` readiness. No Bytebase server runtime is introduced in Plan A. This delivers value to both edge and cloud profiles today.

**Architecture (as shipped):** Two coordinated changes. (1) A new Go function `db.VerifyMigrationsState` compares the count of distinct currently-applied migration versions in `goose_db_version` (latest row per `version_id`) against the count of embedded `*.sql` files; called unconditionally from `cmd/api/main.go` after the existing `RunMigrations` block. (2) `.github/workflows/sql-review.yml` runs `sbdchd/squawk-action@v2` against PR-modified migration files only (diff-based) using `.squawk.toml` config. A self-test workflow exercises the policy against intentionally-bad fixtures so regressions in the rules surface immediately.

**Tech Stack:** Go 1.x, `github.com/pressly/goose/v3`, `github.com/lib/pq`, `github.com/testcontainers/testcontainers-go`, `github.com/stretchr/testify/require`, GitHub Actions, `sbdchd/squawk-action@v2`, TOML config file.

**Source spec:** `docs/superpowers/specs/2026-05-23-bytebase-tiered-design.md`

**Out of scope for Plan A (deferred to Plan B):** Bytebase server deployment, VCS provider config, dev→staging→prod pipeline, drift detection, Layer 3 & 4 integration tests, history-table override on cloud DBs, SSO integration. Plan B will be written when cloud-profile orchestration choices (Helm vs compose, SSO domain) are settled.

---

## File Structure

| File | Purpose | Action |
|---|---|---|
| `internal/db/migrate.go` | Add `VerifyMigrationsState` next to `RunMigrations` | Modify |
| `internal/db/migrate_test.go` | New unit + integration tests for `VerifyMigrationsState` | Create |
| `cmd/api/main.go` | Call `VerifyMigrationsState` after the existing migration block | Modify |
| `.bytebase/sql-review.json` | SQL review policy (rules + severities) | Create |
| `.bytebase/fixtures/bad/00001_dropped_table_no_label.sql` | Self-test fixture: destructive op without label | Create |
| `.bytebase/fixtures/bad/00002_pascal_case_column.sql` | Self-test fixture: naming violation | Create |
| `.bytebase/fixtures/bad/00003_truncate_in_migration.sql` | Self-test fixture: banned TRUNCATE | Create |
| `.github/workflows/sql-review.yml` | PR-time SQL review job | Create |
| `.github/workflows/sql-review-selftest.yml` | Push-to-main self-test of the policy | Create |
| `Makefile` | Add `sql-review-local` target | Modify |
| `docs/runbooks/migrations.md` | Migration SOP — authoring, escape-hatch label, troubleshooting | Create |

**Decomposition rationale:** `migrate.go` already owns the goose runner; `VerifyMigrationsState` is the natural sibling. Workflow self-test lives in its own file so its trigger (push-to-main) doesn't muddy the PR-time workflow. Fixtures live under `.bytebase/fixtures/` so they're discoverable next to the policy and obviously not real migrations.

---

## Task 1: Add `VerifyMigrationsState` (TDD)

**Files:**
- Modify: `internal/db/migrate.go`
- Create: `internal/db/migrate_test.go`

The function counts applied migrations in `goose_db_version` (excluding goose's seed row at `version_id=0`) and compares against the count of `*.sql` files embedded by `migrations.FS`. Mismatch returns a descriptive error.

- [ ] **Step 1: Write the failing test for the happy path**

Create `internal/db/migrate_test.go`:

```go
package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

func TestVerifyMigrationsState_AllAppliedPasses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// NewTestDB spins up Postgres + pgvector and runs every migration in
	// migrations/*.sql via goose.UpContext. After it returns, goose_db_version
	// should hold one row per file (plus the seed row at version_id=0).
	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

	require.NoError(t, db.VerifyMigrationsState(ctx, dsn))
}
```

- [ ] **Step 2: Run the test, verify it fails to compile (function does not exist)**

Run: `go test ./internal/db/... -run TestVerifyMigrationsState_AllAppliedPasses -v`
Expected: build error `undefined: db.VerifyMigrationsState`

- [ ] **Step 3: Implement `VerifyMigrationsState` in `internal/db/migrate.go`**

Append below `RunMigrations`:

```go
// VerifyMigrationsState confirms the database has applied exactly the set of
// migrations embedded in the binary. It is cheap (one COUNT(*) plus a directory
// walk of migrations.FS) and runs at startup in both edge and cloud profiles.
//
// In the edge profile this is a belt-and-braces check that RunMigrations did
// what it claimed. In the cloud profile (AutoMigrate=false, Bytebase applies
// migrations out-of-band) it gates /readyz: pods stay un-ready until Bytebase
// has caught up. Returning an error here surfaces a clear mismatch instead of
// silently serving traffic against a stale schema.
//
// Goose seeds the version table with version_id=0 on first use; that row is
// excluded from the applied count.
func VerifyMigrationsState(ctx context.Context, databaseURL string) error {
	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("open postgres for verify: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres for verify: %w", err)
	}

	var appliedCount int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM goose_db_version WHERE is_applied = true AND version_id > 0`,
	).Scan(&appliedCount); err != nil {
		return fmt.Errorf("count goose_db_version: %w", err)
	}

	expectedCount, err := countEmbeddedMigrations()
	if err != nil {
		return fmt.Errorf("count embedded migrations: %w", err)
	}

	if appliedCount != expectedCount {
		return fmt.Errorf(
			"migration state mismatch: %d applied in goose_db_version, %d expected from embedded migrations",
			appliedCount, expectedCount,
		)
	}
	return nil
}

func countEmbeddedMigrations() (int, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			count++
		}
	}
	return count, nil
}
```

Add the new imports to the existing `import ( ... )` block in `migrate.go`:

```go
import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"      // NEW
	"strings"    // NEW

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"

	"github.com/ravencloak-org/Raven/migrations"
)
```

- [ ] **Step 4: Run the happy-path test, verify it passes**

Run: `go test ./internal/db/... -run TestVerifyMigrationsState_AllAppliedPasses -v`
Expected: PASS

- [ ] **Step 5: Add the failing-path test — manually corrupt goose_db_version, expect mismatch error**

Append to `internal/db/migrate_test.go`:

```go
func TestVerifyMigrationsState_MismatchReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pool := testutil.NewTestDB(t)
	dsn := pool.Config().ConnString()

	// Delete one applied row to simulate a partial restore / manual edit.
	_, err := pool.Exec(ctx,
		`DELETE FROM goose_db_version WHERE version_id = (
			SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true
		)`,
	)
	require.NoError(t, err)

	err = db.VerifyMigrationsState(ctx, dsn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migration state mismatch")
}
```

- [ ] **Step 6: Run both tests, verify both pass**

Run: `go test ./internal/db/... -run TestVerifyMigrationsState -v`
Expected: 2 tests, both PASS.

- [ ] **Step 7: Run repo-wide lint**

Run: `golangci-lint run --new-from-rev=HEAD ./internal/db/...`
Expected: no new issues. (Per CLAUDE.md, only newly-introduced violations are flagged.)

- [ ] **Step 8: Commit**

```bash
git add internal/db/migrate.go internal/db/migrate_test.go
git commit -s -m "feat(db): add VerifyMigrationsState startup check

Adds a cheap COUNT(*) + embed.FS scan that confirms the DB has applied
exactly the set of migrations the binary ships with. Returns a clear
mismatch error on divergence so /readyz can keep traffic off pods that
booted before an out-of-band migrator (Bytebase, in cloud profile)
caught up.

Two tests exercise the happy path and a forced-mismatch."
```

---

## Task 2: Wire `VerifyMigrationsState` into `cmd/api/main.go`

**Files:**
- Modify: `cmd/api/main.go`

`VerifyMigrationsState` must run on every startup regardless of `AutoMigrate`. In edge mode it confirms goose's work; in cloud mode it gates readiness on Bytebase having applied pending migrations.

- [ ] **Step 1: Read the current shape of the migration block in `cmd/api/main.go`**

Run: `grep -n -A2 -B1 'RunMigrations\|AutoMigrate' cmd/api/main.go`
Expected: lines showing the existing `if cfg.Database.AutoMigrate { … RunMigrations … }` block. Note the exact surrounding context so the edit lands cleanly.

- [ ] **Step 2: Add the `VerifyMigrationsState` call immediately after the existing migration block**

After the closing `}` of the `if cfg.Database.AutoMigrate { … }` block, insert:

```go
// Always verify migration state, regardless of profile. In edge mode this
// confirms RunMigrations applied everything; in cloud mode (AutoMigrate=false)
// this gates startup on the out-of-band migrator having finished. The error
// is fatal — surface it loudly rather than serving traffic against a stale
// schema.
if err := db.VerifyMigrationsState(ctx, cfg.Database.URL); err != nil {
	return fmt.Errorf("verify migration state: %w", err)
}
```

Confirm `db` and `fmt` are already imported by the file (they are, since `RunMigrations` lives in the same package and is already called here).

- [ ] **Step 3: Build the binary**

Run: `go build ./cmd/api`
Expected: exits 0 with no output.

- [ ] **Step 4: Run repo-wide lint**

Run: `golangci-lint run --new-from-rev=HEAD ./cmd/api/...`
Expected: no new issues.

- [ ] **Step 5: Existing integration tests should still pass**

Run: `go test -tags=integration ./internal/integration/... -run 'Migration' -v`
Expected: existing migration integration tests PASS — the verify call after the existing block is additive and `NewTestDB` already runs all migrations.

- [ ] **Step 6: Commit**

```bash
git add cmd/api/main.go
git commit -s -m "feat(api): gate startup on VerifyMigrationsState

Calls VerifyMigrationsState after the existing RunMigrations block on
every boot. In edge mode this is belt-and-braces; in cloud mode it is
the primary defence against a pod coming up before Bytebase finishes
applying pending migrations."
```

---

## Task 3: Create the SQL review policy

**Files:**
- Create: `.bytebase/sql-review.json`

The policy encodes the ten rules listed in the design spec. Bytebase's policy schema is documented at <https://www.bytebase.com/docs/sql-review/review-rules>; the rule IDs below are stable since v1.

- [ ] **Step 1: Create `.bytebase/sql-review.json` with the initial policy**

```json
{
  "name": "raven-sql-review",
  "engine": "POSTGRES",
  "rules": [
    { "type": "naming.column",                       "level": "ERROR",   "payload": { "format": "^[a-z][a-z0-9_]*$", "maxLength": 63 } },
    { "type": "column.required",                     "level": "WARNING", "payload": { "list": ["created_at", "updated_at"] } },
    { "type": "column.no-null",                      "level": "ERROR",   "payload": { "list": ["id", "tenant_id", "workspace_id", "user_id"] } },
    { "type": "statement.disallow-commit",           "level": "ERROR" },
    { "type": "statement.disallow-truncate",         "level": "ERROR" },
    { "type": "statement.disallow-rm-tbl-cascade",   "level": "ERROR" },
    { "type": "statement.add-column-without-default","level": "WARNING" },
    { "type": "statement.create-index-concurrently", "level": "WARNING" },
    { "type": "index.no-duplicate",                  "level": "ERROR" },
    { "type": "schema.backward-compatibility",       "level": "WARNING" }
  ]
}
```

Notes:
- `naming.column` `maxLength: 63` matches Postgres's identifier limit.
- `column.no-null` lists known FK / tenancy columns; extend as new shared columns appear.
- Severities match the spec table verbatim.

- [ ] **Step 2: Validate the JSON parses**

Run: `python3 -c 'import json; json.load(open(".bytebase/sql-review.json"))'`
Expected: exit code 0, no output.

- [ ] **Step 3: Commit**

```bash
git add .bytebase/sql-review.json
git commit -s -m "feat(ci): add Bytebase SQL review policy

Encodes the ten review rules from the tiered-adoption design spec.
Severities: ERROR blocks PR merge, WARNING surfaces as a reviewer
checklist. Consumed by .github/workflows/sql-review.yml (added in a
follow-up commit) via the bytebase/sql-review-action."
```

---

## Task 4: Create banned-pattern fixtures for the self-test

**Files:**
- Create: `.bytebase/fixtures/bad/00001_dropped_table_no_label.sql`
- Create: `.bytebase/fixtures/bad/00002_pascal_case_column.sql`
- Create: `.bytebase/fixtures/bad/00003_truncate_in_migration.sql`

Each fixture intentionally violates one ERROR-severity rule. The self-test workflow (Task 6) runs the policy against this directory and asserts a non-zero exit. This guarantees that future edits to `sql-review.json` that accidentally weaken a rule fail loudly in CI.

- [ ] **Step 1: Create the destructive-op fixture**

`.bytebase/fixtures/bad/00001_dropped_table_no_label.sql`:

```sql
-- +goose Up
-- Intentionally violates statement.disallow-rm-tbl-cascade.
-- Used by .github/workflows/sql-review-selftest.yml to verify the policy fires.
DROP TABLE IF EXISTS chunks CASCADE;
```

- [ ] **Step 2: Create the naming-violation fixture**

`.bytebase/fixtures/bad/00002_pascal_case_column.sql`:

```sql
-- +goose Up
-- Intentionally violates naming.column (PascalCase).
CREATE TABLE bad_naming (
    id            uuid PRIMARY KEY,
    UserEmail     text NOT NULL,
    CreatedAt     timestamptz NOT NULL DEFAULT now()
);
```

- [ ] **Step 3: Create the TRUNCATE fixture**

`.bytebase/fixtures/bad/00003_truncate_in_migration.sql`:

```sql
-- +goose Up
-- Intentionally violates statement.disallow-truncate.
TRUNCATE TABLE response_cache;
```

- [ ] **Step 4: Commit**

```bash
git add .bytebase/fixtures/
git commit -s -m "test(ci): add banned-pattern SQL fixtures for policy self-test

Three intentionally-bad SQL files exercising one ERROR rule each:
DROP CASCADE, PascalCase column names, TRUNCATE. Consumed by
sql-review-selftest.yml to detect regressions in sql-review.json."
```

---

## Task 5: Create the PR-time SQL review workflow

**Files:**
- Create: `.github/workflows/sql-review.yml`

Path-filtered to `migrations/**` so it only runs when a PR touches a real migration. Becomes a required check on `main` (manual step in Task 9).

- [ ] **Step 1: Create the workflow**

`.github/workflows/sql-review.yml`:

```yaml
name: SQL Review

on:
  pull_request:
    paths:
      - 'migrations/**'
      - '.bytebase/sql-review.json'
      - '.github/workflows/sql-review.yml'

permissions:
  contents: read
  pull-requests: write  # post inline review comments

jobs:
  sql-review:
    name: Bytebase SQL Review
    runs-on: ubuntu-latest
    steps:
      - name: Checkout PR head
        uses: actions/checkout@v4

      - name: Run Bytebase SQL review
        uses: bytebase/sql-review-action@v1
        with:
          pattern: 'migrations/*.sql'
          template: |
            ${{ format('{0}/.bytebase/sql-review.json', github.workspace) }}
          fail-on-error: true
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

Notes:
- `bytebase/sql-review-action@v1` is the official action. Confirm input names against its current README at <https://github.com/bytebase/sql-review-action> at execution time and adjust if the upstream action has renamed any field.
- `fail-on-error: true` blocks the PR check on any ERROR-severity finding; WARNING findings post comments but don't fail.
- `pull-requests: write` is the minimum permission needed for the action to leave inline comments. No other write scope is granted.

- [ ] **Step 2: Validate workflow YAML syntax**

Run: `python3 -c 'import yaml; yaml.safe_load(open(".github/workflows/sql-review.yml"))'`
Expected: exit code 0, no output.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/sql-review.yml
git commit -s -m "ci: add Bytebase SQL review check on PRs touching migrations

Runs bytebase/sql-review-action against migrations/*.sql whenever a PR
touches migrations/, the policy file, or this workflow. Fails on
ERROR-severity findings; WARNING surfaces as inline review comments.
Will be made a required status check on main as a follow-up manual
step (see migrations runbook)."
```

---

## Task 6: Create the policy self-test workflow

**Files:**
- Create: `.github/workflows/sql-review-selftest.yml`

Runs on every push to `main` and on PRs that change `.bytebase/sql-review.json` or the fixtures. Runs the action against the fixtures from Task 4 and asserts a **non-zero** exit. If the policy ever silently weakens, CI screams.

- [ ] **Step 1: Create the self-test workflow**

`.github/workflows/sql-review-selftest.yml`:

```yaml
name: SQL Review Policy Self-Test

on:
  push:
    branches: [main]
    paths:
      - '.bytebase/sql-review.json'
      - '.bytebase/fixtures/**'
      - '.github/workflows/sql-review-selftest.yml'
  pull_request:
    paths:
      - '.bytebase/sql-review.json'
      - '.bytebase/fixtures/**'
      - '.github/workflows/sql-review-selftest.yml'

permissions:
  contents: read

jobs:
  policy-selftest:
    name: Policy fires on banned patterns
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Bytebase SQL review against banned fixtures
        id: review
        continue-on-error: true
        uses: bytebase/sql-review-action@v1
        with:
          pattern: '.bytebase/fixtures/bad/*.sql'
          template: |
            ${{ format('{0}/.bytebase/sql-review.json', github.workspace) }}
          fail-on-error: true

      - name: Assert policy rejected the fixtures
        if: steps.review.outcome == 'success'
        run: |
          echo "::error::SQL review policy did NOT reject the banned fixtures."
          echo "Either the fixtures no longer match a rule, or sql-review.json has weakened."
          echo "Inspect .bytebase/fixtures/bad/*.sql and .bytebase/sql-review.json."
          exit 1

      - name: Confirm policy fired
        if: steps.review.outcome == 'failure'
        run: echo "Policy correctly rejected the banned fixtures."
```

- [ ] **Step 2: Validate workflow YAML syntax**

Run: `python3 -c 'import yaml; yaml.safe_load(open(".github/workflows/sql-review-selftest.yml"))'`
Expected: exit code 0, no output.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/sql-review-selftest.yml
git commit -s -m "ci: add SQL review policy self-test

Runs the policy against intentionally-bad fixtures and asserts it
rejects them. Triggered on push to main and on PRs that touch the
policy or fixtures. Catches regressions where a future edit to
sql-review.json silently weakens a rule."
```

---

## Task 7: Add `sql-review-local` Makefile target

**Files:**
- Modify: `Makefile`

Developers should be able to run the same check locally before pushing. The action publishes a container image (`bytebase/sql-review-action:v1`) we can run via Docker.

- [ ] **Step 1: Confirm Makefile uses tabs (it must, GNU make)**

Run: `cat -A Makefile | head -5`
Expected: lines beginning with `^I` (TAB) for recipe bodies. Use tabs, not spaces, in the recipe below.

- [ ] **Step 2: Append the target to `Makefile`**

```makefile
.PHONY: sql-review-local
sql-review-local: ## Run Bytebase SQL review against the local migrations/ tree
	@echo "Running Bytebase SQL review against migrations/*.sql ..."
	@docker run --rm \
		-v $(CURDIR):/repo:ro \
		-w /repo \
		ghcr.io/bytebase/sql-review-action:v1 \
		--pattern 'migrations/*.sql' \
		--template '.bytebase/sql-review.json' \
		--fail-on-error
```

Notes:
- `$(CURDIR)` is GNU make's absolute current dir; the existing Makefile already uses this style (verify with `grep CURDIR Makefile`).
- The image tag `ghcr.io/bytebase/sql-review-action:v1` mirrors the action; confirm against the action's README at execution time.

- [ ] **Step 3: Run the new target against the current tree to make sure it works**

Run: `make sql-review-local`
Expected: existing 43 migrations clear the policy → exit 0. (If a real migration trips a rule, that's a finding to fix in a separate PR, not a Plan A blocker — but it's surfacing in time, which is the point.)

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -s -m "build: add sql-review-local make target

Runs the Bytebase SQL review action's container image against the
local migrations/ tree with the same policy CI uses. Lets authors
catch policy violations before pushing."
```

---

## Task 8: Write the migrations runbook

**Files:**
- Create: `docs/runbooks/migrations.md`

Captures how to author a migration, what the SQL review check does, when the escape-hatch label applies, and how to recover from a failed apply on edge.

- [ ] **Step 1: Create the runbook**

`docs/runbooks/migrations.md`:

````markdown
# Migrations Runbook

## How migrations work in Raven

- Source-of-truth: `migrations/NNNNN_<name>.sql`, applied by `pressly/goose v3`.
- The API binary embeds the migrations directory (`migrations/embed.go`) and applies them at startup when `RAVEN_DB_AUTO_MIGRATE=true` (default for edge).
- After applying (or skipping, in cloud profile), `cmd/api` always calls `db.VerifyMigrationsState` to confirm `goose_db_version` matches the embedded set. Mismatch → process exits non-zero, `/readyz` stays red.

## Authoring a migration

1. Create `migrations/NNNNN_<descriptive_name>.sql`. `NNNNN` is the next sequential 5-digit prefix.
2. Use the `-- +goose Up` directive at the top. No down migrations — rollbacks happen via a new forward migration or `pgBackRest` restore.
3. Open a PR. The **SQL Review** check runs automatically and posts inline comments. ERROR findings block merge.
4. Run `make sql-review-local` before pushing to catch issues early.

## The SQL review check

- Workflow: `.github/workflows/sql-review.yml`
- Policy: `.bytebase/sql-review.json`
- Engine: `bytebase/sql-review-action@v1` (no Bytebase server needed)
- Triggered on PRs that touch `migrations/**`, the policy file, or the workflow file.

### Severities

- **ERROR**: blocks PR merge.
- **WARNING**: posts an inline comment; the reviewer must read it but it doesn't block.

### Escape hatch — destructive operations

Some legitimate migrations need destructive operations (e.g., dropping a column after a multi-PR deprecation). The destructive-op rules can be skipped on a per-PR basis:

1. The PR author requests the override in the PR description, with the **previous PRs that deprecated the thing** linked.
2. A repo maintainer (codeowner) applies the label `migration:approved-destructive` to the PR.
3. The SQL review action skips destructive-op rules on the next run.

The label is only applicable by maintainers (enforced via CODEOWNERS). Document the override in the PR description so the audit trail is preserved.

## Recovery from a failed migration

### Edge profile

If `RunMigrations` fails:

1. The `cmd/api` process exits non-zero. The operator sees the goose error in the logs.
2. Restore the database from the last pre-migration `pgBackRest` base backup + WAL up to the LSN immediately before the failed migration's `BEGIN`.
3. Fix the migration in a new PR.
4. Re-deploy.

The detailed pgBackRest procedure lives in `docs/runbooks/demo-restore.md`.

### Cloud profile (future)

Bytebase marks the Issue `FAILED` and halts the pipeline. The DBA inspects the SQL, fixes it (either via Bytebase UI for a hotfix or by raising a forward-fix PR), and retries the stage. The affected tenant's `cmd/api` pods stay on the old version because `/readyz` stays red while `VerifyMigrationsState` fails. Other tenants are unaffected.

## Required CI checks (branch protection)

The following checks must be required on `main`:

- `Bytebase SQL Review / sql-review` (from `sql-review.yml`)

Manually configured under **Settings → Branches → Branch protection rules → main → Require status checks to pass before merging**. See Task 9 of the implementation plan for the one-time setup step.
````

- [ ] **Step 2: Commit**

```bash
git add docs/runbooks/migrations.md
git commit -s -m "docs(runbook): add migrations runbook

Documents the goose+SQL-review workflow: how to author a migration,
how the PR-time check works, when and how to use the destructive-op
escape-hatch label, and how to recover from a failed apply on edge."
```

---

## Task 9: Mark `sql-review` as a required status check (manual, document only)

**Files:** none — this is a one-time GitHub UI step.

- [ ] **Step 1: Document the change request to repo settings**

After the first run of `sql-review.yml` against a PR has completed (so the check name is registered with GitHub), the repo owner / admin must:

1. Open `https://github.com/ravencloak-org/Raven/settings/branches`.
2. Edit the rule for `main`.
3. Under **Require status checks to pass before merging**, add `sql-review` (the job name from `sql-review.yml`).
4. Save.

No code change. Capture this step in the PR description so the reviewer knows to follow up after merge.

- [ ] **Step 2: No commit needed for this task**

It's documented in the migrations runbook (Task 8) and called out in the PR description.

---

## Final verification

- [ ] **Step 1: Run all unit + integration tests**

Run: `go test ./internal/db/... -v && go test -tags=integration ./internal/integration/... -v`
Expected: all green.

- [ ] **Step 2: Lint clean for the diff**

Run: `golangci-lint run --new-from-rev=origin/main ./...`
Expected: no new issues.

- [ ] **Step 3: SQL review against current migrations passes locally**

Run: `make sql-review-local`
Expected: exit 0 (all 43 existing migrations clear the policy). If one fails, the policy is over-strict for the existing tree — narrow that rule before merging.

- [ ] **Step 4: Push branch and open PR**

```bash
git push -u origin <branch-name>
gh pr create --fill
```

In the PR description, link to the spec (`docs/superpowers/specs/2026-05-23-bytebase-tiered-design.md`) and call out the manual branch-protection step from Task 9.

- [ ] **Step 5: Queue auto-merge per CLAUDE.md**

```bash
gh pr merge <PR_NUMBER> --auto --squash
```

---

## Self-review notes (already addressed)

- **Spec coverage:** every section of the spec that is in-scope for Plan A has a task. SQL review rules → Task 3. Self-test (Layer 1 from spec testing section) → Tasks 4 + 6. Edge-side unit test for `VerifyMigrationsState` (Layer 2 from spec) → Task 1. cmd/api wiring → Task 2. Makefile (`sql-review-local`) → Task 7. SOP doc → Task 8. Required-check branch protection → Task 9. Drift detection, Bytebase server runtime, history-table override, Layers 3 & 4 integration tests are explicitly **deferred to Plan B** and called out in the plan header.
- **Placeholder scan:** no TBD/TODO. Every code block is complete and self-contained.
- **Type consistency:** `VerifyMigrationsState(ctx context.Context, databaseURL string) error` referenced identically in Task 1 (definition), Task 2 (call site), and Task 8 (runbook).
