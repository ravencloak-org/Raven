// Package main is the entry point for the Asynq background worker process.
// It connects to Valkey using the same config as the API server and processes
// async tasks (document processing, URL scraping, KB reindexing, webhook delivery).
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/jobs"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
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
	storageClient := storage.NewSeaweedFSClient(cfg.SeaweedFS.MasterURL, nil)

	srv := queue.NewServer(queue.ServerConfig{
		RedisAddr:   cfg.Valkey.URL,
		Concurrency: cfg.Queue.Concurrency,
		MaxRetry:    cfg.Queue.MaxRetry,
		Logger:      logger,
	})

	// Register email delivery handler.
	srv.Mux().HandleFunc(queue.TypeSendEmail, jobs.HandleSendEmail(notifRepo))

	// Register the webhook delivery handler on the server mux.
	webhookRepo := repository.NewWebhookRepository(pool)
	webhookDeliveryHandler := jobs.NewWebhookDeliveryHandler(pool, webhookRepo, logger)
	srv.Mux().Handle(queue.TypeWebhookDelivery, webhookDeliveryHandler)

	// Register document processing handler (overrides the stub in queue.Server).
	srv.Mux().HandleFunc(queue.TypeDocumentProcess,
		jobs.NewDocumentProcessHandler(pool, docRepo, chunkRepo, storageClient, logger))

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
