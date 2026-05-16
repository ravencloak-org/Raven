package queue

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hibiken/asynq"

	"github.com/ravencloak-org/Raven/internal/model"
)

// Client wraps asynq.Client with convenience methods for enqueueing tasks.
type Client struct {
	inner    *asynq.Client
	logger   *slog.Logger
	maxRetry int
}

// ClientOption configures the queue Client.
type ClientOption func(*Client)

// WithMaxRetry sets the maximum retry count for enqueued tasks.
func WithMaxRetry(n int) ClientOption {
	return func(c *Client) {
		c.maxRetry = n
	}
}

// WithLogger sets the logger for the queue client.
func WithLogger(l *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = l
	}
}

// NewClient creates a new queue Client connected to the given Valkey/Redis address.
// Accepts both "host:port" and "redis://host:port" formats.
func NewClient(redisAddr string, opts ...ClientOption) *Client {
	redisAddr = strings.TrimPrefix(redisAddr, "redis://")
	c := &Client{
		inner:    asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr}),
		logger:   slog.Default(),
		maxRetry: 5,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Close closes the underlying asynq client.
func (c *Client) Close() error {
	return c.inner.Close()
}

// EnqueueDocumentProcess enqueues a document processing task on the default queue.
func (c *Client) EnqueueDocumentProcess(ctx context.Context, p DocumentProcessPayload) error {
	task, err := NewDocumentProcessTask(p)
	if err != nil {
		return fmt.Errorf("create document process task: %w", err)
	}
	info, err := c.inner.EnqueueContext(ctx, task,
		asynq.MaxRetry(c.maxRetry),
		asynq.Queue("default"),
	)
	if err != nil {
		return fmt.Errorf("enqueue document process task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"document_id", p.DocumentID,
	)
	return nil
}

// EnqueueURLScrape enqueues a URL scraping task on the default queue.
func (c *Client) EnqueueURLScrape(ctx context.Context, p URLScrapePayload) error {
	task, err := NewURLScrapeTask(p)
	if err != nil {
		return fmt.Errorf("create url scrape task: %w", err)
	}
	info, err := c.inner.EnqueueContext(ctx, task,
		asynq.MaxRetry(c.maxRetry),
		asynq.Queue("default"),
	)
	if err != nil {
		return fmt.Errorf("enqueue url scrape task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"url", p.URL,
	)
	return nil
}

// EnqueueReindex enqueues a knowledge-base reindex task on the low-priority queue.
func (c *Client) EnqueueReindex(ctx context.Context, p ReindexPayload) error {
	task, err := NewReindexTask(p)
	if err != nil {
		return fmt.Errorf("create reindex task: %w", err)
	}
	info, err := c.inner.EnqueueContext(ctx, task,
		asynq.MaxRetry(c.maxRetry),
		asynq.Queue("low"),
	)
	if err != nil {
		return fmt.Errorf("enqueue reindex task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"knowledge_base_id", p.KnowledgeBaseID,
	)
	return nil
}

// EnqueueAirbyteSync enqueues an Airbyte connector sync task on the default queue.
func (c *Client) EnqueueAirbyteSync(ctx context.Context, p AirbyteSyncPayload) error {
	task, err := NewAirbyteSyncTask(p)
	if err != nil {
		return fmt.Errorf("create airbyte sync task: %w", err)
	}
	info, err := c.inner.EnqueueContext(ctx, task,
		asynq.MaxRetry(c.maxRetry),
		asynq.Queue("default"),
	)
	if err != nil {
		return fmt.Errorf("enqueue airbyte sync task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"connector_id", p.ConnectorID,
	)
	return nil
}

// EnqueueSendEmail enqueues an outbound email delivery task on the default queue.
// Optional asynq.Option values (e.g. asynq.TaskID for deduplication) may be passed as opts.
func (c *Client) EnqueueSendEmail(ctx context.Context, p model.SendEmailPayload, opts ...asynq.Option) error {
	task, err := NewSendEmailTask(p)
	if err != nil {
		return fmt.Errorf("create send-email task: %w", err)
	}
	baseOpts := []asynq.Option{asynq.MaxRetry(c.maxRetry), asynq.Queue("default")}
	info, err := c.inner.EnqueueContext(ctx, task, append(baseOpts, opts...)...)
	if err != nil {
		return fmt.Errorf("enqueue send-email task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"org_id", p.OrgID,
		"config_id", p.ConfigID,
	)
	return nil
}

// EnqueueWebhookDelivery enqueues a webhook delivery task on the default queue.
// Callers may pass additional asynq.Option values (e.g. asynq.ProcessIn, asynq.Unique)
// that are appended after the defaults.
func (c *Client) EnqueueWebhookDelivery(ctx context.Context, p WebhookDeliveryPayload, opts ...asynq.Option) error {
	task, err := NewWebhookDeliveryTask(p)
	if err != nil {
		return fmt.Errorf("create webhook delivery task: %w", err)
	}
	baseOpts := []asynq.Option{
		asynq.MaxRetry(0), // retries are managed by the job handler
		asynq.Queue("default"),
	}
	info, err := c.inner.EnqueueContext(ctx, task, append(baseOpts, opts...)...)
	if err != nil {
		return fmt.Errorf("enqueue webhook delivery task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"webhook_id", p.WebhookID,
		"delivery_id", p.DeliveryID,
		"event_type", p.EventType,
	)
	return nil
}

// EnqueueTrialEmail enqueues a trial lifecycle email notification on the default queue.
// emailType must be one of: "trial_expiring_soon", "data_deletion_warning".
func (c *Client) EnqueueTrialEmail(ctx context.Context, orgID, subscriptionID, emailType string) error {
	task, err := NewTrialEmailTask(TrialEmailPayload{
		OrgID:          orgID,
		SubscriptionID: subscriptionID,
		EmailType:      emailType,
	})
	if err != nil {
		return fmt.Errorf("create trial email task: %w", err)
	}
	info, err := c.inner.EnqueueContext(ctx, task,
		asynq.MaxRetry(c.maxRetry),
		asynq.Queue("default"),
	)
	if err != nil {
		return fmt.Errorf("enqueue trial email task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"org_id", orgID,
		"email_type", emailType,
	)
	return nil
}

// EnqueueArchiveOrgData enqueues an archive-org-data task on the low-priority queue.
func (c *Client) EnqueueArchiveOrgData(ctx context.Context, orgID string) error {
	task, err := NewArchiveOrgDataTask(OrgDataPayload{OrgID: orgID})
	if err != nil {
		return fmt.Errorf("create archive org data task: %w", err)
	}
	info, err := c.inner.EnqueueContext(ctx, task,
		asynq.MaxRetry(c.maxRetry),
		asynq.Queue("low"),
	)
	if err != nil {
		return fmt.Errorf("enqueue archive org data task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"org_id", orgID,
	)
	return nil
}

// EnqueueDeleteOrgData enqueues a hard-delete-org-data task on the low-priority queue.
func (c *Client) EnqueueDeleteOrgData(ctx context.Context, orgID string) error {
	task, err := NewDeleteOrgDataTask(OrgDataPayload{OrgID: orgID})
	if err != nil {
		return fmt.Errorf("create delete org data task: %w", err)
	}
	info, err := c.inner.EnqueueContext(ctx, task,
		asynq.MaxRetry(c.maxRetry),
		asynq.Queue("low"),
	)
	if err != nil {
		return fmt.Errorf("enqueue delete org data task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"org_id", orgID,
	)
	return nil
}
