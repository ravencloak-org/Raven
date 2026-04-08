package jobs

import (
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/queue"
)

// Cron expressions for scheduled jobs.
const (
	// CronRecrawl runs the source re-crawl check every 6 hours.
	CronRecrawl = "0 */6 * * *"

	// CronCleanup runs the session/event cleanup daily at 2 AM UTC.
	CronCleanup = "0 2 * * *"

	// CronUsageAggregation runs the usage aggregation every hour at minute 5.
	CronUsageAggregation = "5 * * * *"
)

// SchedulerConfig holds the dependencies needed to set up the cron scheduler.
type SchedulerConfig struct {
	RedisAddr   string
	Pool        *pgxpool.Pool
	QueueClient *queue.Client
	Logger      *slog.Logger
}

// Scheduler wraps an asynq.Scheduler and the handler mux for processing
// periodic tasks. It owns both the scheduler (which enqueues tasks on a cron
// schedule) and the handlers that process them.
type Scheduler struct {
	scheduler *asynq.Scheduler
	handlers  *asynq.ServeMux
	logger    *slog.Logger
}

// NewScheduler creates a new cron scheduler that registers all periodic tasks
// and returns both the scheduler and a ServeMux with the matching handlers.
func NewScheduler(cfg SchedulerConfig) (*Scheduler, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Create the asynq scheduler that enqueues tasks on cron schedules.
	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		&asynq.SchedulerOpts{
			LogLevel: asynq.WarnLevel,
		},
	)

	// Register periodic tasks.
	recrawlTask, err := NewRecrawlTask(RecrawlPayload{})
	if err != nil {
		return nil, fmt.Errorf("create recrawl task: %w", err)
	}
	if _, err := scheduler.Register(CronRecrawl, recrawlTask,
		asynq.Queue("default"),
		asynq.MaxRetry(3),
	); err != nil {
		return nil, fmt.Errorf("register recrawl cron: %w", err)
	}

	cleanupTask, err := NewCleanupTask(CleanupPayload{})
	if err != nil {
		return nil, fmt.Errorf("create cleanup task: %w", err)
	}
	if _, err := scheduler.Register(CronCleanup, cleanupTask,
		asynq.Queue("low"),
		asynq.MaxRetry(2),
	); err != nil {
		return nil, fmt.Errorf("register cleanup cron: %w", err)
	}

	usageTask, err := NewUsageAggregationTask(UsageAggregationPayload{})
	if err != nil {
		return nil, fmt.Errorf("create usage aggregation task: %w", err)
	}
	if _, err := scheduler.Register(CronUsageAggregation, usageTask,
		asynq.Queue("default"),
		asynq.MaxRetry(3),
	); err != nil {
		return nil, fmt.Errorf("register usage aggregation cron: %w", err)
	}

	voiceUsageTask, err := NewVoiceUsageTask(VoiceUsagePayload{})
	if err != nil {
		return nil, fmt.Errorf("create voice usage aggregation task: %w", err)
	}
	if _, err := scheduler.Register(CronVoiceUsageAggregation, voiceUsageTask,
		asynq.Queue("default"),
		asynq.MaxRetry(3),
	); err != nil {
		return nil, fmt.Errorf("register voice usage aggregation cron: %w", err)
	}

	// Build a ServeMux with handlers for each scheduled task type.
	mux := asynq.NewServeMux()

	recrawlHandler := NewRecrawlHandler(cfg.Pool, cfg.QueueClient, cfg.Logger)
	mux.Handle(TypeRecrawlSources, recrawlHandler)

	cleanupHandler := NewCleanupHandler(cfg.Pool, cfg.Logger)
	mux.Handle(TypeCleanupSessions, cleanupHandler)

	usageHandler := NewUsageAggregationHandler(cfg.Pool, cfg.Logger)
	mux.Handle(TypeUsageAggregation, usageHandler)

	voiceUsageHandler := NewVoiceUsageHandler(cfg.Pool, cfg.Logger)
	mux.Handle(TypeVoiceUsageAggregation, voiceUsageHandler)

	cfg.Logger.Info("scheduler configured",
		"recrawl_cron", CronRecrawl,
		"cleanup_cron", CronCleanup,
		"usage_aggregation_cron", CronUsageAggregation,
		"voice_usage_aggregation_cron", CronVoiceUsageAggregation,
	)

	return &Scheduler{
		scheduler: scheduler,
		handlers:  mux,
		logger:    cfg.Logger,
	}, nil
}

// Start begins the cron scheduler. This call blocks until the scheduler is
// shut down via Shutdown().
func (s *Scheduler) Start() error {
	s.logger.Info("starting cron scheduler")
	return s.scheduler.Start()
}

// Shutdown stops the cron scheduler.
func (s *Scheduler) Shutdown() {
	s.logger.Info("shutting down cron scheduler")
	s.scheduler.Shutdown()
}

// Handlers returns the ServeMux containing all scheduled job handlers.
// The caller should register this mux with an asynq.Server so that the
// enqueued periodic tasks are actually processed.
func (s *Scheduler) Handlers() *asynq.ServeMux {
	return s.handlers
}
