package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultDeepgramBaseURL = "https://api.deepgram.com"
	defaultDeepgramModel   = "nova-2"
	deepgramTimeout        = 120 * time.Second
)

// DeepgramConfig holds settings for the Deepgram STT provider.
type DeepgramConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

// deepgramProvider implements Provider using the Deepgram REST API.
type deepgramProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewDeepgramProvider creates a Deepgram-backed STT provider.
func NewDeepgramProvider(cfg DeepgramConfig) (Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("stt/deepgram: API key is required")
	}
	model := cfg.Model
	if model == "" {
		model = defaultDeepgramModel
	}
	base := cfg.BaseURL
	if base == "" {
		base = defaultDeepgramBaseURL
	}
	return &deepgramProvider{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: base,
		client: &http.Client{
			Timeout: deepgramTimeout,
		},
	}, nil
}

func (d *deepgramProvider) Name() ProviderName {
	return ProviderDeepgram
}

// deepgramResponse is the minimal JSON schema for a Deepgram pre-recorded transcription.
type deepgramResponse struct {
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Transcript string  `json:"transcript"`
				Confidence float64 `json:"confidence"`
				Words      []struct {
					Word       string  `json:"word"`
					Start      float64 `json:"start"`
					End        float64 `json:"end"`
					Confidence float64 `json:"confidence"`
				} `json:"words"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
	Metadata struct {
		Duration    float64 `json:"duration"`
		RequestID   string  `json:"request_id"`
		ModelInfo   map[string]any `json:"model_info"`
		DetectedLang string `json:"detected_language"`
	} `json:"metadata"`
}

// Transcribe sends audio to the Deepgram pre-recorded API and returns the result.
func (d *deepgramProvider) Transcribe(ctx context.Context, audio io.Reader, opts TranscribeOpts) (*TranscribeResult, error) {
	body, err := io.ReadAll(audio)
	if err != nil {
		return nil, fmt.Errorf("stt/deepgram: failed to read audio: %w", err)
	}

	endpoint, err := d.buildURL(opts)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("stt/deepgram: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+d.apiKey)
	req.Header.Set("Content-Type", contentTypeForEncoding(opts.Encoding))

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stt/deepgram: request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("stt/deepgram: failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("stt/deepgram: API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var dgResp deepgramResponse
	if err := json.NewDecoder(resp.Body).Decode(&dgResp); err != nil {
		return nil, fmt.Errorf("stt/deepgram: failed to decode response: %w", err)
	}

	return d.toResult(&dgResp, opts), nil
}

func (d *deepgramProvider) buildURL(opts TranscribeOpts) (string, error) {
	u, err := url.Parse(d.baseURL + "/v1/listen")
	if err != nil {
		return "", fmt.Errorf("stt/deepgram: invalid base URL: %w", err)
	}
	q := u.Query()
	model := opts.Model
	if model == "" {
		model = d.model
	}
	q.Set("model", model)
	q.Set("smart_format", "true")
	q.Set("punctuate", "true")

	if opts.Language != "" {
		q.Set("language", opts.Language)
	} else {
		q.Set("detect_language", "true")
	}
	if opts.SampleRate > 0 {
		q.Set("sample_rate", fmt.Sprintf("%d", opts.SampleRate))
	}
	if opts.Encoding != "" {
		q.Set("encoding", opts.Encoding)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (d *deepgramProvider) toResult(resp *deepgramResponse, _ TranscribeOpts) *TranscribeResult {
	result := &TranscribeResult{
		Duration: time.Duration(resp.Metadata.Duration * float64(time.Second)),
		Language: resp.Metadata.DetectedLang,
	}

	if len(resp.Results.Channels) > 0 && len(resp.Results.Channels[0].Alternatives) > 0 {
		alt := resp.Results.Channels[0].Alternatives[0]
		result.Text = alt.Transcript
		result.Confidence = alt.Confidence

		words := make([]WordTimestamp, 0, len(alt.Words))
		for _, w := range alt.Words {
			words = append(words, WordTimestamp{
				Word:       w.Word,
				Start:      time.Duration(w.Start * float64(time.Second)),
				End:        time.Duration(w.End * float64(time.Second)),
				Confidence: w.Confidence,
			})
		}
		result.Words = words
	}

	return result
}

// contentTypeForEncoding maps an audio encoding name to the appropriate HTTP Content-Type.
func contentTypeForEncoding(encoding string) string {
	switch encoding {
	case "mp3":
		return "audio/mpeg"
	case "opus", "ogg":
		return "audio/ogg"
	case "flac":
		return "audio/flac"
	case "wav", "linear16":
		return "audio/wav"
	case "webm":
		return "audio/webm"
	default:
		// Deepgram auto-detects when Content-Type is generic.
		return "audio/wav"
	}
}
