package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
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
// migrations embedded in the binary. It compares the SET of version_ids
// recorded in goose_db_version against the SET parsed from embedded migration
// filenames. A simple COUNT(*) check is fragile against intentional or
// accidental version gaps (e.g. a hot-deploy that runs the SQL by hand before
// the file lands in main, or a renumbering that skips an id): equal counts
// can mask a different SET on each side. The set-based comparison surfaces
// exactly which versions are out of sync.
//
// In the edge profile this is a belt-and-braces check that RunMigrations did
// what it claimed. In the cloud profile (AutoMigrate=false, Bytebase applies
// migrations out-of-band) it gates /readyz: pods stay un-ready until Bytebase
// has caught up. Returning an error here surfaces a clear mismatch instead of
// silently serving traffic against a stale schema.
//
// Goose seeds the version table with version_id=0 on first use; that row is
// excluded from the applied set.
func VerifyMigrationsState(ctx context.Context, databaseURL string) error {
	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("open postgres for verify: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres for verify: %w", err)
	}

	applied, err := appliedVersionSet(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("read goose_db_version: %w", err)
	}

	embedded, err := embeddedVersionSet()
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	// Symmetric difference. "orphan" = applied to DB but not present in the
	// embedded set (usually a binary rollback to an older release; serious).
	// "unapplied" = present in embedded set but not applied (deploy gap;
	// `goose up` needed).
	var orphan, unapplied []int64
	for v := range applied {
		if _, ok := embedded[v]; !ok {
			orphan = append(orphan, v)
		}
	}
	for v := range embedded {
		if _, ok := applied[v]; !ok {
			unapplied = append(unapplied, v)
		}
	}

	if len(orphan) == 0 && len(unapplied) == 0 {
		return nil
	}

	sort.Slice(orphan, func(i, j int) bool { return orphan[i] < orphan[j] })
	sort.Slice(unapplied, func(i, j int) bool { return unapplied[i] < unapplied[j] })

	switch {
	case len(orphan) > 0 && len(unapplied) > 0:
		return fmt.Errorf(
			"migration state mismatch: orphan applied migrations not in embedded set: %v; unapplied migrations: %v",
			orphan, unapplied,
		)
	case len(orphan) > 0:
		return fmt.Errorf(
			"migration state mismatch: orphan applied migrations not in embedded set: %v",
			orphan,
		)
	default:
		return fmt.Errorf(
			"migration state mismatch: unapplied migrations: %v",
			unapplied,
		)
	}
}

// appliedVersionSet returns the set of version_ids currently applied in
// goose_db_version. goose_db_version is append-only: every Up and Down appends
// a row, so a rollback+re-apply produces multiple is_applied=true rows for the
// same version_id. We keep only the LATEST row per version_id (highest id)
// and treat the version as applied iff that latest row has is_applied=true.
// bool_and across the history would incorrectly drop versions that were
// rolled back and re-applied.
func appliedVersionSet(ctx context.Context, sqlDB *sql.DB) (map[int64]struct{}, error) {
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT g.version_id
		FROM goose_db_version g
		WHERE g.version_id > 0
		  AND g.is_applied = true
		  AND g.id = (
		      SELECT MAX(sub.id)
		      FROM goose_db_version sub
		      WHERE sub.version_id = g.version_id
		  )`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	set := make(map[int64]struct{})
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		set[v] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return set, nil
}

// embeddedVersionSet returns the set of version_ids parsed from the embedded
// migrations FS. Filenames follow goose's NNNNN_description.sql convention;
// the leading numeric prefix is the version_id.
func embeddedVersionSet() (map[int64]struct{}, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, err
	}
	set := make(map[int64]struct{})
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		name := e.Name()
		// Take everything up to the first underscore as the version id.
		idx := strings.IndexByte(name, '_')
		if idx <= 0 {
			return nil, fmt.Errorf("migration filename %q has no version prefix", name)
		}
		v, err := strconv.ParseInt(name[:idx], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse version from migration filename %q: %w", name, err)
		}
		set[v] = struct{}{}
	}
	return set, nil
}
