// Package main is the entry point for the Asynq background worker process.
// It connects to Valkey using the same config as the API server and processes
// async tasks (document processing, URL scraping, KB reindexing, webhook delivery).
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/email"
	rpcClient "github.com/ravencloak-org/Raven/internal/grpc"
	pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
	"github.com/ravencloak-org/Raven/internal/jobs"
	"github.com/ravencloak-org/Raven/internal/posthog"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/resilience"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := slog.Default()

	// Connect to the database so job handlers can access it.
	pool, err := db.New(context.Background(), cfg.Database.URL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	notifRepo := repository.NewNotificationRepository(pool)
	docRepo := repository.NewDocumentRepository(pool)
	chunkRepo := repository.NewChunkRepository(pool)
	airbyteRepo := repository.NewAirbyteRepository(pool)
	// Bound startup S3 probing with a timeout so a blackholed storage
	// endpoint can't hang worker boot indefinitely.
	storageCtx, cancelStorage := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStorage()
	storageClient, err := storage.NewS3Client(storageCtx, storage.S3Config{
		Endpoint:        cfg.S3.Endpoint,
		Region:          cfg.S3.Region,
		Bucket:          cfg.S3.Bucket,
		AccessKeyID:     cfg.S3.AccessKeyID,
		SecretAccessKey: cfg.S3.SecretAccessKey,
		UsePathStyle:    cfg.S3.UsePathStyle,
	})
	if err != nil {
		log.Fatalf("failed to initialise S3 storage: %v", err)
	}

	// Queue client used by webhook dispatch fan-out when job handlers emit
	// outbound webhook events. Re-uses the same Valkey/Asynq instance as the
	// server: deliveries land back on the worker mux's webhook handler.
	queueClient := queue.NewClient(cfg.Valkey.URL)
	defer func() { _ = queueClient.Close() }()

	srv := queue.NewServer(queue.ServerConfig{
		RedisAddr:   cfg.Valkey.URL,
		Concurrency: cfg.Queue.Concurrency,
		MaxRetry:    cfg.Queue.MaxRetry,
		Logger:      logger,
	})

	// Apply per-task-type deadlines to every handler registered on this mux.
	// Centralising the deadline here means new handlers automatically inherit
	// a budget without per-handler context.WithTimeout boilerplate.
	srv.Mux().Use(jobs.DeadlineMiddleware(jobs.BudgetsFromConfig(cfg.Asynq), cfg.Asynq.DefaultBudget))

	// Register email delivery handler.
	srv.Mux().HandleFunc(queue.TypeSendEmail, jobs.HandleSendEmail(notifRepo))

	// Register the webhook delivery handler on the server mux.
	webhookRepo := repository.NewWebhookRepository(pool)
	webhookDeliveryHandler := jobs.NewWebhookDeliveryHandler(pool, webhookRepo, logger)
	srv.Mux().Handle(queue.TypeWebhookDelivery, webhookDeliveryHandler)

	// Webhook dispatcher reused by document and Airbyte handlers to fan out
	// `document.processed` and `sync.completed` events on the success path.
	webhookSvc := service.NewWebhookService(webhookRepo, pool, queueClient)

	// gRPC client to the Python AI worker — drives the per-chunk embedding
	// call inside document-process. Mirrors the API-side wiring in
	// cmd/api/main.go so behaviour (timeout, breaker thresholds, telemetry)
	// is identical across both processes. Failure to construct the policy
	// or dial the worker leaves embed=nil and the handler silently
	// degrades to BM25-only retrieval.
	var embedFn jobs.EmbedFunc
	aiPolicy, policyErr := resilience.NewPolicy("ai-worker",
		resilience.WithTimeout(cfg.Server.AIWorkerTimeout),
		resilience.WithBreakerThreshold(cfg.Server.AIWorkerBreakerThreshold),
		resilience.WithBreakerCooldown(cfg.Server.AIWorkerBreakerCooldown),
		resilience.WithBreakerHalfOpenMax(cfg.Server.AIWorkerBreakerHalfOpenMax),
	)
	if policyErr != nil {
		logger.Warn("ai-worker resilience policy invalid; document chunks will be indexed without embeddings",
			"error", policyErr)
	}
	aiBreaker := resilience.NewBreaker(aiPolicy,
		resilience.WithIsSuccessful(resilience.IsGRPCCallerError),
		resilience.WithObservability(
			otel.Meter("github.com/ravencloak-org/Raven/internal/resilience"),
			otel.Tracer("github.com/ravencloak-org/Raven/internal/resilience"),
		),
	)
	grpcClient, grpcErr := rpcClient.NewClient(cfg.GRPC.WorkerAddr, aiPolicy, aiBreaker)
	if grpcErr != nil {
		logger.Warn("ai-worker gRPC client unavailable; document chunks will be indexed without embeddings",
			"address", cfg.GRPC.WorkerAddr, "error", grpcErr)
	} else if policyErr != nil {
		// policy missing — skip embed path
		_ = grpcClient.Close()
		grpcClient = nil
	} else {
		defer func() { _ = grpcClient.Close() }()
		worker := grpcClient.Worker()
		// LLM-provider repo lookup so the embedder can resolve the org's
		// default provider when the caller doesn't pin one. Python's
		// GetEmbedding rejects empty `provider` with NotFound; chat
		// already does this lookup in chat.go.
		llmRepo := repository.NewLLMProviderRepository(pool)
		resolveProvider := func(ctx context.Context, orgID string) string {
			var providerType string
			err := db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error {
				cfg, err := llmRepo.GetDefault(ctx, tx, orgID)
				if err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						return nil
					}
					return err
				}
				if cfg != nil {
					providerType = string(cfg.Provider)
				}
				return nil
			})
			if err != nil {
				logger.Warn("resolve org default provider failed",
					"org_id", orgID, "error", err)
			}
			return providerType
		}
		embedFn = func(ctx context.Context, orgID, text, provider string) ([]float32, int, string, error) {
			if provider == "" {
				provider = resolveProvider(ctx, orgID)
			}
			if provider == "" {
				// Last-ditch default. Matches the chat path's literal
				// fallback so behaviour stays consistent across both.
				provider = "ollama"
			}
			resp, err := worker.GetEmbedding(ctx, &pb.EmbeddingRequest{
				Text:     text,
				OrgId:    orgID,
				Provider: provider,
			})
			if err != nil {
				return nil, 0, "", err
			}
			return resp.GetEmbedding(), int(resp.GetDimensions()), "", nil
		}
	}

	// Register document processing handler (overrides the stub in queue.Server).
	srv.Mux().HandleFunc(queue.TypeDocumentProcess,
		jobs.NewDocumentProcessHandler(pool, docRepo, chunkRepo, storageClient, embedFn, logger, webhookSvc))

	// Register the Airbyte sync handler so connector runs fire `sync.completed`.
	airbyteSyncHandler := jobs.NewAirbyteSyncHandler(pool, airbyteRepo, logger, webhookSvc)
	srv.Mux().Handle(queue.TypeAirbyteSync, airbyteSyncHandler)

	// --- M9: post-session email summaries (#257) ---
	// Reuses the ConversationRepository landed in #349; SetSummary is added
	// on top in internal/repository/conversation_summary.go.
	convoRepo := repository.NewConversationRepository(pool)
	prefsRepo := repository.NewNotificationPreferencesRepository(pool)

	var sender email.Sender
	sesCfg := email.Config{
		Region:      cfg.SES.Region,
		FromAddress: cfg.SES.FromAddress,
		FromName:    cfg.SES.FromName,
		SMTPUser:    cfg.SES.SMTPUsername,
		SMTPPass:    cfg.SES.SMTPPassword,
	}
	if err := sesCfg.Validate(); err != nil {
		// Only fall back to the stub sender when running in debug mode OR
		// when the operator has explicitly opted in via env. Outside those
		// cases a misconfigured SES should fail startup so summary jobs
		// don't silently succeed without delivering emails.
		allowStub := cfg.Server.Mode == "debug" || os.Getenv("ALLOW_STUB_EMAIL_SENDER") == "true"
		if !allowStub {
			log.Fatalf("SES not configured for email summaries: %v", err)
		}
		logger.Warn("SES not fully configured; using stub email sender (dev/test only)", "error", err.Error())
		sender = email.NewStubSender(logger)
	} else {
		s, err := email.NewSESSender(sesCfg, logger)
		if err != nil {
			log.Fatalf("failed to initialise SES sender: %v", err)
		}
		sender = s
	}

	summaryPrefs := summaryPrefsAdapter{repo: prefsRepo}
	summarizer := jobs.NewHTTPSummarizer(cfg.EmailSummary.SummarizerBaseURL)
	posthogClient := posthog.NewClient(cfg.PostHog.APIKey, cfg.PostHog.Host)

	srv.Mux().HandleFunc(queue.TypeEmailSummary, jobs.HandleEmailSummary(jobs.EmailSummaryHandlerDeps{
		Logger:       logger,
		Sessions:     convoRepo,
		Prefs:        summaryPrefs,
		Summarizer:   summarizer,
		Sender:       sender,
		PostHog:      posthogClient,
		FrontendBase: cfg.EmailSummary.FrontendBaseURL,
		SupportEmail: cfg.EmailSummary.SupportAddress,
		UnsubSecret:  cfg.EmailSummary.UnsubscribeSecret,
		UnsubBaseURL: cfg.EmailSummary.UnsubscribeBaseURL,
	}))

	errCh := make(chan error, 1)

	// Start worker in a goroutine so we can listen for shutdown signals.
	go func() {
		if err := srv.Start(); err != nil {
			errCh <- err
		}
	}()

	// Wait for interrupt signal or server error.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down worker", "signal", sig)
	case err := <-errCh:
		logger.Error("asynq server error, shutting down", "error", err)
	}

	srv.Shutdown()
	// pool.Close() is handled by defer above.
	logger.Info("worker exited gracefully")
}

// summaryPrefsAdapter bridges repository.NotificationPreferencesRepository to
// jobs.PreferenceChecker. The summary handler asks "is this (user, workspace)
// opted in?"; we answer via the repo's effective-enabled query.
type summaryPrefsAdapter struct {
	repo *repository.NotificationPreferencesRepository
}

func (a summaryPrefsAdapter) IsEnabled(ctx context.Context, orgID, userID, workspaceID string) (bool, error) {
	return a.repo.GetEmailSummariesEnabled(ctx, orgID, userID, workspaceID)
}
