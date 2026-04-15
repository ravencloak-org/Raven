//go:build integration

package integration

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
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/service"
)

var (
	testPool       *pgxpool.Pool
	testSearchSvc  *service.SearchService
	testDocSvc     *service.DocumentService
	testSourceSvc  *service.SourceService
	testCacheRepo  *repository.SemanticCacheRepository
	testSearchRepo *repository.SearchRepository
	testDocRepo    *repository.DocumentRepository
	testSourceRepo *repository.SourceRepository
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start Postgres container with pgvector
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg18",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "raven_test",
			"POSTGRES_PASSWORD": "raven_test",
			"POSTGRES_DB":       "raven_test",
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
		panic(fmt.Sprintf("failed to start container: %v", err))
	}
	host, err := container.Host(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get container host: %v", err))
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		panic(fmt.Sprintf("failed to get container port: %v", err))
	}

	dsn := fmt.Sprintf("postgres://raven_test:raven_test@%s:%s/raven_test?sslmode=disable", host, port.Port())

	// Resolve migration path relative to this source file
	_, thisFile, _, _ := runtime.Caller(0)
	migDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")

	// Run goose migrations
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(fmt.Sprintf("failed to open sql.DB: %v", err))
	}
	if err := goose.SetDialect("postgres"); err != nil {
		panic(fmt.Sprintf("failed to set goose dialect: %v", err))
	}
	if err := goose.Up(sqlDB, migDir); err != nil {
		panic(fmt.Sprintf("failed to run migrations: %v", err))
	}
	sqlDB.Close()

	// Create pgxpool for tests
	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		panic(fmt.Sprintf("failed to create pool: %v", err))
	}

	// Initialize repositories
	testSearchRepo = repository.NewSearchRepository(testPool)
	testCacheRepo = repository.NewSemanticCacheRepository(testPool)
	testDocRepo = repository.NewDocumentRepository(testPool)
	testSourceRepo = repository.NewSourceRepository(testPool)

	// Initialize services
	testSearchSvc = service.NewSearchService(testSearchRepo, testPool)
	testDocSvc = service.NewDocumentService(testDocRepo, testPool)
	testSourceSvc = service.NewSourceService(testSourceRepo, testPool)

	// Run tests, then clean up explicitly (os.Exit skips defers).
	code := m.Run()

	testPool.Close()
	_ = container.Terminate(ctx)

	os.Exit(code)
}
