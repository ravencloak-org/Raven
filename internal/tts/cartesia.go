package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	cartesiaDefaultBaseURL = "https://api.cartesia.ai"
	cartesiaDefaultModel   = "sonic-2"
	cartesiaDefaultVoice   = "a0e99841-438c-4a64-b679-ae501e7d6091" // Cartesia default English voice
)

// CartesiaConfig holds settings for the Cartesia Sonic TTS provider.
type CartesiaConfig struct {
	APIKey  string
	VoiceID string
	Model   string
	BaseURL string
}

// cartesiaProvider implements Provider using the Cartesia Sonic API.
type cartesiaProvider struct {
	apiKey  string
	voiceID string
	model   string
	baseURL string
	client  *http.Client
}

// NewCartesiaProvider creates a TTS provider backed by the Cartesia Sonic API.
func NewCartesiaProvider(cfg CartesiaConfig) (Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("cartesia: API key is required")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = cartesiaDefaultBaseURL
	}
	voiceID := cfg.VoiceID
	if voiceID == "" {
		voiceID = cartesiaDefaultVoice
	}
	model := cfg.Model
	if model == "" {
		model = cartesiaDefaultModel
	}
	return &cartesiaProvider{
		apiKey:  cfg.APIKey,
		voiceID: voiceID,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Name returns the provider identifier.
func (p *cartesiaProvider) Name() string { return "cartesia" }

// cartesiaTTSRequest is the JSON body sent to Cartesia's /tts/bytes endpoint.
type cartesiaTTSRequest struct {
	ModelID    string              `json:"model_id"`
	Transcript string              `json:"transcript"`
	Voice      cartesiaVoice       `json:"voice"`
	OutputFmt  cartesiaOutputFmt   `json:"output_format"`
	Language   string              `json:"language,omitempty"`
}

type cartesiaVoice struct {
	Mode string `json:"mode"`
	ID   string `json:"id"`
}

type cartesiaOutputFmt struct {
	Container  string `json:"container"`
	SampleRate int    `json:"sample_rate"`
}

// Synthesize converts text to audio using the Cartesia Sonic API.
// For texts with multiple sentences it performs sentence-boundary dispatch:
// each sentence is synthesized independently and the audio chunks are
// concatenated into a single stream for lower perceived latency.
func (p *cartesiaProvider) Synthesize(ctx context.Context, text string, opts SynthesizeOpts) (io.ReadCloser, error) {
	sentences := SplitSentences(text)
	if len(sentences) == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	voiceID := p.voiceID
	if opts.VoiceID != "" {
		voiceID = opts.VoiceID
	}

	container := resolveCartesiaContainer(opts.Format)
	sampleRate := opts.SampleRate
	if sampleRate <= 0 {
		sampleRate = 24000
	}

	// For a single sentence, stream directly.
	if len(sentences) == 1 {
		return p.synthesizeSingle(ctx, sentences[0], voiceID, container, sampleRate, opts.Language)
	}

	// Multiple sentences: synthesize each and concatenate.
	var buf bytes.Buffer
	for _, sentence := range sentences {
		rc, err := p.synthesizeSingle(ctx, sentence, voiceID, container, sampleRate, opts.Language)
		if err != nil {
			return nil, fmt.Errorf("cartesia: sentence synthesis failed: %w", err)
		}
		if _, err := io.Copy(&buf, rc); err != nil {
			_ = rc.Close()
			return nil, fmt.Errorf("cartesia: reading sentence audio: %w", err)
		}
		_ = rc.Close()
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// synthesizeSingle sends one text chunk to the Cartesia API and returns the
// audio response body.
func (p *cartesiaProvider) synthesizeSingle(ctx context.Context, text, voiceID, container string, sampleRate int, language string) (io.ReadCloser, error) {
	reqBody := cartesiaTTSRequest{
		ModelID:    p.model,
		Transcript: text,
		Voice: cartesiaVoice{
			Mode: "id",
			ID:   voiceID,
		},
		OutputFmt: cartesiaOutputFmt{
			Container:  container,
			SampleRate: sampleRate,
		},
		Language: language,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("cartesia: marshal request: %w", err)
	}

	url := p.baseURL + "/tts/bytes"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cartesia: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("Cartesia-Version", "2024-06-10")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cartesia: HTTP request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			_ = resp.Body.Close()
		}()
		errBody, _ := io.ReadAll(resp.Body)
		slog.Error("cartesia: non-200 response", "status", resp.StatusCode, "body", string(errBody))
		return nil, fmt.Errorf("cartesia: API returned %d: %s", resp.StatusCode, string(errBody))
	}

	return resp.Body, nil
}

// resolveCartesiaContainer maps AudioFormat to the Cartesia container name.
func resolveCartesiaContainer(f AudioFormat) string {
	switch f {
	case AudioFormatPCM:
		return "raw"
	case AudioFormatOPUS:
		return "ogg"
	case AudioFormatMP3:
		return "mp3"
	default:
		return "mp3"
	}
}
