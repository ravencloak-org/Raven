// Package jobs defines scheduled (cron) task types and payloads for the Asynq
// periodic task scheduler. These complement the on-demand tasks in internal/queue
// with time-based recurring jobs: source re-crawling, session/event cleanup,
// and usage aggregation for billing.
package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// Task type constants for scheduled (cron) jobs.
const (
	// TypeRecrawlSources triggers re-crawling of web sources whose crawl_frequency
	// indicates they are due for a refresh.
	TypeRecrawlSources = "scheduled:recrawl_sources"

	// TypeCleanupSessions removes expired sessions and stale processing events
	// that are older than the configured retention period.
	TypeCleanupSessions = "scheduled:cleanup_sessions"

	// TypeUsageAggregation rolls up API usage metrics per org/workspace for billing.
	TypeUsageAggregation = "scheduled:usage_aggregation"
)

// RecrawlPayload is the payload for the source re-crawl scheduled task.
// An empty payload means "check all sources"; a non-empty OrgID scopes the run.
type RecrawlPayload struct {
	// OrgID optionally restricts the re-crawl to a single organisation.
	// When empty, all orgs are checked.
	OrgID string `json:"org_id,omitempty"`
}

// CleanupPayload is the payload for the session/event cleanup scheduled task.
type CleanupPayload struct {
	// SessionMaxAgeDays is the number of days after which idle sessions are purged.
	// Defaults to 30 if zero.
	SessionMaxAgeDays int `json:"session_max_age_days,omitempty"`

	// EventRetentionDays is the number of days to keep processing events.
	// Defaults to 90 if zero.
	EventRetentionDays int `json:"event_retention_days,omitempty"`
}

// UsageAggregationPayload is the payload for the usage aggregation scheduled task.
type UsageAggregationPayload struct {
	// OrgID optionally restricts aggregation to a single organisation.
	OrgID string `json:"org_id,omitempty"`

	// WindowMinutes is the look-back window in minutes for aggregation.
	// Defaults to 60 (one hour) if zero.
	WindowMinutes int `json:"window_minutes,omitempty"`
}

// NewRecrawlTask creates a new Asynq task for the source re-crawl job.
func NewRecrawlTask(p RecrawlPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal RecrawlPayload: %w", err)
	}
	return asynq.NewTask(TypeRecrawlSources, data), nil
}

// NewCleanupTask creates a new Asynq task for the cleanup job.
func NewCleanupTask(p CleanupPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal CleanupPayload: %w", err)
	}
	return asynq.NewTask(TypeCleanupSessions, data), nil
}

// NewUsageAggregationTask creates a new Asynq task for usage aggregation.
func NewUsageAggregationTask(p UsageAggregationPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal UsageAggregationPayload: %w", err)
	}
	return asynq.NewTask(TypeUsageAggregation, data), nil
}
