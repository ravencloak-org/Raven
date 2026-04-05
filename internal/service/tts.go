package service

import (
	"context"
	"io"
	"log/slog"

	"github.com/ravencloak-org/Raven/internal/tts"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// TTSService provides text-to-speech synthesis using a configured provider.
type TTSService struct {
	provider tts.Provider
}

// NewTTSService creates a new TTSService with the given provider.
func NewTTSService(provider tts.Provider) *TTSService {
	return &TTSService{provider: provider}
}

// Synthesize converts text to audio using the configured TTS provider.
func (s *TTSService) Synthesize(ctx context.Context, text string, opts tts.SynthesizeOpts) (io.ReadCloser, error) {
	rc, err := s.provider.Synthesize(ctx, text, opts)
	if err != nil {
		slog.ErrorContext(ctx, "TTSService.Synthesize error",
			"provider", s.provider.Name(),
			"error", err,
		)
		return nil, apierror.NewInternal("text-to-speech synthesis failed")
	}
	return rc, nil
}

// ProviderName returns the name of the active TTS provider.
func (s *TTSService) ProviderName() string {
	return s.provider.Name()
}
