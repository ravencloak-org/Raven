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
	"github.com/pgvector/pgvector-go"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/storage"
)

// EmbedFunc generates a vector embedding for a chunk's text. Returned
// as a function so the document-process job can stay decoupled from
// the AI-worker gRPC client — production wires
// `internal/grpc.Client.Worker().GetEmbedding`, tests pass a stub.
//
// `provider` is the LLM provider slug ("ollama", "openai", ...) used
// to route to the right embedding backend on the Python side; empty
// string lets the worker fall back to the org's default.
type EmbedFunc func(ctx context.Context, orgID, text, provider string) (embedding []float32, dimensions int, modelName string, err error)

// MarkdownSection represents a single section extracted from a markdown document
// by splitting on heading boundaries.
type MarkdownSection struct {
	Heading string
	Content string
}

// SplitMarkdownByHeadings splits markdown content into sections based on ## headings.
// The first section (before or including the first heading) captures any preamble
// and the # title. Each subsequent ## heading starts a new section.
//
// Sections are then passed through capChunkBySize so any single section larger
// than maxChunkRuneCount is broken into multiple smaller chunks on paragraph
// boundaries. Without this guard, a doc with a single huge body (typical of
// PDF text extraction and long-form markdown without ## headings) becomes one
// >75 KB chunk that the embedding model rejects with "input length exceeds
// the context length" — nomic-embed-text in particular caps at ~8K tokens.
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

	return capSectionSize(sections, maxChunkRuneCount, chunkOverlapRuneCount)
}

// Token-budget guard for the embedding step. nomic-embed-text caps at
// ~8K tokens. The rune→token ratio varies hard by content:
//   - clean English markdown: ~4 runes/token
//   - PDF-extracted text:     ~2 runes/token  (broken hyphenation,
//                              odd whitespace, OCR-like artefacts)
// Sizing for the worse case keeps the PDF path working too. 6000 runes
// → ~3K tokens for PDFs, ~1.5K tokens for clean markdown — comfortable
// margin under the 8K cap and still big enough that typical sections
// stay in one chunk.
//
// chunkOverlapRuneCount preserves a sliding window of context across
// split boundaries so the next chunk doesn't start mid-thought.
const (
	maxChunkRuneCount     = 6000
	chunkOverlapRuneCount = 400
)

// capSectionSize splits any section whose Content exceeds maxRunes into
// multiple smaller sections, preserving the parent's Heading on each
// piece. Splits on paragraph boundaries (blank lines) first; if a single
// paragraph is still too long, falls back to a rune-window split with
// `overlap` runes of repetition so semantic context survives the cut.
//
// Pure function — testable without DB or gRPC.
func capSectionSize(sections []MarkdownSection, maxRunes, overlap int) []MarkdownSection {
	if maxRunes <= 0 {
		return sections
	}
	out := make([]MarkdownSection, 0, len(sections))
	for _, s := range sections {
		if len([]rune(s.Content)) <= maxRunes {
			out = append(out, s)
			continue
		}
		for _, piece := range splitByParagraphsWithCap(s.Content, maxRunes, overlap) {
			out = append(out, MarkdownSection{Heading: s.Heading, Content: piece})
		}
	}
	return out
}

