package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: spin up miniredis and return a ResponseCache + the miniredis server.
func setupCache(t *testing.T, ttl time.Duration) (*ResponseCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return NewResponseCache(client, ttl), mr
}

func TestCacheKey_Deterministic(t *testing.T) {
	// Same inputs must always produce the same key.
	key1 := CacheKey("kb-123", "What is RAG?")
	key2 := CacheKey("kb-123", "What is RAG?")
	assert.Equal(t, key1, key2)

	// Different kb_id must produce a different key.
	key3 := CacheKey("kb-999", "What is RAG?")
	assert.NotEqual(t, key1, key3)

	// Different query must produce a different key.
	key4 := CacheKey("kb-123", "How does RAG work?")
	assert.NotEqual(t, key1, key4)

	// Key must start with the expected prefix.
	assert.True(t, len(key1) > len(KeyPrefix))
	assert.Equal(t, KeyPrefix, key1[:len(KeyPrefix)])
}

func TestCacheKey_NormalizesQuery(t *testing.T) {
	// Leading/trailing whitespace and case differences should produce the same key.
	key1 := CacheKey("kb-1", "Hello World")
	key2 := CacheKey("kb-1", "  hello world  ")
	key3 := CacheKey("kb-1", "HELLO WORLD")
	key4 := CacheKey("kb-1", "  HELLO world ")

	assert.Equal(t, key1, key2)
	assert.Equal(t, key1, key3)
	assert.Equal(t, key1, key4)
}

func TestCache_SetAndGet(t *testing.T) {
	rc, _ := setupCache(t, 5*time.Minute)
	ctx := context.Background()

	resp := &CachedResponse{
		Text: "RAG stands for Retrieval-Augmented Generation.",
		Sources: []CachedSource{
			{
				DocumentID:   "doc-1",
				DocumentName: "ML Guide",
				ChunkText:    "RAG combines retrieval with generation...",
				Score:        0.95,
			},
		},
		Model:    "gpt-4o",
		CachedAt: time.Now().UTC().Truncate(time.Second),
	}

	err := rc.Set(ctx, "kb-1", "What is RAG?", resp)
	require.NoError(t, err)

	got, err := rc.Get(ctx, "kb-1", "What is RAG?")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, resp.Text, got.Text)
	assert.Equal(t, resp.Model, got.Model)
	assert.Equal(t, resp.CachedAt.Unix(), got.CachedAt.Unix())
	require.Len(t, got.Sources, 1)
	assert.Equal(t, "doc-1", got.Sources[0].DocumentID)
	assert.Equal(t, "ML Guide", got.Sources[0].DocumentName)
	assert.InDelta(t, float64(0.95), float64(got.Sources[0].Score), 0.001)
}

func TestCache_GetMiss(t *testing.T) {
	rc, _ := setupCache(t, 5*time.Minute)
	ctx := context.Background()

	got, err := rc.Get(ctx, "kb-nonexistent", "some query")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCache_TTLExpiry(t *testing.T) {
	rc, mr := setupCache(t, 2*time.Second)
	ctx := context.Background()

	resp := &CachedResponse{
		Text:     "answer",
		CachedAt: time.Now().UTC(),
	}

	err := rc.Set(ctx, "kb-1", "q", resp)
	require.NoError(t, err)

	// Should be present immediately.
	got, err := rc.Get(ctx, "kb-1", "q")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Fast-forward past TTL.
	mr.FastForward(3 * time.Second)

	got, err = rc.Get(ctx, "kb-1", "q")
	require.NoError(t, err)
	assert.Nil(t, got, "expected cache miss after TTL expiry")
}

func TestCache_Invalidate(t *testing.T) {
	rc, _ := setupCache(t, 5*time.Minute)
	ctx := context.Background()

	resp := &CachedResponse{Text: "cached", CachedAt: time.Now().UTC()}
	require.NoError(t, rc.Set(ctx, "kb-1", "q1", resp))

	// Confirm it exists.
	got, err := rc.Get(ctx, "kb-1", "q1")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Invalidate.
	require.NoError(t, rc.Invalidate(ctx, "kb-1", "q1"))

	// Should now be a miss.
	got, err = rc.Get(ctx, "kb-1", "q1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCache_InvalidateKB(t *testing.T) {
	rc, _ := setupCache(t, 5*time.Minute)
	ctx := context.Background()

	resp := &CachedResponse{Text: "answer", CachedAt: time.Now().UTC()}

	// Set three entries in the same KB.
	require.NoError(t, rc.Set(ctx, "kb-A", "q1", resp))
	require.NoError(t, rc.Set(ctx, "kb-A", "q2", resp))
	require.NoError(t, rc.Set(ctx, "kb-A", "q3", resp))

	// Set one entry in a different KB.
	require.NoError(t, rc.Set(ctx, "kb-B", "q1", resp))

	// Confirm all are present.
	for _, q := range []string{"q1", "q2", "q3"} {
		got, err := rc.Get(ctx, "kb-A", q)
		require.NoError(t, err)
		require.NotNil(t, got, "expected cache hit for kb-A/%s", q)
	}
	got, err := rc.Get(ctx, "kb-B", "q1")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Invalidate all of kb-A.
	require.NoError(t, rc.InvalidateKB(ctx, "kb-A"))

	// All kb-A entries should be gone.
	for _, q := range []string{"q1", "q2", "q3"} {
		got, err := rc.Get(ctx, "kb-A", q)
		require.NoError(t, err)
		assert.Nil(t, got, "expected cache miss for kb-A/%s after KB invalidation", q)
	}

	// kb-B should be unaffected.
	got, err = rc.Get(ctx, "kb-B", "q1")
	require.NoError(t, err)
	require.NotNil(t, got, "expected kb-B entry to survive kb-A invalidation")
}

func TestCache_InvalidateKB_Empty(t *testing.T) {
	rc, _ := setupCache(t, 5*time.Minute)
	ctx := context.Background()

	// Invalidating a KB with no entries should be a no-op (no error).
	err := rc.InvalidateKB(ctx, "kb-nonexistent")
	require.NoError(t, err)
}

func TestNewResponseCache_DefaultTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	rc := NewResponseCache(client, 0)
	assert.Equal(t, DefaultTTL, rc.ttl)
}
