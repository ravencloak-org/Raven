// Atlas configuration for Raven — Git-style version-control of the Postgres
// SCHEMA ONLY (never data). Atlas runs ALONGSIDE goose; goose remains the
// applier of record (see Makefile `migrate-up`). Atlas is used purely for
// schema review: `atlas schema inspect` regenerates db/schema.sql, and
// `atlas schema diff` detects drift between a goose-migrated DB and that file.
//
// See docs/adr/0010-atlas-schema-version-control.md for the rationale
// (Doltgres rejected — no extension support; Atlas chosen for schema VCS).
//
// Runtime DB is Postgres 18 + pgvector. Atlas's built-in `docker://` dev
// images are stock Postgres and LACK pgvector, so the dev-url MUST point at a
// pgvector-capable Postgres. The Makefile (`schema-inspect` / `schema-diff`)
// spins an ephemeral pgvector/pgvector:pg18 container via podman, pre-creates
// the three extensions and the raven_app / raven_admin roles that the schema
// references, and exports its URL as ATLAS_DEV_URL. (The `docker` block that
// would let Atlas build a custom image inline is an Atlas Pro feature; the
// external-dev-url approach keeps this working on the free Community Edition.)

variable "dev_url" {
  type    = string
  default = getenv("ATLAS_DEV_URL")
}

variable "db_url" {
  type    = string
  default = getenv("DATABASE_URL")
}

env "local" {
  // Declarative schema-as-code: the canonical, version-controlled snapshot.
  // Produced verbatim by `atlas schema inspect` against a goose-migrated DB.
  src = "file://db/schema.sql"

  // The goose migration directory, read with Atlas's goose dir-format token.
  migration {
    dir = "file://migrations?format=goose"
  }

  // Scratch database Atlas uses to materialise and compare schema states.
  // Must be a CLEAN pgvector-capable Postgres with uuid-ossp / vector /
  // pg_trgm extensions and the raven_app / raven_admin roles pre-installed.
  dev = var.dev_url

  // Only manage the public schema; everything Raven owns lives there.
  schemas = ["public"]
}
