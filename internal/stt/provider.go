// Package stt provides speech-to-text provider abstractions and implementations.
// Supported backends: Deepgram (cloud) and faster-whisper (self-hosted).
package stt

import (
	"context"
	"io"
	"time"
)

// ProviderName identifies a speech-to-text backend.
type ProviderName string

// Supported provider names.
const (
	ProviderDeepgram ProviderName = "deepgram"
	ProviderWhisper  ProviderName = "whisper"
)

// ValidProviders is the set of recognised STT provider names.
var ValidProviders = map[ProviderName]bool{
	ProviderDeepgram: true,
	ProviderWhisper:  true,
}

// TranscribeOpts configures a single transcription request.
type TranscribeOpts struct {
	// Language is a BCP-47 language tag (e.g. "en", "de"). Empty means auto-detect.
	Language string
	// Model selects the provider-specific model variant.
	Model string
	// SampleRate in Hz (e.g. 16000). Zero means the provider default.
	SampleRate int
	// Encoding describes the audio format (e.g. "linear16", "opus", "mp3").
	Encoding string
}

// WordTimestamp captures a single word with its time offsets in the audio.
type WordTimestamp struct {
	Word       string        `json:"word"`
	Start      time.Duration `json:"start"`
	End        time.Duration `json:"end"`
	Confidence float64       `json:"confidence"`
}

// TranscribeResult holds the output of a transcription request.
type TranscribeResult struct {
	// Text is the full transcribed text.
	Text string `json:"text"`
	// Confidence is the overall confidence score (0..1). Zero if unavailable.
	Confidence float64 `json:"confidence"`
	// Words contains per-word timestamps when available.
	Words []WordTimestamp `json:"words,omitempty"`
	// Duration is the duration of the audio that was processed.
	Duration time.Duration `json:"duration"`
	// Language is the detected or requested language code.
	Language string `json:"language,omitempty"`
}

// Provider defines the interface every STT backend must implement.
type Provider interface {
	// Transcribe processes the audio from reader and returns a transcription result.
	Transcribe(ctx context.Context, audio io.Reader, opts TranscribeOpts) (*TranscribeResult, error)
	// Name returns the provider identifier.
	Name() ProviderName
}
