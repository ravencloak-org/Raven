package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"github.com/ravencloak-org/Raven/internal/model"
)

// ChunkRepository handles database operations for chunks and embeddings.
// All operations accept a pgx.Tx with org_id set for RLS enforcement.
type ChunkRepository struct {
	pool *pgxpool.Pool
}

// NewChunkRepository creates a new ChunkRepository backed by pool.
func NewChunkRepository(pool *pgxpool.Pool) *ChunkRepository {
	return &ChunkRepository{pool: pool}
}

const chunkColumns = `id, org_id, knowledge_base_id, document_id, source_id,
	content, chunk_index, token_count, page_number, heading,
	chunk_type, COALESCE(metadata, '{}') AS metadata, created_at`

func scanChunk(row pgx.Row) (*model.Chunk, error) {
	var c model.Chunk
	err := row.Scan(
		&c.ID,
		&c.OrgID,
		&c.KnowledgeBaseID,
		&c.DocumentID,
		&c.SourceID,
		&c.Content,
		&c.ChunkIndex,
		&c.TokenCount,
		&c.PageNumber,
		&c.Heading,
		&c.ChunkType,
		&c.Metadata,
		&c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func scanChunks(rows pgx.Rows) ([]model.Chunk, error) {
	var chunks []model.Chunk
	for rows.Next() {
		var c model.Chunk
		if err := rows.Scan(
			&c.ID,
			&c.OrgID,
			&c.KnowledgeBaseID,
			&c.DocumentID,
			&c.SourceID,
			&c.Content,
			&c.ChunkIndex,
			&c.TokenCount,
			&c.PageNumber,
			&c.Heading,
			&c.ChunkType,
			&c.Metadata,
			&c.CreatedAt,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

const embeddingColumns = `id, org_id, chunk_id, embedding, model_name, model_version, dimensions, created_at`

func scanEmbedding(row pgx.Row) (*model.Embedding, error) {
	var e model.Embedding
	err := row.Scan(
		&e.ID,
		&e.OrgID,
		&e.ChunkID,
		&e.Embedding,
		&e.ModelName,
		&e.ModelVersion,
		&e.Dimensions,
		&e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// CreateChunk inserts a new chunk and returns the persisted record.
func (r *ChunkRepository) CreateChunk(ctx context.Context, tx pgx.Tx, chunk *model.Chunk) (*model.Chunk, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO chunks (org_id, knowledge_base_id, document_id, source_id,
			content, chunk_index, token_count, page_number, heading, chunk_type, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING `+chunkColumns,
		chunk.OrgID,
		chunk.KnowledgeBaseID,
		chunk.DocumentID,
		chunk.SourceID,
		chunk.Content,
		chunk.ChunkIndex,
		chunk.TokenCount,
		chunk.PageNumber,
		chunk.Heading,
		chunk.ChunkType,
		chunk.Metadata,
	)
	c, err := scanChunk(row)
	if err != nil {
		return nil, fmt.Errorf("ChunkRepository.CreateChunk: %w", err)
	}
	return c, nil
}

// CreateEmbedding inserts a new embedding and returns the persisted record.
func (r *ChunkRepository) CreateEmbedding(ctx context.Context, tx pgx.Tx, embedding *model.Embedding) (*model.Embedding, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO embeddings (org_id, chunk_id, embedding, model_name, model_version, dimensions)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+embeddingColumns,
		embedding.OrgID,
		embedding.ChunkID,
		pgvector.NewVector(embedding.Embedding.Slice()),
		embedding.ModelName,
		embedding.ModelVersion,
		embedding.Dimensions,
	)
	e, err := scanEmbedding(row)
	if err != nil {
		return nil, fmt.Errorf("ChunkRepository.CreateEmbedding: %w", err)
	}
	return e, nil
}

// GetChunksByDocument returns all chunks for a given document, ordered by chunk index.
func (r *ChunkRepository) GetChunksByDocument(ctx context.Context, tx pgx.Tx, orgID, docID string) ([]model.Chunk, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+chunkColumns+`
		 FROM chunks
		 WHERE org_id = $1 AND document_id = $2
		 ORDER BY chunk_index`,
		orgID, docID,
	)
	if err != nil {
		return nil, fmt.Errorf("ChunkRepository.GetChunksByDocument: %w", err)
	}
	defer rows.Close()

	chunks, err := scanChunks(rows)
	if err != nil {
		return nil, fmt.Errorf("ChunkRepository.GetChunksByDocument scan: %w", err)
	}
	return chunks, nil
}

// GetChunksBySource returns all chunks for a given source, ordered by chunk index.
func (r *ChunkRepository) GetChunksBySource(ctx context.Context, tx pgx.Tx, orgID, sourceID string) ([]model.Chunk, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+chunkColumns+`
		 FROM chunks
		 WHERE org_id = $1 AND source_id = $2
		 ORDER BY chunk_index`,
		orgID, sourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("ChunkRepository.GetChunksBySource: %w", err)
	}
	defer rows.Close()

	chunks, err := scanChunks(rows)
	if err != nil {
		return nil, fmt.Errorf("ChunkRepository.GetChunksBySource scan: %w", err)
	}
	return chunks, nil
}

// DeleteChunksByDocument removes all chunks (and their cascaded embeddings) for a document.
func (r *ChunkRepository) DeleteChunksByDocument(ctx context.Context, tx pgx.Tx, orgID, docID string) error {
	_, err := tx.Exec(ctx,
		`DELETE FROM chunks WHERE org_id = $1 AND document_id = $2`,
		orgID, docID,
	)
	if err != nil {
		return fmt.Errorf("ChunkRepository.DeleteChunksByDocument: %w", err)
	}
	return nil
}

// DeleteChunksBySource removes all chunks (and their cascaded embeddings) for a source.
func (r *ChunkRepository) DeleteChunksBySource(ctx context.Context, tx pgx.Tx, orgID, sourceID string) error {
	_, err := tx.Exec(ctx,
		`DELETE FROM chunks WHERE org_id = $1 AND source_id = $2`,
		orgID, sourceID,
	)
	if err != nil {
		return fmt.Errorf("ChunkRepository.DeleteChunksBySource: %w", err)
	}
	return nil
}

// SearchByVector performs a cosine similarity search against the embeddings table
// using the HNSW index. It returns up to `limit` chunks with their similarity scores,
// ordered by closest match first (lowest cosine distance = highest similarity).
func (r *ChunkRepository) SearchByVector(ctx context.Context, tx pgx.Tx, orgID, kbID string, vector []float32, limit int) ([]model.ChunkWithScore, error) {
	rows, err := tx.Query(ctx,
		`SELECT c.id, c.org_id, c.knowledge_base_id, c.document_id, c.source_id,
			c.content, c.chunk_index, c.token_count, c.page_number, c.heading,
			c.chunk_type, COALESCE(c.metadata, '{}') AS metadata, c.created_at,
			1 - (e.embedding <=> $3) AS score
		 FROM embeddings e
		 JOIN chunks c ON c.id = e.chunk_id
		 WHERE e.org_id = $1 AND c.knowledge_base_id = $2
		 ORDER BY e.embedding <=> $3
		 LIMIT $4`,
		orgID, kbID, pgvector.NewVector(vector), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ChunkRepository.SearchByVector: %w", err)
	}
	defer rows.Close()

	var results []model.ChunkWithScore
	for rows.Next() {
		var cws model.ChunkWithScore
		if err := rows.Scan(
			&cws.ID,
			&cws.OrgID,
			&cws.KnowledgeBaseID,
			&cws.DocumentID,
			&cws.SourceID,
			&cws.Content,
			&cws.ChunkIndex,
			&cws.TokenCount,
			&cws.PageNumber,
			&cws.Heading,
			&cws.ChunkType,
			&cws.Metadata,
			&cws.CreatedAt,
			&cws.Score,
		); err != nil {
			return nil, fmt.Errorf("ChunkRepository.SearchByVector scan: %w", err)
		}
		results = append(results, cws)
	}
	return results, rows.Err()
}
