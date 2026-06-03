package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/storage"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// normaliseContentType strips MIME parameters (charset, boundary, etc.)
// so the allow-list check works on the media type alone. Go's
// http.DetectContentType returns "text/plain; charset=utf-8" for most
// text-shaped files; without this strip step every plain-text / markdown
// upload 400s against the bare "text/plain" allow-list entry.
//
// Falls back to the raw value when ParseMediaType can't parse — better
// to surface a clear "not allowed" than to crash on a malformed header.
func normaliseContentType(raw string) string {
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return raw
	}
	return mediaType
}

// DocumentProcessEnqueuer is the narrow slice of the queue client this
// service needs. Kept as an interface so the unit tests can pass a stub
// without standing up a real Valkey.
type DocumentProcessEnqueuer interface {
	EnqueueDocumentProcess(ctx context.Context, p queue.DocumentProcessPayload) error
}

// UploadService contains business logic for document uploads.
type UploadService struct {
	repo         *repository.DocumentRepository
	pool         *pgxpool.Pool
	store        storage.Client
	queueCli     DocumentProcessEnqueuer
	maxSizeBytes int64
	allowedTypes map[string]bool
	// kbGuard enforces KBStatusGate freeze semantics on the ingest path
	// (ADR-0004 / issue #725). Optional: nil means unit tests that focus
	// on file-type / size validation don't need a real KB fixture.
	kbGuard *KBStatusGuard
}

// NewUploadService creates a new UploadService.
//
// queueCli may be nil — in that case the service writes the document row
// with status=queued but skips the background enqueue. Production wiring
// in cmd/api/main.go always supplies a real client; nil is reserved for
// tests and degraded modes where the queue can't be reached.
func NewUploadService(
	repo *repository.DocumentRepository,
	pool *pgxpool.Pool,
	store storage.Client,
	queueCli DocumentProcessEnqueuer,
	maxSizeBytes int64,
	allowedTypes []string,
) *UploadService {
	allowed := lo.SliceToMap(allowedTypes, func(t string) (string, bool) {
		return strings.ToLower(t), true
	})
	return &UploadService{
		repo:         repo,
		pool:         pool,
		store:        store,
		queueCli:     queueCli,
		maxSizeBytes: maxSizeBytes,
		allowedTypes: allowed,
	}
}

// WithKBStatusGuard wires the lifecycle freeze gate so uploads against a
// read_only_private / dmca_pending KB return 409 / 423 instead of writing
// a row the user can't subsequently act on. Chainable at construction.
func (s *UploadService) WithKBStatusGuard(g *KBStatusGuard) *UploadService {
	s.kbGuard = g
	return s
}

// UploadParams holds the parameters for a document upload.
type UploadParams struct {
	OrgID           string
	KnowledgeBaseID string
	FileName        string
	FileType        string
	FileSizeBytes   int64
	UploadedBy      string
	Reader          io.Reader
}

// Upload validates, deduplicates, stores, and records a new document upload.
func (s *UploadService) Upload(ctx context.Context, params UploadParams) (*model.Document, error) {
	// Lifecycle gate (issue #725). Run before any expensive work so a
	// frozen KB rejects the request before we read the file into memory.
	if err := s.kbGuard.Require(ctx, params.OrgID, params.KnowledgeBaseID, KBActionIngest); err != nil {
		return nil, err
	}

	// Validate file type. Normalise away MIME parameters (charset etc.) so
	// "text/plain; charset=utf-8" matches the bare "text/plain" entry in
	// the allow-list. The user-facing error retains the original header
	// so a misconfigured client can see what was actually rejected.
	if !s.allowedTypes[strings.ToLower(normaliseContentType(params.FileType))] {
		return nil, apierror.NewBadRequest(fmt.Sprintf("file type not allowed: %s", params.FileType))
	}

	// Validate file size.
	if params.FileSizeBytes > s.maxSizeBytes {
		return nil, apierror.NewBadRequest(fmt.Sprintf(
			"file too large: %d bytes exceeds maximum %d bytes",
			params.FileSizeBytes, s.maxSizeBytes,
		))
	}

	// Read entire file to compute hash; buffer in memory.
	// For very large files we would stream to disk, but at 50 MB max this is fine.
	data, err := io.ReadAll(params.Reader)
	if err != nil {
		return nil, apierror.NewInternal("failed to read uploaded file: " + err.Error())
	}

	// Compute SHA-256 hash.
	hash := sha256.Sum256(data)
	fileHash := hex.EncodeToString(hash[:])

	// Check for duplicates within the same KB.
	var existingDoc *model.Document
	err = db.WithOrgID(ctx, s.pool, params.OrgID, func(tx pgx.Tx) error {
		var findErr error
		existingDoc, findErr = s.repo.FindByHash(ctx, tx, params.OrgID, params.KnowledgeBaseID, fileHash)
		return findErr
	})
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return nil, apierror.NewInternal("failed to check for duplicate: " + err.Error())
	}
	if existingDoc != nil {
		return nil, &apierror.AppError{
			Code:    409,
			Message: "Conflict",
			Detail:  fmt.Sprintf("duplicate file: document %s has the same content (hash: %s)", existingDoc.ID, fileHash),
		}
	}

	// Upload to storage.
	fid, err := s.store.Upload(ctx, params.FileName, strings.NewReader(string(data)))
	if err != nil {
		return nil, apierror.NewInternal("failed to upload to storage: " + err.Error())
	}

	// Create document record in DB.
	fileSizeBytes := params.FileSizeBytes
	newDoc := &model.Document{
		OrgID:            params.OrgID,
		KnowledgeBaseID:  params.KnowledgeBaseID,
		FileName:         params.FileName,
		FileType:         params.FileType,
		FileSizeBytes:    &fileSizeBytes,
		FileHash:         fileHash,
		StoragePath:      fid,
		ProcessingStatus: model.ProcessingStatusQueued,
		UploadedBy:       params.UploadedBy,
	}
	var doc *model.Document
	err = db.WithOrgID(ctx, s.pool, params.OrgID, func(tx pgx.Tx) error {
		var createErr error
		doc, createErr = s.repo.Create(ctx, tx, newDoc)
		return createErr
	})
	if err != nil {
		// Attempt to clean up the stored file on DB failure.
		_ = s.store.Delete(ctx, fid)
		return nil, apierror.NewInternal("failed to create document record: " + err.Error())
	}

	// Hand the document off to the Asynq worker for parsing → chunking →
	// embedding. We log + continue on enqueue failure rather than rolling
	// back the upload: the row is already in the DB with status=queued, so
	// an operator (or a future reaper job) can re-enqueue it. Failing the
	// HTTP request after the file has been stored would be more confusing
	// for the user than a delayed processing run.
	if s.queueCli != nil {
		if enqErr := s.queueCli.EnqueueDocumentProcess(ctx, queue.DocumentProcessPayload{
			DocumentID:      doc.ID,
			OrgID:           doc.OrgID,
			KnowledgeBaseID: doc.KnowledgeBaseID,
		}); enqErr != nil {
			slog.Default().Error("upload: enqueue document process failed",
				"document_id", doc.ID,
				"org_id", doc.OrgID,
				"kb_id", doc.KnowledgeBaseID,
				"error", enqErr.Error(),
			)
		}
	}

	return doc, nil
}
