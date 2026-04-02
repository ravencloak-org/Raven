package model

import "time"

// ConnectorStatus represents the lifecycle state of an Airbyte connector.
type ConnectorStatus string

// Valid connector status values.
const (
	ConnectorStatusActive  ConnectorStatus = "active"
	ConnectorStatusPaused  ConnectorStatus = "paused"
	ConnectorStatusError   ConnectorStatus = "error"
	ConnectorStatusDeleted ConnectorStatus = "deleted"
)

// ValidConnectorStatuses is the set of valid connector status enum values.
var ValidConnectorStatuses = map[ConnectorStatus]bool{
	ConnectorStatusActive:  true,
	ConnectorStatusPaused:  true,
	ConnectorStatusError:   true,
	ConnectorStatusDeleted: true,
}

// SyncMode represents how an Airbyte connector synchronises data.
type SyncMode string

// Valid sync mode values.
const (
	SyncModeFullRefresh SyncMode = "full_refresh"
	SyncModeIncremental SyncMode = "incremental"
	SyncModeCDC         SyncMode = "cdc"
)

// ValidSyncModes is the set of valid sync mode enum values.
var ValidSyncModes = map[SyncMode]bool{
	SyncModeFullRefresh: true,
	SyncModeIncremental: true,
	SyncModeCDC:         true,
}

// AirbyteConnector represents an Airbyte connector configuration row.
type AirbyteConnector struct {
	ID              string          `json:"id"`
	OrgID           string          `json:"org_id"`
	KnowledgeBaseID string          `json:"knowledge_base_id"`
	Name            string          `json:"name"`
	ConnectorType   string          `json:"connector_type"`
	Config          map[string]any  `json:"config"`           // TODO(#111): encrypt like llm_provider_configs in a follow-up
	SyncMode        SyncMode        `json:"sync_mode"`
	ScheduleCron    *string         `json:"schedule_cron,omitempty"`
	Status          ConnectorStatus `json:"status"`
	LastSyncAt      *time.Time      `json:"last_sync_at,omitempty"`
	LastSyncStatus  *string         `json:"last_sync_status,omitempty"`
	LastSyncRecords int             `json:"last_sync_records"`
	CreatedBy       *string         `json:"created_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ConnectorResponse is the API response DTO for an Airbyte connector.
// The config field is omitted to avoid leaking sensitive credentials.
type ConnectorResponse struct {
	ID              string          `json:"id"`
	OrgID           string          `json:"org_id"`
	KnowledgeBaseID string          `json:"knowledge_base_id"`
	Name            string          `json:"name"`
	ConnectorType   string          `json:"connector_type"`
	SyncMode        SyncMode        `json:"sync_mode"`
	ScheduleCron    *string         `json:"schedule_cron,omitempty"`
	Status          ConnectorStatus `json:"status"`
	LastSyncAt      *time.Time      `json:"last_sync_at,omitempty"`
	LastSyncStatus  *string         `json:"last_sync_status,omitempty"`
	LastSyncRecords int             `json:"last_sync_records"`
	CreatedBy       *string         `json:"created_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ToResponse converts an AirbyteConnector (internal) to a ConnectorResponse (API-safe).
func (c *AirbyteConnector) ToResponse() *ConnectorResponse {
	return &ConnectorResponse{
		ID:              c.ID,
		OrgID:           c.OrgID,
		KnowledgeBaseID: c.KnowledgeBaseID,
		Name:            c.Name,
		ConnectorType:   c.ConnectorType,
		SyncMode:        c.SyncMode,
		ScheduleCron:    c.ScheduleCron,
		Status:          c.Status,
		LastSyncAt:      c.LastSyncAt,
		LastSyncStatus:  c.LastSyncStatus,
		LastSyncRecords: c.LastSyncRecords,
		CreatedBy:       c.CreatedBy,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

// CreateConnectorRequest is the payload for POST .../connectors.
type CreateConnectorRequest struct {
	KnowledgeBaseID string         `json:"knowledge_base_id" binding:"required"`
	Name            string         `json:"name" binding:"required,min=1,max=255"`
	ConnectorType   string         `json:"connector_type" binding:"required,min=1,max=100"`
	Config          map[string]any `json:"config,omitempty"`
	SyncMode        SyncMode       `json:"sync_mode,omitempty"`
	ScheduleCron    *string        `json:"schedule_cron,omitempty"`
}

// UpdateConnectorRequest is the payload for PUT .../connectors/:connector_id.
type UpdateConnectorRequest struct {
	Name         *string          `json:"name,omitempty" binding:"omitempty,min=1,max=255"`
	Config       map[string]any   `json:"config,omitempty"`
	SyncMode     *SyncMode        `json:"sync_mode,omitempty"`
	ScheduleCron *string          `json:"schedule_cron,omitempty"`
	Status       *ConnectorStatus `json:"status,omitempty"`
}

// SyncHistory represents a single Airbyte sync run record.
type SyncHistory struct {
	ID            string     `json:"id"`
	ConnectorID   string     `json:"connector_id"`
	OrgID         string     `json:"org_id"`
	Status        string     `json:"status"`
	RecordsSynced int        `json:"records_synced"`
	RecordsFailed int        `json:"records_failed"`
	BytesSynced   int64      `json:"bytes_synced"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
}

// SyncHistoryResponse is the API response DTO for sync history.
type SyncHistoryResponse struct {
	ID            string     `json:"id"`
	ConnectorID   string     `json:"connector_id"`
	Status        string     `json:"status"`
	RecordsSynced int        `json:"records_synced"`
	RecordsFailed int        `json:"records_failed"`
	BytesSynced   int64      `json:"bytes_synced"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
}

// ToResponse converts a SyncHistory (internal) to a SyncHistoryResponse (API-safe).
func (h *SyncHistory) ToResponse() *SyncHistoryResponse {
	return &SyncHistoryResponse{
		ID:            h.ID,
		ConnectorID:   h.ConnectorID,
		Status:        h.Status,
		RecordsSynced: h.RecordsSynced,
		RecordsFailed: h.RecordsFailed,
		BytesSynced:   h.BytesSynced,
		StartedAt:     h.StartedAt,
		CompletedAt:   h.CompletedAt,
		ErrorMessage:  h.ErrorMessage,
	}
}

// ConnectorListResponse wraps a paginated list of connectors.
type ConnectorListResponse struct {
	Data     []ConnectorResponse `json:"data"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}
