package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"

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
