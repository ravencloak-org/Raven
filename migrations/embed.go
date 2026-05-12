// Package migrations exposes the goose-managed SQL migration files via an
// embedded filesystem so the API binary can apply them at startup when
// RAVEN_DB_AUTO_MIGRATE=true is set.
//
// Operators still run `make migrate-up` (which uses the on-disk files
// directly via the goose CLI) by default; the embed is opt-in.
package migrations

import "embed"

// FS holds every .sql migration file at the time of build. Goose reads from
// this via goose.SetBaseFS when db.RunMigrations is invoked.
//
//go:embed *.sql
var FS embed.FS
