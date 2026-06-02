package repository

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
)

func TestSearchColumnsNotEmpty(t *testing.T) {
	if searchColumns == "" {
		t.Error("searchColumns should not be empty")
	}
}

func TestNewSearchRepository(t *testing.T) {
	repo := NewSearchRepository(nil)
	if repo == nil {
		t.Fatal("NewSearchRepository returned nil")
	}
}

func TestChunkWithRankFields(t *testing.T) {
	// Verify that the ChunkWithRank model has the expected fields.
	chunk := model.ChunkWithRank{
		ID:              "chunk-1",
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-1",
		Content:         "test content",
		ChunkIndex:      0,
		ChunkType:       "text",
		Rank:            0.75,
		Highlight:       "<b>test</b> content",
	}

	if chunk.ID != "chunk-1" {
		t.Errorf("expected ID 'chunk-1', got %q", chunk.ID)
	}
	if chunk.OrgID != "org-1" {
		t.Errorf("expected OrgID 'org-1', got %q", chunk.OrgID)
	}
	if chunk.KnowledgeBaseID != "kb-1" {
		t.Errorf("expected KnowledgeBaseID 'kb-1', got %q", chunk.KnowledgeBaseID)
	}
	if chunk.Content != "test content" {
		t.Errorf("expected Content 'test content', got %q", chunk.Content)
	}
	if chunk.ChunkIndex != 0 {
		t.Errorf("expected ChunkIndex 0, got %d", chunk.ChunkIndex)
	}
	if chunk.ChunkType != "text" {
		t.Errorf("expected ChunkType 'text', got %q", chunk.ChunkType)
	}
	if chunk.Rank != 0.75 {
		t.Errorf("expected Rank 0.75, got %f", chunk.Rank)
	}
	if chunk.Highlight != "<b>test</b> content" {
		t.Errorf("unexpected Highlight: %q", chunk.Highlight)
	}
}
