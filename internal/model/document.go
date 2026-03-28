package model

import "time"

// ProcessingStatus represents the lifecycle state of a document's processing pipeline.
type ProcessingStatus string

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

// Document represents an uploaded document stored in SeaweedFS.
type Document struct {
	ID               string           `json:"id"`
	OrgID            string           `json:"org_id"`
	KnowledgeBaseID  string           `json:"knowledge_base_id"`
	FileName         string           `json:"file_name"`
	FileType         string           `json:"file_type,omitempty"`
	FileSizeBytes    int64            `json:"file_size_bytes,omitempty"`
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

// UploadDocumentResponse is the response returned after a successful document upload.
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
