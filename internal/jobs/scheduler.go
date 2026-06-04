package jobs

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/queue"
)

// Cron expressions for scheduled jobs.
const (
	// CronRecrawl runs the source re-crawl check every 6 hours.
	CronRecrawl = "0 */6 * * *"

	// CronCleanup runs the session/event cleanup daily at 2 AM UTC.
	CronCleanup = "0 2 * * *"

	// CronTrialLifecycle runs the trial/grace/archive pipeline daily at 3 AM UTC.
	CronTrialLifecycle = "0 3 * * *"

	// CronUsageAggregation runs the usage aggregation every hour at minute 5.
	CronUsageAggregation = "5 * * * *"

	// CronMarketplaceDMCASweep runs the DMCA counter-notice-window sweep
	// daily at 4 AM UTC. Offset from cleanup (2 AM) and trial-lifecycle
	// (3 AM) so the daily-cluster doesn't all hit the DB pool at once.
	CronMarketplaceDMCASweep = "0 4 * * *"

	// CronMarketplaceSlugHoldSweep runs the Marketplace slug-hold sweep
	// daily at 5 AM UTC — staggered an hour after the DMCA sweep so the
	// two daily-marketplace crons don't contend for the same DB window
	// (issue #727).
	CronMarketplaceSlugHoldSweep = "0 5 * * *"
)

// SchedulerConfig holds the dependencies needed to set up the cron scheduler.
type SchedulerConfig struct {
	RedisAddr   string
	Pool        *pgxpool.Pool
	QueueClient *queue.Client
	Logger      *slog.Logger
	Asynq       config.AsynqConfig

	// DMCAService drives the daily DMCA counter-notice-window sweep
	// (issue #736). Optional — when nil the DMCA cron is not registered
	// so the worker can boot in environments where the Marketplace is
	// disabled (e.g. self-hosted edge nodes). cmd/api/main.go wires
	// this; integration tests that don't exercise the sweep can leave
	// it nil.
	DMCAService *marketplace.DMCAService
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

	trialLifecycleTask, err := NewTrialLifecycleTask(TrialLifecyclePayload{})
	if err != nil {
		return nil, fmt.Errorf("create trial lifecycle task: %w", err)
	}
	if _, err := scheduler.Register(CronTrialLifecycle, trialLifecycleTask,
		asynq.Queue("default"),
		asynq.MaxRetry(3),
	); err != nil {
		return nil, fmt.Errorf("register trial lifecycle cron: %w", err)
	}

	// Marketplace DMCA sweep — only registered when the DMCAService is
	// wired (skipped on edge nodes that disable the marketplace).
	if cfg.DMCAService != nil {
		dmcaTask, err := NewMarketplaceDMCASweepTask(MarketplaceDMCASweepPayload{})
		if err != nil {
			return nil, fmt.Errorf("create DMCA sweep task: %w", err)
		}
		if _, err := scheduler.Register(CronMarketplaceDMCASweep, dmcaTask,
			asynq.Queue("low"),
			asynq.MaxRetry(2),
		); err != nil {
			return nil, fmt.Errorf("register DMCA sweep cron: %w", err)
		}
	}

	slugSweepTask, err := NewMarketplaceSlugHoldSweepTask(MarketplaceSlugHoldSweepPayload{})
	if err != nil {
		return nil, fmt.Errorf("create marketplace slug-hold sweep task: %w", err)
	}
	if _, err := scheduler.Register(CronMarketplaceSlugHoldSweep, slugSweepTask,
		asynq.Queue("low"),
		asynq.MaxRetry(2),
	); err != nil {
		return nil, fmt.Errorf("register marketplace slug-hold sweep cron: %w", err)
	}

	// Build a ServeMux with handlers for each scheduled task type.
	mux := asynq.NewServeMux()

	// Apply per-task-type deadlines to every handler registered on this mux.
	// Centralising the deadline here means new handlers automatically inherit
	// a budget without needing per-handler context.WithTimeout boilerplate.
	asynqDefaultBudget := cfg.Asynq.DefaultBudget
	if asynqDefaultBudget == 0 {
		asynqDefaultBudget = 1 * time.Minute
	}
	mux.Use(DeadlineMiddleware(BudgetsFromConfig(cfg.Asynq), asynqDefaultBudget))

	recrawlHandler := NewRecrawlHandler(cfg.Pool, cfg.QueueClient, cfg.Logger)
	mux.Handle(TypeRecrawlSources, recrawlHandler)

	cleanupHandler := NewCleanupHandler(cfg.Pool, cfg.Logger)
	mux.Handle(TypeCleanupSessions, cleanupHandler)

	usageHandler := NewUsageAggregationHandler(cfg.Pool, cfg.Logger)
	mux.Handle(TypeUsageAggregation, usageHandler)

	voiceUsageHandler := NewVoiceUsageHandler(cfg.Pool, cfg.Logger)
	mux.Handle(TypeVoiceUsageAggregation, voiceUsageHandler)

	trialLifecycleHandler := NewTrialLifecycleHandler(cfg.Pool, cfg.QueueClient, cfg.Logger)
	mux.Handle(TypeTrialLifecycle, trialLifecycleHandler)

	if cfg.DMCAService != nil {
		dmcaHandler := NewDMCASweeper(cfg.DMCAService, cfg.Logger)
		mux.Handle(TypeMarketplaceDMCASweep, dmcaHandler)
	}

	slugSweepHandler := NewMarketplaceSlugHoldSweepHandler(cfg.Pool, cfg.Logger)
	mux.Handle(TypeMarketplaceSlugHoldSweep, slugSweepHandler)

	logFields := []any{
		"recrawl_cron", CronRecrawl,
		"cleanup_cron", CronCleanup,
		"usage_aggregation_cron", CronUsageAggregation,
		"voice_usage_aggregation_cron", CronVoiceUsageAggregation,
		"trial_lifecycle_cron", CronTrialLifecycle,
		"marketplace_slug_hold_sweep_cron", CronMarketplaceSlugHoldSweep,
	}
	if cfg.DMCAService != nil {
		logFields = append(logFields, "marketplace_dmca_sweep_cron", CronMarketplaceDMCASweep)
	}
	cfg.Logger.Info("scheduler configured", logFields...)

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
