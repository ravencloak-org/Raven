package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ravencloak-org/Raven/internal/model"
)

// TestConnectionResult is what the LLM-provider test endpoint hands back
// to callers. Ok=true means we made a successful authenticated call to
// the provider; Detail carries provider-side context (model list size,
// HTTP code, error message) so the UI can render something meaningful.
type TestConnectionResult struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// TestConnection probes the given LLM provider with the supplied
// credentials WITHOUT persisting anything. It exists so the Add-Provider
// UI can short-circuit a bad key / unreachable endpoint before the user
// commits — the previous flow stored the row and only surfaced the
// failure later, on the first chat request, which 500'd inside the RAG
// pipeline.
//
// The probe is a lightweight read endpoint per provider — model lists
// for openai/anthropic/custom (cheap, no token cost), the /api/tags
// endpoint for ollama (the canonical "is this daemon alive" probe).
// Network/IO errors bubble up as TestConnectionResult{OK:false, Detail}.
func TestConnection(ctx context.Context, req model.CreateLLMProviderRequest) (TestConnectionResult, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	// String-compared switch so this also catches 'ollama' which the
	// frontend already speaks but the backend enum doesn't yet have a
	// constant for. Adding the enum value is tracked separately.
	switch string(req.Provider) {
	case string(model.LLMProviderOpenAI):
		return probeOpenAI(ctx, client, req.APIKey, "https://api.openai.com")
	case string(model.LLMProviderAnthropic):
		return probeAnthropic(ctx, client, req.APIKey)
	case "ollama":
		base := strings.TrimRight(deref(req.BaseURL, "http://localhost:11434"), "/")
		return probeOllama(ctx, client, base)
	case string(model.LLMProviderCustom):
		// OpenAI-compatible by convention; reuse the OpenAI probe but
		// against the user-supplied Base URL.
		base := strings.TrimRight(deref(req.BaseURL, ""), "/")
		if base == "" {
			return TestConnectionResult{OK: false, Detail: "Base URL is required for custom providers"}, nil
		}
		return probeOpenAI(ctx, client, req.APIKey, base)
	}
	return TestConnectionResult{OK: false, Detail: fmt.Sprintf("unsupported provider: %s", req.Provider)}, nil
}

func probeOpenAI(ctx context.Context, c *http.Client, apiKey, baseURL string) (TestConnectionResult, error) {
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	r.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := c.Do(r)
	if err != nil {
		return TestConnectionResult{OK: false, Detail: fmt.Sprintf("connection failed: %v", err)}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	return interpretListModelsResponse(resp)
}

func probeAnthropic(ctx context.Context, c *http.Client, apiKey string) (TestConnectionResult, error) {
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models", nil)
	r.Header.Set("x-api-key", apiKey)
	r.Header.Set("anthropic-version", "2023-06-01")
	resp, err := c.Do(r)
	if err != nil {
		return TestConnectionResult{OK: false, Detail: fmt.Sprintf("connection failed: %v", err)}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	return interpretListModelsResponse(resp)
}

func probeOllama(ctx context.Context, c *http.Client, baseURL string) (TestConnectionResult, error) {
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	resp, err := c.Do(r)
	if err != nil {
		return TestConnectionResult{OK: false, Detail: fmt.Sprintf("connection failed: %v", err)}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return TestConnectionResult{
			OK:     false,
			Detail: fmt.Sprintf("Ollama returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
		}, nil
	}
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TestConnectionResult{OK: false, Detail: "Ollama responded but the body wasn't /api/tags JSON"}, nil
	}
	return TestConnectionResult{
		OK:     true,
		Detail: fmt.Sprintf("Connected — %d model(s) installed locally", len(body.Models)),
	}, nil
}

func interpretListModelsResponse(resp *http.Response) (TestConnectionResult, error) {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	switch {
	case resp.StatusCode == http.StatusOK:
		// Best effort to count returned models; provider shapes vary but
		// all include some array under data/models.
		var parsed struct {
			Data   []json.RawMessage `json:"data"`
			Models []json.RawMessage `json:"models"`
		}
		_ = json.Unmarshal(body, &parsed)
		n := len(parsed.Data) + len(parsed.Models)
		if n > 0 {
			return TestConnectionResult{OK: true, Detail: fmt.Sprintf("Connected — %d model(s) available", n)}, nil
		}
		return TestConnectionResult{OK: true, Detail: "Connected"}, nil
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return TestConnectionResult{OK: false, Detail: fmt.Sprintf("Auth rejected (HTTP %d). Double-check the API key.", resp.StatusCode)}, nil
	default:
		return TestConnectionResult{
			OK:     false,
			Detail: fmt.Sprintf("Provider returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
		}, nil
	}
}

func deref(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}
