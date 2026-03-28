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
type HybridSearchRequest struct {
	Query string `json:"q"`
	KBID  string `json:"kb_id"`
	TopK  int    `json:"top_k"`
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
