package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	piperDefaultEndpoint = "http://localhost:5000"
	piperDefaultVoice    = "en_US-amy-medium"
)

// PiperConfig holds settings for the self-hosted Piper TTS provider.
type PiperConfig struct {
	Endpoint string
	Voice    string
}

// piperProvider implements Provider using a Piper TTS HTTP server.
type piperProvider struct {
	endpoint string
	voice    string
	client   *http.Client
}

// NewPiperProvider creates a TTS provider backed by a self-hosted Piper TTS
// HTTP server. Piper is MIT-licensed and suitable for edge/offline deployments.
func NewPiperProvider(cfg PiperConfig) Provider {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = piperDefaultEndpoint
	}
	voice := cfg.Voice
	if voice == "" {
		voice = piperDefaultVoice
	}
	return &piperProvider{
		endpoint: endpoint,
		voice:    voice,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name returns the provider identifier.
func (p *piperProvider) Name() string { return "piper" }

// Synthesize converts text to audio by calling the Piper HTTP API.
// For multi-sentence text it uses sentence-boundary dispatch.
func (p *piperProvider) Synthesize(ctx context.Context, text string, opts SynthesizeOpts) (io.ReadCloser, error) {
	sentences := SplitSentences(text)
	if len(sentences) == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	voice := p.voice
	if opts.VoiceID != "" {
		voice = opts.VoiceID
	}

	if len(sentences) == 1 {
		return p.synthesizeSingle(ctx, sentences[0], voice)
	}

	var buf bytes.Buffer
	for _, sentence := range sentences {
		rc, err := p.synthesizeSingle(ctx, sentence, voice)
		if err != nil {
			return nil, fmt.Errorf("piper: sentence synthesis failed: %w", err)
		}
		if _, err := io.Copy(&buf, rc); err != nil {
			_ = rc.Close()
			return nil, fmt.Errorf("piper: reading sentence audio: %w", err)
		}
		_ = rc.Close()
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// synthesizeSingle calls the Piper HTTP server for a single text chunk.
// Piper's HTTP API typically accepts GET /api/tts?text=...&voice=...
func (p *piperProvider) synthesizeSingle(ctx context.Context, text, voice string) (io.ReadCloser, error) {
	u, err := url.Parse(p.endpoint + "/api/tts")
	if err != nil {
		return nil, fmt.Errorf("piper: parse URL: %w", err)
	}
	q := u.Query()
	q.Set("text", text)
	q.Set("voice", voice)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("piper: create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("piper: HTTP request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			_ = resp.Body.Close()
		}()
		errBody, _ := io.ReadAll(resp.Body)
		slog.Error("piper: non-200 response", "status", resp.StatusCode, "body", string(errBody))
		return nil, fmt.Errorf("piper: API returned %d: %s", resp.StatusCode, string(errBody))
	}

	return resp.Body, nil
}
