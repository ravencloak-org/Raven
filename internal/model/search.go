package model

import "time"

// ChunkWithRank is a chunk paired with its full-text search relevance rank.
type ChunkWithRank struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	DocumentID      *string   `json:"document_id,omitempty"`
	SourceID        *string   `json:"source_id,omitempty"`
	Content         string    `json:"content"`
	ChunkIndex      int       `json:"chunk_index"`
	TokenCount      *int      `json:"token_count,omitempty"`
	PageNumber      *int      `json:"page_number,omitempty"`
	Heading         *string   `json:"heading,omitempty"`
	ChunkType       string    `json:"chunk_type"`
	CreatedAt       time.Time `json:"created_at"`
	Rank            float64   `json:"rank"`
	Highlight       string    `json:"highlight,omitempty"`
}

// SearchRequest is the validated input for a full-text search.
type SearchRequest struct {
	Query  string   `json:"q"`
	Limit  int      `json:"limit"`
	DocIDs []string `json:"doc_ids,omitempty"`
}

// SearchResponse wraps a list of ranked chunks for the API response.
type SearchResponse struct {
	Results []ChunkWithRank `json:"results"`
	Total   int             `json:"total"`
}

// HybridSearchRequest is the validated input for a hybrid (vector + BM25) search.
//
// It models the JSON body posted to
// POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/hybrid-search.
//
// `Embedding` is the pre-computed query embedding the vector leg searches
// against. Raven's API service does not embed text itself today (the
// production RAG pipeline embeds inside the Python `ai-worker`), so callers
// that want a true hybrid response must supply this field. When `Embedding`
// is empty, the vector leg is skipped and the response degrades to a
// BM25-only ranking — this is intentional and lets keyword-only callers
// reuse the same route. See `internal/service/search.go`'s `HybridSearch`.
type HybridSearchRequest struct {
	Query     string            `json:"query"`
	TopK      int               `json:"top_k,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"`
	DocIDs    []string          `json:"doc_ids,omitempty"`
	Embedding []float32         `json:"embedding,omitempty"`
}

// HybridSearchHTTPResponse wraps a list of hybrid search results with the
// effective parameters the server actually used. Distinct from
// HybridSearchResponse because the HTTP surface echoes back the query and
// the clamped top_k so clients can detect server-side adjustments.
type HybridSearchHTTPResponse struct {
	Results []HybridSearchResult `json:"results"`
	Query   string               `json:"query"`
	TopK    int                  `json:"top_k"`
}

// HybridSearchResult is a single result from a hybrid search with its fused score.
type HybridSearchResult struct {
	ChunkID         string    `json:"chunk_id"`
	OrgID           string    `json:"org_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	DocumentID      *string   `json:"document_id,omitempty"`
	SourceID        *string   `json:"source_id,omitempty"`
	Content         string    `json:"content"`
	ChunkIndex      int       `json:"chunk_index"`
	TokenCount      *int      `json:"token_count,omitempty"`
	PageNumber      *int      `json:"page_number,omitempty"`
	Heading         *string   `json:"heading,omitempty"`
	ChunkType       string    `json:"chunk_type"`
	CreatedAt       time.Time `json:"created_at"`
	VectorScore     float64   `json:"vector_score,omitempty"`  // Cosine similarity score (0-1)
	BM25Score       float64   `json:"bm25_score,omitempty"`    // BM25 relevance score
	RRFScore        float64   `json:"rrf_score"`               // Fused Reciprocal Rank Fusion score
	VectorRank      int       `json:"vector_rank,omitempty"`   // Rank position in vector results
	BM25Rank        int       `json:"bm25_rank,omitempty"`     // Rank position in BM25 results
}

// HybridSearchResponse wraps a list of hybrid search results for the API response.
type HybridSearchResponse struct {
	Results []HybridSearchResult `json:"results"`
	Total   int                  `json:"total"`
}
