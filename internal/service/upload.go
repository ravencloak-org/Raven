package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/storage"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// UploadService contains business logic for document uploads.
type UploadService struct {
	repo         *repository.DocumentRepository
	pool         *pgxpool.Pool
	store        storage.Client
	maxSizeBytes int64
	allowedTypes map[string]bool
}

// NewUploadService creates a new UploadService.
func NewUploadService(
	repo *repository.DocumentRepository,
	pool *pgxpool.Pool,
	store storage.Client,
	maxSizeBytes int64,
	allowedTypes []string,
) *UploadService {
	allowed := make(map[string]bool, len(allowedTypes))
	for _, t := range allowedTypes {
		allowed[strings.ToLower(t)] = true
	}
	return &UploadService{
		repo:         repo,
		pool:         pool,
		store:        store,
		maxSizeBytes: maxSizeBytes,
		allowedTypes: allowed,
	}
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
	// Validate file type.
	if !s.allowedTypes[strings.ToLower(params.FileType)] {
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

	return doc, nil
}
