package queue

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDocumentProcessTask(t *testing.T) {
	p := DocumentProcessPayload{
		OrgID:           "org-123",
		DocumentID:      "doc-456",
		KnowledgeBaseID: "kb-789",
	}

	task, err := NewDocumentProcessTask(p)
	require.NoError(t, err)
	assert.Equal(t, TypeDocumentProcess, task.Type())

	// Verify roundtrip: payload should deserialise back to the original struct.
	var got DocumentProcessPayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestNewURLScrapeTask(t *testing.T) {
	p := URLScrapePayload{
		OrgID:           "org-123",
		SourceID:        "src-456",
		KnowledgeBaseID: "kb-789",
		URL:             "https://example.com/docs",
		CrawlDepth:      2,
	}

	task, err := NewURLScrapeTask(p)
	require.NoError(t, err)
	assert.Equal(t, TypeURLScrape, task.Type())

	var got URLScrapePayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestNewReindexTask(t *testing.T) {
	p := ReindexPayload{
		OrgID:           "org-123",
		KnowledgeBaseID: "kb-789",
	}

	task, err := NewReindexTask(p)
	require.NoError(t, err)
	assert.Equal(t, TypeReindex, task.Type())

	var got ReindexPayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestDocumentProcessPayloadJSON(t *testing.T) {
	p := DocumentProcessPayload{
		OrgID:           "org-1",
		DocumentID:      "doc-2",
		KnowledgeBaseID: "kb-3",
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	var decoded DocumentProcessPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, p, decoded)
}

func TestURLScrapePayloadJSON(t *testing.T) {
	p := URLScrapePayload{
		OrgID:           "org-1",
		SourceID:        "src-2",
		KnowledgeBaseID: "kb-3",
		URL:             "https://example.com",
		CrawlDepth:      0,
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	var decoded URLScrapePayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, p, decoded)
}

func TestReindexPayloadJSON(t *testing.T) {
	p := ReindexPayload{
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-3",
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	var decoded ReindexPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, p, decoded)
}
