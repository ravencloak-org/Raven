// Package main is the entry point for the Asynq background worker process.
// It connects to Valkey using the same config as the API server and processes
// async tasks (document processing, URL scraping, KB reindexing).
package main

import (
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/queue"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := slog.Default()

	srv := queue.NewServer(queue.ServerConfig{
		RedisAddr:   cfg.Valkey.URL,
		Concurrency: cfg.Queue.Concurrency,
		MaxRetry:    cfg.Queue.MaxRetry,
		Logger:      logger,
	})

	// Start worker in a goroutine so we can listen for shutdown signals.
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("asynq server error: %v", err)
		}
	}()

	// Wait for interrupt signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("received signal, shutting down worker", "signal", sig)

	srv.Shutdown()
	logger.Info("worker exited gracefully")
}
