package model

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

// ChunkType represents the content classification of a chunk.
type ChunkType string

// Valid ChunkType values.
const (
	ChunkTypeText         ChunkType = "text"
	ChunkTypeTable        ChunkType = "table"
	ChunkTypeImageCaption ChunkType = "image_caption"
	ChunkTypeCode         ChunkType = "code"
)

// Chunk represents a content fragment extracted from a document or source.
type Chunk struct {
	ID              string         `json:"id"`
	OrgID           string         `json:"org_id"`
	KnowledgeBaseID string         `json:"knowledge_base_id"`
	DocumentID      *string        `json:"document_id,omitempty"`
	SourceID        *string        `json:"source_id,omitempty"`
	Content         string         `json:"content"`
	ChunkIndex      int            `json:"chunk_index"`
	TokenCount      *int           `json:"token_count,omitempty"`
	PageNumber      *int           `json:"page_number,omitempty"`
	Heading         *string        `json:"heading,omitempty"`
	ChunkType       ChunkType      `json:"chunk_type"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
}

// Embedding stores a vector representation of a chunk.
type Embedding struct {
	ID           string          `json:"id"`
	OrgID        string          `json:"org_id"`
	ChunkID      string          `json:"chunk_id"`
	Embedding    pgvector.Vector `json:"embedding"`
	ModelName    string          `json:"model_name"`
	ModelVersion *string         `json:"model_version,omitempty"`
	Dimensions   int             `json:"dimensions"`
	CreatedAt    time.Time       `json:"created_at"`
}

// ChunkWithScore pairs a chunk with its similarity score from a vector search.
type ChunkWithScore struct {
	Chunk
	Score float64 `json:"score"`
}
