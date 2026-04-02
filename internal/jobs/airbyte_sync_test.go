package jobs_test

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/jobs"
)

func TestComputeChunkHash_Deterministic(t *testing.T) {
	content := "Hello, world!"
	hash1 := jobs.ComputeChunkHash(content)
	hash2 := jobs.ComputeChunkHash(content)
	if hash1 != hash2 {
		t.Errorf("expected identical hashes, got %s and %s", hash1, hash2)
	}
	// SHA-256 produces 64 hex chars.
	if len(hash1) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash1))
	}
}

func TestComputeChunkHash_DifferentContent(t *testing.T) {
	hash1 := jobs.ComputeChunkHash("content A")
	hash2 := jobs.ComputeChunkHash("content B")
	if hash1 == hash2 {
		t.Error("expected different hashes for different content")
	}
}

func TestComputeChunkHash_EmptyString(t *testing.T) {
	hash := jobs.ComputeChunkHash("")
	// SHA-256 of empty string is a well-known value.
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expected {
		t.Errorf("expected %s, got %s", expected, hash)
	}
}
