# Bytebase — Tiered Adoption (Edge + Cloud) Design

**Date:** 2026-05-23
**Status:** Draft (awaiting approval)
**Owner:** Jobin Lawrance

## Summary

Adopt [Bytebase](https://github.com/bytebase/bytebase) as a database DevOps layer on top of Raven's existing `pressly/goose` migration tooling, **without** changing the migration source-of-truth and **without** adding new mandatory infrastructure to the edge deployment profile.

The adoption is tiered:

- **Edge profile (Pi / single-VPS):** unchanged. `goose` continues to apply embedded migrations at API startup.
- **Cloud profile (multi-tenant SaaS):** Bytebase runs in the control plane, gates migrations through a `dev → staging → prod` pipeline with human approval, and writes to the same `goose_db_version` history table goose uses.
- **PR-time (everyone, always):** Bytebase's SQL Review GitHub Action lints `migrations/*.sql` against a policy and blocks merge on unsafe DDL. No runtime server needed for this path.

This solves four pain points the user identified: SQL review before merge, schema drift / audit visibility, multi-env approval workflow, and decoupling schema changes from API binary startup — while honoring the standing constraint that Raven must remain deployable on Raspberry Pi-class edge nodes with minimal footprint.

## Current State

| Aspect | Today |
|---|---|
| Migration tool | `github.com/pressly/goose/v3` |
| Migration files | 43 files in `migrations/NNNNN_*.sql` (00001–00043) |
| Embedding | `migrations/embed.go` via `go:embed` |
| Runner | `internal/db/migrate.go` → `RunMigrations(ctx, databaseURL)` |
| Trigger | `cmd/api/main.go` when `cfg.Database.AutoMigrate == true` |
| Driver | `github.com/lib/pq` |
| History table | `public.goose_db_version` (goose default) |

## Architecture

Two deployment profiles share one source-of-truth (the `migrations/` directory in this repo).

```
                migrations/*.sql (canonical, goose-numbered)
                embed.go + go:embed (unchanged)
                         │
        ┌────────────────┴────────────────┐
        │                                 │
   EDGE profile                      CLOUD profile
   (Pi / single-VPS)                 (multi-tenant SaaS)
        │                                 │
   AutoMigrate=true                  AutoMigrate=false
   goose.UpContext()                 Bytebase server (control plane)
   at cmd/api startup                ├─ GitOps watches migrations/
        │                            ├─ dev → staging → prod
   goose_db_version                  ├─ approval gates
        │                            └─ applies to tenant DBs
   No Bytebase runtime                  ↓
                                     goose_db_version (shared)
        │                                 │
        └────────────────┬────────────────┘
                         │
            PR-time (both profiles)
            Bytebase SQL Review GitHub Action
            runs against migrations/*.sql diff
            fails PR on unsafe DDL
```

### Key decisions

1. **Bytebase is control-plane only.** Never bundled into the edge compose file. Cloud-only runtime dependency.
2. **`migrations/*.sql` stays canonical.** Goose numbering (`NNNNN_name.sql`) is unchanged. We do not re-author migrations in Bytebase's UI; Bytebase consumes the directory via GitOps.
3. **Shared history table.** Both goose and Bytebase write to `public.goose_db_version` using the same row schema. Bytebase is configured to point at this table rather than creating its own `bytebase` schema in tenant DBs.
4. **Profile selection via existing config.** Reuse `cfg.Database.AutoMigrate`; no new `Deployment.Mode` enum.
5. **Edge users see zero change.** Same binary, same compose, same boot behavior.

## PR-time SQL Review (Layer 1)

**Tool:** `bytebase/sql-review-action` (official GitHub Action, no server required).

**Workflow:** new job `sql-review` in `.github/workflows/sql-review.yml`. Triggers on PRs that touch `migrations/**`. Wired into branch protection on `main` as a required check.

**Policy file:** `.bytebase/sql-review.json`. Initial ruleset:

| Rule | Severity | Reason |
|---|---|---|
| `naming.column` (snake_case) | ERROR | Consistency with 43 existing migrations |
| `column.required` (created_at, updated_at on new tables) | WARNING | Matches existing convention |
| `column.no-null` on PK / FK columns | ERROR | Catch obvious mistakes |
| `statement.disallow-commit` | ERROR | Migrations must be transactional |
| `statement.disallow-truncate` | ERROR | Destructive ops need explicit override |
| `statement.disallow-rm-tbl-cascade` | ERROR | Destructive ops need explicit override |
| `statement.add-column-without-default` (non-null) | WARNING | Forces backfill thinking |
| `statement.create-index-concurrently` | WARNING | Edge OK, cloud tenants may be large |
| `index.no-duplicate` | ERROR | Catch redundant indexes |
| `schema.backward-compatibility` | WARNING | Heads-up on breaking renames/drops |

**Failure UX:** action posts inline review comments on offending file/line. ERROR blocks merge; WARNING surfaces as a reviewer checklist.

**Escape hatch:** PR label `migration:approved-destructive` skips destructive-op rules. Label applicable only by maintainers (codeowner enforcement). Documented in the migration SOP.

## GitOps Wiring + History Coexistence (Cloud, Layer 2)

Bytebase **pulls** from GitHub.

1. Cloud-mode Bytebase server has a project `raven` with a VCS provider bound to `github.com/ravencloak-org/Raven`, branch `main`, base path `migrations/`.
2. On every push to `main` touching `migrations/**`, Bytebase's webhook receiver enumerates new files (goose numeric prefix doubles as a valid Bytebase migration version) and opens an Issue of type "Database Schema Update" per file.
3. The Issue runs through a pipeline with three stages — `dev → staging → prod` — each applying to its tenant DB pool, with per-stage approval policies:
   - `dev`: auto-apply
   - `staging`: auto-apply after dev success
   - `prod`: requires one human approver (Bytebase role: `DBA` or `Owner`)
4. SQL Review runs again on each stage (defense in depth) using the same policy file as PR-time.

### History-table coexistence

Bytebase by default writes to its own `bytebase` schema per managed DB. We override this:

- Each Bytebase database connection is registered with a migration-history hint pointing at `public.goose_db_version`.
- When Bytebase applies migration `00044_foo.sql`, it writes `(version_id=44, is_applied=true, tstamp=now())` — same row shape goose writes.
- When goose applies the same file on an edge node, it writes the same row.
- A migration is "applied" iff its row exists with `is_applied=true`. Both systems agree.

**Trade-off accepted:** Bytebase's own schema-comparison features (its `bytebase` metadata schema) won't exist in tenant DBs. We choose shared state with goose over those.

### Drift detection

- Bytebase's drift-detection job runs hourly per tenant DB. Diffs live schema against expected schema derived from applied `migrations/*.sql`. Opens a `drift`-tagged Issue on mismatch.
- Edge profile adds a startup check `db.VerifyMigrationsState(ctx)` (see next section) — cheap row-count assertion against `goose_db_version`.

## Cloud-mode Config + `cmd/api` Changes

**Config:** reuse existing `cfg.Database.AutoMigrate`. No new field.

| Profile | `AutoMigrate` | Source |
|---|---|---|
| Edge (default) | `true` | Hardcoded default in `config.Defaults()` |
| Cloud | `false` | `RAVEN_DB_AUTO_MIGRATE=false` in cloud Helm/compose values |

**`cmd/api/main.go` changes:**

```go
// existing
if cfg.Database.AutoMigrate {
    if err := db.RunMigrations(ctx, cfg.Database.URL); err != nil {
        return fmt.Errorf("run migrations: %w", err)
    }
}

// NEW — always run, both profiles
if err := db.VerifyMigrationsState(ctx, cfg.Database.URL); err != nil {
    return fmt.Errorf("verify migrations: %w", err)
}
```

`VerifyMigrationsState` lives next to `RunMigrations` in `internal/db/migrate.go`. It compares `count(* from goose_db_version where is_applied=true)` to `len(migrations.FS files)`. Mismatch → return error → `cmd/api` exits or readiness probe stays red.

**Readiness:** `/readyz` depends on `VerifyMigrationsState` passing. `/healthz` (liveness) does not — we don't want K8s killing pods during a temporary mismatch.

**Cloud deploy ordering (Kubernetes / ArgoCD):**

1. CI merges PR to `main`.
2. Bytebase webhook fires → creates Issue → pipeline starts.
3. Bytebase applies migration to staging tenants (auto).
4. Human approves prod stage in Bytebase UI.
5. Bytebase applies to prod tenants → writes `goose_db_version` row.
6. Bytebase "applied" event triggers ArgoCD/Flux sync of `cmd/api`.
7. `cmd/api` pods roll out.
8. Each pod runs `VerifyMigrationsState` → passes → ready.

If step 7 fires before step 5 completes for a tenant: `VerifyMigrationsState` fails on the affected pod, readiness stays red, traffic doesn't shift, K8s holds at old version. Self-healing.

**Edge profile:** unchanged code path — `AutoMigrate=true`, goose runs at startup, `VerifyMigrationsState` confirms goose did its job. Belt and braces.

## Error Handling, Rollback, Audit

### Failure modes

| Failure | Detection | Response |
|---|---|---|
| Edge: goose migration fails mid-apply | `RunMigrations` returns error → `cmd/api` exits non-zero | Operator runbook: `pgBackRest restore` to last good base + WAL replay to pre-migration LSN. Existing playbook. |
| Cloud: Bytebase fails to apply to a tenant | Bytebase Issue marked `FAILED`, pipeline halts | Tenant's API pods stay on old version (readiness gating). DBA fixes in Bytebase UI, retries stage. Other tenants unaffected. |
| Cloud: API pod starts before Bytebase finishes | `VerifyMigrationsState` fails → `/readyz` red | K8s holds traffic on old pods. Self-heals when Bytebase catches up. No alert paged. |
| Drift detected (manual edit, partial restore) | Bytebase hourly drift job → `drift`-tagged Issue | Alerts on-call via existing OpenObserve → PagerDuty path. DBA investigates. |
| SQL Review false positive | PR author argues with bot | Maintainer applies `migration:approved-destructive` label. Documented in SOP. |

### Rollbacks

Forward-only. Rollbacks happen via:

- (a) a new forward migration that undoes the change, authored as a normal PR, **or**
- (b) `pgBackRest` restore when forward-fix isn't feasible.

Bytebase's auto-rollback feature is **disabled** in project config — we don't want anyone trusting auto-generated rollback SQL.

### Audit trail (SOC2 / GDPR evidence)

- **PR-time:** GitHub records authorship/review/merge. SQL Review action results stored as check-run annotations.
- **Cloud-apply:** Bytebase Issue records who created (webhook), who approved each stage (human user), exact SQL, timestamp, target DB, success/failure, duration. Retained in Bytebase metadata DB. Exported daily to OpenObserve via Bytebase Activity API.
- **Edge-apply:** `goose_db_version` rows + existing app audit-log `migration_applied` events at startup.

**Known compliance gap:** edge deployments lack per-stage human approval (single-operator boxes can't ask themselves). Documented in the threat model. Cloud SaaS is the profile that needs the controls.

## Testing Strategy

**Layer 1 — PR SQL Review action.**

- Self-test: fixture PR in CI with an intentionally banned pattern must be rejected. `.github/workflows/sql-review-selftest.yml`, runs on push to `main`.
- Policy file `.bytebase/sql-review.json` linted in CI with `bb sql-review --check-policy`.

**Layer 2 — Migration unit tests.**

- Existing `migrations/migrations_test.go` validates goose can parse and apply every file against a throwaway Postgres (testcontainers). Keep as-is.
- **Add:** test that runs all migrations then `db.VerifyMigrationsState` — asserts pass. Plus a negative test (delete a row → expect failure).

**Layer 3 — Integration test for Bytebase ingest (cloud only).**

- New `internal/integration/bytebase_test.go` (gated by `//go:build integration`):
  1. Spin up Postgres + Bytebase via testcontainers.
  2. Configure project against a local git fixture with 2 migration files.
  3. Trigger sync; assert Bytebase opens an Issue per file.
  4. Approve programmatically via Bytebase API.
  5. Assert files applied to target Postgres AND `goose_db_version` rows present.
- Runs in CI on `migrations/**` or `internal/db/**` changes only.

**Layer 4 — Drift detection test.**

- Apply all migrations.
- Manually `ALTER TABLE` out-of-band.
- Trigger Bytebase drift scan via API.
- Assert `drift`-tagged Issue is opened.

**Out of scope:**

- Bytebase server's own correctness (third-party; pin a known-good version).
- GitHub webhook path end-to-end (would need real GitHub; test the receiver-side by feeding migration files directly).
- New tests for edge migration path — `migrations_test.go` already covers it.

**Local dev:** `make sql-review-local` runs the action against current branch's `migrations/**` diff so authors get the same feedback they'd get from CI before pushing.

## What Changes vs What Stays

| Area | Change? |
|---|---|
| `migrations/*.sql` | No change. Goose numbering preserved. |
| `migrations/embed.go` | No change. |
| `internal/db/migrate.go` | Add `VerifyMigrationsState(ctx, url) error`. |
| `cmd/api/main.go` | Add unconditional call to `VerifyMigrationsState` after the existing `AutoMigrate` block. |
| `internal/config/config.go` | No change (reuse `Database.AutoMigrate`). |
| `.github/workflows/` | New `sql-review.yml`, `sql-review-selftest.yml`. |
| `.bytebase/sql-review.json` | New policy file. |
| `Makefile` | New `sql-review-local` target. |
| Edge compose | No change. |
| Cloud Helm/compose | New `bytebase` service in control plane; `RAVEN_DB_AUTO_MIGRATE=false` on `cmd/api`. |
| Branch protection on `main` | Add `sql-review` as required check. |
| Migration SOP doc | Update with escape-hatch label + Bytebase approval flow. |

## Open Questions / Plan-Time Decisions

- Exact Bytebase version to pin (latest stable at plan time).
- Helm chart vs raw compose for cloud Bytebase deployment — depends on the cloud profile's overall orchestration choice, which is outside this spec.
- Whether to expose Bytebase UI behind the Raven control-plane SSO (SuperTokens) or as a separate auth domain initially. Defer to the cloud-profile rollout plan.

## References

- `internal/db/migrate.go` — current goose runner
- `migrations/` — 43 SQL files (00001–00043)
- `migrations/embed.go`, `migrations/migrations_test.go`
- Bytebase docs: <https://www.bytebase.com/docs/>
- Bytebase SQL Review GH Action: <https://github.com/bytebase/sql-review-action>
- pgBackRest restore runbook: existing internal docs
- OpenObserve audit log pipeline: existing internal docs

## Squawk Backlog Snapshot (2026-05-25)

Initial `squawk` run against the 43 existing migrations surfaced 271
findings under the default ruleset. These are pre-existing debt, NOT
blockers for Plan A — the PR-time workflow is configured to lint only
files modified in a PR (same convention as `golangci-lint --new-from-rev`
in this repo).

Counts by rule:

| Rule | Count |
|------|-------|
| `require-timeout-settings` | 82 |
| `prefer-text-field` | 68 |
| `ban-drop-table` | 40 |
| `prefer-bigint-over-int` | 33 |
| `require-concurrent-index-deletion` | 20 |
| `ban-drop-column` | 13 |
| `require-concurrent-index-creation` | 9 |
| `renaming-column` | 2 |
| Other | 4 |

Tracking issue to be opened post-merge for incremental cleanup. Each
legitimate destructive op should land an inline `-- squawk-ignore <rule>`
comment with rationale; other findings should be fixed via forward
migrations where feasible.
