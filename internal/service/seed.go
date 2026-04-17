package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/storage"
	"github.com/ravencloak-org/Raven/internal/tmdb"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

const (
	demoOrgName  = "Raven Demo"
	demoOrgSlug  = "raven-demo"
	demoWSName   = "Movies"
	demoKBName   = "Movie Database"
	demoKBDesc   = "Top-rated movies from TMDB across multiple genres"
)

// SeedService orchestrates demo data creation: org, workspace, KB, and movie documents.
type SeedService struct {
	orgSvc  *OrgService
	wsSvc   *WorkspaceService
	kbSvc   *KBService
	docRepo *repository.DocumentRepository
	pool    *pgxpool.Pool
	tmdbCli *tmdb.Client
	store   storage.Client
	qCli    *queue.Client
}

// NewSeedService creates a new SeedService with all required dependencies.
func NewSeedService(
	orgSvc *OrgService,
	wsSvc *WorkspaceService,
	kbSvc *KBService,
	docRepo *repository.DocumentRepository,
	pool *pgxpool.Pool,
	tmdbCli *tmdb.Client,
	store storage.Client,
	qCli *queue.Client,
) *SeedService {
	return &SeedService{
		orgSvc:  orgSvc,
		wsSvc:   wsSvc,
		kbSvc:   kbSvc,
		docRepo: docRepo,
		pool:    pool,
		tmdbCli: tmdbCli,
		store:   store,
		qCli:    qCli,
	}
}

// SeedDemo creates (or returns) a demo org/workspace/KB populated with TMDB movies.
// It is idempotent: if the "raven-demo" org already exists, existing IDs are returned.
func (s *SeedService) SeedDemo(ctx context.Context, size string) (*model.SeedResult, error) {
	// 1. Idempotency check — look for existing demo org by slug.
	existing, err := s.orgSvc.GetBySlug(ctx, demoOrgSlug)
	if err != nil {
		return nil, apierror.NewInternal("failed to check for existing demo org: " + err.Error())
	}
	if existing != nil {
		return s.buildExistingResult(ctx, existing)
	}

	// 2. Create the demo org (this also auto-creates a "Default" workspace, but we
	//    create our own "Movies" workspace explicitly).
	org, err := s.orgSvc.Create(ctx, model.CreateOrgRequest{Name: demoOrgName})
	if err != nil {
		return nil, fmt.Errorf("seed: create org: %w", err)
	}

	// 3. Create workspace.
	ws, err := s.wsSvc.Create(ctx, org.ID, model.CreateWorkspaceRequest{Name: demoWSName})
	if err != nil {
		return nil, fmt.Errorf("seed: create workspace: %w", err)
	}

	// 4. Create knowledge base.
	kb, err := s.kbSvc.Create(ctx, org.ID, ws.ID, model.CreateKBRequest{
		Name:        demoKBName,
		Description: demoKBDesc,
	})
	if err != nil {
		return nil, fmt.Errorf("seed: create knowledge base: %w", err)
	}

	// 5. Fetch movies from TMDB.
	if s.tmdbCli == nil {
		return nil, apierror.NewInternal("TMDB client not configured — set TMDB_API_KEY")
	}
	movies, err := s.tmdbCli.FetchTopByGenres(ctx, size)
	if err != nil {
		return nil, fmt.Errorf("seed: fetch movies: %w", err)
	}

	// 6. For each movie: render markdown, upload to SeaweedFS, create document, enqueue.
	enqueued := 0
	var failures int
	for _, movie := range movies {
		if processErr := s.processMovie(ctx, org.ID, kb.ID, movie); processErr != nil {
			slog.Warn("seed: failed to process movie, skipping",
				"movie_title", movie.Title,
				"error", processErr,
			)
			failures++
			continue
		}
		enqueued++
	}

	if enqueued == 0 && failures > 0 {
		return nil, apierror.NewInternal(fmt.Sprintf("seed: all %d movies failed to process", failures))
	}

	return &model.SeedResult{
		OrgID:             org.ID,
		WorkspaceID:       ws.ID,
		KBID:              kb.ID,
		DocumentsEnqueued: enqueued,
		PipelineStatus:    "processing",
	}, nil
}

// processMovie renders a single movie to markdown, uploads it, creates a document record,
// and enqueues a processing job.
func (s *SeedService) processMovie(ctx context.Context, orgID, kbID string, movie tmdb.MovieDetail) error {
	md := tmdb.RenderMarkdown(movie)
	filename := fmt.Sprintf("%s.md", sanitizeFilename(movie.Title))

	// Upload markdown content to SeaweedFS.
	fid, err := s.store.Upload(ctx, filename, strings.NewReader(md))
	if err != nil {
		return fmt.Errorf("upload %q: %w", filename, err)
	}

	// Create document record via repository (needs RLS transaction).
	fileSize := int64(len(md))
	doc := &model.Document{
		OrgID:           orgID,
		KnowledgeBaseID: kbID,
		FileName:        filename,
		FileType:        "text/markdown",
		FileSizeBytes:   &fileSize,
		StoragePath:     fid,
		Title:           movie.Title,
		Metadata: map[string]any{
			"source":    "tmdb",
			"tmdb_id":   movie.ID,
			"genre":     genreNames(movie.Genres),
			"seed_demo": true,
		},
	}

	var created *model.Document
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var createErr error
		created, createErr = s.docRepo.Create(ctx, tx, doc)
		return createErr
	})
	if err != nil {
		return fmt.Errorf("create document record: %w", err)
	}

	// Enqueue document processing job.
	if err := s.qCli.EnqueueDocumentProcess(ctx, queue.DocumentProcessPayload{
		OrgID:           orgID,
		DocumentID:      created.ID,
		KnowledgeBaseID: kbID,
	}); err != nil {
		return fmt.Errorf("enqueue processing: %w", err)
	}

	return nil
}

// buildExistingResult looks up the workspace and KB for an already-seeded org.
func (s *SeedService) buildExistingResult(ctx context.Context, org *model.Organization) (*model.SeedResult, error) {
	workspaces, err := s.wsSvc.ListByOrg(ctx, org.ID)
	if err != nil {
		return nil, apierror.NewInternal("seed: list workspaces: " + err.Error())
	}

	// Find the "Movies" workspace.
	var wsID string
	for _, ws := range workspaces {
		if ws.Slug == toSlug(demoWSName) {
			wsID = ws.ID
			break
		}
	}
	if wsID == "" && len(workspaces) > 0 {
		// Fallback to first workspace.
		wsID = workspaces[0].ID
	}

	// Find the KB in that workspace.
	var kbID string
	if wsID != "" {
		kbs, kbErr := s.kbSvc.ListByWorkspace(ctx, org.ID, wsID)
		if kbErr == nil {
			for _, kb := range kbs {
				if kb.Slug == toSlug(demoKBName) {
					kbID = kb.ID
					break
				}
			}
			if kbID == "" && len(kbs) > 0 {
				kbID = kbs[0].ID
			}
		}
	}

	return &model.SeedResult{
		OrgID:             org.ID,
		WorkspaceID:       wsID,
		KBID:              kbID,
		DocumentsEnqueued: 0,
		PipelineStatus:    "ready",
	}, nil
}

// sanitizeFilename replaces non-alphanumeric characters with hyphens for safe filenames.
func sanitizeFilename(name string) string {
	s := strings.ToLower(name)
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// genreNames extracts genre name strings from TMDB genre structs.
func genreNames(genres []tmdb.Genre) []string {
	names := make([]string, len(genres))
	for i, g := range genres {
		names[i] = g.Name
	}
	return names
}
