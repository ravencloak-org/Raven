package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

// ServerConfig holds configuration for the Asynq worker server.
type ServerConfig struct {
	RedisAddr   string
	Concurrency int
	MaxRetry    int
	Logger      *slog.Logger
}

// Server wraps an asynq.Server and its mux for processing tasks.
type Server struct {
	inner  *asynq.Server
	mux    *asynq.ServeMux
	logger *slog.Logger
}

// errorHandler implements asynq.ErrorHandler to log task failures.
type errorHandler struct {
	logger *slog.Logger
}

func (h *errorHandler) HandleError(_ context.Context, task *asynq.Task, err error) {
	h.logger.Error("task failed",
		"type", task.Type(),
		"error", err,
	)
}

// NewServer creates a new Asynq worker server with the given configuration.
// Handlers are stubs that log the task payload; real processing will be added
// in subsequent issues (#14-#17).
func NewServer(cfg ServerConfig) *Server {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.MaxRetry <= 0 {
		cfg.MaxRetry = 5
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			RetryDelayFunc: asynq.DefaultRetryDelayFunc,
			ErrorHandler:   &errorHandler{logger: cfg.Logger},
		},
	)

	mux := asynq.NewServeMux()
	s := &Server{
		inner:  srv,
		mux:    mux,
		logger: cfg.Logger,
	}

	// Register stub handlers for all task types.
	mux.HandleFunc(TypeDocumentProcess, s.handleDocumentProcess)
	mux.HandleFunc(TypeURLScrape, s.handleURLScrape)
	mux.HandleFunc(TypeReindex, s.handleReindex)

	return s
}

// Mux returns the underlying ServeMux so callers can register additional handlers
// or replace stubs with real implementations.
func (s *Server) Mux() *asynq.ServeMux {
	return s.mux
}

// Start begins processing tasks. This call blocks until the server is shut down.
func (s *Server) Start() error {
	s.logger.Info("starting asynq worker server")
	return s.inner.Start(s.mux)
}

// Shutdown gracefully stops the worker server.
func (s *Server) Shutdown() {
	s.logger.Info("shutting down asynq worker server")
	s.inner.Shutdown()
}

// ── Stub handlers ───────────────────────────────────────────────────────────

func (s *Server) handleDocumentProcess(_ context.Context, task *asynq.Task) error {
	var p DocumentProcessPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal DocumentProcessPayload: %w", err)
	}
	s.logger.Info("stub: processing document",
		"org_id", p.OrgID,
		"document_id", p.DocumentID,
		"knowledge_base_id", p.KnowledgeBaseID,
	)
	return nil
}

func (s *Server) handleURLScrape(_ context.Context, task *asynq.Task) error {
	var p URLScrapePayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal URLScrapePayload: %w", err)
	}
	s.logger.Info("stub: scraping URL",
		"org_id", p.OrgID,
		"source_id", p.SourceID,
		"knowledge_base_id", p.KnowledgeBaseID,
		"url", p.URL,
		"crawl_depth", p.CrawlDepth,
	)
	return nil
}

func (s *Server) handleReindex(_ context.Context, task *asynq.Task) error {
	var p ReindexPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal ReindexPayload: %w", err)
	}
	s.logger.Info("stub: reindexing knowledge base",
		"org_id", p.OrgID,
		"knowledge_base_id", p.KnowledgeBaseID,
	)
	return nil
}
