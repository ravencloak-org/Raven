package repository

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ravencloak-org/Raven/internal/model"
)

// ClickHouseEmbeddingRepository stores and queries vector embeddings in
// ClickHouse using QBit columns for efficient similarity search without
// in-memory HNSW indexes.
type ClickHouseEmbeddingRepository struct {
	conn driver.Conn
}

// NewClickHouseEmbeddingRepository creates a new repository backed by a
// ClickHouse connection. The connection must already be established.
func NewClickHouseEmbeddingRepository(conn driver.Conn) *ClickHouseEmbeddingRepository {
	return &ClickHouseEmbeddingRepository{conn: conn}
}

// EnsureSchema creates the embeddings table if it does not already exist.
// The table uses MergeTree engine partitioned by org_id for tenant isolation.
// The embedding column uses Array(Float32) with a QBit index for
// bit-plane vector search.
func (r *ClickHouseEmbeddingRepository) EnsureSchema(ctx context.Context) error {
	ddl := `
		CREATE TABLE IF NOT EXISTS embeddings (
			org_id       UUID,
			kb_id        UUID,
			chunk_id     UUID,
			embedding    Array(Float32),
			model_name   String DEFAULT '',
			created_at   DateTime64(3) DEFAULT now64(3),
			INDEX idx_embedding embedding TYPE qbit GRANULARITY 1
		) ENGINE = MergeTree()
		PARTITION BY org_id
		ORDER BY (org_id, kb_id, chunk_id)
	`
	if err := r.conn.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("ClickHouseEmbeddingRepository.EnsureSchema: %w", err)
	}
	return nil
}

// StoreEmbedding inserts a single embedding vector into ClickHouse.
func (r *ClickHouseEmbeddingRepository) StoreEmbedding(ctx context.Context, orgID, kbID, chunkID string, embedding []float32, modelName string) error {
	sql := `INSERT INTO embeddings (org_id, kb_id, chunk_id, embedding, model_name) VALUES ($1, $2, $3, $4, $5)`
	if err := r.conn.Exec(ctx, sql, orgID, kbID, chunkID, embedding, modelName); err != nil {
		return fmt.Errorf("ClickHouseEmbeddingRepository.StoreEmbedding: %w", err)
	}
	return nil
}

// StoreBatch inserts multiple embeddings in a single batch operation.
func (r *ClickHouseEmbeddingRepository) StoreBatch(ctx context.Context, records []model.ClickHouseEmbedding) error {
	if len(records) == 0 {
		return nil
	}
	batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO embeddings (org_id, kb_id, chunk_id, embedding, model_name)`)
	if err != nil {
		return fmt.Errorf("ClickHouseEmbeddingRepository.StoreBatch prepare: %w", err)
	}
	for _, rec := range records {
		if err := batch.Append(rec.OrgID, rec.KBID, rec.ChunkID, rec.Embedding, rec.ModelName); err != nil {
			return fmt.Errorf("ClickHouseEmbeddingRepository.StoreBatch append: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("ClickHouseEmbeddingRepository.StoreBatch send: %w", err)
	}
	return nil
}

// SearchSimilar performs a QBit-based cosine distance search, returning the
// topK most similar chunks. QBit indexes allow tunable precision at query
// time without needing an in-memory HNSW graph.
func (r *ClickHouseEmbeddingRepository) SearchSimilar(ctx context.Context, orgID, kbID string, queryEmbedding []float32, topK int) ([]model.ClickHouseSearchResult, error) {
	sql := `
		SELECT
			chunk_id,
			cosineDistance(embedding, $1) AS distance
		FROM embeddings
		WHERE org_id = $2 AND kb_id = $3
		ORDER BY distance ASC
		LIMIT $4
	`
	rows, err := r.conn.Query(ctx, sql, queryEmbedding, orgID, kbID, topK)
	if err != nil {
		return nil, fmt.Errorf("ClickHouseEmbeddingRepository.SearchSimilar: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close error is non-actionable

	var results []model.ClickHouseSearchResult
	for rows.Next() {
		var res model.ClickHouseSearchResult
		if err := rows.Scan(&res.ChunkID, &res.Distance); err != nil {
			return nil, fmt.Errorf("ClickHouseEmbeddingRepository.SearchSimilar scan: %w", err)
		}
		res.Score = 1 - res.Distance // cosine similarity = 1 - cosine distance
		results = append(results, res)
	}
	return results, rows.Err()
}

// DeleteByKB removes all embeddings for a knowledge base within an org.
func (r *ClickHouseEmbeddingRepository) DeleteByKB(ctx context.Context, orgID, kbID string) error {
	sql := `ALTER TABLE embeddings DELETE WHERE org_id = $1 AND kb_id = $2`
	if err := r.conn.Exec(ctx, sql, orgID, kbID); err != nil {
		return fmt.Errorf("ClickHouseEmbeddingRepository.DeleteByKB: %w", err)
	}
	return nil
}

// DeleteByChunk removes a specific chunk's embedding.
func (r *ClickHouseEmbeddingRepository) DeleteByChunk(ctx context.Context, orgID, kbID, chunkID string) error {
	sql := `ALTER TABLE embeddings DELETE WHERE org_id = $1 AND kb_id = $2 AND chunk_id = $3`
	if err := r.conn.Exec(ctx, sql, orgID, kbID, chunkID); err != nil {
		return fmt.Errorf("ClickHouseEmbeddingRepository.DeleteByChunk: %w", err)
	}
	return nil
}

// CountByOrg returns the total number of embeddings stored for an organisation.
func (r *ClickHouseEmbeddingRepository) CountByOrg(ctx context.Context, orgID string) (int64, error) {
	var count int64
	err := r.conn.QueryRow(ctx, `SELECT count() FROM embeddings WHERE org_id = $1`, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ClickHouseEmbeddingRepository.CountByOrg: %w", err)
	}
	return count, nil
}
