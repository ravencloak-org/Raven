//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/internal/storage"
	"github.com/ravencloak-org/Raven/internal/testutil"
	"github.com/ravencloak-org/Raven/internal/tmdb"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ---------------------------------------------------------------------------
// Test fakes
// ---------------------------------------------------------------------------

// inMemoryStorage is an in-memory implementation of storage.Client that the
// seed pipeline can upload markdown blobs to without talking to SeaweedFS.
type inMemoryStorage struct {
	mu    sync.Mutex
	files map[string][]byte
	seq   int
}

func newInMemoryStorage() *inMemoryStorage {
	return &inMemoryStorage{files: make(map[string][]byte)}
}

func (s *inMemoryStorage) Upload(_ context.Context, _ string, r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	fid := fmt.Sprintf("fake-fid-%d", s.seq)
	s.files[fid] = b
	return fid, nil
}

func (s *inMemoryStorage) Download(_ context.Context, fid string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.files[fid]
	if !ok {
		return nil, fmt.Errorf("inMemoryStorage: fid %q not found", fid)
	}
	return io.NopCloser(strings.NewReader(string(b))), nil
}

func (s *inMemoryStorage) Delete(_ context.Context, fid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, fid)
	return nil
}

func (s *inMemoryStorage) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.files)
}

var _ storage.Client = (*inMemoryStorage)(nil)

// newFakeTMDBServer returns an httptest server that mimics the subset of the
// TMDB API exercised by tmdb.Client.FetchTopByGenres with size="small":
// /genre/movie/list, /discover/movie (5 genres × 10 movies = 50 unique),
// /movie/{id} for per-movie details.
func newFakeTMDBServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Genre names must match tmdb.targetGenres; IDs match real TMDB values
	// but only their uniqueness matters for the test.
	genres := []tmdb.Genre{
		{ID: 28, Name: "Action"},
		{ID: 35, Name: "Comedy"},
		{ID: 18, Name: "Drama"},
		{ID: 878, Name: "Science Fiction"},
		{ID: 16, Name: "Animation"},
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/genre/movie/list", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tmdb.GenreListResponse{Genres: genres})
	})

	mux.HandleFunc("/discover/movie", func(w http.ResponseWriter, r *http.Request) {
		genreID, _ := strconv.Atoi(r.URL.Query().Get("with_genres"))
		results := make([]tmdb.MovieSummary, 0, 10)
		for i := 1; i <= 10; i++ {
			// Compose stable unique IDs so the five genres together yield 50
			// distinct movie IDs, matching the issue's "50 documents" target.
			movieID := genreID*1000 + i
			results = append(results, tmdb.MovieSummary{
				ID:          movieID,
				Title:       fmt.Sprintf("Movie %d", movieID),
				Overview:    fmt.Sprintf("Overview for movie %d", movieID),
				ReleaseDate: "2020-01-01",
				VoteAverage: 8.5,
				VoteCount:   2000,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tmdb.DiscoverResponse{Results: results})
	})

	mux.HandleFunc("/movie/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/movie/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		detail := tmdb.MovieDetail{
			ID:          id,
			Title:       fmt.Sprintf("Movie %d", id),
			Overview:    fmt.Sprintf("Detailed overview for movie %d", id),
			ReleaseDate: "2020-01-01",
			VoteAverage: 8.5,
			VoteCount:   2000,
			Runtime:     120,
			Genres:      []tmdb.Genre{genres[0]},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(detail)
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Fixture + helpers
// ---------------------------------------------------------------------------

type seedFixture struct {
	pool        *pgxpool.Pool
	seedSvc     *service.SeedService
	storage     *inMemoryStorage
	miniredis   *miniredis.Miniredis
	queueClient *queue.Client
	tmdbServer  *httptest.Server
	router      *gin.Engine
	httpServer  *httptest.Server
	seedKey     string
}

// buildSeedFixture wires a SeedService against a real postgres container and a
// miniredis-backed asynq client, and stands up the admin HTTP route with the
// same X-Seed-Key middleware used in cmd/api/main.go.
//
// Pass tmdbAPIKey="" to leave SeedService.tmdbCli nil (exercises the missing-
// key path).
func buildSeedFixture(t *testing.T, tmdbAPIKey string) *seedFixture {
	t.Helper()

	pool := testutil.NewTestDB(t)

	// Repositories.
	orgRepo := repository.NewOrgRepository(pool)
	wsRepo := repository.NewWorkspaceRepository(pool)
	kbRepo := repository.NewKBRepository(pool)
	docRepo := repository.NewDocumentRepository(pool)

	// Services — quota nil is safe: OrgService has no quota, WorkspaceService
	// doesn't consult it in Create, KBService short-circuits when quota == nil.
	wsSvc := service.NewWorkspaceService(wsRepo, pool, nil)
	orgSvc := service.NewOrgService(orgRepo, wsSvc)
	kbSvc := service.NewKBService(kbRepo, pool, nil)

	// Storage + queue fakes.
	store := newInMemoryStorage()

	mr := miniredis.RunT(t)
	qc := queue.NewClient(mr.Addr(), queue.WithMaxRetry(1))
	t.Cleanup(func() { _ = qc.Close() })

	// TMDB fake.
	tmdbServer := newFakeTMDBServer(t)
	t.Cleanup(tmdbServer.Close)

	var tmdbCli *tmdb.Client
	if tmdbAPIKey != "" {
		tmdbCli = tmdb.NewClient(tmdbServer.URL, tmdbAPIKey, tmdbServer.Client())
	}

	seedSvc := service.NewSeedService(orgSvc, wsSvc, kbSvc, docRepo, pool, tmdbCli, store, qc)

	// Build router with the seed-key middleware that mirrors cmd/api/main.go.
	const seedKey = "test-seed-key"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	admin := r.Group("/api/v1/admin")
	admin.Use(func(c *gin.Context) {
		if c.GetHeader("X-Seed-Key") == seedKey {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "valid X-Seed-Key header required"})
	})
	h := handler.NewSeedHandler(seedSvc)
	admin.POST("/seed-demo", h.SeedDemo)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	return &seedFixture{
		pool:        pool,
		seedSvc:     seedSvc,
		storage:     store,
		miniredis:   mr,
		queueClient: qc,
		tmdbServer:  tmdbServer,
		router:      r,
		httpServer:  ts,
		seedKey:     seedKey,
	}
}

func (f *seedFixture) postSeedDemo(t *testing.T) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		f.httpServer.URL+"/api/v1/admin/seed-demo?size=small", nil)
	require.NoError(t, err)
	req.Header.Set("X-Seed-Key", f.seedKey)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, body
}

