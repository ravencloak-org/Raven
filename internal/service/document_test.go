package service

import (
	"testing"

	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/model"
)

func TestValidStatusTransitions(t *testing.T) {
	tests := []struct {
		name    string
		from    model.ProcessingStatus
		to      model.ProcessingStatus
		allowed bool
	}{
		// queued transitions
		{"queued to crawling", model.ProcessingStatusQueued, model.ProcessingStatusCrawling, true},
		{"queued to failed", model.ProcessingStatusQueued, model.ProcessingStatusFailed, true},
		{"queued to parsing", model.ProcessingStatusQueued, model.ProcessingStatusParsing, false},
		{"queued to ready", model.ProcessingStatusQueued, model.ProcessingStatusReady, false},

		// crawling transitions
		{"crawling to parsing", model.ProcessingStatusCrawling, model.ProcessingStatusParsing, true},
		{"crawling to failed", model.ProcessingStatusCrawling, model.ProcessingStatusFailed, true},
		{"crawling to ready", model.ProcessingStatusCrawling, model.ProcessingStatusReady, false},

		// parsing transitions
		{"parsing to chunking", model.ProcessingStatusParsing, model.ProcessingStatusChunking, true},
		{"parsing to failed", model.ProcessingStatusParsing, model.ProcessingStatusFailed, true},
		{"parsing to embedding", model.ProcessingStatusParsing, model.ProcessingStatusEmbedding, false},

		// chunking transitions
		{"chunking to embedding", model.ProcessingStatusChunking, model.ProcessingStatusEmbedding, true},
		{"chunking to failed", model.ProcessingStatusChunking, model.ProcessingStatusFailed, true},
		{"chunking to ready", model.ProcessingStatusChunking, model.ProcessingStatusReady, false},

		// embedding transitions
		{"embedding to ready", model.ProcessingStatusEmbedding, model.ProcessingStatusReady, true},
		{"embedding to failed", model.ProcessingStatusEmbedding, model.ProcessingStatusFailed, true},
		{"embedding to queued", model.ProcessingStatusEmbedding, model.ProcessingStatusQueued, false},

		// terminal/recovery transitions
		{"failed to queued", model.ProcessingStatusFailed, model.ProcessingStatusQueued, true},
		{"failed to reprocessing", model.ProcessingStatusFailed, model.ProcessingStatusReprocessing, true},
		{"failed to crawling", model.ProcessingStatusFailed, model.ProcessingStatusCrawling, false},
		{"ready to reprocessing", model.ProcessingStatusReady, model.ProcessingStatusReprocessing, true},
		{"ready to queued", model.ProcessingStatusReady, model.ProcessingStatusQueued, false},

		// reprocessing transitions
		{"reprocessing to crawling", model.ProcessingStatusReprocessing, model.ProcessingStatusCrawling, true},
		{"reprocessing to failed", model.ProcessingStatusReprocessing, model.ProcessingStatusFailed, true},
		{"reprocessing to ready", model.ProcessingStatusReprocessing, model.ProcessingStatusReady, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := isTransitionAllowed(tt.from, tt.to)
			if allowed != tt.allowed {
				t.Errorf("transition %s -> %s: expected allowed=%v, got %v",
					tt.from, tt.to, tt.allowed, allowed)
			}
		})
	}
}

// isTransitionAllowed checks if a status transition is valid.
// This mirrors the logic used in DocumentService.UpdateStatus.
func isTransitionAllowed(from, to model.ProcessingStatus) bool {
	allowed, ok := validStatusTransitions[from]
	if !ok {
		return false
	}
	return lo.Contains(allowed, to)
}
