package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// validStatusTransitions maps current status to the set of allowed next statuses.
var validStatusTransitions = map[model.ProcessingStatus][]model.ProcessingStatus{
	model.ProcessingStatusQueued:       {model.ProcessingStatusCrawling, model.ProcessingStatusFailed},
	model.ProcessingStatusCrawling:     {model.ProcessingStatusParsing, model.ProcessingStatusFailed},
	model.ProcessingStatusParsing:      {model.ProcessingStatusChunking, model.ProcessingStatusFailed},
	model.ProcessingStatusChunking:     {model.ProcessingStatusEmbedding, model.ProcessingStatusFailed},
	model.ProcessingStatusEmbedding:    {model.ProcessingStatusReady, model.ProcessingStatusFailed},
	model.ProcessingStatusFailed:       {model.ProcessingStatusQueued, model.ProcessingStatusReprocessing},
	model.ProcessingStatusReady:        {model.ProcessingStatusReprocessing},
	model.ProcessingStatusReprocessing: {model.ProcessingStatusCrawling, model.ProcessingStatusFailed},
}

// CacheInvalidator is the minimal subset of the semantic-cache repository
// the document service needs. Splitting it out keeps the dependency
// optional and keeps the import graph acyclic.
type CacheInvalidator interface {
	InvalidateKB(ctx context.Context, orgID, kbID string) (int64, error)
}

// DocumentService contains business logic for document management.
type DocumentService struct {
	repo             *repository.DocumentRepository
	pool             *pgxpool.Pool
	cacheInvalidator CacheInvalidator // optional; see #256
}

// NewDocumentService creates a new DocumentService.
func NewDocumentService(repo *repository.DocumentRepository, pool *pgxpool.Pool) *DocumentService {
	return &DocumentService{repo: repo, pool: pool}
}

// WithCacheInvalidator wires the semantic-cache repository so that document
// mutations flush the KB cache in the background. Returns the receiver so
// that main.go can chain the call fluently.
func (s *DocumentService) WithCacheInvalidator(inv CacheInvalidator) *DocumentService {
	s.cacheInvalidator = inv
	return s
}

// invalidateKBCache is a fire-and-forget helper that flushes the semantic
// cache for kbID. Any error is logged by the caller / repo layer — the
// document mutation MUST NOT fail because cache flushing failed.
func (s *DocumentService) invalidateKBCache(orgID, kbID string) {
	if s.cacheInvalidator == nil || orgID == "" || kbID == "" {
		return
	}
	go func() {
		// Use a detached context with a conservative deadline; the issue's
		// acceptance criterion is < 500 ms for this flush.
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		if _, err := s.cacheInvalidator.InvalidateKB(ctx, orgID, kbID); err != nil {
			// Fire-and-forget: swallowing silently hides real failures from
			// operators. Log via slog.Default() so the invalidation can be
			// observed without failing the parent mutation.
			slog.Default().Warn("semantic_cache_invalidate_failed",
				"org_id", orgID,
				"kb_id", kbID,
				"error", err.Error(),
			)
		}
	}()
}

// GetByID retrieves a document by ID within an org.
func (s *DocumentService) GetByID(ctx context.Context, orgID, docID string) (*model.Document, error) {
	var doc *model.Document
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		doc, err = s.repo.GetByID(ctx, tx, orgID, docID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("document not found")
		}
		return nil, apierror.NewInternal("failed to fetch document: " + err.Error())
	}
	return doc, nil
}

// List returns paginated documents for a knowledge base.
func (s *DocumentService) List(ctx context.Context, orgID, kbID string, page, pageSize int) (*model.DocumentListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var docs []model.Document
	var total int
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		docs, total, err = s.repo.List(ctx, tx, orgID, kbID, page, pageSize)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list documents: " + err.Error())
	}
	docs = lo.Ternary(docs == nil, []model.Document{}, docs)
	return &model.DocumentListResponse{
		Documents: docs,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

// Update applies partial updates to a document's metadata fields.
func (s *DocumentService) Update(ctx context.Context, orgID, docID string, req model.UpdateDocumentRequest) (*model.Document, error) {
	var doc *model.Document
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		doc, err = s.repo.Update(ctx, tx, orgID, docID, req.Title, req.Metadata)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("document not found")
		}
		return nil, apierror.NewInternal("failed to update document: " + err.Error())
	}
	if doc != nil {
		s.invalidateKBCache(orgID, doc.KnowledgeBaseID)
	}
	return doc, nil
}

// Delete removes a document by ID within an org. On success we fire-and-
// forget a semantic-cache invalidation for the owning KB (#256) so that
// stale answers are not served for content that no longer exists.
func (s *DocumentService) Delete(ctx context.Context, orgID, docID string) error {
	var kbID string
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Look up the owning KB before the row disappears so invalidation
		// can target it. A failure here is non-fatal for the delete itself,
		// but we log it so stale cache entries are visible to ops rather
		// than silently orphaned.
		if doc, lookupErr := s.repo.GetByID(ctx, tx, orgID, docID); lookupErr != nil {
			slog.Default().Warn(
				"document_delete_kb_lookup_failed",
				slog.String("org_id", orgID),
				slog.String("doc_id", docID),
				slog.Any("error", lookupErr),
			)
		} else if doc != nil {
			kbID = doc.KnowledgeBaseID
		}
		return s.repo.Delete(ctx, tx, orgID, docID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("document not found")
		}
		return apierror.NewInternal("failed to delete document: " + err.Error())
	}
	s.invalidateKBCache(orgID, kbID)
	return nil
}

// UpdateStatus transitions a document's processing status with validation.
func (s *DocumentService) UpdateStatus(ctx context.Context, orgID, docID string, newStatus model.ProcessingStatus, errorMsg string) error {
	// First, get the current document to check its status.
	var current *model.Document
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		current, err = s.repo.GetByID(ctx, tx, orgID, docID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return apierror.NewNotFound("document not found")
		}
		return apierror.NewInternal("failed to fetch document for status update: " + err.Error())
	}

	// Validate the transition.
	allowed, ok := validStatusTransitions[current.ProcessingStatus]
	if !ok {
		return apierror.NewBadRequest("no transitions allowed from status: " + string(current.ProcessingStatus))
	}
	if !lo.Contains(allowed, newStatus) {
		return apierror.NewBadRequest(
			"invalid status transition from " + string(current.ProcessingStatus) + " to " + string(newStatus),
		)
	}

	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.UpdateStatus(ctx, tx, orgID, docID, newStatus, errorMsg)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("document not found")
		}
		return apierror.NewInternal("failed to update document status: " + err.Error())
	}
	return nil
}
