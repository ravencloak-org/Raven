// Package cache provides an exact-match response cache backed by Valkey (Redis-compatible).
//
// Cache keys are derived from sha256(kb_id + normalised_query), ensuring
// deterministic lookups. Each knowledge base maintains a secondary set of
// its cache keys so that per-KB invalidation can be performed without a
// full key scan.
//
// This is the foundation layer for Phase 2 semantic caching.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultTTL is the default time-to-live for cached responses.
	DefaultTTL = 1 * time.Hour

	// KeyPrefix is the Redis key prefix for all RAG response cache entries.
	KeyPrefix = "raven:cache:rag:"

	// kbKeysPrefix is the prefix for per-KB secondary index sets.
	kbKeysPrefix = "raven:cache:kb_keys:"
)

// CachedResponse holds the cached RAG response.
type CachedResponse struct {
	Text     string         `json:"text"`
	Sources  []CachedSource `json:"sources"`
	Model    string         `json:"model"`
	CachedAt time.Time      `json:"cached_at"`
}

// CachedSource represents a single source document reference in a cached response.
type CachedSource struct {
	DocumentID   string  `json:"document_id"`
	DocumentName string  `json:"document_name"`
	ChunkText    string  `json:"chunk_text"`
	Score        float32 `json:"score"`
}

// ResponseCache provides exact-match caching for RAG responses.
type ResponseCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewResponseCache creates a new ResponseCache. If ttl is zero, DefaultTTL is used.
func NewResponseCache(client *redis.Client, ttl time.Duration) *ResponseCache {
	if ttl == 0 {
		ttl = DefaultTTL
	}
	return &ResponseCache{client: client, ttl: ttl}
}

// CacheKey generates a deterministic cache key from a knowledge-base ID and query string.
// The query is normalised (lowercased + trimmed) before hashing.
func CacheKey(kbID, query string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", kbID, normalizeQuery(query))))
	return KeyPrefix + hex.EncodeToString(h[:])
}

// kbSetKey returns the Redis key for the secondary set that tracks all cache
// keys belonging to a given knowledge base.
func kbSetKey(kbID string) string {
	return kbKeysPrefix + kbID
}

// normalizeQuery lowercases and trims whitespace for consistent cache hits.
func normalizeQuery(q string) string {
	return strings.ToLower(strings.TrimSpace(q))
}

// Get retrieves a cached response. Returns nil, nil if not found.
func (c *ResponseCache) Get(ctx context.Context, kbID, query string) (*CachedResponse, error) {
	key := CacheKey(kbID, query)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var resp CachedResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("cache unmarshal: %w", err)
	}
	return &resp, nil
}

// Set stores a response in the cache with TTL. It also adds the cache key to
// the per-KB secondary index set for efficient KB-level invalidation.
func (c *ResponseCache) Set(ctx context.Context, kbID, query string, resp *CachedResponse) error {
	key := CacheKey(kbID, query)

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}

	pipe := c.client.Pipeline()
	pipe.Set(ctx, key, data, c.ttl)
	pipe.SAdd(ctx, kbSetKey(kbID), key)
	// Keep the KB set alive as long as we have entries; refresh TTL on every write.
	pipe.Expire(ctx, kbSetKey(kbID), c.ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache set pipeline: %w", err)
	}
	return nil
}

// Invalidate removes a specific cache entry and its reference from the KB set.
func (c *ResponseCache) Invalidate(ctx context.Context, kbID, query string) error {
	key := CacheKey(kbID, query)

	pipe := c.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.SRem(ctx, kbSetKey(kbID), key)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache invalidate: %w", err)
	}
	return nil
}

// InvalidateKB removes all cache entries for a knowledge base.
// It reads the per-KB secondary index set, deletes every listed key, then
// deletes the set itself.
func (c *ResponseCache) InvalidateKB(ctx context.Context, kbID string) error {
	setKey := kbSetKey(kbID)

	members, err := c.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return fmt.Errorf("cache invalidate kb smembers: %w", err)
	}

	if len(members) == 0 {
		return nil
	}

	// Delete all cached response keys + the set itself in one pipeline.
	pipe := c.client.Pipeline()
	for _, key := range members {
		pipe.Del(ctx, key)
	}
	pipe.Del(ctx, setKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache invalidate kb pipeline: %w", err)
	}
	return nil
}
