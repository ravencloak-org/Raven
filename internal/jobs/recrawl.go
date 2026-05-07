package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
)

// RecrawlHandler handles the scheduled re-crawling of web sources based on
// their configured crawl_frequency (daily, weekly, monthly). Sources set to
// "manual" are skipped.
type RecrawlHandler struct {
	pool        *pgxpool.Pool
	queueClient *queue.Client
	logger      *slog.Logger
}

// NewRecrawlHandler creates a RecrawlHandler.
func NewRecrawlHandler(pool *pgxpool.Pool, queueClient *queue.Client, logger *slog.Logger) *RecrawlHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecrawlHandler{
		pool:        pool,
		queueClient: queueClient,
		logger:      logger,
	}
}

// ProcessTask implements asynq.Handler for the re-crawl scheduled job.
func (h *RecrawlHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var payload RecrawlPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal RecrawlPayload: %w", err)
	}

	h.logger.Info("starting scheduled source re-crawl",
		"org_id", payload.OrgID,
	)

	sources, err := h.findDueSources(ctx, payload.OrgID)
	if err != nil {
		return fmt.Errorf("find due sources: %w", err)
	}

	h.logger.Info("found sources due for re-crawl", "count", len(sources))

	var enqueueErrors int
	for _, src := range sources {
		if err := h.queueClient.EnqueueURLScrape(ctx, queue.URLScrapePayload{
			OrgID:           src.OrgID,
			SourceID:        src.ID,
			KnowledgeBaseID: src.KnowledgeBaseID,
			URL:             src.URL,
			CrawlDepth:      src.CrawlDepth,
		}); err != nil {
			h.logger.Error("failed to enqueue re-crawl for source",
				"source_id", src.ID,
				"org_id", src.OrgID,
				"error", err,
			)
			enqueueErrors++
			continue
		}

		h.logger.Info("enqueued re-crawl for source",
			"source_id", src.ID,
			"org_id", src.OrgID,
			"url", src.URL,
		)
	}

	if enqueueErrors > 0 {
		// Log failures but return nil so asynq does not retry the entire task.
		// Retrying would re-enqueue sources that already succeeded, causing
		// duplicate processing. Failed sources will be picked up on the next
		// scheduled run.
		h.logger.Warn("some sources failed to enqueue; they will be retried on the next scheduled run",
			"failed", enqueueErrors,
			"total", len(sources),
		)
	}
	return nil
}

// findDueSources queries the database for sources whose crawl_frequency
// indicates they are due for a refresh based on their updated_at timestamp.
func (h *RecrawlHandler) findDueSources(ctx context.Context, orgID string) ([]model.Source, error) {
	// Build the query. Sources with manual frequency are always excluded.
	// A source is "due" when its updated_at is older than the interval implied
	// by its crawl_frequency.
	q := `
		SELECT id, org_id, knowledge_base_id, source_type, url,
		       crawl_depth, crawl_frequency, processing_status,
		       COALESCE(processing_error, '') AS processing_error,
		       COALESCE(title, '') AS title,
		       pages_crawled, COALESCE(metadata, '{}') AS metadata,
		       COALESCE(created_by::text, '') AS created_by,
		       created_at, updated_at
		FROM sources
		WHERE crawl_frequency != 'manual'
		  AND processing_status NOT IN ('crawling', 'queued')
		  AND (
		    (crawl_frequency = 'daily'   AND updated_at < NOW() - INTERVAL '1 day')
		    OR (crawl_frequency = 'weekly'  AND updated_at < NOW() - INTERVAL '7 days')
		    OR (crawl_frequency = 'monthly' AND updated_at < NOW() - INTERVAL '30 days')
		  )`

	args := []any{}
	if orgID != "" {
		q += ` AND org_id = $1`
		args = append(args, orgID)
	}

	q += ` ORDER BY updated_at ASC`

	rows, err := h.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query due sources: %w", err)
	}
	defer rows.Close()

	var sources []model.Source
	for rows.Next() {
		var s model.Source
		if err := rows.Scan(
			&s.ID, &s.OrgID, &s.KnowledgeBaseID, &s.SourceType, &s.URL,
			&s.CrawlDepth, &s.CrawlFrequency, &s.ProcessingStatus,
			&s.ProcessingError, &s.Title, &s.PagesCrawled, &s.Metadata,
			&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan source row: %w", err)
		}
		sources = append(sources, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source rows: %w", err)
	}

	return sources, nil
}

// frequencyToDuration maps a CrawlFrequency to the minimum age a source's
// updated_at must have before it is considered due for re-crawl. Exported for
// testing convenience.
func frequencyToDuration(freq model.CrawlFrequency) time.Duration {
	switch freq {
	case model.CrawlFrequencyDaily:
		return 24 * time.Hour
	case model.CrawlFrequencyWeekly:
		return 7 * 24 * time.Hour
	case model.CrawlFrequencyMonthly:
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}
