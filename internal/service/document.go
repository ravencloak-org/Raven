package service

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// validStatusTransitions maps current status to the set of allowed next statuses.
var validStatusTransitions = map[model.ProcessingStatus][]model.ProcessingStatus{
	model.ProcessingStatusQueued:     {model.ProcessingStatusProcessing},
	model.ProcessingStatusProcessing: {model.ProcessingStatusCompleted, model.ProcessingStatusFailed},
	model.ProcessingStatusFailed:     {model.ProcessingStatusQueued},
}

// DocumentService contains business logic for document management.
type DocumentService struct {
	repo *repository.DocumentRepository
	pool *pgxpool.Pool
}

// NewDocumentService creates a new DocumentService.
func NewDocumentService(repo *repository.DocumentRepository, pool *pgxpool.Pool) *DocumentService {
	return &DocumentService{repo: repo, pool: pool}
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
	if docs == nil {
		docs = []model.Document{}
	}
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
	return doc, nil
}

// Delete removes a document by ID within an org.
func (s *DocumentService) Delete(ctx context.Context, orgID, docID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, orgID, docID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("document not found")
		}
		return apierror.NewInternal("failed to delete document: " + err.Error())
	}
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
	valid := false
	for _, s := range allowed {
		if s == newStatus {
			valid = true
			break
		}
	}
	if !valid {
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
