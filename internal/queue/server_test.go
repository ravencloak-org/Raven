package queue

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) (*Server, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	s := NewServer(ServerConfig{
		RedisAddr:   mr.Addr(),
		Concurrency: 2,
		MaxRetry:    3,
	})
	return s, mr
}

func TestNewServerDefaults(t *testing.T) {
	mr := miniredis.RunT(t)

	s := NewServer(ServerConfig{
		RedisAddr: mr.Addr(),
	})
	assert.NotNil(t, s)
	assert.NotNil(t, s.mux)
	assert.NotNil(t, s.logger)
}

func TestServerMuxRouting(t *testing.T) {
	s, _ := newTestServer(t)
	mux := s.Mux()
	assert.NotNil(t, mux)
}

// TestHandleDocumentProcess verifies the stub handler deserialises the payload
// and returns nil (success).
func TestHandleDocumentProcess(t *testing.T) {
	s, _ := newTestServer(t)

	p := DocumentProcessPayload{
		OrgID:           "org-1",
		DocumentID:      "doc-2",
		KnowledgeBaseID: "kb-3",
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	task := asynq.NewTask(TypeDocumentProcess, data)
	err = s.handleDocumentProcess(context.Background(), task)
	assert.NoError(t, err)
}

// TestHandleURLScrape verifies the stub handler deserialises the payload
// and returns nil (success).
func TestHandleURLScrape(t *testing.T) {
	s, _ := newTestServer(t)

	p := URLScrapePayload{
		OrgID:           "org-1",
		SourceID:        "src-2",
		KnowledgeBaseID: "kb-3",
		URL:             "https://example.com",
		CrawlDepth:      1,
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	task := asynq.NewTask(TypeURLScrape, data)
	err = s.handleURLScrape(context.Background(), task)
	assert.NoError(t, err)
}

// TestHandleReindex verifies the stub handler deserialises the payload
// and returns nil (success).
func TestHandleReindex(t *testing.T) {
	s, _ := newTestServer(t)

	p := ReindexPayload{
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-3",
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	task := asynq.NewTask(TypeReindex, data)
	err = s.handleReindex(context.Background(), task)
	assert.NoError(t, err)
}

// TestHandleDocumentProcessInvalidPayload verifies that the handler returns
// an error for malformed JSON.
func TestHandleDocumentProcessInvalidPayload(t *testing.T) {
	s, _ := newTestServer(t)

	task := asynq.NewTask(TypeDocumentProcess, []byte("not-json"))
	err := s.handleDocumentProcess(context.Background(), task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal DocumentProcessPayload")
}

// TestHandleURLScrapeInvalidPayload verifies that the handler returns
// an error for malformed JSON.
func TestHandleURLScrapeInvalidPayload(t *testing.T) {
	s, _ := newTestServer(t)

	task := asynq.NewTask(TypeURLScrape, []byte("not-json"))
	err := s.handleURLScrape(context.Background(), task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal URLScrapePayload")
}

// TestHandleReindexInvalidPayload verifies that the handler returns
// an error for malformed JSON.
func TestHandleReindexInvalidPayload(t *testing.T) {
	s, _ := newTestServer(t)

	task := asynq.NewTask(TypeReindex, []byte("not-json"))
	err := s.handleReindex(context.Background(), task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal ReindexPayload")
}
