//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/internal/tmdb"
)

// --- test doubles -----------------------------------------------------------

type noopQuotaChecker struct{}

func (noopQuotaChecker) CheckKBQuota(context.Context, string) error          { return nil }
func (noopQuotaChecker) CheckSeatQuota(context.Context, string) error        { return nil }
func (noopQuotaChecker) CheckVoiceMinuteQuota(context.Context, string) error { return nil }
func (noopQuotaChecker) GetConcurrentVoiceLimit(context.Context, string) int { return 100 }
func (noopQuotaChecker) GetUsage(context.Context, string) (*model.UsageResponse, error) {
	return &model.UsageResponse{}, nil
}
func (noopQuotaChecker) GetOrgSubscription(context.Context, string) (*model.OrgSubscription, error) {
	return nil, nil
}

type memStorage struct {
	mu    sync.Mutex
	files map[string][]byte
}

func newMemStorage() *memStorage { return &memStorage{files: map[string][]byte{}} }

func (m *memStorage) Upload(_ context.Context, filename string, reader io.Reader) (string, error) {
	buf, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	fid := fmt.Sprintf("3,%s-%s", uuid.NewString()[:8], strings.TrimSuffix(filename, ".md"))
	m.mu.Lock()
	m.files[fid] = buf
	m.mu.Unlock()
	return fid, nil
}

func (m *memStorage) Download(_ context.Context, fid string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[fid]
	if !ok {
		return nil, fmt.Errorf("memStorage: not found: %s", fid)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memStorage) Delete(_ context.Context, fid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, fid)
	return nil
}

func (m *memStorage) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.files)
}

// --- TMDB mock --------------------------------------------------------------

type tmdbFixture struct {
	genres          []tmdb.Genre
	byGenre         map[int][]tmdb.MovieSummary
	details         map[int]tmdb.MovieDetail
	genresFailStatus int
}

func newTMDBServer(t *testing.T, fx tmdbFixture) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/genre/movie/list", func(w http.ResponseWriter, _ *http.Request) {
		if fx.genresFailStatus != 0 {
			w.WriteHeader(fx.genresFailStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tmdb.GenreListResponse{Genres: fx.genres})
	})

	mux.HandleFunc("/discover/movie", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.URL.Query().Get("with_genres"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tmdb.DiscoverResponse{Results: fx.byGenre[id]})
	})

	mux.HandleFunc("/movie/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/movie/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		detail, ok := fx.details[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(detail)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// fxDefault returns a fixture with 5 genres and 10 unique movies; movies 3 and 7
// appear in two genres each so the pipeline's dedup is exercised.
func fxDefault() tmdbFixture {
	genres := []tmdb.Genre{
		{ID: 28, Name: "Action"},
		{ID: 35, Name: "Comedy"},
		{ID: 18, Name: "Drama"},
		{ID: 878, Name: "Science Fiction"},
		{ID: 16, Name: "Animation"},
	}
	byGenre := map[int][]tmdb.MovieSummary{
		28:  {summary(1, "Action One"), summary(2, "Action Two"), summary(3, "Action+Comedy")},
		35:  {summary(3, "Action+Comedy"), summary(4, "Comedy One"), summary(5, "Comedy Two")},
		18:  {summary(6, "Drama One"), summary(7, "Drama+SciFi")},
		878: {summary(7, "Drama+SciFi"), summary(8, "SciFi One")},
		16:  {summary(9, "Animation One"), summary(10, "Animation Two")},
	}
	details := map[int]tmdb.MovieDetail{}
	for _, list := range byGenre {
		for _, s := range list {
			if _, ok := details[s.ID]; ok {
				continue
			}
			details[s.ID] = detail(s.ID, s.Title)
		}
	}
	return tmdbFixture{genres: genres, byGenre: byGenre, details: details}
}

func summary(id int, title string) tmdb.MovieSummary {
	return tmdb.MovieSummary{
		ID: id, Title: title, Overview: "Summary for " + title,
		ReleaseDate: "2020-01-01", VoteAverage: 7.5, VoteCount: 5000,
	}
}

func detail(id int, title string) tmdb.MovieDetail {
	return tmdb.MovieDetail{
		ID: id, Title: title, Overview: "Overview for " + title,
		ReleaseDate: "2020-01-01", VoteAverage: 7.5, VoteCount: 5000, Runtime: 120,
		Genres: []tmdb.Genre{{ID: 28, Name: "Action"}},
		Credits: tmdb.Credits{
			Cast: []tmdb.CastMember{{Name: "Lead Actor", Character: "Hero", Order: 0}},
			Crew: []tmdb.CrewMember{{Name: "Director Name", Job: "Director"}},
		},
		Reviews:  tmdb.Reviews{Results: []tmdb.Review{{Author: "user", Content: "Great movie"}}},
		Keywords: tmdb.Keywords{Keywords: []tmdb.Keyword{{Name: "sample"}}},
	}
}

// --- helpers ----------------------------------------------------------------

func newMiniredisQueue(t *testing.T) (*queue.Client, *asynq.Inspector) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	qCli := queue.NewClient(mr.Addr())
	t.Cleanup(func() { _ = qCli.Close() })

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: mr.Addr()})
	t.Cleanup(func() { _ = inspector.Close() })

	return qCli, inspector
}

