// Package queue provides Asynq-based async job definitions and processing
// backed by Valkey (Redis-compatible). Job types cover document processing,
// URL scraping, and knowledge-base reindexing.
package queue

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// Task type constants used for routing tasks to the correct handler.
const (
	TypeDocumentProcess = "document:process"
	TypeURLScrape       = "url:scrape"
	TypeReindex         = "kb:reindex"
	TypeAirbyteSync     = "airbyte:sync"
)

// DocumentProcessPayload is the payload for document processing tasks.
type DocumentProcessPayload struct {
	OrgID           string `json:"org_id"`
	DocumentID      string `json:"document_id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
}

// URLScrapePayload is the payload for URL scraping tasks.
type URLScrapePayload struct {
	OrgID           string `json:"org_id"`
	SourceID        string `json:"source_id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	URL             string `json:"url"`
	CrawlDepth      int    `json:"crawl_depth"`
}

// ReindexPayload is the payload for knowledge-base reindex tasks.
type ReindexPayload struct {
	OrgID           string `json:"org_id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
}

// NewDocumentProcessTask creates a new Asynq task for document processing.
func NewDocumentProcessTask(p DocumentProcessPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal DocumentProcessPayload: %w", err)
	}
	return asynq.NewTask(TypeDocumentProcess, data), nil
}

// NewURLScrapeTask creates a new Asynq task for URL scraping.
func NewURLScrapeTask(p URLScrapePayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal URLScrapePayload: %w", err)
	}
	return asynq.NewTask(TypeURLScrape, data), nil
}

// NewReindexTask creates a new Asynq task for knowledge-base reindexing.
func NewReindexTask(p ReindexPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal ReindexPayload: %w", err)
	}
	return asynq.NewTask(TypeReindex, data), nil
}

// AirbyteSyncPayload is the payload for Airbyte connector sync tasks.
type AirbyteSyncPayload struct {
	ConnectorID     string `json:"connector_id"`
	OrgID           string `json:"org_id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
}

// NewAirbyteSyncTask creates a new Asynq task for an Airbyte connector sync.
func NewAirbyteSyncTask(p AirbyteSyncPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal AirbyteSyncPayload: %w", err)
	}
	return asynq.NewTask(TypeAirbyteSync, data), nil
}
