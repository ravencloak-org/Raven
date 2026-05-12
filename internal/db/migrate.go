package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq" // postgres driver for the database/sql handle goose requires
	"github.com/pressly/goose/v3"

	"github.com/ravencloak-org/Raven/migrations"
)

// RunMigrations applies every pending goose migration against the database
// pointed to by databaseURL. Migrations are read from the embedded filesystem
// in the migrations package, so the API binary is self-contained — operators
// do not need to ship the migrations/ directory alongside it.
//
// Called from cmd/api/main.go when cfg.Database.AutoMigrate is true. Returns
// the first error it encounters; partial-state DBs are the operator's problem
// to recover from (typically via pgBackRest restore — see the Upgrades and
// Backups docs).
func RunMigrations(ctx context.Context, databaseURL string) error {
	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("open postgres for migration: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres for migration: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	defer goose.SetBaseFS(nil)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose.SetDialect: %w", err)
	}
	if err := goose.UpContext(ctx, sqlDB, "."); err != nil {
		return fmt.Errorf("goose.Up: %w", err)
	}
	return nil
}