func buildSeedService(t *testing.T, tmdbURL string, qCli *queue.Client) (*service.SeedService, *memStorage) {
	t.Helper()
	orgRepo := repository.NewOrgRepository(testPool)
	wsRepo := repository.NewWorkspaceRepository(testPool)
	kbRepo := repository.NewKBRepository(testPool)
	docRepo := repository.NewDocumentRepository(testPool)

	quota := noopQuotaChecker{}
	wsSvc := service.NewWorkspaceService(wsRepo, testPool, quota)
	kbSvc := service.NewKBService(kbRepo, testPool, quota)
	orgSvc := service.NewOrgService(orgRepo, wsSvc)

	tmdbCli := tmdb.NewClient(tmdbURL, "test-api-key", nil)
	store := newMemStorage()

	return service.NewSeedService(orgSvc, wsSvc, kbSvc, docRepo, testPool, tmdbCli, store, qCli), store
}

// removeDemoOrg drops the raven-demo org if one exists. Called at setup and
// teardown of each subtest so they don't interfere via the shared slug.
func removeDemoOrg(t *testing.T, ctx context.Context) {
	t.Helper()
	conn, err := testPool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET ROLE raven_admin")
	require.NoError(t, err)
	defer func() { _, _ = conn.Exec(ctx, "RESET ROLE") }()

	_, err = conn.Exec(ctx, "DELETE FROM organizations WHERE slug = 'raven-demo'")
	require.NoError(t, err)
}

