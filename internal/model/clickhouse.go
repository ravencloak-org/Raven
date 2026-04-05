package model

// ClickHouseEmbedding represents an embedding record stored in ClickHouse.
type ClickHouseEmbedding struct {
	OrgID     string    `json:"org_id"`
	KBID      string    `json:"kb_id"`
	ChunkID   string    `json:"chunk_id"`
	Embedding []float32 `json:"embedding"`
	ModelName string    `json:"model_name"`
}

// ClickHouseSearchResult is a single result from a ClickHouse QBit vector search.
type ClickHouseSearchResult struct {
	ChunkID  string  `json:"chunk_id"`
	Distance float64 `json:"distance"` // cosine distance (0 = identical)
	Score    float64 `json:"score"`    // cosine similarity (1 - distance)
}

// VectorBackend identifies which vector storage backend to use for an org.
type VectorBackend string

const (
	// VectorBackendPgvector uses PostgreSQL pgvector with HNSW indexes (default).
	VectorBackendPgvector VectorBackend = "pgvector"
	// VectorBackendClickHouse uses ClickHouse QBit columns (enterprise).
	VectorBackendClickHouse VectorBackend = "clickhouse"
)
