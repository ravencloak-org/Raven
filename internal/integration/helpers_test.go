//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
)

type testOrg struct {
	OrgID       string
	WorkspaceID string
	KBID        string
	UserID      string
}

// seedOrg creates a full org hierarchy (org -> user -> workspace -> knowledge_base)
// using the raven_admin role to bypass RLS. Returns the IDs needed for subsequent inserts.
func seedOrg(t *testing.T, ctx context.Context, name string) testOrg {
	t.Helper()
	org := testOrg{
		OrgID:       uuid.NewString(),
		WorkspaceID: uuid.NewString(),
		KBID:        uuid.NewString(),
		UserID:      uuid.NewString(),
	}

	conn, err := testPool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET ROLE raven_admin")
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)`,
		org.OrgID, name, name)
	require.NoError(t, err)

	// Insert a user so documents can reference uploaded_by
	_, err = conn.Exec(ctx, `INSERT INTO users (id, org_id, email, display_name) VALUES ($1, $2, $3, $4)`,
		org.UserID, org.OrgID, name+"@test.local", name+"-user")
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `INSERT INTO workspaces (id, org_id, name, slug) VALUES ($1, $2, $3, $4)`,
		org.WorkspaceID, org.OrgID, name+"-ws", name+"-ws")
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `INSERT INTO knowledge_bases (id, org_id, workspace_id, name, slug) VALUES ($1, $2, $3, $4, $5)`,
		org.KBID, org.OrgID, org.WorkspaceID, name+"-kb", name+"-kb")
	require.NoError(t, err)

	_, err = conn.Exec(ctx, "RESET ROLE")
	require.NoError(t, err)

	return org
}

// seedKB creates an additional knowledge base within an existing org/workspace.
func seedKB(t *testing.T, ctx context.Context, orgID, workspaceID, name string) string {
	t.Helper()
	kbID := uuid.NewString()
	conn, err := testPool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET ROLE raven_admin")
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `INSERT INTO knowledge_bases (id, org_id, workspace_id, name, slug) VALUES ($1, $2, $3, $4, $5)`,
		kbID, orgID, workspaceID, name, name)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, "RESET ROLE")
	require.NoError(t, err)

	return kbID
}

func insertDocument(t *testing.T, ctx context.Context, orgID, kbID, userID, fileName, status string) string {
	t.Helper()
	docID := uuid.NewString()
	err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
            INSERT INTO documents (id, org_id, knowledge_base_id, file_name, file_type, file_size_bytes, file_hash, storage_path, processing_status, uploaded_by)
            VALUES ($1, $2, $3, $4, 'text/markdown', 2048, $5, '/test/path', $6::processing_status, $7::uuid)`,
			docID, orgID, kbID, fileName, uuid.NewString(), status, userID)
		return err
	})
	require.NoError(t, err)
	return docID
}

func insertChunk(t *testing.T, ctx context.Context, orgID, kbID, docID string, index int, content, heading string, tokenCount int) string {
	t.Helper()
	chunkID := uuid.NewString()
	err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
            INSERT INTO chunks (id, org_id, knowledge_base_id, document_id, content, chunk_index, token_count, page_number, heading, chunk_type)
            VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, 'text')`,
			chunkID, orgID, kbID, docID, content, index, tokenCount, heading)
		return err
	})
	require.NoError(t, err)
	return chunkID
}

func insertEmbedding(t *testing.T, ctx context.Context, orgID, chunkID string, embedding []float32) {
	t.Helper()
	err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
            INSERT INTO embeddings (id, org_id, chunk_id, embedding, model_name, dimensions)
            VALUES ($1, $2, $3, $4, 'text-embedding-3-small', 1536)`,
			uuid.NewString(), orgID, chunkID, embedding)
		return err
	})
	require.NoError(t, err)
}

func insertCacheEntry(t *testing.T, ctx context.Context, orgID, kbID, queryText string, embedding []float32, hitCount int) string {
	t.Helper()
	id := uuid.NewString()
	err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
            INSERT INTO response_cache (id, org_id, kb_id, query_text, query_embedding, response_text, sources, model_name, hit_count, expires_at)
            VALUES ($1, $2, $3, $4, $5, 'cached response for: ' || $4, '[]', 'gpt-4', $6, NOW() + INTERVAL '1 hour')`,
			id, orgID, kbID, queryText, embedding, hitCount)
		return err
	})
	require.NoError(t, err)
	return id
}

func insertExpiredCacheEntry(t *testing.T, ctx context.Context, orgID, kbID, queryText string, embedding []float32) string {
	t.Helper()
	id := uuid.NewString()
	err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
            INSERT INTO response_cache (id, org_id, kb_id, query_text, query_embedding, response_text, sources, model_name, hit_count, expires_at)
            VALUES ($1, $2, $3, $4, $5, 'expired response', '[]', 'gpt-4', 0, NOW() - INTERVAL '1 second')`,
			id, orgID, kbID, queryText, embedding)
		return err
	})
	require.NoError(t, err)
	return id
}

func cleanupOrg(t *testing.T, ctx context.Context, orgID string) {
	t.Helper()
	conn, err := testPool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET ROLE raven_admin")
	require.NoError(t, err)

	// CASCADE deletes handle dependent rows (users, workspaces, knowledge_bases, documents, etc.)
	_, err = conn.Exec(ctx, "DELETE FROM organizations WHERE id = $1", orgID)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, "RESET ROLE")
	require.NoError(t, err)
}

// testdataDir returns the absolute path to the testdata/ directory relative to
// this source file. This works regardless of the working directory.
func testdataDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "testdata")
}

func loadFixture[T any](t *testing.T, filename string) T {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(), filename))
	require.NoError(t, err)
	var result T
	require.NoError(t, json.Unmarshal(data, &result))
	return result
}

func generateEmbedding(seed int) []float32 {
	vec := make([]float32, 1536)
	for j := range vec {
		vec[j] = float32(math.Sin(float64(seed)*0.1 + float64(j)*0.01))
	}
	return vec
}
