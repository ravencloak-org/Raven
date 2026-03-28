package model_test

import (
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/ravencloak-org/Raven/internal/model"
)

func TestChunkType_Values(t *testing.T) {
	tests := []struct {
		ct   model.ChunkType
		want string
	}{
		{model.ChunkTypeText, "text"},
		{model.ChunkTypeTable, "table"},
		{model.ChunkTypeImageCaption, "image_caption"},
		{model.ChunkTypeCode, "code"},
	}
	for _, tt := range tests {
		if string(tt.ct) != tt.want {
			t.Errorf("ChunkType = %q, want %q", tt.ct, tt.want)
		}
	}
}

func TestChunk_Fields(t *testing.T) {
	docID := "doc-1"
	tokenCount := 100
	pageNum := 1
	heading := "Introduction"

	c := model.Chunk{
		ID:              "chunk-1",
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-1",
		DocumentID:      &docID,
		SourceID:        nil,
		Content:         "Hello world",
		ChunkIndex:      0,
		TokenCount:      &tokenCount,
		PageNumber:      &pageNum,
		Heading:         &heading,
		ChunkType:       model.ChunkTypeText,
		Metadata:        map[string]any{"key": "value"},
		CreatedAt:       time.Now(),
	}

	if c.ID != "chunk-1" {
		t.Errorf("Chunk.ID = %q, want %q", c.ID, "chunk-1")
	}
	if c.DocumentID == nil || *c.DocumentID != "doc-1" {
		t.Error("Chunk.DocumentID should be doc-1")
	}
	if c.SourceID != nil {
		t.Error("Chunk.SourceID should be nil")
	}
	if c.ChunkType != model.ChunkTypeText {
		t.Errorf("Chunk.ChunkType = %q, want %q", c.ChunkType, model.ChunkTypeText)
	}
	if c.Metadata["key"] != "value" {
		t.Error("Chunk.Metadata[key] should be value")
	}
}

func TestEmbedding_Fields(t *testing.T) {
	vec := pgvector.NewVector([]float32{0.1, 0.2, 0.3})
	modelVersion := "v1"

	e := model.Embedding{
		ID:           "emb-1",
		OrgID:        "org-1",
		ChunkID:      "chunk-1",
		Embedding:    vec,
		ModelName:    "text-embedding-ada-002",
		ModelVersion: &modelVersion,
		Dimensions:   3,
		CreatedAt:    time.Now(),
	}

	if e.ID != "emb-1" {
		t.Errorf("Embedding.ID = %q, want %q", e.ID, "emb-1")
	}
	if e.ChunkID != "chunk-1" {
		t.Errorf("Embedding.ChunkID = %q, want %q", e.ChunkID, "chunk-1")
	}
	if len(e.Embedding.Slice()) != 3 {
		t.Errorf("Embedding vector length = %d, want 3", len(e.Embedding.Slice()))
	}
	if e.ModelVersion == nil || *e.ModelVersion != "v1" {
		t.Error("Embedding.ModelVersion should be v1")
	}
	if e.Dimensions != 3 {
		t.Errorf("Embedding.Dimensions = %d, want 3", e.Dimensions)
	}
}

func TestChunkWithScore_EmbedScore(t *testing.T) {
	cws := model.ChunkWithScore{
		Chunk: model.Chunk{
			ID:      "chunk-1",
			Content: "test content",
		},
		Score: 0.95,
	}

	if cws.Score != 0.95 {
		t.Errorf("ChunkWithScore.Score = %f, want 0.95", cws.Score)
	}
	if cws.Content != "test content" {
		t.Errorf("ChunkWithScore.Content = %q, want %q", cws.Content, "test content")
	}
}