// countDocuments returns (total, queued) counts for a knowledge base.
func countDocuments(t *testing.T, pool *pgxpool.Pool, kbID string) (total, queued int) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM documents WHERE knowledge_base_id = $1`, kbID,
	).Scan(&total))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM documents
		 WHERE knowledge_base_id = $1 AND processing_status = 'queued'`, kbID,
	).Scan(&queued))
	return total, queued
}

// pendingAsynqTasks returns the count of tasks sitting in the default queue.
func pendingAsynqTasks(t *testing.T, mr *miniredis.Miniredis) int {
	t.Helper()
	insp := asynq.NewInspector(asynq.RedisClientOpt{Addr: mr.Addr()})
	t.Cleanup(func() { _ = insp.Close() })
	info, err := insp.GetQueueInfo("default")
	require.NoError(t, err)
	return info.Pending + info.Active
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSeedDemo_Integration_FullPipeline exercises the full admin seed endpoint:
// a real postgres DB, real services, a fake TMDB at httptest, an in-memory
// storage client, and a miniredis-backed asynq queue.  Asserts that the demo
// org/workspace/KB rows are written, that exactly 50 documents land in the
// `queued` state, and that 50 asynq tasks are enqueued on the default queue.
func TestSeedDemo_Integration_FullPipeline(t *testing.T) {
	f := buildSeedFixture(t, "dummy-api-key")

	resp, body := f.postSeedDemo(t)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", body)

	var got model.SeedResult
	require.NoError(t, json.Unmarshal(body, &got))

	assert.NotEmpty(t, got.OrgID, "org_id")
	assert.NotEmpty(t, got.WorkspaceID, "workspace_id")
	assert.NotEmpty(t, got.KBID, "kb_id")
	assert.Equal(t, 50, got.DocumentsEnqueued, "documents_enqueued")
	assert.Equal(t, "processing", got.PipelineStatus, "pipeline_status")

	// Verify the tenant rows landed in Postgres.
	ctx := context.Background()
	var orgSlug, wsSlug, kbSlug string
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT slug FROM organizations WHERE id = $1`, got.OrgID,
	).Scan(&orgSlug))
	assert.Equal(t, "raven-demo", orgSlug)

	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT slug FROM workspaces WHERE id = $1`, got.WorkspaceID,
	).Scan(&wsSlug))
	assert.Equal(t, "movies", wsSlug)

	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT slug FROM knowledge_bases WHERE id = $1`, got.KBID,
	).Scan(&kbSlug))
	assert.Equal(t, "movie-database", kbSlug)

	// 50 documents, all in `queued` state.
	total, queued := countDocuments(t, f.pool, got.KBID)
	assert.Equal(t, 50, total, "total documents")
	assert.Equal(t, 50, queued, "queued documents")

	// 50 asynq tasks enqueued on the default queue.
	assert.Equal(t, 50, pendingAsynqTasks(t, f.miniredis), "asynq pending tasks")

	// Markdown bodies uploaded to the fake storage.
	assert.Equal(t, 50, f.storage.count(), "uploaded blobs")
}

// TestSeedDemo_Integration_Idempotent verifies that re-invoking the seed
// endpoint returns the same org/workspace/KB IDs and does NOT insert duplicate
// document rows.
func TestSeedDemo_Integration_Idempotent(t *testing.T) {
	f := buildSeedFixture(t, "dummy-api-key")

	resp1, body1 := f.postSeedDemo(t)
	require.Equal(t, http.StatusOK, resp1.StatusCode, "first call body=%s", body1)
	var first model.SeedResult
	require.NoError(t, json.Unmarshal(body1, &first))

	resp2, body2 := f.postSeedDemo(t)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "second call body=%s", body2)
	var second model.SeedResult
	require.NoError(t, json.Unmarshal(body2, &second))

	assert.Equal(t, first.OrgID, second.OrgID, "org_id should be stable")
	assert.Equal(t, first.WorkspaceID, second.WorkspaceID, "workspace_id should be stable")
	assert.Equal(t, first.KBID, second.KBID, "kb_id should be stable")

	// Idempotent short-circuit returns 0 enqueued on repeat calls.
	assert.Equal(t, 0, second.DocumentsEnqueued, "repeat call should not re-enqueue")

	// Document count is still 50 — no duplicates.
	total, _ := countDocuments(t, f.pool, first.KBID)
	assert.Equal(t, 50, total, "no duplicate documents on second call")
}

// TestSeedDemo_Integration_MissingTMDBKey asserts that when the TMDB client is
// unconfigured (TMDB_API_KEY absent in prod), the handler surfaces a clear
// 500 with "TMDB_API_KEY" in the error body rather than crashing further
// down the pipeline.
func TestSeedDemo_Integration_MissingTMDBKey(t *testing.T) {
	f := buildSeedFixture(t, "") // no API key → tmdbCli is nil

	resp, body := f.postSeedDemo(t)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode, "body=%s", body)
	assert.Contains(t, string(body), "TMDB_API_KEY",
		"error message should mention TMDB_API_KEY so operators know the fix")

	// No org should have been committed on the failure path, but even if a
	// partial insert occurred, no documents should exist.
	ctx := context.Background()
	var docCount int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM documents`,
	).Scan(&docCount))
	assert.Equal(t, 0, docCount, "no documents should be persisted when TMDB is unconfigured")
}

// TestSeedDemo_Integration_RequiresSeedKey guards the admin middleware: a
// request without the X-Seed-Key header must be rejected with 401 before the
// handler runs.
func TestSeedDemo_Integration_RequiresSeedKey(t *testing.T) {
	f := buildSeedFixture(t, "dummy-api-key")

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		f.httpServer.URL+"/api/v1/admin/seed-demo?size=small", nil)
	require.NoError(t, err)
	// Deliberately omit X-Seed-Key.

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
