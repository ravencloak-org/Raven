package model

import "time"

// SourceType represents the kind of web source being crawled.
type SourceType string

// SourceTypeWebPage, SourceTypeWebSite, SourceTypeSitemap, and SourceTypeRSSFeed
// are the valid source types.
const (
	SourceTypeWebPage SourceType = "web_page"
	SourceTypeWebSite SourceType = "web_site"
	SourceTypeSitemap SourceType = "sitemap"
	SourceTypeRSSFeed SourceType = "rss_feed"
)

// CrawlFrequency determines how often a source is re-crawled.
type CrawlFrequency string

// CrawlFrequencyManual, CrawlFrequencyDaily, CrawlFrequencyWeekly, and CrawlFrequencyMonthly
// are the valid crawl frequency values.
const (
	CrawlFrequencyManual  CrawlFrequency = "manual"
	CrawlFrequencyDaily   CrawlFrequency = "daily"
	CrawlFrequencyWeekly  CrawlFrequency = "weekly"
	CrawlFrequencyMonthly CrawlFrequency = "monthly"
)

// ProcessingStatus tracks the lifecycle of a source crawl job.
type ProcessingStatus string

// ProcessingStatusQueued, ProcessingStatusProcessing, ProcessingStatusCompleted,
// and ProcessingStatusFailed are the valid processing status values.
const (
	ProcessingStatusQueued     ProcessingStatus = "queued"
	ProcessingStatusProcessing ProcessingStatus = "processing"
	ProcessingStatusCompleted  ProcessingStatus = "completed"
	ProcessingStatusFailed     ProcessingStatus = "failed"
)

// Source represents a web source attached to a knowledge base.
type Source struct {
	ID               string           `json:"id"`
	OrgID            string           `json:"org_id"`
	KnowledgeBaseID  string           `json:"knowledge_base_id"`
	SourceType       SourceType       `json:"source_type"`
	URL              string           `json:"url"`
	CrawlDepth       int              `json:"crawl_depth"`
	CrawlFrequency   CrawlFrequency   `json:"crawl_frequency"`
	ProcessingStatus ProcessingStatus `json:"processing_status"`
	ProcessingError  string           `json:"processing_error,omitempty"`
	Title            string           `json:"title,omitempty"`
	PagesCrawled     int              `json:"pages_crawled"`
	Metadata         map[string]any   `json:"metadata"`
	CreatedBy        string           `json:"created_by,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// CreateSourceRequest is the payload for POST .../sources.
type CreateSourceRequest struct {
	SourceType     SourceType     `json:"source_type" binding:"required"`
	URL            string         `json:"url" binding:"required"`
	CrawlDepth     *int           `json:"crawl_depth,omitempty"`
	CrawlFrequency CrawlFrequency `json:"crawl_frequency,omitempty"`
	Title          string         `json:"title,omitempty" binding:"omitempty,max=500"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// UpdateSourceRequest is the payload for PUT .../sources/:source_id.
type UpdateSourceRequest struct {
	URL            *string         `json:"url,omitempty"`
	CrawlDepth     *int            `json:"crawl_depth,omitempty"`
	CrawlFrequency *CrawlFrequency `json:"crawl_frequency,omitempty"`
	Title          *string         `json:"title,omitempty" binding:"omitempty,max=500"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
}

// SourceListResponse wraps a paginated list of sources.
type SourceListResponse struct {
	Data     []Source `json:"data"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}
