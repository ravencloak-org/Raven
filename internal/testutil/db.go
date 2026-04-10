// Package testutil provides shared helpers for Go tests including database containers,
// fixture factories, and gRPC stub implementations.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq" // register postgres driver for goose migrations
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDBOption configures NewTestDB behaviour.
type TestDBOption func(*testDBConfig)

type testDBConfig struct {
	migrationsDir string // override for the default migrations directory
}

// WithMigrationsDir overrides the default (runtime.Caller-based) migrations
// directory. Use this when the binary is executed from a non-standard working
// directory, inside Docker, or with `go test -C`.
func WithMigrationsDir(dir string) TestDBOption {
	return func(c *testDBConfig) {
		c.migrationsDir = dir
	}
}

// NewTestDB spins up a real PostgreSQL container using pgvector, runs all migrations,
// and returns a pool. Container is terminated when t ends.
func NewTestDB(t *testing.T, opts ...TestDBOption) *pgxpool.Pool {
	t.Helper()

	var cfg testDBConfig
	for _, o := range opts {
		o(&cfg)
	}

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg17",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "raven_test",
			"POSTGRES_PASSWORD": "raven_test_pass",
			"POSTGRES_DB":       "raven_test",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	connStr := fmt.Sprintf(
		"host=%s port=%s user=raven_test password=raven_test_pass dbname=raven_test sslmode=disable",
		host, port.Port(),
	)

	// Wait for DB to be fully ready.
	var db *sql.DB
	db, err = sql.Open("postgres", connStr)
	require.NoError(t, err)
	for i := 0; i < 30; i++ {
		if pingErr := db.PingContext(ctx); pingErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	require.NoError(t, db.PingContext(ctx), "database must be reachable")

	RunMigrations(t, db, cfg.migrationsDir)
	_ = db.Close()

	pgxConnStr := fmt.Sprintf(
		"postgres://raven_test:raven_test_pass@%s:%s/raven_test?sslmode=disable",
		host, port.Port(),
	)
	pool, err := pgxpool.New(ctx, pgxConnStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

// RunMigrations applies all goose migrations from the repo migrations/ dir.
// An optional overrideDir can be provided (first non-empty string wins) to
// bypass the default runtime.Caller-based resolution.
func RunMigrations(t *testing.T, db *sql.DB, overrideDir ...string) {
	t.Helper()

	var migrationsDir string
	for _, d := range overrideDir {
		if d != "" {
			migrationsDir = d
			break
		}
	}

	if migrationsDir == "" {
		// Resolve migrations dir relative to this file: internal/testutil/ -> repo root/migrations/
		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("runtime.Caller failed to retrieve file path")
		}
		migrationsDir = filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
	}

	if _, err := os.Stat(migrationsDir); err != nil {
		t.Fatalf("migrations directory not found at %s: %v", migrationsDir, err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose.SetDialect: %v", err)
	}
	if err := goose.Up(db, migrationsDir); err != nil {
		t.Fatalf("goose.Up: %v", err)
	}
}
