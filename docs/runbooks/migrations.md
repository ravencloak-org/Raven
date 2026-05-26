# Migrations Runbook

## How migrations work in Raven

- **Source-of-truth:** `migrations/NNNNN_<name>.sql`, applied by `pressly/goose v3`.
- **Embedding:** the API binary embeds `migrations/` at build time (`migrations/embed.go`). Operators do not ship the SQL files separately.
- **Apply path (today):** edge profile sets `RAVEN_DB_AUTO_MIGRATE=true` (default); `db.RunMigrations` applies pending migrations at startup.
- **Apply path (cloud, Plan B):** `RAVEN_DB_AUTO_MIGRATE=false`; an out-of-band migrator (Bytebase server, introduced in Plan B) applies migrations through a `dev → staging → prod` pipeline with human approval at the prod stage.
- **Startup safety check:** `db.VerifyMigrationsState` ALWAYS runs after `RunMigrations`. It compares the count of distinct currently-applied versions in `goose_db_version` (latest row per `version_id`) with the count of embedded `*.sql` files. Mismatch → `log.Fatalf` → process exits → `/readyz` stays red. This gates the cloud profile on its out-of-band migrator catching up; on edge it is belt-and-braces.

## Authoring a migration

1. Create `migrations/NNNNN_<descriptive_name>.sql`. `NNNNN` is the next sequential 5-digit prefix.
2. Use the `-- +goose Up` directive at the top. **No down migrations** — rollbacks happen via a new forward migration or `pgBackRest` restore.
3. Run the local lint before pushing:

   ```bash
   make sql-lint-local
   ```

   This runs squawk against the migrations modified on your branch versus `origin/main` — same files CI will lint on your PR.

4. Open a PR. The **SQL Review** GitHub Actions check runs `squawk` against the modified migrations and posts inline review comments on findings. Findings fail the check → PR cannot merge until resolved.

## The SQL review check

- **Workflow:** `.github/workflows/sql-review.yml`
- **Engine:** `sbdchd/squawk-action@v2` (server-less, downloads the `squawk` CLI binary in CI)
- **Config:** `.squawk.toml` at repo root (`assume_in_transaction=true`, `pg_version=18.0`, default ruleset)
- **Scope:** lints only migrations **modified in the current PR** (diff-based selection vs the PR base). Pre-existing migrations are NOT re-linted, mirroring this repo's `golangci-lint --new-from-rev=HEAD` convention.

The pre-existing backlog (271 findings across the 43 shipped migrations under squawk's default ruleset) is tracked in the design spec at `docs/superpowers/specs/2026-05-23-bytebase-tiered-design.md`. It will be paid down incrementally — do NOT bundle backlog cleanup into unrelated PRs.

### Escape hatch — intentional rule violation

Some legitimate migrations need an operation squawk would normally flag (e.g. dropping a column after a multi-PR deprecation, or doing a small `CREATE INDEX` synchronously on a table guaranteed to be empty at apply time).

Per-statement override is an inline comment immediately before the offending SQL:

```sql
-- +goose Up
-- Removes the legacy zitadel_users mirror table; superseded by SuperTokens
-- session storage in migration 00037. Confirmed empty in prod via Bytebase
-- before this PR; see ADR-2026-05-12 for the full sequence.
-- squawk-ignore ban-drop-table
DROP TABLE zitadel_users;
```

- The `-- squawk-ignore <rule-name>` comment skips that one rule for the next statement.
- Multiple rules in one comment: `-- squawk-ignore ban-drop-column, renaming-column`.
- File-wide skip is supported (`-- squawk-ignore-file <rule>`) but use sparingly — it removes a guard rail for everything in that file.
- **Reviewer responsibility:** every `squawk-ignore` line in a PR diff requires explicit reviewer attention. Look at the comment immediately above for the rationale. If it's not justified, reject the PR.

### Policy self-test

`.github/workflows/sql-review-selftest.yml` runs squawk against `.squawk/fixtures/bad/*.sql` and asserts the rules fire. If a future PR weakens `.squawk.toml` (e.g. adds a default-on rule to `excluded_rules`), the self-test fails — catching the regression at PR time.

The fixtures intentionally violate `ban-drop-table`, `adding-required-field`, and `require-concurrent-index-creation`. They are NOT real migrations and are not under the `migrations/` directory.

## Recovery from a failed migration

### Edge profile

If `RunMigrations` fails:

1. The `cmd/api` process exits non-zero. The operator sees the goose error in the API logs.
2. Restore the database from the last pre-migration `pgBackRest` base backup + WAL replay up to the LSN immediately before the failed migration's `BEGIN`. Detailed procedure: `docs/runbooks/demo-restore.md`.
3. Fix the migration in a new PR.
4. Re-deploy.

If `RunMigrations` succeeds but `VerifyMigrationsState` fails:

- This indicates an embedded-file count vs `goose_db_version` mismatch. Most likely cause: a partial restore from backup that left `goose_db_version` out of sync.
- Inspect `goose_db_version` (`SELECT version_id, is_applied, tstamp FROM goose_db_version ORDER BY id`).
- Run `goose status` against the live DB using the on-disk migrations directory (not the embed) for a side-by-side view.
- Resolve by either re-applying missed migrations through `goose up` or restoring further back via `pgBackRest`.

### Cloud profile (planned, Plan B)

Bytebase marks the migration Issue `FAILED` and halts the dev/staging/prod pipeline. The DBA inspects the SQL in the Bytebase UI, fixes it (either via a hotfix issue in Bytebase or by raising a forward-fix PR), and retries the stage. The affected tenant's `cmd/api` pods stay on the old version because `/readyz` stays red while `VerifyMigrationsState` fails. Other tenants are unaffected.

## Required CI checks (branch protection)

The following job must be a required status check on `main`:

- `Squawk migration lint` (the `sql-review` job in `.github/workflows/sql-review.yml`)

This is configured manually under **Settings → Branches → Branch protection rules → main → Require status checks to pass before merging**. Add the check name above. The check is registered with GitHub only after the workflow has run at least once.

## References

- Design spec: `docs/superpowers/specs/2026-05-23-bytebase-tiered-design.md`
- Implementation plan: `docs/superpowers/plans/2026-05-23-bytebase-tiered-plan-a.md`
- Squawk rules reference: https://squawkhq.com/docs/rules
- Squawk CLI: https://squawkhq.com/docs/cli
- pgBackRest restore: `docs/runbooks/demo-restore.md`
