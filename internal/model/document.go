package model

import "time"

// ProcessingStatus represents the processing lifecycle of a document.
type ProcessingStatus string

// Valid processing statuses for a document.
const (
	ProcessingStatusQueued       ProcessingStatus = "queued"
	ProcessingStatusCrawling     ProcessingStatus = "crawling"
	ProcessingStatusParsing      ProcessingStatus = "parsing"
	ProcessingStatusChunking     ProcessingStatus = "chunking"
	ProcessingStatusEmbedding    ProcessingStatus = "embedding"
	ProcessingStatusReady        ProcessingStatus = "ready"
	ProcessingStatusFailed       ProcessingStatus = "failed"
	ProcessingStatusReprocessing ProcessingStatus = "reprocessing"
)

// Document represents a file uploaded to a knowledge base.
type Document struct {
	ID               string           `json:"id"`
	OrgID            string           `json:"org_id"`
	KnowledgeBaseID  string           `json:"knowledge_base_id"`
	FileName         string           `json:"file_name"`
	FileType         string           `json:"file_type,omitempty"`
	FileSizeBytes    *int64           `json:"file_size_bytes,omitempty"`
	FileHash         string           `json:"file_hash,omitempty"`
	StoragePath      string           `json:"storage_path,omitempty"`
	ProcessingStatus ProcessingStatus `json:"processing_status"`
	ProcessingError  string           `json:"processing_error,omitempty"`
	Title            string           `json:"title,omitempty"`
	PageCount        *int             `json:"page_count,omitempty"`
	Metadata         map[string]any   `json:"metadata"`
	UploadedBy       string           `json:"uploaded_by,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// UpdateDocumentRequest is the payload for PUT .../documents/:doc_id.
type UpdateDocumentRequest struct {
	Title    *string        `json:"title,omitempty" binding:"omitempty,max=500"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// UploadDocumentResponse is returned after a successful document upload.
type UploadDocumentResponse struct {
	ID               string           `json:"id"`
	OrgID            string           `json:"org_id"`
	KnowledgeBaseID  string           `json:"knowledge_base_id"`
	FileName         string           `json:"file_name"`
	FileType         string           `json:"file_type"`
	FileSizeBytes    int64            `json:"file_size_bytes"`
	FileHash         string           `json:"file_hash"`
	StoragePath      string           `json:"storage_path"`
	ProcessingStatus ProcessingStatus `json:"processing_status"`
	UploadedBy       string           `json:"uploaded_by,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
}

// DocumentListResponse wraps a paginated list of documents.
type DocumentListResponse struct {
	Documents []Document `json:"documents"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}
