package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
)

// AirbyteSyncHandler processes Airbyte connector sync tasks.
// In this initial implementation, the handler creates a sync history record,
// logs a placeholder message (real Airbyte API integration is a follow-up),
// and completes the sync run.
type AirbyteSyncHandler struct {
	pool   *pgxpool.Pool
	repo   *repository.AirbyteRepository
	logger *slog.Logger
}

// NewAirbyteSyncHandler creates a new AirbyteSyncHandler.
func NewAirbyteSyncHandler(pool *pgxpool.Pool, repo *repository.AirbyteRepository, logger *slog.Logger) *AirbyteSyncHandler {
	return &AirbyteSyncHandler{pool: pool, repo: repo, logger: logger}
}

// ProcessTask implements asynq.Handler for Airbyte sync tasks.
func (h *AirbyteSyncHandler) ProcessTask(_ context.Context, t *asynq.Task) error {
	var payload queue.AirbyteSyncPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal AirbyteSyncPayload: %w", err)
	}

	h.logger.Info("starting airbyte sync",
		"connector_id", payload.ConnectorID,
		"org_id", payload.OrgID,
		"kb_id", payload.KnowledgeBaseID,
	)

	ctx := context.Background()

	// Step 1: Create a sync history record (status: running).
	var syncRunID string
	err := db.WithOrgID(ctx, h.pool, payload.OrgID, func(tx pgx.Tx) error {
		run, err := h.repo.CreateSyncRun(ctx, tx, payload.ConnectorID, payload.OrgID)
		if err != nil {
			return err
		}
		syncRunID = run.ID
		return nil
	})
	if err != nil {
		return fmt.Errorf("create sync run: %w", err)
	}

	// Step 2: Call the Airbyte API to trigger a sync.
	// TODO(#111): Replace this placeholder with real Airbyte REST API calls.
	// For now, we simulate a successful sync with 0 records.
	h.logger.Info("placeholder: would call Airbyte API here",
		"connector_id", payload.ConnectorID,
		"sync_run_id", syncRunID,
	)

	// Step 3: In a real implementation, incoming records would be processed here:
	//   - Chunk text content
	//   - Compute chunk_hash (SHA-256 of content) for dedup
	//   - Upsert chunks with dedup on (org_id, knowledge_base_id, source_id, chunk_hash)

	// Step 4: Complete the sync run.
	err = db.WithOrgID(ctx, h.pool, payload.OrgID, func(tx pgx.Tx) error {
		return h.repo.CompleteSyncRun(ctx, tx, syncRunID, payload.OrgID, "completed", 0, 0, "")
	})
	if err != nil {
		return fmt.Errorf("complete sync run: %w", err)
	}

	h.logger.Info("airbyte sync completed",
		"connector_id", payload.ConnectorID,
		"sync_run_id", syncRunID,
	)

	return nil
}

// ComputeChunkHash computes a SHA-256 hash of the given content for dedup purposes.
// This is exported for use by the chunking pipeline when processing Airbyte records.
func ComputeChunkHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}
