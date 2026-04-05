package repository

import (
	"testing"
)

func TestNewClickHouseEmbeddingRepository(t *testing.T) {
	// nil conn is valid for unit testing (connection is only used at runtime).
	repo := NewClickHouseEmbeddingRepository(nil)
	if repo == nil {
		t.Fatal("NewClickHouseEmbeddingRepository returned nil")
	}
}

func TestNewClickHouseEmbeddingRepository_NotNil(t *testing.T) {
	repo := NewClickHouseEmbeddingRepository(nil)
	if repo.conn != nil {
		t.Error("expected nil conn for test repository")
	}
}
