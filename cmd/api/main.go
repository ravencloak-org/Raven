package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/telemetry"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialise OpenTelemetry (no-op when disabled or no endpoint).
	otelEndpoint := ""
	if cfg.OTel.Enabled {
		otelEndpoint = cfg.OTel.Endpoint
	}
	otelShutdown, err := telemetry.InitProvider(
		context.Background(),
		cfg.OTel.ServiceName,
		"0.1.0",
		otelEndpoint,
		cfg.Server.Mode,
	)
	if err != nil {
		log.Fatalf("failed to initialise telemetry: %v", err)
	}
	defer func() {
		if err := otelShutdown(context.Background()); err != nil {
			log.Printf("telemetry shutdown error: %v", err)
		}
	}()

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Initialise Valkey client for rate limiting.
	valkeyClient := redis.NewClient(&redis.Options{
		Addr: cfg.Valkey.URL,
	})

	// Build rate limiter using config-driven limits.
	rl := middleware.NewRateLimiter(valkeyClient, slog.Default())

	// Create router
	router := gin.Default()

	// Global middleware order: OTel → SecurityHeaders → CORS → ErrorHandler
	router.Use(middleware.OTelMiddleware())
	router.Use(middleware.SecurityHeadersMiddleware())
	router.Use(middleware.CORSMiddleware(&cfg.CORS))
	router.Use(apierror.ErrorHandler())

	// Apply rate limiting by user ID and org ID using config-driven defaults.
	router.Use(middleware.ByUserID(rl, cfg.RateLimit.DefaultUserLimit))
	router.Use(middleware.ByOrgID(rl, cfg.RateLimit.DefaultOrgLimit))

	// Infrastructure endpoint — intentionally outside the versioned group.
	router.GET("/healthz", handler.HealthCheck)

	// Protected API routes — JWT validation applied per-group, not globally.
	// This allows health checks and other public endpoints to remain unauthenticated.
	//
	// NOTE: The repository layer is responsible for applying RLS by executing
	//   SET LOCAL app.current_org_id = '<uuid>'
	// using the org_id stored in the Gin context key middleware.ContextKeyOrgID.
	api := router.Group("/api/v1")
	api.Use(middleware.JWTMiddleware(&cfg.Keycloak))
	{
		api.GET("/ping", handler.Ping)
	}

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
