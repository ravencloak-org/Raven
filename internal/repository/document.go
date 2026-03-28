package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// DocumentRepository handles database operations for documents.
// All operations use a pgx.Tx with org_id set for RLS enforcement.
type DocumentRepository struct {
	pool *pgxpool.Pool
}

// NewDocumentRepository creates a new DocumentRepository.
func NewDocumentRepository(pool *pgxpool.Pool) *DocumentRepository {
	return &DocumentRepository{pool: pool}
}

const docColumns = `id, org_id, knowledge_base_id, file_name,
	COALESCE(file_type, '') AS file_type,
	COALESCE(file_size_bytes, 0) AS file_size_bytes,
	COALESCE(file_hash, '') AS file_hash,
	COALESCE(storage_path, '') AS storage_path,
	processing_status,
	COALESCE(processing_error, '') AS processing_error,
	COALESCE(title, '') AS title,
	page_count,
	metadata,
	COALESCE(uploaded_by::text, '') AS uploaded_by,
	created_at, updated_at`

func scanDocument(row pgx.Row) (*model.Document, error) {
	var doc model.Document
	err := row.Scan(
		&doc.ID,
		&doc.OrgID,
		&doc.KnowledgeBaseID,
		&doc.FileName,
		&doc.FileType,
		&doc.FileSizeBytes,
		&doc.FileHash,
		&doc.StoragePath,
		&doc.ProcessingStatus,
		&doc.ProcessingError,
		&doc.Title,
		&doc.PageCount,
		&doc.Metadata,
		&doc.UploadedBy,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// Create inserts a new document record and returns the persisted row.
func (r *DocumentRepository) Create(ctx context.Context, tx pgx.Tx, orgID, kbID, fileName, fileType string, fileSizeBytes int64, fileHash, storagePath, uploadedBy string) (*model.Document, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO documents (org_id, knowledge_base_id, file_name, file_type, file_size_bytes, file_hash, storage_path, uploaded_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, '')::uuid)
		 RETURNING `+docColumns,
		orgID, kbID, fileName, fileType, fileSizeBytes, fileHash, storagePath, uploadedBy,
	)
	doc, err := scanDocument(row)
	if err != nil {
		return nil, fmt.Errorf("DocumentRepository.Create: %w", err)
	}
	return doc, nil
}

// FindByHash returns a document with the given hash in the specified knowledge base, or nil if none exists.
func (r *DocumentRepository) FindByHash(ctx context.Context, tx pgx.Tx, orgID, kbID, fileHash string) (*model.Document, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+docColumns+`
		 FROM documents
		 WHERE org_id = $1 AND knowledge_base_id = $2 AND file_hash = $3
		 LIMIT 1`,
		orgID, kbID, fileHash,
	)
	doc, err := scanDocument(row)
	if err != nil {
		return nil, err // may be pgx.ErrNoRows
	}
	return doc, nil
}
