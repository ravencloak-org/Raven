package stt

import "fmt"

// Config holds all STT-related configuration.
type Config struct {
	Provider ProviderName
	Deepgram DeepgramConfig
	Whisper  WhisperConfig
}

// NewProvider creates a Provider based on the given configuration.
// It returns an error if the provider name is unsupported or if required
// credentials are missing.
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case ProviderDeepgram:
		return NewDeepgramProvider(cfg.Deepgram)
	case ProviderWhisper:
		return NewWhisperProvider(cfg.Whisper), nil
	case "":
		// Default to deepgram when an API key is configured, otherwise whisper.
		if cfg.Deepgram.APIKey != "" {
			return NewDeepgramProvider(cfg.Deepgram)
		}
		return NewWhisperProvider(cfg.Whisper), nil
	default:
		return nil, fmt.Errorf("stt: unsupported provider %q", cfg.Provider)
	}
}
