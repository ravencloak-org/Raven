package repository

import (
	"testing"
)

func TestDocColumns_NotEmpty(t *testing.T) {
	if docColumns == "" {
		t.Error("docColumns constant should not be empty")
	}
}

func TestNewDocumentRepository(t *testing.T) {
	repo := NewDocumentRepository(nil)
	if repo == nil {
		t.Error("NewDocumentRepository should return non-nil")
	}
}
