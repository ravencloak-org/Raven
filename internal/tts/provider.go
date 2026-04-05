// Package tts defines the text-to-speech provider abstraction and includes
// Cartesia Sonic (cloud) and Piper (self-hosted) implementations.
package tts

import (
	"context"
	"io"
)

// AudioFormat enumerates supported output audio formats.
type AudioFormat string

// Supported audio formats.
const (
	AudioFormatPCM  AudioFormat = "pcm"
	AudioFormatMP3  AudioFormat = "mp3"
	AudioFormatOPUS AudioFormat = "opus"
)

// SynthesizeOpts contains options that callers pass to control synthesis.
type SynthesizeOpts struct {
	// VoiceID selects the voice to use (provider-specific).
	VoiceID string
	// Language is a BCP-47 language tag (e.g. "en-US").
	Language string
	// Format is the desired output audio format (default: mp3).
	Format AudioFormat
	// SampleRate in Hz (e.g. 24000). Zero means provider default.
	SampleRate int
	// Speed is a multiplier (1.0 = normal). Zero means provider default.
	Speed float64
}

// Provider is the interface every TTS backend must implement.
type Provider interface {
	// Synthesize converts text to audio and returns a stream of audio data.
	// The caller is responsible for closing the returned ReadCloser.
	Synthesize(ctx context.Context, text string, opts SynthesizeOpts) (io.ReadCloser, error)

	// Name returns the provider name (e.g. "cartesia", "piper").
	Name() string
}
