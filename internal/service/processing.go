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

// ProcessingEventService contains business logic for processing event management.
type ProcessingEventService struct {
	repo    *repository.ProcessingEventRepository
	docRepo *repository.DocumentRepository
	pool    *pgxpool.Pool
}

// NewProcessingEventService creates a new ProcessingEventService.
func NewProcessingEventService(
	repo *repository.ProcessingEventRepository,
	docRepo *repository.DocumentRepository,
	pool *pgxpool.Pool,
) *ProcessingEventService {
	return &ProcessingEventService{repo: repo, docRepo: docRepo, pool: pool}
}

// Transition validates and records a processing status transition for a document.
// It updates the document's processing_status and inserts a processing_event record
// within a single transaction.
func (s *ProcessingEventService) Transition(
	ctx context.Context,
	orgID, docID string,
	toStatus model.ProcessingStatus,
	errorMsg string,
) (*model.ProcessingEvent, error) {
	var evt *model.ProcessingEvent

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Fetch the current document to get the current status.
		doc, err := s.docRepo.GetByID(ctx, tx, orgID, docID)
		if err != nil {
			return err
		}

		fromStatus := doc.ProcessingStatus

		// Validate the transition.
		allowed, ok := validStatusTransitions[fromStatus]
		if !ok {
			return apierror.NewBadRequest(
				"no transitions allowed from status: " + string(fromStatus),
			)
		}
		valid := false
		for _, s := range allowed {
			if s == toStatus {
				valid = true
				break
			}
		}
		if !valid {
			return apierror.NewBadRequest(
				"invalid status transition from " + string(fromStatus) + " to " + string(toStatus),
			)
		}

		// Update the document status.
		if err := s.docRepo.UpdateStatus(ctx, tx, orgID, docID, toStatus, errorMsg); err != nil {
			return err
		}

		// Record the processing event.
		evt, err = s.repo.Create(ctx, tx, &model.ProcessingEvent{
			OrgID:        orgID,
			DocumentID:   &docID,
			FromStatus:   &fromStatus,
			ToStatus:     toStatus,
			ErrorMessage: errorMsg,
		})
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("document not found")
		}
		// Pass through AppErrors as-is.
		if _, ok := err.(*apierror.AppError); ok {
			return nil, err
		}
		return nil, apierror.NewInternal("failed to transition document status: " + err.Error())
	}
	return evt, nil
}

// ListByDocumentID returns all processing events for a document.
func (s *ProcessingEventService) ListByDocumentID(ctx context.Context, orgID, docID string) (*model.ProcessingEventListResponse, error) {
	var events []model.ProcessingEvent

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Verify the document exists.
		_, err := s.docRepo.GetByID(ctx, tx, orgID, docID)
		if err != nil {
			return err
		}

		events, err = s.repo.ListByDocumentID(ctx, tx, orgID, docID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("document not found")
		}
		if _, ok := err.(*apierror.AppError); ok {
			return nil, err
		}
		return nil, apierror.NewInternal("failed to list processing events: " + err.Error())
	}
	if events == nil {
		events = []model.ProcessingEvent{}
	}
	return &model.ProcessingEventListResponse{
		Events: events,
		Total:  len(events),
	}, nil
}
