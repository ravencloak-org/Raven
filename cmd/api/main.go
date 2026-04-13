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

	"github.com/ravencloak-org/Raven/internal/cache"
	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/hyperswitch"
	_ "github.com/ravencloak-org/Raven/docs/swagger" // swagger docs
	rpcClient "github.com/ravencloak-org/Raven/internal/grpc"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/posthog"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/internal/storage"
	"github.com/ravencloak-org/Raven/internal/stt"
	"github.com/ravencloak-org/Raven/internal/telemetry"
	"github.com/ravencloak-org/Raven/internal/tts"
	"github.com/ravencloak-org/Raven/pkg/apierror"
	lk "github.com/ravencloak-org/Raven/pkg/livekit"
)

// securityEvaluatorAdapter bridges the SecurityService to the middleware.SecurityEvaluator
// interface without creating a circular import dependency.
type securityEvaluatorAdapter struct {
	svc *service.SecurityService
}

func (a *securityEvaluatorAdapter) EvaluateRequest(ctx context.Context, orgID, clientIP, path, method, userAgent string) (*middleware.SecurityRuleAction, error) {
	action, err := a.svc.EvaluateRequest(ctx, orgID, clientIP, path, method, userAgent)
	if err != nil {
		return nil, err
	}
	if action == nil {
		return nil, nil
	}
	return &middleware.SecurityRuleAction{
		Block:    action.Block,
		Throttle: action.Throttle,
		RuleID:   action.RuleID,
		RuleName: action.RuleName,
	}, nil
}

// apiKeyLookupAdapter bridges the APIKeyRepository to the middleware.APIKeyLookup
// interface without creating a circular import dependency.
type apiKeyLookupAdapter struct {
	repo *repository.APIKeyRepository
}

func (a *apiKeyLookupAdapter) LookupByHash(ctx context.Context, keyHash string) (*middleware.APIKeyLookupResult, error) {
	ak, err := a.repo.GetByKeyHashNoTx(ctx, keyHash)
	if err != nil {
		return nil, err
	}
	return apiKeyToLookupResult(ak), nil
}

// userLookupAdapter bridges UserRepository to the middleware.UserResolver interface.
type userLookupAdapter struct {
	repo *repository.UserRepository
}

func (a *userLookupAdapter) GetByExternalID(ctx context.Context, externalID string) (string, *string, error) {
	u, err := a.repo.GetByExternalID(ctx, externalID)
	if err != nil {
		return "", nil, err
	}
	return u.ID, u.OrgID, nil
}

