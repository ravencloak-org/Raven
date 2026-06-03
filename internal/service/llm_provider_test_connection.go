package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
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
//
// Custom and Ollama base URLs are user-supplied, so we validate them
// against an SSRF policy (scheme allowlist, no embedded credentials,
// no loopback / private / link-local / metadata addresses) before
// dialing — otherwise the admin-only test endpoint becomes an
// authenticated SSRF primitive against internal infrastructure.
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
		if err := validateExternalURL(ctx, base); err != nil {
			return TestConnectionResult{OK: false, Detail: fmt.Sprintf("Base URL rejected: %v", err)}, nil
		}
		return probeOllama(ctx, client, base)
	case string(model.LLMProviderCustom):
		// OpenAI-compatible by convention; reuse the OpenAI probe but
		// against the user-supplied Base URL.
		base := strings.TrimRight(deref(req.BaseURL, ""), "/")
		if base == "" {
			return TestConnectionResult{OK: false, Detail: "Base URL is required for custom providers"}, nil
		}
		if err := validateExternalURL(ctx, base); err != nil {
			return TestConnectionResult{OK: false, Detail: fmt.Sprintf("Base URL rejected: %v", err)}, nil
		}
		return probeOpenAI(ctx, client, req.APIKey, base)
	}
	return TestConnectionResult{OK: false, Detail: fmt.Sprintf("unsupported provider: %s", req.Provider)}, nil
}

// validateExternalURL parses base and rejects values that would let an
// admin-supplied URL probe internal infrastructure. The rules:
//
//   - scheme MUST be http or https,
//   - userinfo (credentials embedded in URL) is rejected,
//   - host MUST be present,
//   - any resolved IP that is loopback, private, link-local,
//     unspecified, or cloud-metadata (169.254.169.254) is rejected.
//
// Set RAVEN_LLM_ALLOW_LOOPBACK=1 to bypass the loopback ban — useful for
// dev when probing a local Ollama / vLLM. RAVEN_LLM_ALLOW_PRIVATE=1
// similarly relaxes the RFC1918 ban for self-hosted Raven on a LAN.
func validateExternalURL(ctx context.Context, raw string) error {
	if raw == "" {
		return errors.New("URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q not allowed (use http or https)", u.Scheme)
	}
	if u.User != nil {
		return errors.New("embedded credentials not allowed")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("host is empty")
	}
	allowLoopback := os.Getenv("RAVEN_LLM_ALLOW_LOOPBACK") == "1"
	allowPrivate := os.Getenv("RAVEN_LLM_ALLOW_PRIVATE") == "1"
	// Resolve so a hostname that points at a private IP also gets rejected.
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("host lookup failed: %w", err)
	}
	if len(addrs) == 0 {
		return errors.New("host did not resolve")
	}
	for _, ipa := range addrs {
		ip := ipa.IP
		if ip.IsUnspecified() {
			return fmt.Errorf("address %s is unspecified", ip)
		}
		// Cloud-metadata endpoint — never allowed.
		if ip.Equal(net.ParseIP("169.254.169.254")) {
			return fmt.Errorf("address %s is cloud metadata", ip)
		}
		if ip.IsLoopback() && !allowLoopback {
			return fmt.Errorf("address %s is loopback (set RAVEN_LLM_ALLOW_LOOPBACK=1 to allow)", ip)
		}
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("address %s is link-local", ip)
		}
		if ip.IsPrivate() && !allowPrivate {
			return fmt.Errorf("address %s is private (set RAVEN_LLM_ALLOW_PRIVATE=1 to allow)", ip)
		}
	}
	return nil
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
	switch resp.StatusCode {
	case http.StatusOK:
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
	case http.StatusUnauthorized, http.StatusForbidden:
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
