package model

import "time"

// WebhookStatus enumerates webhook endpoint lifecycle states.
type WebhookStatus string

// Supported webhook statuses.
const (
	WebhookStatusActive WebhookStatus = "active"
	WebhookStatusPaused WebhookStatus = "paused"
	WebhookStatusFailed WebhookStatus = "failed"
)

// WebhookEventType enumerates events that trigger webhook delivery.
type WebhookEventType string

// Supported webhook event types.
const (
	WebhookEventLeadGenerated         WebhookEventType = "lead.generated"
	WebhookEventConversationEscalated WebhookEventType = "conversation.escalated"
	WebhookEventDocumentProcessed     WebhookEventType = "document.processed"
	WebhookEventSyncCompleted         WebhookEventType = "sync.completed"
)

// WebhookConfig is a registered webhook endpoint for an org.
type WebhookConfig struct {
	ID              string            `json:"id"`
	OrgID           string            `json:"org_id"`
	Name            string            `json:"name"`
	URL             string            `json:"url"`
	Secret          string            `json:"-"`
	Events          []string          `json:"events"`
	Headers         map[string]string `json:"headers,omitempty"`
	Status          WebhookStatus     `json:"status"`
	MaxRetries      int               `json:"max_retries"`
	LastTriggeredAt *time.Time        `json:"last_triggered_at,omitempty"`
	FailureCount    int               `json:"failure_count"`
	CreatedBy       string            `json:"created_by,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// WebhookDelivery records a single delivery attempt for a webhook event.
type WebhookDelivery struct {
	ID             string         `json:"id"`
	WebhookID      string         `json:"webhook_id"`
	OrgID          string         `json:"org_id"`
	EventType      string         `json:"event_type"`
	Payload        map[string]any `json:"payload"`
	ResponseStatus *int           `json:"response_status,omitempty"`
	ResponseBody   string         `json:"response_body,omitempty"`
	AttemptCount   int            `json:"attempt_count"`
	Success        bool           `json:"success"`
	DeliveredAt    *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// CreateWebhookRequest is the payload for registering a new webhook.
type CreateWebhookRequest struct {
	Name       string            `json:"name" binding:"required,min=2,max=255"`
	URL        string            `json:"url" binding:"required,url"`
	Secret     string            `json:"secret" binding:"required,min=8"`
	Events     []string          `json:"events" binding:"required,min=1"`
	Headers    map[string]string `json:"headers,omitempty"`
	MaxRetries *int              `json:"max_retries,omitempty"`
}

// UpdateWebhookRequest is the payload for updating an existing webhook.
type UpdateWebhookRequest struct {
	Name       *string           `json:"name,omitempty"`
	URL        *string           `json:"url,omitempty"`
	Secret     *string           `json:"secret,omitempty"`
	Events     []string          `json:"events,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Status     *WebhookStatus    `json:"status,omitempty"`
	MaxRetries *int              `json:"max_retries,omitempty"`
}

// DispatchWebhookPayload is the Asynq job payload for async webhook delivery.
type DispatchWebhookPayload struct {
	WebhookID string         `json:"webhook_id"`
	OrgID     string         `json:"org_id"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
}
