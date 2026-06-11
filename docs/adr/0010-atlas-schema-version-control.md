# 0010 — Atlas for Postgres schema version-control (alongside goose)

**Status:** Accepted
**Date:** 2026-06-11

## Decision

We adopt [Atlas](https://atlasgo.io) (`ariga/atlas`) to give Raven **Git-style
version-control of the Postgres schema only** — never data. Atlas runs
**alongside** the existing [goose](https://github.com/pressly/goose) migrations;
it does **not** replace them. The division of labour is:

- **goose remains the applier of record.** All schema changes ship as numbered
  goose SQL files in `migrations/` and are applied with
  `goose -dir migrations postgres "$DATABASE_URL" up` (Makefile `migrate-up`).
  Runtime and CI behaviour are unchanged.
- **Atlas owns the schema-as-code artifact.** `db/schema.sql` is the canonical,
  declarative snapshot of the schema, produced verbatim by `atlas schema inspect`
  against a database that has had every goose migration applied. This file is the
  Dolt-like object: `git diff`, `git blame`, branch, and merge the *shape* of the
  schema, review it in PRs, and detect drift.

Two Make targets drive this (see `Makefile`):

- `make schema-inspect` — spin an ephemeral pgvector Postgres, `goose up`, then
  regenerate `db/schema.sql` from it.
- `make schema-diff` — same dev DB, then `atlas schema diff` of the live
  goose-migrated schema against `db/schema.sql`; reports any drift.

CI enforces the invariant: `.github/workflows/atlas-schema.yml` runs
`atlas schema diff` on every PR that touches `migrations/`, `db/schema.sql`, or
`atlas.hcl`, and fails if `db/schema.sql` was not regenerated to match a schema
change. Per-migration SQL linting continues to live in `sql-review.yml` (squawk).

## Why Atlas, and why not Doltgres

We evaluated **Doltgres** (a Postgres-wire-compatible database with native
Git-style branch/diff/merge of *both* schema and data) as the way to get
versioned schemas. **Doltgres was rejected: it has no Postgres extension
support.** Raven's schema depends on extensions that Doltgres cannot load —
`migrations/00001_extensions_and_types.sql` runs `CREATE EXTENSION "vector"`
(pgvector), `"uuid-ossp"`, and `"pg_trgm"`, and the schema uses `vector(768)`
columns with `USING hnsw (... vector_cosine_ops)` indexes. Without the ability
to load pgvector's DDL, Doltgres cannot even materialise our schema, so its
versioning benefits are unreachable for this codebase. The **runtime database
stays Postgres 18 + pgvector**; we did not, and will not, swap the engine to get
schema versioning.

Atlas, by contrast, runs against our real Postgres + pgvector: it inspects the
live schema (vector columns and HNSW indexes round-trip faithfully) and diffs it
against the declarative `db/schema.sql`. It gives us the schema-as-code and
drift-detection we wanted from Dolt without changing the runtime engine or
displacing goose.

## How it works (and the constraints we accepted)

- **pgvector-capable dev database.** Atlas needs a "dev database" as scratch
  space to materialise and compare schema states. Atlas's built-in
  `docker://postgres/N` images are **stock Postgres with no pgvector**, so they
  cannot host our schema. The Make targets and CI instead point Atlas's dev-url
  at a `pgvector/pgvector:pg18` Postgres (ephemeral, torn down after each run),
  pre-seeded with the three extensions and the `raven_app` / `raven_admin` roles
  the schema references. (`atlas.hcl`'s `docker` block, which would build a
  custom dev image inline, is an Atlas Pro feature; the external-dev-url approach
  keeps everything on the free Community Edition.)
- **Extensions are not re-emitted into `db/schema.sql`.** The free-tier `atlas`
  binary rejects `CREATE EXTENSION` inside a declarative schema source
  ("extensions are available to logged-in users only"). `atlas schema inspect`
  therefore does not emit the `CREATE EXTENSION` lines, and re-adding them by
  hand would break `make schema-diff`. Extensions stay owned by goose migration
  `00001`; `db/schema.sql` documents this in its header. The pgvector round-trip
  is still proven in the file by the `vector(768)` columns and the
  `USING HNSW (... vector_cosine_ops)` indexes Atlas inspected and reproduced.
- **`atlas migrate lint` is not used.** Atlas made `migrate lint` an Atlas-Pro
  feature in v0.38. We do not depend on it: squawk already lints individual
  migrations in `sql-review.yml`, and the Atlas value we want here —
  schema-as-code + drift detection — comes from `inspect`/`diff`, which are free.

## Consequences

- Every schema-changing PR regenerates `db/schema.sql` via `make schema-inspect`;
  CI fails otherwise. Reviewers see the schema delta as a readable SQL diff in
  addition to the goose migration.
- No change to runtime, deployment, or the goose workflow. Atlas is a
  development/CI-time tool only; it never touches production data and never
  applies migrations.
- A new dependency on the `atlas` CLI for contributors who regenerate the schema
  (`brew install ariga/tap/atlas`). Not required to build, run, or test Raven.

## Alternatives considered

- **Doltgres (versioned DB engine).** Rejected — no extension support, cannot
  load pgvector; would also mean swapping the runtime engine. (See above.)
- **Hand-maintained `schema.sql`.** Rejected — drifts from migrations silently;
  the whole point is a *generated*, diffable artifact with CI enforcement.
- **Replacing goose with Atlas versioned migrations.** Rejected for now — goose
  is battle-tested in this repo, the migration set is large (00001–00056+), and
  `atlas migrate lint` (the main reason to switch the applier) is Pro-gated.
  Atlas-as-reviewer alongside goose-as-applier gets the schema-VCS benefit at
  near-zero migration cost, and leaves a clean path to reassess later.
