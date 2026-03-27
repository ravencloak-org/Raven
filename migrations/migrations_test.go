package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testDBUser = "raven_test"
	testDBPass = "raven_test_pass"
	testDBName = "raven_test"
)

// allExpectedTables lists every table the migrations should create.
var allExpectedTables = []string{
	"organizations",
	"users",
	"workspaces",
	"workspace_members",
	"knowledge_bases",
	"documents",
	"sources",
	"chunks",
	"embeddings",
	"llm_provider_configs",
	"api_keys",
	"chat_sessions",
	"chat_messages",
	"processing_events",
}

// rlsTables are the tables that must have RLS enabled (all tables with org_id).
var rlsTables = []string{
	"users",
	"workspaces",
	"workspace_members",
	"knowledge_bases",
	"documents",
	"sources",
	"chunks",
	"embeddings",
	"llm_provider_configs",
	"api_keys",
	"chat_sessions",
	"chat_messages",
	"processing_events",
}

// allExpectedTypes lists every custom ENUM type.
var allExpectedTypes = []string{
	"org_status",
	"user_status",
	"workspace_role",
	"kb_status",
	"processing_status",
	"source_type",
	"crawl_frequency",
	"chunk_type",
	"llm_provider",
	"provider_status",
	"api_key_status",
	"message_role",
}

// startPostgresContainer spins up a pgvector-enabled PostgreSQL container.
func startPostgresContainer(ctx context.Context, t *testing.T) (testcontainers.Container, string) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg17",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     testDBUser,
			"POSTGRES_PASSWORD": testDBPass,
			"POSTGRES_DB":       testDBName,
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port.Port(), testDBUser, testDBPass, testDBName,
	)

	return container, dsn
}

// migrationsDir returns the absolute path to the migrations directory.
func migrationsDir(t *testing.T) string {
	t.Helper()
	// The test binary runs from the migrations/ directory itself.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	// Verify at least one migration file exists.
	matches, _ := filepath.Glob(filepath.Join(dir, "*.sql"))
	if len(matches) == 0 {
		t.Fatalf("no SQL migration files found in %s", dir)
	}
	return dir
}

func TestMigrationsUpAndDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	container, dsn := startPostgresContainer(ctx, t)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close database: %v", err)
		}
	}()

	// Wait for DB to be fully ready.
	for i := 0; i < 30; i++ {
		if err := db.PingContext(ctx); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("database not reachable: %v", err)
	}

	migDir := migrationsDir(t)

	// --- Run all migrations UP ---
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("failed to set goose dialect: %v", err)
	}
	if err := goose.Up(db, migDir); err != nil {
		t.Fatalf("goose up failed: %v", err)
	}

	t.Run("all_tables_exist", func(t *testing.T) {
		for _, table := range allExpectedTables {
			var exists bool
			err := db.QueryRowContext(ctx,
				`SELECT EXISTS (
					SELECT 1 FROM information_schema.tables
					WHERE table_schema = 'public' AND table_name = $1
				)`, table).Scan(&exists)
			if err != nil {
				t.Errorf("failed to check table %s: %v", table, err)
			}
			if !exists {
				t.Errorf("expected table %s to exist", table)
			}
		}
	})

	t.Run("rls_enabled", func(t *testing.T) {
		for _, table := range rlsTables {
			var rlsEnabled bool
			err := db.QueryRowContext(ctx,
				`SELECT relrowsecurity FROM pg_class WHERE relname = $1`,
				table).Scan(&rlsEnabled)
			if err != nil {
				t.Errorf("failed to check RLS for table %s: %v", table, err)
			}
			if !rlsEnabled {
				t.Errorf("expected RLS to be enabled on table %s", table)
			}
		}
	})

	t.Run("custom_types_exist", func(t *testing.T) {
		for _, typeName := range allExpectedTypes {
			var exists bool
			err := db.QueryRowContext(ctx,
				`SELECT EXISTS (
					SELECT 1 FROM pg_type WHERE typname = $1
				)`, typeName).Scan(&exists)
			if err != nil {
				t.Errorf("failed to check type %s: %v", typeName, err)
			}
			if !exists {
				t.Errorf("expected custom type %s to exist", typeName)
			}
		}
	})

	t.Run("updated_at_trigger_works", func(t *testing.T) {
		// Insert an organization then update it, verifying updated_at changes.
		_, err := db.ExecContext(ctx,
			`INSERT INTO organizations (id, name, slug) VALUES (uuid_generate_v4(), 'Test Org', 'test-org')`)
		if err != nil {
			t.Fatalf("failed to insert test organization: %v", err)
		}

		var beforeUpdate time.Time
		err = db.QueryRowContext(ctx,
			`SELECT updated_at FROM organizations WHERE slug = 'test-org'`).Scan(&beforeUpdate)
		if err != nil {
			t.Fatalf("failed to read updated_at: %v", err)
		}

		// Small sleep to ensure timestamp difference.
		time.Sleep(50 * time.Millisecond)

		_, err = db.ExecContext(ctx,
			`UPDATE organizations SET name = 'Test Org Updated' WHERE slug = 'test-org'`)
		if err != nil {
			t.Fatalf("failed to update test organization: %v", err)
		}

		var afterUpdate time.Time
		err = db.QueryRowContext(ctx,
			`SELECT updated_at FROM organizations WHERE slug = 'test-org'`).Scan(&afterUpdate)
		if err != nil {
			t.Fatalf("failed to read updated_at after update: %v", err)
		}

		if !afterUpdate.After(beforeUpdate) {
			t.Errorf("expected updated_at to advance after UPDATE; before=%v after=%v",
				beforeUpdate, afterUpdate)
		}
	})

	// --- Run all migrations DOWN ---
	t.Run("clean_rollback", func(t *testing.T) {
		if err := goose.DownTo(db, migDir, 0); err != nil {
			t.Fatalf("goose down failed: %v", err)
		}

		// After rolling back all migrations, only the goose version table should remain.
		rows, err := db.QueryContext(ctx,
			`SELECT table_name FROM information_schema.tables
			 WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`)
		if err != nil {
			t.Fatalf("failed to query tables after rollback: %v", err)
		}
		defer func() { _ = rows.Close() }()

		var remaining []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("failed to scan table name: %v", err)
			}
			remaining = append(remaining, name)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("error iterating rows after rollback: %v", err)
		}

		// goose_db_version is expected to remain.
		sort.Strings(remaining)
		for _, name := range remaining {
			if name != "goose_db_version" {
				t.Errorf("unexpected table after rollback: %s", name)
			}
		}

		// Verify custom types are gone.
		for _, typeName := range allExpectedTypes {
			var exists bool
			err := db.QueryRowContext(ctx,
				`SELECT EXISTS (
					SELECT 1 FROM pg_type
					WHERE typname = $1
					AND typnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public')
				)`, typeName).Scan(&exists)
			if err != nil {
				t.Errorf("failed to check type %s after rollback: %v", typeName, err)
			}
			if exists {
				t.Errorf("expected custom type %s to be dropped after rollback", typeName)
			}
		}
	})
}
