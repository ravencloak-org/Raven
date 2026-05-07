# Resilience plan — codebase survey (2026-05-07T22:35:55Z)

## apierror package
pkg/apierror/apierror.go:1:package apierror
pkg/apierror/apierror.go:100:func ErrorHandler() gin.HandlerFunc {
NOTE: package lives under pkg/ (not internal/ or cmd/); the original grep was scoped too narrowly.

## Asynq handler files (ProcessTask receivers)
internal/jobs/voice_usage.go:62:func (h *VoiceUsageHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
internal/jobs/webhook_delivery.go:128:func (h *WebhookDeliveryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
internal/jobs/cleanup.go:40:func (h *CleanupHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
internal/jobs/recrawl.go:39:func (h *RecrawlHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
internal/jobs/usage.go:37:func (h *UsageAggregationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {

## Config: ServerConfig fields
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

## Config: GRPCConfig fields
type GRPCConfig struct {
	WorkerAddr string `mapstructure:"worker_addr"`
}

## main.go http.Server construction (around line 855)
		authHandler.Callback(c)
	})

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Graceful shutdown with signal handling
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	go func() {
		log.Printf("Raven API server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	log.Println("shutting down server...")

	// Give outstanding requests 5 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exited gracefully")
}

## main.go gRPC client construction (around line 313)
		chEmbeddingRepo = repository.NewClickHouseEmbeddingRepository(chConn)
		if err := chEmbeddingRepo.EnsureSchema(context.Background()); err != nil {
			slog.Warn("ClickHouse schema init failed", "error", err)
			chEmbeddingRepo = nil
		}
	}

	// --- gRPC client for AI worker ---
	grpcClient, err := rpcClient.NewClient(cfg.GRPC.WorkerAddr)
	if err != nil {
		log.Fatalf("failed to connect to AI worker: %v", err)
	}
	defer func() {
		if err := grpcClient.Close(); err != nil {
			log.Printf("grpc client close error: %v", err)
		}
	}()

	// --- Wire storage client ---
	seaweedClient := storage.NewSeaweedFSClient(cfg.SeaweedFS.MasterURL, nil)

	// --- Wire billing & quota ---
	subCache := service.NewValkeySubscriptionCache(valkeyClient)
	quotaChecker := service.NewQuotaChecker(billingRepo, subCache, pool)

	// --- Wire services ---

## Module path (from go.mod)
module github.com/ravencloak-org/Raven
