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
	file_size_bytes, COALESCE(file_hash, '') AS file_hash,
	COALESCE(storage_path, '') AS storage_path,
	processing_status, COALESCE(processing_error, '') AS processing_error,
	COALESCE(title, '') AS title, page_count,
	COALESCE(metadata, '{}') AS metadata,
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

// Create inserts a new document record.
func (r *DocumentRepository) Create(ctx context.Context, tx pgx.Tx, doc *model.Document) (*model.Document, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO documents (org_id, knowledge_base_id, file_name, file_type,
			file_size_bytes, file_hash, storage_path, title, metadata, uploaded_by)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''),
			NULLIF($7, ''), NULLIF($8, ''), COALESCE($9::jsonb, '{}'), NULLIF($10, '')::uuid)
		 RETURNING `+docColumns,
		doc.OrgID, doc.KnowledgeBaseID, doc.FileName, doc.FileType,
		doc.FileSizeBytes, doc.FileHash, doc.StoragePath, doc.Title,
		doc.Metadata, doc.UploadedBy,
	)
	created, err := scanDocument(row)
	if err != nil {
		return nil, fmt.Errorf("DocumentRepository.Create: %w", err)
	}
	return created, nil
}

// GetByID fetches a document by its primary key within an org.
func (r *DocumentRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, docID string) (*model.Document, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+docColumns+`
		 FROM documents
		 WHERE id = $1 AND org_id = $2`,
		docID, orgID,
	)
	doc, err := scanDocument(row)
	if err != nil {
		return nil, fmt.Errorf("DocumentRepository.GetByID: %w", err)
	}
	return doc, nil
}

// List returns paginated documents for a knowledge base within an org.
func (r *DocumentRepository) List(ctx context.Context, tx pgx.Tx, orgID, kbID string, page, pageSize int) ([]model.Document, int, error) {
	var total int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM documents WHERE org_id = $1 AND knowledge_base_id = $2`,
		orgID, kbID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("DocumentRepository.List count: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := tx.Query(ctx,
		`SELECT `+docColumns+`
		 FROM documents
		 WHERE org_id = $1 AND knowledge_base_id = $2
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4`,
		orgID, kbID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("DocumentRepository.List query: %w", err)
	}
	defer rows.Close()

	var docs []model.Document
	for rows.Next() {
		var doc model.Document
		if err := rows.Scan(
			&doc.ID, &doc.OrgID, &doc.KnowledgeBaseID, &doc.FileName,
			&doc.FileType, &doc.FileSizeBytes, &doc.FileHash, &doc.StoragePath,
			&doc.ProcessingStatus, &doc.ProcessingError, &doc.Title,
			&doc.PageCount, &doc.Metadata, &doc.UploadedBy,
			&doc.CreatedAt, &doc.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("DocumentRepository.List scan: %w", err)
		}
		docs = append(docs, doc)
	}
	return docs, total, rows.Err()
}

// Update applies partial updates to a document's metadata fields.
func (r *DocumentRepository) Update(ctx context.Context, tx pgx.Tx, orgID, docID string, title *string, metadata map[string]any) (*model.Document, error) {
	row := tx.QueryRow(ctx,
		`UPDATE documents
		 SET
		   title    = COALESCE($3, title),
		   metadata = CASE WHEN $4::jsonb IS NOT NULL THEN $4::jsonb ELSE metadata END,
		   updated_at = NOW()
		 WHERE id = $1 AND org_id = $2
		 RETURNING `+docColumns,
		docID, orgID, title, metadata,
	)
	doc, err := scanDocument(row)
	if err != nil {
		return nil, fmt.Errorf("DocumentRepository.Update: %w", err)
	}
	return doc, nil
}

// Delete hard-deletes a document by ID within an org.
func (r *DocumentRepository) Delete(ctx context.Context, tx pgx.Tx, orgID, docID string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM documents WHERE id = $1 AND org_id = $2`,
		docID, orgID,
	)
	if err != nil {
		return fmt.Errorf("DocumentRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("DocumentRepository.Delete: document %s not found", docID)
	}
	return nil
}

// UpdateStatus updates the processing status (and optional error message) of a document.
func (r *DocumentRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, orgID, docID string, status model.ProcessingStatus, errorMsg string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE documents
		 SET processing_status = $3,
		     processing_error  = NULLIF($4, ''),
		     updated_at        = NOW()
		 WHERE id = $1 AND org_id = $2`,
		docID, orgID, status, errorMsg,
	)
	if err != nil {
		return fmt.Errorf("DocumentRepository.UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("DocumentRepository.UpdateStatus: document %s not found", docID)
	}
	return nil
}
