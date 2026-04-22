package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPSummarizer calls POST /internal/summarize on the Python AI worker.
// The worker exposes an internal HTTP server alongside its main gRPC server
// for non-RAG operations where streaming is not required.
type HTTPSummarizer struct {
	baseURL string
	http    *http.Client
}

// NewHTTPSummarizer returns a summarizer pointing at baseURL. A trailing slash
// is stripped so callers can pass "http://ai-worker:8090" or
// "http://ai-worker:8090/" interchangeably. The default AI worker HTTP port
// is 8090 (see ai-worker/raven_worker/config.py and RAVEN_AI_WORKER_HTTP_URL
// in .env.example).
func NewHTTPSummarizer(baseURL string) *HTTPSummarizer {
	return &HTTPSummarizer{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Summarize posts req to /internal/summarize and returns the decoded response.
// The call uses the task's context so Asynq retry/timeout policy is honoured.
func (s *HTTPSummarizer) Summarize(ctx context.Context, req SummarizeRequest) (*SummarizeResponse, error) {
	if s.baseURL == "" {
		return nil, fmt.Errorf("summarizer: base URL not configured")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("summarizer: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/internal/summarize", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("summarizer: build req: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("summarizer: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		// Read up to 4KB of body for a useful error message without risking
		// unbounded memory on a misbehaving worker.
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("summarizer: status %d: %s", resp.StatusCode, string(buf))
	}

	var out SummarizeResponse
	// Cap successful responses too — subject + 5 bullets is ~1 KiB; 256 KiB
	// is a comfortable bound that prevents a misbehaving worker from
	// exhausting Asynq worker memory with an arbitrarily large JSON stream.
	const maxSuccessBody = 256 * 1024
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxSuccessBody)).Decode(&out); err != nil {
		return nil, fmt.Errorf("summarizer: decode: %w", err)
	}
	return &out, nil
}

// compile-time interface assertion
var _ SummaryGenerator = (*HTTPSummarizer)(nil)