func apiKeyToLookupResult(ak *model.APIKey) *middleware.APIKeyLookupResult {
	return &middleware.APIKeyLookupResult{
		ID:              ak.ID,
		OrgID:           ak.OrgID,
		WorkspaceID:     ak.WorkspaceID,
		KnowledgeBaseID: ak.KnowledgeBaseID,
		AllowedDomains:  ak.AllowedDomains,
		RateLimit:       ak.RateLimit,
		Status:          string(ak.Status),
	}
}

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

	// Initialise eBPF subsystem (no-op when unavailable or disabled).
	if cfg.EBPF.Enabled {
		ebpfManager, err := initEBPF(&cfg.EBPF)
		if err != nil {
			log.Printf("eBPF subsystem degraded: %v", err)
		}
		defer ebpfManager.Stop()
	}

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Initialise Valkey client for rate limiting.
	valkeyClient := redis.NewClient(&redis.Options{
		Addr: cfg.Valkey.URL,
	})

	// Build rate limiter using config-driven limits.
	rl := middleware.NewRateLimiter(valkeyClient, slog.Default())
	tierResolver := middleware.NewValkeyTierResolver(valkeyClient, slog.Default())
	tierCfg := middleware.TierConfig{
		Free: middleware.TierLimits{
			GeneralRPM:    cfg.RateLimit.FreeGeneralRPM,
			CompletionRPM: cfg.RateLimit.FreeCompletionRPM,
		},
		Pro: middleware.TierLimits{
			GeneralRPM:    cfg.RateLimit.ProGeneralRPM,
			CompletionRPM: cfg.RateLimit.ProCompletionRPM,
		},
		Enterprise: middleware.TierLimits{
			GeneralRPM:    cfg.RateLimit.EnterpriseGeneralRPM,
			CompletionRPM: cfg.RateLimit.EnterpriseCompletionRPM,
		},
		WidgetRPM: cfg.RateLimit.WidgetRPM,
	}

	// --- Response cache ---
	// Exact-match RAG response cache backed by Valkey. Will be passed to the
	// chat service once #34 (chat endpoint) merges.
	responseCache := cache.NewResponseCache(valkeyClient, 1*time.Hour)
	_ = responseCache // TODO(#34): pass to chat service

	// --- Asynq queue client ---
	// The queue client is initialised here and will be passed to services that
	// need to enqueue async jobs (document processing, URL scraping, reindexing).
	// Real wiring happens in subsequent issues (#14-#17).
	queueClient := queue.NewClient(cfg.Valkey.URL,
		queue.WithMaxRetry(cfg.Queue.MaxRetry),
		queue.WithLogger(slog.Default()),
	)
	defer func() {
		if err := queueClient.Close(); err != nil {
			log.Printf("queue client close error: %v", err)
		}
	}()
	// queueClient is passed to services that enqueue async jobs.

	// --- Database pool ---
	pool, err := db.New(context.Background(), cfg.Database.URL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// --- ClickHouse connection (enterprise, optional) ---
	chConn, err := db.NewClickHouse(context.Background(), db.ClickHouseConfig{
		Host:     cfg.ClickHouse.Host,
		Port:     cfg.ClickHouse.Port,
		Database: cfg.ClickHouse.Database,
		User:     cfg.ClickHouse.User,
		Password: cfg.ClickHouse.Password,
	})
	if err != nil {
		// ClickHouse is optional — log and continue without it.
		slog.Warn("ClickHouse connection failed; vector search will use pgvector only", "error", err)
	}
	if chConn != nil {
		defer chConn.Close() //nolint:errcheck // best-effort cleanup on shutdown
	}

	// --- Wire repositories ---
	orgRepo := repository.NewOrgRepository(pool)
	wsRepo := repository.NewWorkspaceRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	kbRepo := repository.NewKBRepository(pool)
	sourceRepo := repository.NewSourceRepository(pool)
	docRepo := repository.NewDocumentRepository(pool)
	searchRepo := repository.NewSearchRepository(pool)
	_ = repository.NewChunkRepository(pool) // wired for future service/handler layers
	llmRepo := repository.NewLLMProviderRepository(pool)
	processingEventRepo := repository.NewProcessingEventRepository()
	apiKeyRepo := repository.NewAPIKeyRepository(pool)
	routingRepo := repository.NewRoutingRepository(pool)
	airbyteRepo := repository.NewAirbyteRepository(pool)
	securityRepo := repository.NewSecurityRepository(pool)
	strangerRepo := repository.NewStrangerRepository(pool)
	notifRepo := repository.NewNotificationRepository(pool)
	identityRepo := repository.NewIdentityRepository(pool)
	semCacheRepo := repository.NewSemanticCacheRepository(pool)
	webhookRepo := repository.NewWebhookRepository(pool)
	leadRepo := repository.NewLeadRepository(pool)
	whatsappRepo := repository.NewWhatsAppRepository(pool)
	billingRepo := repository.NewBillingRepository(pool)

	// --- ClickHouse embedding repository (enterprise, optional) ---
	var chEmbeddingRepo *repository.ClickHouseEmbeddingRepository
	if chConn != nil {
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
	wsSvc := service.NewWorkspaceService(wsRepo, pool, quotaChecker)
	orgSvc := service.NewOrgService(orgRepo, wsSvc)
	userSvc := service.NewUserService(userRepo)
	kbSvc := service.NewKBService(kbRepo, pool, quotaChecker)
	sourceSvc := service.NewSourceService(sourceRepo, pool)
	docSvc := service.NewDocumentService(docRepo, pool)
	searchSvc := service.NewSearchService(searchRepo, pool)
	hybridRetrievalSvc := service.NewHybridRetrievalService(
		searchRepo, chEmbeddingRepo, pool,
		model.VectorBackend(cfg.ClickHouse.VectorBackend),
		cfg.ClickHouse.ChunkThreshold,
	)
	_ = hybridRetrievalSvc // TODO: wire into chat handler for enterprise hybrid search
	llmSvc, err := service.NewLLMProviderService(llmRepo, pool, cfg.Encryption.AESKey)
	if err != nil {
		log.Fatalf("failed to initialise LLM provider service: %v", err)
	}
	uploadSvc := service.NewUploadService(docRepo, pool, seaweedClient, cfg.Upload.MaxSizeBytes, cfg.Upload.AllowedTypes)
	processingSvc := service.NewProcessingEventService(processingEventRepo, docRepo, pool)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, pool)
	routingSvc := service.NewRoutingService(routingRepo, kbRepo, pool)
	airbyteSvc := service.NewAirbyteService(airbyteRepo, pool, queueClient)
	securitySvc := service.NewSecurityService(securityRepo, pool, valkeyClient)
	strangerSvc := service.NewStrangerService(strangerRepo, pool)
	posthogClient := posthog.NewClient(cfg.PostHog.APIKey, cfg.PostHog.Host)
	identitySvc := service.NewIdentityService(identityRepo, posthogClient)
	notifSvc := service.NewNotificationService(notifRepo, queueClient)
	webhookSvc := service.NewWebhookService(webhookRepo, pool, queueClient)
	leadSvc := service.NewLeadService(leadRepo)
	whatsappSvc := service.NewWhatsAppService(whatsappRepo, pool)
	hsClient := hyperswitch.NewClient(cfg.Hyperswitch.BaseURL, cfg.Hyperswitch.APIKey)
	billingSvc := service.NewBillingService(billingRepo, pool, hsClient, cfg.Hyperswitch.WebhookSecret)
	chatRepo := repository.NewChatRepository(pool)
	chatSvc := service.NewChatService(chatRepo, grpcClient, pool)
	voiceRepo := repository.NewVoiceRepository(pool)
	// Instantiate shared LiveKit client for WebRTC room management and token generation.
	lkClient := lk.NewClient(lk.Config{
		APIURL:    cfg.LiveKit.APIURL,
		WSURL:     cfg.LiveKit.WSURL,
		APIKey:    cfg.LiveKit.APIKey,
		APISecret: cfg.LiveKit.APISecret,
	})
	var livekitClient *lk.Client
	if cfg.LiveKit.APIURL != "" && cfg.LiveKit.APIKey != "" && cfg.LiveKit.APISecret != "" {
		livekitClient = lkClient
		slog.Info("LiveKit client initialised", "api_url", cfg.LiveKit.APIURL, "ws_url", cfg.LiveKit.WSURL)
	} else {
		slog.Warn("LiveKit not configured: api_url, api_key, and api_secret are all required; voice session room management disabled")
	}
	voiceSvc := service.NewVoiceService(voiceRepo, pool, livekitClient, cfg.LiveKit.WSURL, quotaChecker)

	// --- Wire WhatsApp-LiveKit bridge ---
	waBridgeRepo := repository.NewWhatsAppBridgeRepository(pool)
	sdpRelay := service.NewLiveKitSDPRelay(lkClient)
	waBridgeSvc := service.NewWhatsAppBridgeService(waBridgeRepo, voiceRepo, pool, lkClient, sdpRelay)

	// --- Wire TTS provider ---
	var ttsProvider tts.Provider
	switch cfg.TTS.Provider {
	case "piper":
		ttsProvider = tts.NewPiperProvider(tts.PiperConfig{
			Endpoint: cfg.TTS.PiperEndpoint,
			Voice:    cfg.TTS.PiperVoice,
		})
		slog.Info("TTS provider initialised", "provider", "piper")
	default: // "cartesia" or unset
		if cfg.TTS.CartesiaAPIKey != "" {
			var err2 error
			ttsProvider, err2 = tts.NewCartesiaProvider(tts.CartesiaConfig{
				APIKey:  cfg.TTS.CartesiaAPIKey,
				VoiceID: cfg.TTS.CartesiaVoiceID,
				Model:   cfg.TTS.CartesiaModel,
				BaseURL: cfg.TTS.CartesiaBaseURL,
			})
			if err2 != nil {
				log.Fatalf("failed to initialise Cartesia TTS provider: %v", err2)
			}
			slog.Info("TTS provider initialised", "provider", "cartesia")
		} else {
			slog.Warn("TTS provider not configured: CARTESIA_API_KEY is empty; TTS endpoint will return 503")
		}
	}
	var ttsSvc *service.TTSService
	if ttsProvider != nil {
		ttsSvc = service.NewTTSService(ttsProvider)
	}

	// --- Wire STT provider ---
	// The provider is selected by RAVEN_STT_PROVIDER ("deepgram" or "whisper").
	// Defaults to Deepgram when RAVEN_STT_DEEPGRAM_API_KEY is set, otherwise falls
	// back to the self-hosted faster-whisper endpoint.
	sttProvider, err := stt.NewProvider(stt.Config{
		Provider: stt.ProviderName(cfg.STT.Provider),
		Deepgram: stt.DeepgramConfig{
			APIKey:  cfg.STT.DeepgramAPIKey,
			Model:   cfg.STT.DeepgramModel,
			BaseURL: cfg.STT.DeepgramBaseURL,
		},
		Whisper: stt.WhisperConfig{
			Endpoint: cfg.STT.WhisperEndpoint,
			Model:    cfg.STT.WhisperModel,
		},
	})
	if err != nil {
		slog.Warn("stt provider not initialised — transcription unavailable", "error", err)
		sttProvider = nil
	} else {
		slog.Info("stt provider initialised", "provider", sttProvider.Name())
	}
	_ = sttProvider // available for future wiring into voice turn transcription

	// --- Wire handlers ---
	orgHandler := handler.NewOrgHandler(orgSvc)
	wsHandler := handler.NewWorkspaceHandler(wsSvc)
	userHandler := handler.NewUserHandler(userSvc)
	kbHandler := handler.NewKBHandler(kbSvc)
	sourceHandler := handler.NewSourceHandler(sourceSvc)
	docHandler := handler.NewDocumentHandler(docSvc)
	searchHandler := handler.NewSearchHandler(searchSvc)
	llmHandler := handler.NewLLMProviderHandler(llmSvc)
	uploadHandler := handler.NewUploadHandler(uploadSvc)
	processingHandler := handler.NewProcessingEventHandler(processingSvc)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeySvc)
	routingHandler := handler.NewRoutingHandler(routingSvc)
	airbyteHandler := handler.NewAirbyteHandler(airbyteSvc)
	securityHandler := handler.NewSecurityHandler(securitySvc)
	strangerHandler := handler.NewStrangerHandler(strangerSvc)
	identityHandler := handler.NewIdentityHandler(identitySvc)
	notifHandler := handler.NewNotificationHandler(notifSvc)
	webhookHandler := handler.NewWebhookHandler(webhookSvc)
	chatHandler := handler.NewChatHandler(chatSvc)
	semCacheHandler := handler.NewSemanticCacheHandler(semCacheRepo)
	voiceHandler := handler.NewVoiceHandler(voiceSvc)
	waBridgeHandler := handler.NewWhatsAppBridgeHandler(waBridgeSvc)
	var ttsHandler *handler.TTSHandler
	if ttsSvc != nil {
		ttsHandler = handler.NewTTSHandler(ttsSvc)
	}
	usageHandler := handler.NewUsageHandler(quotaChecker)
	leadHandler := handler.NewLeadHandler(leadSvc)
	whatsappHandler := handler.NewWhatsAppHandler(whatsappSvc)
	billingHandler := handler.NewBillingHandler(billingSvc)

	// Create router
	router := gin.Default()

	// Global middleware order: OTel → SecurityHeaders → CORS → ErrorHandler
	router.Use(middleware.OTelMiddleware())
	router.Use(middleware.SecurityHeadersMiddleware())
	router.Use(middleware.CORSMiddleware(&cfg.CORS))
	router.Use(apierror.ErrorHandler())

	// Infrastructure endpoint — intentionally outside the versioned group.
	// Excluded from rate limiting.
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
	api.Use(middleware.JWTMiddleware(&cfg.Zitadel))
	api.Use(middleware.UserLookup(&userLookupAdapter{repo: userRepo}))
	// Per-user and per-org flat rate limits (config-driven defaults).
	api.Use(middleware.ByUserID(rl, cfg.RateLimit.DefaultUserLimit))
	api.Use(middleware.ByOrgID(rl, cfg.RateLimit.DefaultOrgLimit))
	// Per-org tier-based rate limit for general API endpoints.
	api.Use(middleware.ByOrgTier(rl, tierResolver, tierCfg, middleware.RouteGroupGeneral))
	{
		api.GET("/ping", handler.Ping)

		// --- Organisation routes ---
		api.POST("/orgs", middleware.RequireOrgRole("org_admin"), orgHandler.Create)
		api.GET("/orgs/:org_id", orgHandler.Get)
		api.PUT("/orgs/:org_id", orgHandler.Update)
		api.DELETE("/orgs/:org_id", middleware.RequireOrgRole("org_admin"), orgHandler.Delete)

		// --- Workspace routes (nested under org) ---
		// ResolveWorkspaceRole looks up the caller's workspace role from the
		// workspace_members table and stores it in the Gin context. It is applied
		// to all routes that contain a :ws_id parameter so that downstream
		// RequireWorkspaceRole checks have the role available.
		resolveWSRole := middleware.ResolveWorkspaceRole(pool)

		ws := api.Group("/orgs/:org_id/workspaces")
		{
			ws.POST("", middleware.RequireOrgRole("org_admin"), wsHandler.Create)
			ws.GET("", wsHandler.List)
			ws.GET("/:ws_id", resolveWSRole, wsHandler.Get)
			ws.PUT("/:ws_id", resolveWSRole, middleware.RequireWorkspaceRole("admin"), wsHandler.Update)
			ws.DELETE("/:ws_id", middleware.RequireOrgRole("org_admin"), wsHandler.Delete)

			// Workspace member management
			ws.POST("/:ws_id/members", resolveWSRole, middleware.RequireWorkspaceRole("admin"), wsHandler.AddMember)
			ws.PUT("/:ws_id/members/:user_id", resolveWSRole, middleware.RequireWorkspaceRole("admin"), wsHandler.UpdateMember)
			ws.DELETE("/:ws_id/members/:user_id", resolveWSRole, middleware.RequireWorkspaceRole("admin"), wsHandler.RemoveMember)

			// Knowledge Base routes (nested under workspace)
			kb := ws.Group("/:ws_id/knowledge-bases", resolveWSRole)
			{
				kb.POST("", middleware.RequireWorkspaceRole("member"), kbHandler.Create)
				kb.GET("", kbHandler.List)
				kb.GET("/:kb_id", kbHandler.Get)
				kb.PUT("/:kb_id", middleware.RequireWorkspaceRole("member"), kbHandler.Update)
				kb.DELETE("/:kb_id", middleware.RequireWorkspaceRole("admin"), kbHandler.Archive)

				// Full-text search (nested under knowledge base)
				kb.GET("/:kb_id/search", searchHandler.Search)

				// Document upload
				kb.POST("/:kb_id/documents/upload", middleware.RequireWorkspaceRole("member"), uploadHandler.Upload)

				// Source routes (nested under knowledge base)
				src := kb.Group("/:kb_id/sources")
				{
					src.POST("", middleware.RequireWorkspaceRole("member"), sourceHandler.Create)
					src.GET("", sourceHandler.List)
					src.GET("/:source_id", sourceHandler.Get)
					src.PUT("/:source_id", middleware.RequireWorkspaceRole("member"), sourceHandler.Update)
					src.DELETE("/:source_id", middleware.RequireWorkspaceRole("admin"), sourceHandler.Delete)
				}

				// Document routes (nested under knowledge base)
				doc := kb.Group("/:kb_id/documents")
				{
					doc.GET("", docHandler.List)
					doc.GET("/:doc_id", docHandler.Get)
					doc.PUT("/:doc_id", middleware.RequireWorkspaceRole("member"), docHandler.Update)
					doc.DELETE("/:doc_id", middleware.RequireWorkspaceRole("admin"), docHandler.Delete)

					// Processing event routes (nested under document)
					doc.GET("/:doc_id/events", processingHandler.ListEvents)
					doc.POST("/:doc_id/transitions", middleware.RequireWorkspaceRole("member"), processingHandler.Transition)
				}

				// API key routes (nested under knowledge base)
				apiKeys := kb.Group("/:kb_id/api-keys")
				{
					apiKeys.POST("", middleware.RequireWorkspaceRole("member"), apiKeyHandler.Create)
					apiKeys.GET("", apiKeyHandler.List)
					apiKeys.DELETE("/:key_id", middleware.RequireWorkspaceRole("admin"), apiKeyHandler.Revoke)
				}
			}
		}

		// --- Airbyte connector routes (nested under org) ---
		connectors := api.Group("/orgs/:org_id/connectors")
		{
			connectors.POST("", middleware.RequireOrgRole("org_admin"), airbyteHandler.Create)
			connectors.GET("", airbyteHandler.List)
			connectors.GET("/:connector_id", airbyteHandler.Get)
			connectors.PUT("/:connector_id", middleware.RequireOrgRole("org_admin"), airbyteHandler.Update)
			connectors.DELETE("/:connector_id", middleware.RequireOrgRole("org_admin"), airbyteHandler.Delete)
			connectors.POST("/:connector_id/sync", middleware.RequireOrgRole("org_admin"), airbyteHandler.TriggerSync)
			connectors.GET("/:connector_id/history", airbyteHandler.GetSyncHistory)
		}

		// --- LLM Provider routes (nested under org) ---
		llm := api.Group("/orgs/:org_id/llm-providers")
		{
			llm.POST("", middleware.RequireOrgRole("org_admin"), llmHandler.Create)
			llm.GET("", llmHandler.List)
			llm.GET("/:provider_id", llmHandler.Get)
			llm.PUT("/:provider_id", middleware.RequireOrgRole("org_admin"), llmHandler.Update)
			llm.DELETE("/:provider_id", middleware.RequireOrgRole("org_admin"), llmHandler.Delete)
			llm.PUT("/:provider_id/default", middleware.RequireOrgRole("org_admin"), llmHandler.SetDefault)
		}

		// --- Routing rule routes (nested under org) ---
		routing := api.Group("/orgs/:org_id/routing-rules")
		{
			routing.POST("", middleware.RequireOrgRole("org_admin"), routingHandler.Create)
			routing.GET("", middleware.RequireOrgRole("org_admin"), routingHandler.List)
			routing.GET("/:rule_id", middleware.RequireOrgRole("org_admin"), routingHandler.Get)
			routing.PUT("/:rule_id", middleware.RequireOrgRole("org_admin"), routingHandler.Update)
			routing.DELETE("/:rule_id", middleware.RequireOrgRole("org_admin"), routingHandler.Delete)
			routing.POST("/resolve", middleware.RequireOrgRole("org_admin"), routingHandler.Resolve)
		}

		// --- Catalog metadata routes (nested under org) ---
		api.GET("/orgs/:org_id/catalog", middleware.RequireOrgRole("org_admin"), routingHandler.ListCatalog)

		// --- Stranger user routes (nested under org, admin only) ---
		strangers := api.Group("/orgs/:org_id/strangers", middleware.RequireOrgRole("org_admin"))
		{
			strangers.GET("", strangerHandler.List)
			strangers.GET("/:id", strangerHandler.Get)
			strangers.POST("/:id/block", strangerHandler.Block)
			strangers.POST("/:id/unblock", strangerHandler.Unblock)
			strangers.PUT("/:id/rate-limit", strangerHandler.SetRateLimit)
			strangers.DELETE("/:id", strangerHandler.Delete)
		}

		// --- Semantic cache management routes (nested under org/kb) ---
		semCache := api.Group("/orgs/:org_id/kbs/:kb_id/cache")
		{
			semCache.DELETE("", middleware.RequireOrgRole("org_admin"), semCacheHandler.InvalidateKBCache)
			semCache.GET("/stats", middleware.RequireOrgRole("org_admin"), semCacheHandler.GetCacheStats)
		}

		// --- Security rules routes (nested under org, admin only) ---
		sec := api.Group("/orgs/:org_id/security")
		{
			secRules := sec.Group("/rules", middleware.RequireOrgRole("org_admin"))
			{
				secRules.POST("", securityHandler.CreateRule)
				secRules.GET("", securityHandler.ListRules)
				secRules.GET("/:rule_id", securityHandler.GetRule)
				secRules.PUT("/:rule_id", securityHandler.UpdateRule)
				secRules.DELETE("/:rule_id", securityHandler.DeleteRule)
				secRules.POST("/invalidate-cache", securityHandler.InvalidateRuleCache)
			}
			sec.GET("/events", middleware.RequireOrgRole("org_admin"), securityHandler.ListEvents)
		}

		// --- Identity / PostHog routes (nested under org) ---
		identity := api.Group("/orgs/:org_id/identity")
		{
			identity.POST("", middleware.RequireOrgRole("org_member"), identityHandler.Identify)
			identity.POST("/track", middleware.RequireOrgRole("org_member"), identityHandler.Track)
			identity.GET("", middleware.RequireOrgRole("org_member"), identityHandler.ListIdentities)
			identity.DELETE("/:id", middleware.RequireOrgRole("org_admin"), identityHandler.DeleteIdentity)
		}

		// --- Notification config and log routes (nested under org, admin only) ---
		notif := api.Group("/orgs/:org_id/notifications", middleware.RequireOrgRole("org_admin"))
		{
			notif.POST("/configs", notifHandler.CreateConfig)
			notif.GET("/configs", notifHandler.ListConfigs)
			notif.PUT("/configs/:id", notifHandler.UpdateConfig)
			notif.DELETE("/configs/:id", notifHandler.DeleteConfig)
			notif.GET("/logs", notifHandler.ListLogs)
		}

		// --- Webhook routes (nested under org, admin only) ---
		webhooks := api.Group("/orgs/:org_id/webhooks", middleware.RequireOrgRole("org_admin"))
		{
			webhooks.POST("", webhookHandler.Create)
			webhooks.GET("", webhookHandler.List)
			webhooks.GET("/:id", webhookHandler.Get)
			webhooks.PUT("/:id", webhookHandler.Update)
			webhooks.DELETE("/:id", webhookHandler.Delete)
			webhooks.GET("/:id/deliveries", webhookHandler.ListDeliveries)
		}

		// --- Voice session routes (nested under org) ---
		voice := api.Group("/orgs/:org_id/voice-sessions")
		{
			voice.POST("", middleware.RequireOrgRole("org_member"), voiceHandler.CreateSession)
			voice.GET("", middleware.RequireOrgRole("org_member"), voiceHandler.ListSessions)
			voice.GET("/:session_id", middleware.RequireOrgRole("org_member"), voiceHandler.GetSession)
			voice.PATCH("/:session_id", middleware.RequireOrgRole("org_member"), voiceHandler.UpdateSessionState)
			voice.POST("/:session_id/token", middleware.RequireOrgRole("org_member"), voiceHandler.GenerateToken)
			voice.POST("/:session_id/turns", middleware.RequireOrgRole("org_member"), voiceHandler.AppendTurn)
			voice.GET("/:session_id/turns", middleware.RequireOrgRole("org_member"), voiceHandler.ListTurns)
		}

		// --- WhatsApp-LiveKit bridge routes (nested under org) ---
		waCallBridge := api.Group("/orgs/:org_id/whatsapp")
		{
			waCallBridge.POST("/calls/:call_id/bridge", middleware.RequireOrgRole("org_member"), waBridgeHandler.CreateBridge)
			waCallBridge.GET("/calls/:call_id/bridge", middleware.RequireOrgRole("org_member"), waBridgeHandler.GetBridge)
			waCallBridge.DELETE("/calls/:call_id/bridge", middleware.RequireOrgRole("org_member"), waBridgeHandler.TeardownBridge)
			waCallBridge.GET("/bridges", middleware.RequireOrgRole("org_member"), waBridgeHandler.ListActiveBridges)
		}

		// --- TTS synthesis route (nested under org) ---
		if ttsHandler != nil {
			api.POST("/orgs/:org_id/tts", middleware.RequireOrgRole("org_member"), ttsHandler.Synthesize)
		}

		// --- Lead intelligence routes (nested under org) ---
		leads := api.Group("/orgs/:org_id/leads", middleware.RequireOrgRole("member"))
		{
			leads.POST("", leadHandler.UpsertLead)
			leads.GET("", leadHandler.ListLeads)
			leads.GET("/export", leadHandler.ExportLeadsCSV)
			leads.GET("/:id", leadHandler.GetLead)
			leads.PUT("/:id", leadHandler.UpdateLead)
			leads.DELETE("/:id", middleware.RequireOrgRole("org_admin"), leadHandler.DeleteLead)
		}

		// --- WhatsApp Business Calling routes (nested under org, admin only) ---
		wa := api.Group("/orgs/:org_id/whatsapp", middleware.RequireOrgRole("org_admin"))
		{
			waPhones := wa.Group("/phone-numbers")
			{
				waPhones.POST("", whatsappHandler.CreatePhoneNumber)
				waPhones.GET("", whatsappHandler.ListPhoneNumbers)
				waPhones.GET("/:phone_id", whatsappHandler.GetPhoneNumber)
				waPhones.PUT("/:phone_id", whatsappHandler.UpdatePhoneNumber)
				waPhones.DELETE("/:phone_id", whatsappHandler.DeletePhoneNumber)
			}
			waCalls := wa.Group("/calls")
			{
				waCalls.POST("", whatsappHandler.InitiateCall)
				waCalls.GET("", whatsappHandler.ListCalls)
				waCalls.GET("/:call_id", whatsappHandler.GetCall)
				waCalls.PATCH("/:call_id", whatsappHandler.UpdateCallState)
			}
		}

		// --- Billing / subscription routes ---
		billing := api.Group("/billing")
		{
			billing.GET("/plans", billingHandler.GetPlans)
			billing.GET("/subscriptions/current", billingHandler.GetCurrentSubscription)
			billing.POST("/subscriptions", billingHandler.Subscribe)
			billing.DELETE("/subscriptions/:id", billingHandler.Unsubscribe)
			billing.POST("/payment-intents", billingHandler.CreatePaymentIntent)
			billing.GET("/usage", usageHandler.GetUsage)
		}

		// --- User / me routes ---
		api.GET("/me", userHandler.GetMe)
		api.PUT("/me", userHandler.UpdateMe)
		api.DELETE("/me", userHandler.DeleteMe)
		api.GET("/users/:user_id", middleware.RequireOrgRole("org_admin"), userHandler.GetUser)

	}

	// Public chat routes — API key authentication (for embeddable chat widget).
	chatAPI := router.Group("/api/v1/chat")
	chatAPI.Use(middleware.APIKeyAuth(&apiKeyLookupAdapter{repo: apiKeyRepo}))
	chatAPI.Use(middleware.SecurityRulesMiddleware(&securityEvaluatorAdapter{svc: securitySvc}))
	chatAPI.Use(middleware.StrangerCheck(strangerSvc, valkeyClient))
	// Don't apply widget rate limit to the group — apply selectively instead.
	{
		// Completion endpoint gets its own per-org completion rate limit.
		chatAPI.POST("/:kb_id/completions",
			middleware.ByOrgTier(rl, tierResolver, tierCfg, middleware.RouteGroupCompletion),
			chatHandler.StreamCompletion)
		// Non-completion widget routes get the widget rate limit.
		chatAPI.GET("/:kb_id/sessions",
			middleware.ByOrgTier(rl, tierResolver, tierCfg, middleware.RouteGroupWidget),
			chatHandler.ListSessions)
		chatAPI.GET("/:kb_id/sessions/:session_id/history",
			middleware.ByOrgTier(rl, tierResolver, tierCfg, middleware.RouteGroupWidget),
			chatHandler.GetHistory)
		chatAPI.DELETE("/:kb_id/sessions/:session_id",
			middleware.ByOrgTier(rl, tierResolver, tierCfg, middleware.RouteGroupWidget),
			chatHandler.DeleteSession)
	}

	// --- Hyperswitch Billing Webhook (public, no JWT — uses HMAC signature verification) ---
	router.POST("/api/v1/billing/webhook", billingHandler.Webhook)

	// --- Meta Graph API Webhook (public, no JWT — Meta sends to a single URL) ---
	metaWebhookHandler := handler.NewMetaWebhookHandler(cfg.Meta.AppSecret, cfg.Meta.WebhookToken, nil)
	router.GET("/webhooks/meta", metaWebhookHandler.VerifyWebhook)
	router.POST("/webhooks/meta", metaWebhookHandler.HandleEvent)

	// Auth routes (authenticated via JWT but org not required — pre-onboarding users).
	authHandler := handler.NewAuthHandler(userSvc)
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/callback", authHandler.Callback)
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
