package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"
)

const (
	defaultWhisperEndpoint = "http://localhost:8000"
	defaultWhisperModel    = "large-v3"
	whisperTimeout         = 300 * time.Second
)

// WhisperConfig holds settings for the faster-whisper STT provider.
type WhisperConfig struct {
	Endpoint string
	Model    string
}

// whisperProvider implements Provider by calling a faster-whisper HTTP endpoint.
// The expected endpoint exposes a POST /v1/transcribe route that accepts
// multipart/form-data with an "audio" file field.
type whisperProvider struct {
	endpoint string
	model    string
	client   *http.Client
}

// NewWhisperProvider creates a faster-whisper-backed STT provider.
func NewWhisperProvider(cfg WhisperConfig) Provider {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultWhisperEndpoint
	}
	model := cfg.Model
	if model == "" {
		model = defaultWhisperModel
	}
	return &whisperProvider{
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			Timeout: whisperTimeout,
		},
	}
}

func (w *whisperProvider) Name() ProviderName {
	return ProviderWhisper
}

// whisperResponse is the expected JSON response from the faster-whisper HTTP service.
type whisperResponse struct {
	Text       string  `json:"text"`
	Language   string  `json:"language"`
	Duration   float64 `json:"duration"`
	Confidence float64 `json:"confidence"`
	Segments   []struct {
		Text       string  `json:"text"`
		Start      float64 `json:"start"`
		End        float64 `json:"end"`
		Confidence float64 `json:"confidence"`
		Words      []struct {
			Word       string  `json:"word"`
			Start      float64 `json:"start"`
			End        float64 `json:"end"`
			Confidence float64 `json:"confidence"`
		} `json:"words"`
	} `json:"segments"`
}

// Transcribe sends audio to the faster-whisper HTTP endpoint as multipart form data.
func (w *whisperProvider) Transcribe(ctx context.Context, audio io.Reader, opts TranscribeOpts) (*TranscribeResult, error) {
	audioBytes, err := io.ReadAll(audio)
	if err != nil {
		return nil, fmt.Errorf("stt/whisper: failed to read audio: %w", err)
	}

	body, contentType, err := w.buildMultipart(audioBytes, opts)
	if err != nil {
		return nil, err
	}

	reqURL := w.endpoint + "/v1/transcribe"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("stt/whisper: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stt/whisper: request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("stt/whisper: failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("stt/whisper: endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	var wResp whisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&wResp); err != nil {
		return nil, fmt.Errorf("stt/whisper: failed to decode response: %w", err)
	}

	return w.toResult(&wResp), nil
}

func (w *whisperProvider) buildMultipart(audioBytes []byte, opts TranscribeOpts) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Audio file part
	fw, err := mw.CreateFormFile("audio", "audio.wav")
	if err != nil {
		return nil, "", fmt.Errorf("stt/whisper: failed to create form file: %w", err)
	}
	if _, err := fw.Write(audioBytes); err != nil {
		return nil, "", fmt.Errorf("stt/whisper: failed to write audio data: %w", err)
	}

	// Model field
	model := opts.Model
	if model == "" {
		model = w.model
	}
	if err := mw.WriteField("model", model); err != nil {
		return nil, "", fmt.Errorf("stt/whisper: failed to write model field: %w", err)
	}

	// Optional language field
	if opts.Language != "" {
		if err := mw.WriteField("language", opts.Language); err != nil {
			return nil, "", fmt.Errorf("stt/whisper: failed to write language field: %w", err)
		}
	}

	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("stt/whisper: failed to close multipart writer: %w", err)
	}

	return &buf, mw.FormDataContentType(), nil
}

func (w *whisperProvider) toResult(resp *whisperResponse) *TranscribeResult {
	result := &TranscribeResult{
		Text:       resp.Text,
		Confidence: resp.Confidence,
		Duration:   time.Duration(resp.Duration * float64(time.Second)),
		Language:   resp.Language,
	}

	// Flatten segment-level words into the top-level Words list.
	for _, seg := range resp.Segments {
		for _, word := range seg.Words {
			result.Words = append(result.Words, WordTimestamp{
				Word:       word.Word,
				Start:      time.Duration(word.Start * float64(time.Second)),
				End:        time.Duration(word.End * float64(time.Second)),
				Confidence: word.Confidence,
			})
		}
	}

	return result
}
