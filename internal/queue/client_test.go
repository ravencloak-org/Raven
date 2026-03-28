package queue

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	c := NewClient(mr.Addr(), WithMaxRetry(3))
	t.Cleanup(func() { _ = c.Close() })
	return c, mr
}

func TestEnqueueDocumentProcess(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	err := c.EnqueueDocumentProcess(ctx, DocumentProcessPayload{
		OrgID:           "org-1",
		DocumentID:      "doc-2",
		KnowledgeBaseID: "kb-3",
	})
	require.NoError(t, err)
}

func TestEnqueueURLScrape(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	err := c.EnqueueURLScrape(ctx, URLScrapePayload{
		OrgID:           "org-1",
		SourceID:        "src-2",
		KnowledgeBaseID: "kb-3",
		URL:             "https://example.com",
		CrawlDepth:      1,
	})
	require.NoError(t, err)
}

func TestEnqueueReindex(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	err := c.EnqueueReindex(ctx, ReindexPayload{
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-3",
	})
	require.NoError(t, err)
}

func TestClientOptions(t *testing.T) {
	mr := miniredis.RunT(t)

	c := NewClient(mr.Addr(), WithMaxRetry(10))
	defer func() { _ = c.Close() }()

	assert.Equal(t, 10, c.maxRetry)
}

func TestClientDefaultMaxRetry(t *testing.T) {
	mr := miniredis.RunT(t)

	c := NewClient(mr.Addr())
	defer func() { _ = c.Close() }()

	assert.Equal(t, 5, c.maxRetry)
}
