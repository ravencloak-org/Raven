// @title           Raven API
// @version         0.2.0
// @description     Multi-tenant knowledge management and AI chat platform.
// @termsOfService  https://ravencloak.io/terms

// @contact.name   Raven Support
// @contact.url    https://ravencloak.io
// @contact.email  support@ravencloak.io

// @license.name  Apache 2.0
// @license.url   https://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your Bearer token as: Bearer <token>

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
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/db"
	_ "github.com/ravencloak-org/Raven/docs/swagger" // swagger docs
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/service"
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

	// --- Database pool ---
	pool, err := db.New(context.Background(), cfg.Database.URL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// --- Wire repositories ---
	orgRepo := repository.NewOrgRepository(pool)
	wsRepo := repository.NewWorkspaceRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	kbRepo := repository.NewKBRepository(pool)

	// --- Wire services ---
	orgSvc := service.NewOrgService(orgRepo)
	wsSvc := service.NewWorkspaceService(wsRepo, pool)
	userSvc := service.NewUserService(userRepo)
	kbSvc := service.NewKBService(kbRepo, pool)

	// --- Wire handlers ---
	orgHandler := handler.NewOrgHandler(orgSvc)
	wsHandler := handler.NewWorkspaceHandler(wsSvc)
	userHandler := handler.NewUserHandler(userSvc)
	kbHandler := handler.NewKBHandler(kbSvc)

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

	// Swagger UI — served at /api/docs (unauthenticated; disable in prod via env).
	router.GET("/api/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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

		// --- Organisation routes ---
		api.POST("/orgs", middleware.RequireOrgRole("org_admin"), orgHandler.Create)
		api.GET("/orgs/:org_id", orgHandler.Get)
		api.PUT("/orgs/:org_id", orgHandler.Update)
		api.DELETE("/orgs/:org_id", middleware.RequireOrgRole("org_admin"), orgHandler.Delete)

		// --- Workspace routes (nested under org) ---
		ws := api.Group("/orgs/:org_id/workspaces")
		{
			ws.POST("", middleware.RequireOrgRole("org_admin"), wsHandler.Create)
			ws.GET("", wsHandler.List)
			ws.GET("/:ws_id", wsHandler.Get)
			ws.PUT("/:ws_id", middleware.RequireWorkspaceRole("admin"), wsHandler.Update)
			ws.DELETE("/:ws_id", middleware.RequireOrgRole("org_admin"), wsHandler.Delete)

			// Workspace member management
			ws.POST("/:ws_id/members", middleware.RequireWorkspaceRole("admin"), wsHandler.AddMember)
			ws.PUT("/:ws_id/members/:user_id", middleware.RequireWorkspaceRole("admin"), wsHandler.UpdateMember)
			ws.DELETE("/:ws_id/members/:user_id", middleware.RequireWorkspaceRole("admin"), wsHandler.RemoveMember)

			// Knowledge Base routes (nested under workspace)
			kb := ws.Group("/:ws_id/knowledge-bases")
			{
				kb.POST("", middleware.RequireWorkspaceRole("member"), kbHandler.Create)
				kb.GET("", kbHandler.List)
				kb.GET("/:kb_id", kbHandler.Get)
				kb.PUT("/:kb_id", middleware.RequireWorkspaceRole("member"), kbHandler.Update)
				kb.DELETE("/:kb_id", middleware.RequireWorkspaceRole("admin"), kbHandler.Archive)
			}
		}

		// --- User / me routes ---
		api.GET("/me", userHandler.GetMe)
		api.PUT("/me", userHandler.UpdateMe)
		api.DELETE("/me", userHandler.DeleteMe)
		api.GET("/users/:user_id", middleware.RequireOrgRole("org_admin"), userHandler.GetUser)
	}

	// Internal routes — no JWT, no rate limiting.
	// Must only be reachable from the compose-internal network (enforce via firewall/network policy).
	internal := router.Group("/api/v1/internal")
	{
		internal.POST("/keycloak-webhook", userHandler.KeycloakWebhook)
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
