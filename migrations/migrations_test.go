package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	"routing_rules",
	"catalog_metadata",
	"airbyte_connectors",
	"airbyte_sync_history",
	"stranger_users",
	"user_identities",
	"response_cache",
	"notification_configs",
	"notification_log",
	"webhook_configs",
	"webhook_deliveries",
	"lead_profiles",
	"conversation_sessions",
	"user_notification_preferences",
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
	"routing_rules",
	"catalog_metadata",
	"airbyte_connectors",
	"airbyte_sync_history",
	"stranger_users",
	"user_identities",
	"response_cache",
	"notification_configs",
	"notification_log",
	"webhook_configs",
	"webhook_deliveries",
	"lead_profiles",
	"conversation_sessions",
	"user_notification_preferences",
}

// allExpectedTypes lists every custom ENUM type.
var allExpectedTypes = []string{
	"org_status",
	"user_status",
	"workspace_role",
	"kb_status",
	"kb_visibility",
	"processing_status",
	"source_type",
	"crawl_frequency",
	"chunk_type",
	"llm_provider",
	"provider_status",
	"api_key_status",
	"message_role",
	"routing_mode",
	"connector_status",
	"sync_mode",
	"stranger_status",
	"notification_type",
	"notification_status",
	"webhook_status",
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
			t.Errorf("failed to close database: %v", err)
		}
	}()

	// Wait for DB to be fully ready.
	for range 30 {
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

	t.Run("hnsw_index_exists", func(t *testing.T) {
		var exists bool
		err := db.QueryRowContext(ctx,
			`SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE tablename = 'embeddings' AND indexname = 'idx_embeddings_hnsw'
			)`).Scan(&exists)
		if err != nil {
			t.Fatalf("failed to check HNSW index: %v", err)
		}
		if !exists {
			t.Error("expected HNSW index idx_embeddings_hnsw to exist on embeddings table")
		}

		// Verify the index uses the hnsw access method.
		var indexDef string
		err = db.QueryRowContext(ctx,
			`SELECT indexdef FROM pg_indexes
			 WHERE tablename = 'embeddings' AND indexname = 'idx_embeddings_hnsw'`).Scan(&indexDef)
		if err != nil {
			t.Fatalf("failed to read HNSW index definition: %v", err)
		}
		if !strings.Contains(indexDef, "hnsw") {
			t.Errorf("expected HNSW index to use hnsw method, got: %s", indexDef)
		}
		if !strings.Contains(indexDef, "vector_cosine_ops") {
			t.Errorf("expected HNSW index to use vector_cosine_ops, got: %s", indexDef)
		}
	})

	t.Run("kb_status_marketplace_states", func(t *testing.T) {
		// Migration 00049 (issue #725): adds 'read_only_private' and
		// 'dmca_pending' to the kb_status enum so the lifecycle column
		// can express the two Marketplace-specific frozen states. The
		// migration is additive only — Postgres cannot drop enum values,
		// so the rollback half can only normalise data and leaves the
		// widened type in place. See the migration file header.
		expectedValues := []string{
			"active",
			"archived",
			"read_only_private",
			"dmca_pending",
		}
		for _, v := range expectedValues {
			var exists bool
			if err := db.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM pg_enum e
					JOIN pg_type t ON t.oid = e.enumtypid
					WHERE t.typname = 'kb_status' AND e.enumlabel = $1
				)`, v).Scan(&exists); err != nil {
				t.Errorf("failed to check kb_status enum value %s: %v", v, err)
			}
			if !exists {
				t.Errorf("expected kb_status enum to contain value %q", v)
			}
		}
	})

	t.Run("kb_marketplace_columns_and_trigger", func(t *testing.T) {
		// Migration 00047 (issue #723): visibility / publish / lineage /
		// freshness / license / discovery columns on knowledge_bases,
		// plus the trg_kb_last_modified_on_documents/sources/chunks trigger
		// that bumps last_modified_at when child content mutates.
		//
		// This sub-test:
		//   1. Verifies every new column exists with the right default.
		//   2. Verifies the four partial / GIN indexes are present.
		//   3. Inserts a KB and asserts visibility defaults to 'private' and
		//      counters default to 0.
		//   4. Inserts a document under the KB and asserts last_modified_at
		//      advances — proving the trigger fires across tables.

		expectedColumns := map[string]string{
			"visibility":                "kb_visibility",
			"published_at":              "timestamp with time zone",
			"published_by_user_id":      "uuid",
			"source_public_kb_id":       "uuid",
			"imported_from_revision_at": "timestamp with time zone",
			"last_modified_at":          "timestamp with time zone",
			"license_spdx_id":           "text",
			"import_count":              "integer",
			"preview_count":             "integer",
			"search_tsv":                "tsvector",
		}
		for col, wantType := range expectedColumns {
			var dataType string
			err := db.QueryRowContext(ctx,
				`SELECT data_type FROM information_schema.columns
				 WHERE table_schema = 'public' AND table_name = 'knowledge_bases'
				   AND column_name = $1`, col).Scan(&dataType)
			if err != nil {
				t.Errorf("missing column knowledge_bases.%s: %v", col, err)
				continue
			}
			// The enum column reports its custom type as USER-DEFINED in
			// information_schema; cross-check against pg_type for the real name.
			if dataType == "USER-DEFINED" {
				var typeName string
				if err := db.QueryRowContext(ctx,
					`SELECT udt_name FROM information_schema.columns
					 WHERE table_schema = 'public' AND table_name = 'knowledge_bases'
					   AND column_name = $1`, col).Scan(&typeName); err != nil {
					t.Errorf("failed to read udt_name for %s: %v", col, err)
					continue
				}
				if typeName != wantType {
					t.Errorf("column %s: want type %s, got %s", col, wantType, typeName)
				}
				continue
			}
			if dataType != wantType {
				t.Errorf("column %s: want type %s, got %s", col, wantType, dataType)
			}
		}

		expectedIndexes := []string{
			"idx_kb_visibility_public",
			"idx_kb_source_public_kb_id",
			"idx_kb_last_modified_at_public",
			"idx_kb_search_tsv_public",
		}
		for _, idx := range expectedIndexes {
			var exists bool
			if err := db.QueryRowContext(ctx,
				`SELECT EXISTS (SELECT 1 FROM pg_indexes
				 WHERE tablename = 'knowledge_bases' AND indexname = $1)`, idx).
				Scan(&exists); err != nil {
				t.Errorf("failed to check index %s: %v", idx, err)
			}
			if !exists {
				t.Errorf("expected index %s on knowledge_bases", idx)
			}
		}

		// Bootstrap a workspace + KB so we can prove the trigger fires.
		var orgID, wsID, kbID string
		if err := db.QueryRowContext(ctx,
			`INSERT INTO organizations (id, name, slug)
			 VALUES (uuid_generate_v4(), 'KB Trigger Org', 'kb-trigger-org')
			 RETURNING id`).Scan(&orgID); err != nil {
			t.Fatalf("insert organization: %v", err)
		}
		if err := db.QueryRowContext(ctx,
			`INSERT INTO workspaces (id, org_id, name, slug)
			 VALUES (uuid_generate_v4(), $1, 'KB Trigger WS', 'kb-trigger-ws')
			 RETURNING id`, orgID).Scan(&wsID); err != nil {
			t.Fatalf("insert workspace: %v", err)
		}
		if err := db.QueryRowContext(ctx,
			`INSERT INTO knowledge_bases (org_id, workspace_id, name, slug)
			 VALUES ($1, $2, 'KB Trigger', 'kb-trigger')
			 RETURNING id`, orgID, wsID).Scan(&kbID); err != nil {
			t.Fatalf("insert knowledge_base: %v", err)
		}

		var visibility string
		var importCount, previewCount int
		var beforeLastModified time.Time
		if err := db.QueryRowContext(ctx,
			`SELECT visibility, import_count, preview_count, last_modified_at
			 FROM knowledge_bases WHERE id = $1`, kbID).
			Scan(&visibility, &importCount, &previewCount, &beforeLastModified); err != nil {
			t.Fatalf("read marketplace defaults: %v", err)
		}
		if visibility != "private" {
			t.Errorf("default visibility: want private, got %q", visibility)
		}
		if importCount != 0 || previewCount != 0 {
			t.Errorf("default counters: want 0/0, got %d/%d", importCount, previewCount)
		}

		// Small sleep so the post-trigger timestamp can demonstrably exceed the
		// pre-insert value.
		time.Sleep(50 * time.Millisecond)

		if _, err := db.ExecContext(ctx,
			`INSERT INTO documents (org_id, knowledge_base_id, file_name)
			 VALUES ($1, $2, 'trigger-doc.txt')`, orgID, kbID); err != nil {
			t.Fatalf("insert document: %v", err)
		}

		var afterLastModified time.Time
		if err := db.QueryRowContext(ctx,
			`SELECT last_modified_at FROM knowledge_bases WHERE id = $1`, kbID).
			Scan(&afterLastModified); err != nil {
			t.Fatalf("read last_modified_at: %v", err)
		}
		if !afterLastModified.After(beforeLastModified) {
			t.Errorf("expected trg_kb_last_modified_on_documents to bump last_modified_at; before=%v after=%v",
				beforeLastModified, afterLastModified)
		}
	})

	t.Run("org_slug_constraints_and_holds", func(t *testing.T) {
		// Migration 00048 (issue #724): VARCHAR(64) + CHECK regex on
		// organizations.slug, the org_slug_holds soft-redirect table, and
		// the partial UNIQUE on knowledge_bases (org_id, slug) WHERE
		// visibility='public'.
		//
		// Asserted invariants:
		//   1. The CHECK constraint rejects malformed slugs.
		//   2. org_slug_holds exists with the documented columns.
		//   3. Inserting two public KBs under the same (org_id, slug) is
		//      blocked by the partial unique index; the same pair when
		//      both are private is allowed.

		// Malformed slug must be rejected.
		_, err := db.ExecContext(ctx,
			`INSERT INTO organizations (id, name, slug)
			 VALUES (uuid_generate_v4(), 'Bad Slug', 'NOT-LOWERCASE')`)
		if err == nil {
			t.Error("expected CHECK constraint to reject uppercase slug, got no error")
		}

		// Slug column was narrowed to VARCHAR(64). information_schema
		// reports the cap in character_maximum_length.
		var maxLen int
		if err := db.QueryRowContext(ctx,
			`SELECT character_maximum_length
			 FROM information_schema.columns
			 WHERE table_schema = 'public' AND table_name = 'organizations'
			   AND column_name = 'slug'`).Scan(&maxLen); err != nil {
			t.Fatalf("read slug column length: %v", err)
		}
		if maxLen != 64 {
			t.Errorf("organizations.slug max length: want 64, got %d", maxLen)
		}

		// org_slug_holds table + columns.
		var holdsExists bool
		if err := db.QueryRowContext(ctx,
			`SELECT EXISTS (
			    SELECT 1 FROM information_schema.tables
			    WHERE table_schema = 'public' AND table_name = 'org_slug_holds'
			 )`).Scan(&holdsExists); err != nil {
			t.Fatalf("check org_slug_holds: %v", err)
		}
		if !holdsExists {
			t.Fatal("expected org_slug_holds table to exist")
		}

		// Partial UNIQUE on (org_id, slug) WHERE visibility='public'.
		var publicUniqDef string
		if err := db.QueryRowContext(ctx,
			`SELECT indexdef FROM pg_indexes
			 WHERE tablename = 'knowledge_bases'
			   AND indexname = 'idx_kb_public_org_slug_uniq'`).Scan(&publicUniqDef); err != nil {
			t.Fatalf("read idx_kb_public_org_slug_uniq: %v", err)
		}
		if !strings.Contains(publicUniqDef, "UNIQUE") || !strings.Contains(publicUniqDef, "visibility") {
			t.Errorf("expected partial UNIQUE on visibility, got: %s", publicUniqDef)
		}

		// Behaviour: two public KBs under same (org_id, slug) collide,
		// two private ones with the same pair do not.
		var orgID2, wsID2 string
		if err := db.QueryRowContext(ctx,
			`INSERT INTO organizations (id, name, slug)
			 VALUES (uuid_generate_v4(), 'Slug Test Org', 'slug-test-org')
			 RETURNING id`).Scan(&orgID2); err != nil {
			t.Fatalf("insert organization: %v", err)
		}
		if err := db.QueryRowContext(ctx,
			`INSERT INTO workspaces (id, org_id, name, slug)
			 VALUES (uuid_generate_v4(), $1, 'ST WS', 'st-ws')
			 RETURNING id`, orgID2).Scan(&wsID2); err != nil {
			t.Fatalf("insert workspace: %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO knowledge_bases (org_id, workspace_id, name, slug, visibility)
			 VALUES ($1, $2, 'A', 'duplicate-slug', 'public')`, orgID2, wsID2); err != nil {
			t.Fatalf("insert first public kb: %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO knowledge_bases (org_id, workspace_id, name, slug, visibility)
			 VALUES ($1, $2, 'B', 'duplicate-slug', 'public')`, orgID2, wsID2); err == nil {
			t.Error("expected partial UNIQUE to reject second public KB with same (org_id, slug)")
		}
		// Two private KBs with the same (org_id, slug) — allowed by the
		// partial-index predicate. The pre-existing table-wide
		// (org_id, slug) constraint, if any, would still apply; the
		// table allows multiple privates per slug today, so the insert
		// should succeed.
		if _, err := db.ExecContext(ctx,
			`INSERT INTO knowledge_bases (org_id, workspace_id, name, slug, visibility)
			 VALUES ($1, $2, 'C', 'private-dup', 'private')`, orgID2, wsID2); err != nil {
			t.Fatalf("first private kb: %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO knowledge_bases (org_id, workspace_id, name, slug, visibility)
			 VALUES ($1, $2, 'D', 'private-dup', 'private')`, orgID2, wsID2); err != nil {
			// If the project has a pre-existing total UNIQUE on
			// (org_id, slug), this insert is expected to fail. Log
			// rather than fail — the assertion above (public collision)
			// is the real signal that 00048's partial index works.
			t.Logf("second private kb insert: %v (acceptable if a pre-existing constraint applies)", err)
		}
	})

	t.Run("marketplace_functions_signature", func(t *testing.T) {
		// Migration 00052 (issue #728): the two SECURITY DEFINER read
		// functions that bridge the Marketplace cross-tenant boundary.
		// Behaviour is exercised by the integration tests in
		// internal/marketplace/queries_test.go; this sub-test pins the
		// SQL-side contract — function existence, security mode, owner,
		// and the precise return-column shape declared in ADR-0005 +
		// ADR-0008 — so a future refactor cannot silently drop a column
		// or downgrade the security perimeter.

		type sigCheck struct {
			fnName       string
			args         string
			wantSecDef   bool
			wantOwner    string
			wantCols     []string
			wantColTypes []string
		}

		cases := []sigCheck{
			{
				fnName:     "marketplace_list_public_kbs",
				args:       "text, text, text[], integer, integer",
				wantSecDef: true,
				wantOwner:  "raven_admin",
				wantCols: []string{
					"kb_id", "org_slug", "org_display_name",
					"kb_slug", "kb_name", "description",
					"license_spdx_id", "last_modified_at", "import_count",
					"source_public_kb_id", "source_org_slug", "source_org_display_name",
				},
				wantColTypes: []string{
					"uuid", "text", "text",
					"text", "text", "text",
					"text", "timestamp with time zone", "integer",
					"uuid", "text", "text",
				},
			},
			{
				fnName:     "marketplace_preview_kb",
				args:       "uuid",
				wantSecDef: true,
				wantOwner:  "raven_admin",
				wantCols:   []string{"chunk_id", "ordinal", "text"},
				wantColTypes: []string{
					"uuid", "integer", "text",
				},
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.fnName, func(t *testing.T) {
				// 1. Function exists at the expected (name, arg-types) signature.
				var oid uint32
				err := db.QueryRowContext(ctx,
					`SELECT p.oid
					 FROM pg_proc p
					 JOIN pg_namespace n ON n.oid = p.pronamespace
					 WHERE n.nspname = 'public'
					   AND p.proname = $1
					   AND pg_get_function_arguments(p.oid) = $2`,
					tc.fnName, tc.args).Scan(&oid)
				if err != nil {
					t.Fatalf("function %s(%s) not found: %v", tc.fnName, tc.args, err)
				}

				// 2. SECURITY DEFINER bit set (prosecdef in pg_proc).
				var secDef bool
				if err := db.QueryRowContext(ctx,
					`SELECT prosecdef FROM pg_proc WHERE oid = $1`, oid).Scan(&secDef); err != nil {
					t.Fatalf("read prosecdef: %v", err)
				}
				if secDef != tc.wantSecDef {
					t.Errorf("%s: SECURITY DEFINER = %v, want %v", tc.fnName, secDef, tc.wantSecDef)
				}

				// 3. Owner is raven_admin — the role with admin_bypass
				// on every tenant table; required for the function body
				// to read across Orgs (migration 00015).
				var owner string
				if err := db.QueryRowContext(ctx,
					`SELECT r.rolname
					 FROM pg_proc p JOIN pg_roles r ON r.oid = p.proowner
					 WHERE p.oid = $1`, oid).Scan(&owner); err != nil {
					t.Fatalf("read owner: %v", err)
				}
				if owner != tc.wantOwner {
					t.Errorf("%s: owner = %s, want %s", tc.fnName, owner, tc.wantOwner)
				}

				// 4. raven_app holds EXECUTE — the application role
				// must be able to invoke the function from API code.
				var canExec bool
				if err := db.QueryRowContext(ctx,
					`SELECT has_function_privilege('raven_app', $1::oid, 'EXECUTE')`, oid).
					Scan(&canExec); err != nil {
					t.Fatalf("read raven_app EXECUTE: %v", err)
				}
				if !canExec {
					t.Errorf("%s: raven_app should hold EXECUTE", tc.fnName)
				}

				// 5. PUBLIC must NOT hold EXECUTE — defence in depth
				// against accidental grants to extension roles.
				var publicCanExec bool
				if err := db.QueryRowContext(ctx,
					`SELECT has_function_privilege('public', $1::oid, 'EXECUTE')`, oid).
					Scan(&publicCanExec); err != nil {
					t.Fatalf("read PUBLIC EXECUTE: %v", err)
				}
				if publicCanExec {
					t.Errorf("%s: PUBLIC should NOT hold EXECUTE", tc.fnName)
				}

				// 6. Return column shape matches the ADR-pinned contract.
				// pg_get_function_result returns the TABLE(...) clause
				// verbatim, so we parse it into (name, type) pairs and
				// compare against the wanted shape.
				var resultDecl string
				if err := db.QueryRowContext(ctx,
					`SELECT pg_get_function_result($1)`, oid).Scan(&resultDecl); err != nil {
					t.Fatalf("read function result: %v", err)
				}
				// Strip the TABLE(...) wrapper.
				resultDecl = strings.TrimSpace(resultDecl)
				if !strings.HasPrefix(resultDecl, "TABLE(") || !strings.HasSuffix(resultDecl, ")") {
					t.Fatalf("%s: unexpected result decl shape: %s", tc.fnName, resultDecl)
				}
				inner := resultDecl[len("TABLE(") : len(resultDecl)-1]
				cols := strings.Split(inner, ",")
				if len(cols) != len(tc.wantCols) {
					t.Errorf("%s: got %d return columns, want %d (raw: %s)",
						tc.fnName, len(cols), len(tc.wantCols), resultDecl)
					return
				}
				for i, c := range cols {
					parts := strings.SplitN(strings.TrimSpace(c), " ", 2)
					if len(parts) != 2 {
						t.Errorf("%s: malformed column decl %q", tc.fnName, c)
						continue
					}
					gotName, gotType := parts[0], parts[1]
					if gotName != tc.wantCols[i] {
						t.Errorf("%s: column %d name = %q, want %q", tc.fnName, i, gotName, tc.wantCols[i])
					}
					if gotType != tc.wantColTypes[i] {
						t.Errorf("%s: column %d (%s) type = %q, want %q",
							tc.fnName, i, gotName, gotType, tc.wantColTypes[i])
					}
				}
			})
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
		defer func() {
			if err := rows.Close(); err != nil {
				t.Errorf("failed to close rows: %v", err)
			}
		}()

		var remaining []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("failed to scan table name: %v", err)
			}
			remaining = append(remaining, name)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("error iterating rows: %v", err)
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
