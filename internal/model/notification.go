package model

import "time"

// NotificationType enumerates the supported notification types.
type NotificationType string

// Supported notification types.
const (
	NotificationTypeConversationSummary NotificationType = "conversation_summary"
	NotificationTypeAdminDigest         NotificationType = "admin_digest"
	NotificationTypeCustom              NotificationType = "custom"
)

// NotificationStatus enumerates the delivery status of a notification.
type NotificationStatus string

// Supported notification statuses.
const (
	NotificationStatusPending NotificationStatus = "pending"
	NotificationStatusSent    NotificationStatus = "sent"
	NotificationStatusFailed  NotificationStatus = "failed"
)

// NotificationConfig represents a notification configuration for an org.
type NotificationConfig struct {
	ID               string           `json:"id"`
	OrgID            string           `json:"org_id"`
	NotificationType NotificationType `json:"notification_type"`
	Recipients       []string         `json:"recipients"`
	Enabled          bool             `json:"enabled"`
	Config           map[string]any   `json:"config"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// NotificationLog represents a single notification delivery record.
type NotificationLog struct {
	ID               string             `json:"id"`
	OrgID            string             `json:"org_id"`
	ConfigID         *string            `json:"config_id,omitempty"`
	NotificationType NotificationType   `json:"notification_type"`
	Recipient        string             `json:"recipient"`
	Subject          string             `json:"subject"`
	Status           NotificationStatus `json:"status"`
	ErrorMessage     *string            `json:"error_message,omitempty"`
	SentAt           *time.Time         `json:"sent_at,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
}

// NotificationLogEntry is an alias for NotificationLog for backward compatibility.
type NotificationLogEntry = NotificationLog

// SendEmailPayload is the Asynq job payload for outbound email delivery.
type SendEmailPayload struct {
	OrgID            string           `json:"org_id"`
	ConfigID         string           `json:"config_id"`
	NotificationType NotificationType `json:"notification_type"`
	Recipients       []string         `json:"recipients"`
	Subject          string           `json:"subject"`
	Body             string           `json:"body"`
}

// CreateNotificationConfigRequest is the payload for creating a notification config.
type CreateNotificationConfigRequest struct {
	NotificationType NotificationType `json:"notification_type" binding:"required,oneof=conversation_summary admin_digest custom"`
	Recipients       []string         `json:"recipients" binding:"required,min=1,dive,email"`
	Enabled          *bool            `json:"enabled,omitempty"`
	Config           map[string]any   `json:"config,omitempty"`
}

// UpdateNotificationConfigRequest is the payload for updating a notification config.
type UpdateNotificationConfigRequest struct {
	Recipients *[]string      `json:"recipients,omitempty" binding:"omitempty,dive,email"`
	Enabled    *bool          `json:"enabled,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
}

// TestEmailRequest is the payload for sending a test email.
type TestEmailRequest struct {
	To      string `json:"to" binding:"required,email"`
	Subject string `json:"subject,omitempty"`
}

// NotificationLogResponse is a paginated list of notification log entries.
type NotificationLogResponse struct {
	Entries []NotificationLogEntry `json:"entries"`
	Total   int                    `json:"total"`
}