// splitByParagraphsWithCap walks paragraphs (separated by blank lines)
// and accumulates them into chunks of <= maxRunes runes. A paragraph
// that on its own exceeds maxRunes is hard-cut on rune boundaries with
// `overlap` runes of repetition between adjacent cuts.
func splitByParagraphsWithCap(text string, maxRunes, overlap int) []string {
	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	flush := func() {
		if current.Len() == 0 {
			return
		}
		chunks = append(chunks, strings.TrimSpace(current.String()))
		current.Reset()
	}

	for _, p := range paragraphs {
		p = strings.TrimRight(p, "\n")
		if p == "" {
			continue
		}
		pRunes := []rune(p)
		if len(pRunes) > maxRunes {
			// Flush whatever we'd accumulated before tackling the monster.
			flush()
			for i := 0; i < len(pRunes); {
				end := i + maxRunes
				if end > len(pRunes) {
					end = len(pRunes)
				}
				chunks = append(chunks, string(pRunes[i:end]))
				if end == len(pRunes) {
					break
				}
				// Slide forward, leaving `overlap` runes of repeat for
				// continuity. Guarantees forward progress when overlap
				// >= maxRunes by clamping below.
				step := maxRunes - overlap
				if step <= 0 {
					step = maxRunes / 2
					if step <= 0 {
						step = 1
					}
				}
				i += step
			}
			continue
		}
		// Would adding this paragraph blow the cap? Flush first.
		candidate := current.Len() + len("\n\n") + len(p)
		if candidate > maxRunes && current.Len() > 0 {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(p)
	}
	flush()
	return chunks
}

// WebhookDispatcher is the slice of WebhookService this job needs to fire
// outbound events. Defined as a local interface so the jobs package does not
// import service, and so tests can substitute a fake. A nil dispatcher is
// supported and means "do not emit webhooks" — useful for unit tests and for
// callers that have not finished wiring webhook delivery.
type WebhookDispatcher interface {
	Dispatch(ctx context.Context, orgID, eventType string, payload map[string]any) error
}

// dispatchAsync runs WebhookDispatcher.Dispatch in a goroutine using a
// detached context (cancellation severed, trace IDs preserved) so that the
// producer's success path never blocks on webhook delivery and never fails
// when delivery fails. Dispatch itself enqueues Asynq tasks, so the actual
// HTTP send is already async; this goroutine only covers the synchronous
// fan-out work (DB insert per delivery, one Asynq enqueue per webhook).
func dispatchAsync(ctx context.Context, d WebhookDispatcher, logger *slog.Logger, orgID, eventType string, payload map[string]any) {
	if d == nil {
		return
	}
	detached := context.WithoutCancel(ctx)
	go func() {
		if err := d.Dispatch(detached, orgID, eventType, payload); err != nil {
			logger.WarnContext(detached, "webhook dispatch failed",
				"event_type", eventType, "org_id", orgID, "error", err)
		}
	}()
}

// NewDocumentProcessHandler returns an asynq.HandlerFunc that processes a queued
// document: downloads markdown from SeaweedFS, splits it into chunks by heading,
// inserts each chunk into the DB, and marks the document as ready.
//
// If webhookDispatcher is non-nil, the handler fires a `document.processed`
// webhook event after the success path completes. Pass nil to disable webhook
// emission (e.g. in tests).
// embed may be nil — in which case chunks are written without
// embeddings and RAG retrieval falls back to BM25-only. Production
// always passes a non-nil EmbedFunc; tests rely on nil for the
// fast-path / chunk-only assertions.
func NewDocumentProcessHandler(
	pool *pgxpool.Pool,
	docRepo *repository.DocumentRepository,
	chunkRepo *repository.ChunkRepository,
	store storage.Client,
	embed EmbedFunc,
	logger *slog.Logger,
	webhookDispatcher WebhookDispatcher,
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
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, logger, err)
			return fmt.Errorf("get document: %w", err)
		}

		if doc.StoragePath == "" {
			failErr := fmt.Errorf("document %s has no storage_path", p.DocumentID)
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, logger, failErr)
			return fmt.Errorf("%w: %w", asynq.SkipRetry, failErr)
		}

		// Download file content from SeaweedFS.
		rc, err := store.Download(ctx, doc.StoragePath)
		if err != nil {
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, logger, err)
			return fmt.Errorf("download from storage: %w", err)
		}
		defer func() { _ = rc.Close() }()

		const maxDocSize = 10 << 20 // 10 MB
		raw, err := io.ReadAll(io.LimitReader(rc, maxDocSize))
		if err != nil {
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, logger, err)
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
			// Fire `document.processed` webhook (best-effort; never blocks the success path).
			dispatchAsync(ctx, webhookDispatcher, logger, p.OrgID,
				string(model.WebhookEventDocumentProcessed),
				map[string]any{
					"document_id":       p.DocumentID,
					"knowledge_base_id": p.KnowledgeBaseID,
					"status":            string(model.ProcessingStatusReady),
					"chunk_count":       0,
				})
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
				created, err := chunkRepo.CreateChunk(ctx, tx, chunk)
				if err != nil {
					return fmt.Errorf("create chunk %d: %w", i, err)
				}

				// Best-effort embed. A retrieval-empty document is still
				// useful via BM25, so we don't fail the whole job when
				// the embedding backend hiccups — log and move on. The
				// embedding gRPC call is per-chunk; with markdown-by-
				// heading splits the chunk count is small (< 50 for any
				// realistic doc) so the latency cost is bounded.
				if embed != nil {
					vec, dims, modelName, embedErr := embed(ctx, p.OrgID, created.Content, "")
					if embedErr != nil {
						logger.Warn("chunk embedding failed; falling back to BM25-only retrieval",
							"document_id", p.DocumentID,
							"chunk_index", created.ChunkIndex,
							"error", embedErr,
						)
					} else if len(vec) > 0 {
						emb := &model.Embedding{
							OrgID:      p.OrgID,
							ChunkID:    created.ID,
							Embedding:  pgvector.NewVector(vec),
							ModelName:  modelName,
							Dimensions: dims,
						}
						if _, err := chunkRepo.CreateEmbedding(ctx, tx, emb); err != nil {
							logger.Warn("persist embedding failed; chunk indexed without vector",
								"document_id", p.DocumentID,
								"chunk_index", created.ChunkIndex,
								"error", err,
							)
						}
					}
				}
			}

			// Update document status to ready within the same transaction.
			return docRepo.UpdateStatus(ctx, tx, p.OrgID, p.DocumentID, model.ProcessingStatusReady, "")
		})
		if err != nil {
			setFailed(ctx, pool, p.OrgID, p.DocumentID, docRepo, logger, err)
			return fmt.Errorf("insert chunks: %w", err)
		}

		logger.Info("document processed",
			"document_id", p.DocumentID,
			"chunks", len(sections),
		)

		// Fire `document.processed` webhook (best-effort; never blocks the success path).
		dispatchAsync(ctx, webhookDispatcher, logger, p.OrgID,
			string(model.WebhookEventDocumentProcessed),
			map[string]any{
				"document_id":       p.DocumentID,
				"knowledge_base_id": p.KnowledgeBaseID,
				"status":            string(model.ProcessingStatusReady),
				"chunk_count":       len(sections),
			})

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
func setFailed(ctx context.Context, pool *pgxpool.Pool, orgID, docID string, docRepo *repository.DocumentRepository, logger *slog.Logger, cause error) {
	if err := updateDocStatus(ctx, pool, orgID, docID, docRepo, model.ProcessingStatusFailed, cause.Error()); err != nil {
		logger.Warn("failed to mark document as failed",
			"document_id", docID,
			"cause", cause.Error(),
			"error", err.Error(),
		)
	}
}
