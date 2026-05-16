// Package queue provides Asynq-based async job definitions and processing
// backed by Valkey (Redis-compatible). Job types cover document processing,
// URL scraping, and knowledge-base reindexing.
package queue

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/ravencloak-org/Raven/internal/model"
)

// Task type constants used for routing tasks to the correct handler.
const (
	TypeDocumentProcess = "document:process"
	TypeURLScrape       = "url:scrape"
	TypeReindex         = "kb:reindex"
	TypeAirbyteSync     = "airbyte:sync"
	// TypeSendEmail is the Asynq task type for outbound email delivery.
	TypeSendEmail       = "notification:send_email"
	TypeWebhookDelivery = "webhook:deliver"

	// TypeTrialExpiringSoon is enqueued when a trial is 2 days from expiry.
	TypeTrialExpiringSoon = "billing:trial_expiring_soon"
	// TypeDataDeletionWarning is enqueued at grace period end to warn before deletion.
	TypeDataDeletionWarning = "billing:data_deletion_warning"
	// TypeArchiveOrgData is enqueued the day after grace period ends.
	TypeArchiveOrgData = "billing:archive_org_data"
	// TypeDeleteOrgData is enqueued 30 days after archiving.
	TypeDeleteOrgData = "billing:delete_org_data"
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

// NewSendEmailTask creates a new Asynq task for outbound email delivery.
func NewSendEmailTask(p model.SendEmailPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal SendEmailPayload: %w", err)
	}
	return asynq.NewTask(TypeSendEmail, data), nil
}

// WebhookDeliveryPayload is the payload for webhook delivery tasks.
type WebhookDeliveryPayload struct {
	DeliveryID string         `json:"delivery_id"`
	WebhookID  string         `json:"webhook_id"`
	OrgID      string         `json:"org_id"`
	EventType  string         `json:"event_type"`
	Payload    map[string]any `json:"payload"`
}

// NewWebhookDeliveryTask creates a new Asynq task for webhook delivery.
// It uses DeliveryID as a unique task identifier to prevent duplicate deliveries.
func NewWebhookDeliveryTask(p WebhookDeliveryPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal WebhookDeliveryPayload: %w", err)
	}
	return asynq.NewTask(TypeWebhookDelivery, data, asynq.TaskID(p.DeliveryID)), nil
}

// TrialEmailPayload is the payload for trial lifecycle email tasks.
type TrialEmailPayload struct {
	OrgID          string `json:"org_id"`
	SubscriptionID string `json:"subscription_id"`
	// EmailType is one of: trial_expiring_soon, data_deletion_warning.
	EmailType string `json:"email_type"`
}

// OrgDataPayload is the payload for org-scoped data management tasks.
type OrgDataPayload struct {
	OrgID string `json:"org_id"`
}

// NewTrialEmailTask creates a new Asynq task for a trial lifecycle email notification.
func NewTrialEmailTask(p TrialEmailPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal TrialEmailPayload: %w", err)
	}
	taskType := TypeTrialExpiringSoon
	if p.EmailType == "data_deletion_warning" {
		taskType = TypeDataDeletionWarning
	}
	return asynq.NewTask(taskType, data), nil
}

// NewArchiveOrgDataTask creates a new Asynq task to archive org data.
func NewArchiveOrgDataTask(p OrgDataPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal OrgDataPayload for archive: %w", err)
	}
	return asynq.NewTask(TypeArchiveOrgData, data), nil
}

// NewDeleteOrgDataTask creates a new Asynq task to hard-delete org data.
func NewDeleteOrgDataTask(p OrgDataPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal OrgDataPayload for delete: %w", err)
	}
	return asynq.NewTask(TypeDeleteOrgData, data), nil
}