func countDemoDocs(t *testing.T, ctx context.Context, orgID string) int {
	t.Helper()
	var n int
	err := db.WithOrgID(ctx, testPool, orgID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM documents
			 WHERE org_id = $1 AND metadata->>'source' = 'tmdb'`,
			orgID,
		).Scan(&n)
	})
	require.NoError(t, err)
	return n
}

// listPending returns pending tasks on the default queue. Asynq's inspector
// returns ErrQueueNotFound until the first enqueue creates the queue metadata,
// which we treat as zero pending tasks.
func listPending(t *testing.T, inspector *asynq.Inspector) []*asynq.TaskInfo {
	t.Helper()
	tasks, err := inspector.ListPendingTasks("default")
	if errors.Is(err, asynq.ErrQueueNotFound) {
		return nil
	}
	require.NoError(t, err)
	return tasks
}

// --- tests ------------------------------------------------------------------

func TestSeedDemoPipeline(t *testing.T) {
	ctx := context.Background()

	t.Run("fresh_seed_creates_pipeline_and_enqueues_all_movies", func(t *testing.T) {
		removeDemoOrg(t, ctx)
		t.Cleanup(func() { removeDemoOrg(t, ctx) })

		fx := fxDefault()
		wantMovies := len(fx.details)

		srv := newTMDBServer(t, fx)
		qCli, inspector := newMiniredisQueue(t)
		seedSvc, store := buildSeedService(t, srv.URL, qCli)

		result, err := seedSvc.SeedDemo(ctx, "small")
		require.NoError(t, err)
		require.NotNil(t, result)

		require.NotEmpty(t, result.OrgID)
		require.NotEmpty(t, result.WorkspaceID)
		require.NotEmpty(t, result.KBID)
		require.Equal(t, "processing", result.PipelineStatus)
		require.Equal(t, wantMovies, result.DocumentsEnqueued)

		orgRepo := repository.NewOrgRepository(testPool)
		org, err := orgRepo.GetBySlug(ctx, "raven-demo")
		require.NoError(t, err)
		require.NotNil(t, org)
		require.Equal(t, result.OrgID, org.ID)

		require.Equal(t, wantMovies, countDemoDocs(t, ctx, result.OrgID))
		require.Equal(t, wantMovies, store.count(), "one markdown upload per movie")

		tasks := listPending(t, inspector)
		require.Len(t, tasks, wantMovies)
		seenDocs := map[string]struct{}{}
		for _, task := range tasks {
			require.Equal(t, queue.TypeDocumentProcess, task.Type)

			var payload queue.DocumentProcessPayload
			require.NoError(t, json.Unmarshal(task.Payload, &payload))
			require.Equal(t, result.OrgID, payload.OrgID)
			require.Equal(t, result.KBID, payload.KnowledgeBaseID)
			require.NotEmpty(t, payload.DocumentID)

			_, dup := seenDocs[payload.DocumentID]
			require.False(t, dup, "document IDs in queue must be unique")
			seenDocs[payload.DocumentID] = struct{}{}
		}
	})

	t.Run("second_call_is_idempotent_and_enqueues_no_new_tasks", func(t *testing.T) {
		removeDemoOrg(t, ctx)
		t.Cleanup(func() { removeDemoOrg(t, ctx) })

		fx := fxDefault()
		wantMovies := len(fx.details)

		srv := newTMDBServer(t, fx)

		qCli1, inspector1 := newMiniredisQueue(t)
		svc1, _ := buildSeedService(t, srv.URL, qCli1)

		first, err := svc1.SeedDemo(ctx, "small")
		require.NoError(t, err)
		require.Equal(t, wantMovies, first.DocumentsEnqueued)
		require.Len(t, listPending(t, inspector1), wantMovies)

		qCli2, inspector2 := newMiniredisQueue(t)
		svc2, store2 := buildSeedService(t, srv.URL, qCli2)

		second, err := svc2.SeedDemo(ctx, "small")
		require.NoError(t, err)
		require.Equal(t, first.OrgID, second.OrgID)
		require.Equal(t, first.WorkspaceID, second.WorkspaceID)
		require.Equal(t, first.KBID, second.KBID)
		require.Equal(t, 0, second.DocumentsEnqueued)
		require.Equal(t, "ready", second.PipelineStatus)

		require.Len(t, listPending(t, inspector2), 0, "idempotent call must not enqueue")
		require.Equal(t, 0, store2.count(), "idempotent call must not upload")
		require.Equal(t, wantMovies, countDemoDocs(t, ctx, first.OrgID), "document count unchanged")
	})

	t.Run("tmdb_auth_failure_surfaces_error_and_creates_no_documents", func(t *testing.T) {
		removeDemoOrg(t, ctx)
		t.Cleanup(func() { removeDemoOrg(t, ctx) })

		fx := fxDefault()
		fx.genresFailStatus = http.StatusUnauthorized // simulates missing/bad TMDB_API_KEY

		srv := newTMDBServer(t, fx)
		qCli, inspector := newMiniredisQueue(t)
		seedSvc, store := buildSeedService(t, srv.URL, qCli)

		_, err := seedSvc.SeedDemo(ctx, "small")
		require.Error(t, err)
		require.Contains(t, err.Error(), "fetch movies")

		require.Len(t, listPending(t, inspector), 0)
		require.Equal(t, 0, store.count())

		// Org/workspace/KB are created before the TMDB call in the current flow,
		// so the org may exist — but no documents should have been inserted.
		orgRepo := repository.NewOrgRepository(testPool)
		if org, _ := orgRepo.GetBySlug(ctx, "raven-demo"); org != nil {
			require.Equal(t, 0, countDemoDocs(t, ctx, org.ID))
		}
	})
}
