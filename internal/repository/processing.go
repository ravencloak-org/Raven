package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/ravencloak-org/Raven/internal/model"
)

// ProcessingEventRepository handles database operations for processing events.
// All operations use a pgx.Tx with org_id set for RLS enforcement.
type ProcessingEventRepository struct{}

// NewProcessingEventRepository creates a new ProcessingEventRepository.
func NewProcessingEventRepository() *ProcessingEventRepository {
	return &ProcessingEventRepository{}
}

const eventColumns = `id, org_id, document_id, source_id,
	from_status, to_status,
	COALESCE(error_message, '') AS error_message,
	duration_ms, created_at`

func scanEvent(row pgx.Row) (*model.ProcessingEvent, error) {
	var evt model.ProcessingEvent
	err := row.Scan(
		&evt.ID,
		&evt.OrgID,
		&evt.DocumentID,
		&evt.SourceID,
		&evt.FromStatus,
		&evt.ToStatus,
		&evt.ErrorMessage,
		&evt.DurationMs,
		&evt.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &evt, nil
}

// Create inserts a new processing event record.
func (r *ProcessingEventRepository) Create(ctx context.Context, tx pgx.Tx, evt *model.ProcessingEvent) (*model.ProcessingEvent, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO processing_events (org_id, document_id, source_id, from_status, to_status, error_message, duration_ms)
		 VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
		 RETURNING `+eventColumns,
		evt.OrgID, evt.DocumentID, evt.SourceID, evt.FromStatus, evt.ToStatus, evt.ErrorMessage, evt.DurationMs,
	)
	created, err := scanEvent(row)
	if err != nil {
		return nil, fmt.Errorf("ProcessingEventRepository.Create: %w", err)
	}
	return created, nil
}

// ListByDocumentID returns all processing events for a given document, ordered by creation time.
func (r *ProcessingEventRepository) ListByDocumentID(ctx context.Context, tx pgx.Tx, orgID, documentID string) ([]model.ProcessingEvent, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+eventColumns+`
		 FROM processing_events
		 WHERE org_id = $1 AND document_id = $2
		 ORDER BY created_at ASC`,
		orgID, documentID,
	)
	if err != nil {
		return nil, fmt.Errorf("ProcessingEventRepository.ListByDocumentID query: %w", err)
	}
	defer rows.Close()

	var events []model.ProcessingEvent
	for rows.Next() {
		var evt model.ProcessingEvent
		if err := rows.Scan(
			&evt.ID, &evt.OrgID, &evt.DocumentID, &evt.SourceID,
			&evt.FromStatus, &evt.ToStatus, &evt.ErrorMessage,
			&evt.DurationMs, &evt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ProcessingEventRepository.ListByDocumentID scan: %w", err)
		}
		events = append(events, evt)
	}
	return events, rows.Err()
}

// ListBySourceID returns all processing events for a given source, ordered by creation time.
func (r *ProcessingEventRepository) ListBySourceID(ctx context.Context, tx pgx.Tx, orgID, sourceID string) ([]model.ProcessingEvent, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+eventColumns+`
		 FROM processing_events
		 WHERE org_id = $1 AND source_id = $2
		 ORDER BY created_at ASC`,
		orgID, sourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("ProcessingEventRepository.ListBySourceID query: %w", err)
	}
	defer rows.Close()

	var events []model.ProcessingEvent
	for rows.Next() {
		var evt model.ProcessingEvent
		if err := rows.Scan(
			&evt.ID, &evt.OrgID, &evt.DocumentID, &evt.SourceID,
			&evt.FromStatus, &evt.ToStatus, &evt.ErrorMessage,
			&evt.DurationMs, &evt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ProcessingEventRepository.ListBySourceID scan: %w", err)
		}
		events = append(events, evt)
	}
	return events, rows.Err()
}
