package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/storage"
)

// MarkdownSection represents a single section extracted from a markdown document
// by splitting on heading boundaries.
type MarkdownSection struct {
	Heading string
	Content string
}

// SplitMarkdownByHeadings splits markdown content into sections based on ## headings.
// The first section (before or including the first heading) captures any preamble
// and the # title. Each subsequent ## heading starts a new section.
func SplitMarkdownByHeadings(content string) []MarkdownSection {
	lines := strings.Split(content, "\n")

	var sections []MarkdownSection
	var currentHeading string
	var currentLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			// Flush the current section if we have accumulated content.
			if len(currentLines) > 0 || currentHeading != "" {
				sections = append(sections, MarkdownSection{
					Heading: currentHeading,
					Content: strings.TrimSpace(strings.Join(currentLines, "\n")),
				})
			}
			currentHeading = strings.TrimPrefix(trimmed, "## ")
			currentLines = []string{line}
		} else {
			currentLines = append(currentLines, line)
		}
	}

	// Flush the last section.
	if len(currentLines) > 0 || currentHeading != "" {
		sections = append(sections, MarkdownSection{
			Heading: currentHeading,
			Content: strings.TrimSpace(strings.Join(currentLines, "\n")),
		})
	}

	return sections
}

// NewDocumentProcessHandler returns an asynq.HandlerFunc that processes a queued
// document: downloads markdown from SeaweedFS, splits it into chunks by heading,
// inserts each chunk into the DB, and marks the document as ready.
func NewDocumentProcessHandler(
	pool *pgxpool.Pool,
	docRepo *repository.DocumentRepository,
	chunkRepo *repository.ChunkRepository,
	store storage.Client,
	logger *slog.Logger,
) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var p queue.DocumentProcessPayload
		if err := json.Unmarshal(task.Payload(), &p); err != nil {
			return fmt.Errorf("unmarshal DocumentProcessPayload: %w", err)
		}

		logger.Info("processing document",
			"org_id", p.OrgID,
			"document_id", p.DocumentID,
			"knowledge_base_id", p.KnowledgeBaseID,
		)

		// Mark document as parsing.
		if err := updateDocStatus(ctx, pool, p.OrgID, p.DocumentID, docRepo, model.ProcessingStatusParsing, ""); err != nil {
			return fmt.Errorf("set status parsing: %w", err)
		}

		// Fetch the document record to get the storage path (SeaweedFS fid).
		var doc *model.Document
		err := db.WithOrgID(ctx, pool, p.OrgID, func(tx pgx.Tx) error {
			var e error
			doc, e = docRepo.GetByID(ctx, tx, p.OrgID, p.DocumentID)
			return e
		})
		if err != nil {
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, err)
			return fmt.Errorf("get document: %w", err)
		}

		if doc.StoragePath == "" {
			failErr := fmt.Errorf("document %s has no storage_path", p.DocumentID)
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, failErr)
			return fmt.Errorf("%w: %w", asynq.SkipRetry, failErr)
		}

		// Download file content from SeaweedFS.
		rc, err := store.Download(ctx, doc.StoragePath)
		if err != nil {
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, err)
			return fmt.Errorf("download from storage: %w", err)
		}
		defer func() { _ = rc.Close() }()

		raw, err := io.ReadAll(rc)
		if err != nil {
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, err)
			return fmt.Errorf("read file content: %w", err)
		}
		content := string(raw)

		// Mark as chunking.
		if err := updateDocStatus(ctx, pool, p.OrgID, p.DocumentID, docRepo, model.ProcessingStatusChunking, ""); err != nil {
			return fmt.Errorf("set status chunking: %w", err)
		}

		// Split markdown into sections by heading.
		sections := SplitMarkdownByHeadings(content)
		if len(sections) == 0 {
			// No content to chunk — mark as ready with zero chunks.
			if err := updateDocStatus(ctx, pool, p.OrgID, p.DocumentID, docRepo, model.ProcessingStatusReady, ""); err != nil {
				return fmt.Errorf("set status ready (empty): %w", err)
			}
			return nil
		}

		// Insert all chunks and update status in a single RLS transaction.
		err = db.WithOrgID(ctx, pool, p.OrgID, func(tx pgx.Tx) error {
			for i, section := range sections {
				if strings.TrimSpace(section.Content) == "" {
					continue
				}
				tokenCount := len(strings.Fields(section.Content))
				chunk := &model.Chunk{
					OrgID:           p.OrgID,
					KnowledgeBaseID: p.KnowledgeBaseID,
					DocumentID:      &p.DocumentID,
					Content:         section.Content,
					ChunkIndex:      i,
					TokenCount:      &tokenCount,
					ChunkType:       model.ChunkTypeText,
					Metadata:        map[string]any{},
				}
				if section.Heading != "" {
					chunk.Heading = &section.Heading
				}
				if _, err := chunkRepo.CreateChunk(ctx, tx, chunk); err != nil {
					return fmt.Errorf("create chunk %d: %w", i, err)
				}
			}

			// Update document status to ready within the same transaction.
			return docRepo.UpdateStatus(ctx, tx, p.OrgID, p.DocumentID, model.ProcessingStatusReady, "")
		})
		if err != nil {
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, err)
			return fmt.Errorf("insert chunks: %w", err)
		}

		logger.Info("document processed",
			"document_id", p.DocumentID,
			"chunks", len(sections),
		)

		return nil
	}
}

// updateDocStatus is a convenience wrapper that updates document processing status
// inside an RLS-scoped transaction.
func updateDocStatus(ctx context.Context, pool *pgxpool.Pool, orgID, docID string, docRepo *repository.DocumentRepository, status model.ProcessingStatus, errMsg string) error {
	return db.WithOrgID(ctx, pool, orgID, func(tx pgx.Tx) error {
		return docRepo.UpdateStatus(ctx, tx, orgID, docID, status, errMsg)
	})
}

// setFailed marks a document as failed, logging any secondary error.
func setFailed(ctx context.Context, pool *pgxpool.Pool, orgID, docID string, docRepo *repository.DocumentRepository, cause error) {
	if err := updateDocStatus(ctx, pool, orgID, docID, docRepo, model.ProcessingStatusFailed, cause.Error()); err != nil {
		slog.Default().Warn("failed to mark document as failed",
			"document_id", docID,
			"cause", cause.Error(),
			"error", err.Error(),
		)
	}
}
