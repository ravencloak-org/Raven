package model

import "time"

// ProcessingEvent records a state transition in a document's processing lifecycle.
type ProcessingEvent struct {
	ID           string           `json:"id"`
	OrgID        string           `json:"org_id"`
	DocumentID   *string          `json:"document_id,omitempty"`
	SourceID     *string          `json:"source_id,omitempty"`
	FromStatus   *ProcessingStatus `json:"from_status,omitempty"`
	ToStatus     ProcessingStatus `json:"to_status"`
	ErrorMessage string           `json:"error_message,omitempty"`
	DurationMs   *int             `json:"duration_ms,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
}

// ProcessingEventListResponse wraps a list of processing events for API responses.
type ProcessingEventListResponse struct {
	Events []ProcessingEvent `json:"events"`
	Total  int               `json:"total"`
}
