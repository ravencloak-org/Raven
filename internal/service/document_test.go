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
		{"queued to processing", model.ProcessingStatusQueued, model.ProcessingStatusProcessing, true},
		{"queued to completed", model.ProcessingStatusQueued, model.ProcessingStatusCompleted, false},
		{"queued to failed", model.ProcessingStatusQueued, model.ProcessingStatusFailed, false},
		{"processing to completed", model.ProcessingStatusProcessing, model.ProcessingStatusCompleted, true},
		{"processing to failed", model.ProcessingStatusProcessing, model.ProcessingStatusFailed, true},
		{"processing to queued", model.ProcessingStatusProcessing, model.ProcessingStatusQueued, false},
		{"completed to any", model.ProcessingStatusCompleted, model.ProcessingStatusQueued, false},
		{"completed to processing", model.ProcessingStatusCompleted, model.ProcessingStatusProcessing, false},
		{"failed to queued", model.ProcessingStatusFailed, model.ProcessingStatusQueued, true},
		{"failed to processing", model.ProcessingStatusFailed, model.ProcessingStatusProcessing, false},
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
