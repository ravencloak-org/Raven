package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// newTestRateLimiter starts a miniredis instance and returns a RateLimiter
// backed by it, along with a cleanup function.
func newTestRateLimiter(t *testing.T) (*RateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rl := NewRateLimiter(client, nil)
	return rl, mr
}

// newRateLimitRouter builds a minimal Gin router that applies the given
// middleware and always responds 200 OK.
func newRateLimitRouter(mw gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

// doRequest fires a GET /test against the router and returns the recorder.
func doRequest(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	return w
}

// ── TestRateLimitBasic ───────────────────────────────────────────────────────

// TestRateLimitBasic verifies that requests under the limit are admitted and
// that the X-RateLimit-* headers are present on every response.
func TestRateLimitBasic(t *testing.T) {
	rl, _ := newTestRateLimiter(t)

	mw := RateLimitMiddleware(rl, 5, func(c *gin.Context) string {
		return "raven:rl:test:basic"
	}, keyPrefixFallback)
	r := newRateLimitRouter(mw)

	for i := 1; i <= 5; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
		if w.Header().Get("X-RateLimit-Limit") != "5" {
			t.Errorf("request %d: X-RateLimit-Limit missing or wrong", i)
		}
		if w.Header().Get("X-RateLimit-Remaining") == "" {
			t.Errorf("request %d: X-RateLimit-Remaining missing", i)
		}
		if w.Header().Get("X-RateLimit-Reset") == "" {
			t.Errorf("request %d: X-RateLimit-Reset missing", i)
		}
	}
}

// TestRateLimitExceeded verifies that the (limit+1)th request is rejected with
// 429 and the correct JSON body + Retry-After header.
func TestRateLimitExceeded(t *testing.T) {
	rl, _ := newTestRateLimiter(t)

	const limit = 3
	mw := RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
		return "raven:rl:test:exceeded"
	}, keyPrefixFallback)
	r := newRateLimitRouter(mw)

	// Exhaust the limit.
	for i := 0; i < limit; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d should succeed, got %d", i+1, w.Code)
		}
	}

	// Next request should be rejected.
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429 response")
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode 429 body: %v", err)
	}
	if body["error"] != "rate_limit_exceeded" {
		t.Errorf("expected error=rate_limit_exceeded, got %v", body["error"])
	}
	if body["retry_after"] == nil {
		t.Error("expected retry_after field in 429 body")
	}

	// Near-boundary assertion: X-RateLimit-Reset must be a Unix timestamp
	// within [now, now+60] seconds — i.e. within the current 1-minute window.
	resetStr := w.Header().Get("X-RateLimit-Reset")
	if resetStr == "" {
		t.Fatal("expected X-RateLimit-Reset header on 429 response")
	}
	resetTs, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		t.Fatalf("X-RateLimit-Reset is not a valid int64: %q", resetStr)
	}
	now := time.Now().Unix()
	if resetTs < now || resetTs > now+60 {
		t.Errorf("X-RateLimit-Reset %d is not within [now(%d), now+60(%d)]", resetTs, now, now+60)
	}
}

// TestRateLimitWindowReset verifies that the sliding window allows requests
// again after time has advanced past the window.
func TestRateLimitWindowReset(t *testing.T) {
	rl, mr := newTestRateLimiter(t)

	const limit = 2
	mw := RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
		return "raven:rl:test:window"
	}, keyPrefixFallback)
	r := newRateLimitRouter(mw)

	// Exhaust the limit.
	for i := 0; i < limit; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d should succeed, got %d", i+1, w.Code)
		}
	}

	// Verify we're now blocked.
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}

	// Fast-forward miniredis clock by 61 seconds so all sorted-set members
	// fall outside the 60-second window.
	mr.FastForward(61 * time.Second)

	// The window has reset; a new request should succeed.
	w = doRequest(r)
	if w.Code != http.StatusOK {
		t.Fatalf("after window reset expected 200, got %d", w.Code)
	}
}

// TestRateLimitValkeyFailureFallback verifies that when Valkey is unavailable
// the middleware admits the request (fail-open) rather than returning an error,
// and that all required X-RateLimit-* headers are still present on the response.
func TestRateLimitValkeyFailureFallback(t *testing.T) {
	rl, mr := newTestRateLimiter(t)

	// Close miniredis to simulate Valkey being down.
	mr.Close()

	mw := RateLimitMiddleware(rl, 1, func(c *gin.Context) string {
		return "raven:rl:test:failure"
	}, keyPrefixFallback)
	r := newRateLimitRouter(mw)

	w := doRequest(r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected fail-open (200) when Valkey is unavailable, got %d", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("expected X-RateLimit-Limit header in fail-open path")
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("expected X-RateLimit-Remaining header in fail-open path")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("expected X-RateLimit-Reset header in fail-open path")
	}
}

// TestRateLimitDifferentKeysIndependent verifies that two different keys have
// independent counters.
func TestRateLimitDifferentKeysIndependent(t *testing.T) {
	rl, _ := newTestRateLimiter(t)

	const limit = 2

	makeRouter := func(key string) *gin.Engine {
		mw := RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
			return key
		}, keyPrefixFallback)
		return newRateLimitRouter(mw)
	}

	r1 := makeRouter("raven:rl:test:keyA")
	r2 := makeRouter("raven:rl:test:keyB")

	// Exhaust key A.
	for i := 0; i < limit; i++ {
		doRequest(r1)
	}

	// Key A is now blocked.
	w := doRequest(r1)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("keyA: expected 429, got %d", w.Code)
	}

	// Key B should still be freely accessible.
	w = doRequest(r2)
	if w.Code != http.StatusOK {
		t.Fatalf("keyB: expected 200, got %d", w.Code)
	}
}

// TestRateLimitRemainingDecrement checks that X-RateLimit-Remaining decreases
// with each request.
func TestRateLimitRemainingDecrement(t *testing.T) {
	rl, _ := newTestRateLimiter(t)

	const limit = 5
	mw := RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
		return "raven:rl:test:decrement"
	}, keyPrefixFallback)
	r := newRateLimitRouter(mw)

	prev := limit
	for i := 0; i < limit; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
		rem, err := strconv.Atoi(w.Header().Get("X-RateLimit-Remaining"))
		if err != nil {
			t.Fatalf("request %d: could not parse X-RateLimit-Remaining: %v", i+1, err)
		}
		if rem >= prev {
			t.Errorf("request %d: remaining should decrease; prev=%d, got=%d", i+1, prev, rem)
		}
		prev = rem
	}
}

// TestByUserID verifies the ByUserID convenience wrapper uses the correct key
// prefix and admits/blocks accordingly.
func TestByUserID(t *testing.T) {
	rl, _ := newTestRateLimiter(t)

	const limit = 2

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Inject user_id into gin context before the rate-limit middleware.
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUserID), "user-abc")
		c.Next()
	})
	r.Use(ByUserID(rl, limit))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < limit; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("ByUserID request %d: expected 200, got %d", i+1, w.Code)
		}
	}
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("ByUserID: expected 429 after limit, got %d", w.Code)
	}
}

// TestByOrgID verifies the ByOrgID convenience wrapper.
func TestByOrgID(t *testing.T) {
	rl, _ := newTestRateLimiter(t)

	const limit = 2

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyOrgID), "org-xyz")
		c.Next()
	})
	r.Use(ByOrgID(rl, limit))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < limit; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("ByOrgID request %d: expected 200, got %d", i+1, w.Code)
		}
	}
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("ByOrgID: expected 429 after limit, got %d", w.Code)
	}
}

// TestByAPIKey verifies the ByAPIKey convenience wrapper hashes the raw key
// before using it as the Valkey key, and enforces rate limiting correctly.
func TestByAPIKey(t *testing.T) {
	const rawKey = "raw-secret-key-123"
	const limit = 2

	rl, mr := newTestRateLimiter(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ContextKeyAPIKey, rawKey)
		c.Next()
	})
	r.Use(ByAPIKey(rl, limit))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < limit; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("ByAPIKey request %d: expected 200, got %d", i+1, w.Code)
		}
	}
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("ByAPIKey: expected 429 after limit, got %d", w.Code)
	}

	// Assert that the raw key was NOT stored in Valkey, and that the hashed
	// form IS present — confirming ByAPIKey hashes the key before use.
	hash := sha256.Sum256([]byte(rawKey))
	expectedValkeyKey := keyPrefixAPIKey + fmt.Sprintf("%x", hash)

	allKeys := mr.Keys()
	for _, k := range allKeys {
		if k == rawKey {
			t.Errorf("raw API key %q must not appear as a Valkey key", rawKey)
		}
	}
	found := false
	for _, k := range allKeys {
		if k == expectedValkeyKey {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected hashed Valkey key %q to be present; keys in store: %v", expectedValkeyKey, allKeys)
	}
}

// TestNoKeyUsesFallbackKey verifies that when the key function returns "" the
// middleware falls back to an IP-based key and still enforces rate limiting,
// rather than silently passing the request through (fail-open).
func TestNoKeyUsesFallbackKey(t *testing.T) {
	rl, _ := newTestRateLimiter(t)

	const limit = 2
	mw := RateLimitMiddleware(rl, limit, func(c *gin.Context) string {
		return "" // no key — simulates identity lookup miss
	}, keyPrefixFallback)
	r := newRateLimitRouter(mw)

	// Requests up to the limit should succeed (fallback key is rate-limited).
	for i := 0; i < limit; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("fallback key: request %d expected 200, got %d", i+1, w.Code)
		}
	}

	// Once the fallback key's limit is exhausted the request must be rejected.
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("fallback key: expected 429 after limit, got %d", w.Code)
	}
}

// TestResetHeaderIsUnixTimestamp verifies that X-RateLimit-Reset looks like a
// reasonable Unix timestamp (within a couple of minutes of now).
func TestResetHeaderIsUnixTimestamp(t *testing.T) {
	rl, _ := newTestRateLimiter(t)

	mw := RateLimitMiddleware(rl, 10, func(c *gin.Context) string {
		return "raven:rl:test:reset-ts"
	}, keyPrefixFallback)
	r := newRateLimitRouter(mw)

	w := doRequest(r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resetStr := w.Header().Get("X-RateLimit-Reset")
	resetTs, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		t.Fatalf("X-RateLimit-Reset is not a valid int64: %q", resetStr)
	}

	now := time.Now().Unix()
	if resetTs < now || resetTs > now+120 {
		t.Errorf("X-RateLimit-Reset %d is not within [now, now+120] (%d)", resetTs, now)
	}
}

// ── Per-org tier-based rate limiting tests ───────────────────────────────────

// newOrgTierRouter builds a Gin router with the ByOrgTier middleware applied,
// injecting orgID into context before the limiter runs.
func newOrgTierRouter(rl *RateLimiter, resolver TierResolver, cfg TierConfig, group RouteGroup, orgID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if orgID != "" {
		r.Use(func(c *gin.Context) {
			c.Set(string(ContextKeyOrgID), orgID)
			c.Next()
		})
	}
	r.Use(ByOrgTier(rl, resolver, cfg, group))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

// TestByOrgTierFreeGeneral verifies free-tier general rate limiting at 60 RPM.
func TestByOrgTierFreeGeneral(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := TierConfig{
		Free: TierLimits{GeneralRPM: 3, CompletionRPM: 1},
	}
	resolver := &StaticTierResolver{Tier: "free"}
	r := newOrgTierRouter(rl, resolver, cfg, RouteGroupGeneral, "org-free-1")

	// 3 requests should succeed.
	for i := 0; i < 3; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 4th should be rejected.
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after free tier limit, got %d", w.Code)
	}

	// Verify Retry-After header.
	if w.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429 response")
	}
}

// TestByOrgTierFreeCompletion verifies free-tier completion rate limiting.
func TestByOrgTierFreeCompletion(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := TierConfig{
		Free: TierLimits{GeneralRPM: 60, CompletionRPM: 2},
	}
	resolver := &StaticTierResolver{Tier: "free"}
	r := newOrgTierRouter(rl, resolver, cfg, RouteGroupCompletion, "org-free-comp")

	for i := 0; i < 2; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("completion request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after completion limit, got %d", w.Code)
	}
}

// TestByOrgTierProHigherLimit verifies that pro tier has a higher limit.
func TestByOrgTierProHigherLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := TierConfig{
		Free: TierLimits{GeneralRPM: 2, CompletionRPM: 1},
		Pro:  TierLimits{GeneralRPM: 5, CompletionRPM: 3},
	}
	resolver := &StaticTierResolver{Tier: "pro"}
	r := newOrgTierRouter(rl, resolver, cfg, RouteGroupGeneral, "org-pro-1")

	// Pro tier should allow 5 requests (vs 2 for free).
	for i := 0; i < 5; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("pro request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after pro tier limit, got %d", w.Code)
	}
}

// TestByOrgTierEnterpriseUnlimited verifies that enterprise tier with
// CompletionRPM == -1 never rate-limits completion requests.
func TestByOrgTierEnterpriseUnlimited(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := TierConfig{
		Enterprise: TierLimits{GeneralRPM: 100, CompletionRPM: -1},
	}
	resolver := &StaticTierResolver{Tier: "enterprise"}
	r := newOrgTierRouter(rl, resolver, cfg, RouteGroupCompletion, "org-ent-1")

	// Even 50 requests should succeed (unlimited).
	for i := 0; i < 50; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("enterprise unlimited request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

// TestByOrgTierWidgetStricterLimit verifies the separate widget rate limit.
func TestByOrgTierWidgetStricterLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := TierConfig{
		Free:      TierLimits{GeneralRPM: 60, CompletionRPM: 10},
		WidgetRPM: 2, // very strict for testing
	}
	resolver := &StaticTierResolver{Tier: "free"}
	r := newOrgTierRouter(rl, resolver, cfg, RouteGroupWidget, "org-widget-1")

	for i := 0; i < 2; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("widget request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after widget limit, got %d", w.Code)
	}
}

// TestByOrgTierWindowReset verifies that the sliding window resets for tier-based
// rate limiting after time has advanced past the window.
func TestByOrgTierWindowReset(t *testing.T) {
	rl, mr := newTestRateLimiter(t)
	cfg := TierConfig{
		Free: TierLimits{GeneralRPM: 2, CompletionRPM: 1},
	}
	resolver := &StaticTierResolver{Tier: "free"}
	r := newOrgTierRouter(rl, resolver, cfg, RouteGroupGeneral, "org-window-reset")

	// Exhaust the limit.
	for i := 0; i < 2; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d should succeed, got %d", i+1, w.Code)
		}
	}

	// Should be blocked now.
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}

	// Fast-forward past the window.
	mr.FastForward(61 * time.Second)

	// Should be allowed again.
	w = doRequest(r)
	if w.Code != http.StatusOK {
		t.Fatalf("after window reset expected 200, got %d", w.Code)
	}
}

// TestByOrgTierDifferentOrgsIndependent verifies that different orgs on the same
// tier have independent counters.
func TestByOrgTierDifferentOrgsIndependent(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := TierConfig{
		Free: TierLimits{GeneralRPM: 2, CompletionRPM: 1},
	}
	resolver := &StaticTierResolver{Tier: "free"}

	r1 := newOrgTierRouter(rl, resolver, cfg, RouteGroupGeneral, "org-A")
	r2 := newOrgTierRouter(rl, resolver, cfg, RouteGroupGeneral, "org-B")

	// Exhaust org A.
	for i := 0; i < 2; i++ {
		doRequest(r1)
	}
	w := doRequest(r1)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("org-A: expected 429, got %d", w.Code)
	}

	// Org B should still succeed.
	w = doRequest(r2)
	if w.Code != http.StatusOK {
		t.Fatalf("org-B: expected 200, got %d", w.Code)
	}
}

// TestByOrgTierNoOrgFallsBackToIP verifies that when no org_id is in context,
// the middleware falls back to IP-based rate limiting.
func TestByOrgTierNoOrgFallsBackToIP(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := TierConfig{
		Free: TierLimits{GeneralRPM: 2, CompletionRPM: 1},
	}
	resolver := &StaticTierResolver{Tier: "free"}
	// Pass empty orgID — no org context injected.
	r := newOrgTierRouter(rl, resolver, cfg, RouteGroupGeneral, "")

	for i := 0; i < 2; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Fatalf("anon request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for anonymous fallback, got %d", w.Code)
	}
}

// TestByOrgTierRateLimitHeaders verifies that X-RateLimit-* and Retry-After
// headers are set correctly on both admitted and rejected responses.
func TestByOrgTierRateLimitHeaders(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := TierConfig{
		Free: TierLimits{GeneralRPM: 2, CompletionRPM: 1},
	}
	resolver := &StaticTierResolver{Tier: "free"}
	r := newOrgTierRouter(rl, resolver, cfg, RouteGroupGeneral, "org-headers")

	// First request — check headers on admitted response.
	w := doRequest(r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") != "2" {
		t.Errorf("expected X-RateLimit-Limit=2, got %q", w.Header().Get("X-RateLimit-Limit"))
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("expected X-RateLimit-Remaining header")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("expected X-RateLimit-Reset header")
	}

	// Exhaust and check 429 headers.
	doRequest(r)
	w = doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}
	if w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("expected X-RateLimit-Remaining=0 on 429, got %q", w.Header().Get("X-RateLimit-Remaining"))
	}
}

// TestValkeyTierResolver verifies that the Valkey-backed resolver reads
// the tier correctly from Valkey and falls back to "free" on miss.
func TestValkeyTierResolver(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	resolver := NewValkeyTierResolver(client, nil)

	// No key set — should default to "free".
	tier := resolver.Resolve(context.Background(), "org-missing")
	if tier != "free" {
		t.Errorf("expected 'free' for missing org, got %q", tier)
	}

	// Set a tier.
	mr.Set("raven:org_tier:org-pro", "pro")
	tier = resolver.Resolve(context.Background(), "org-pro")
	if tier != "pro" {
		t.Errorf("expected 'pro', got %q", tier)
	}

	// Set enterprise.
	mr.Set("raven:org_tier:org-ent", "enterprise")
	tier = resolver.Resolve(context.Background(), "org-ent")
	if tier != "enterprise" {
		t.Errorf("expected 'enterprise', got %q", tier)
	}

	// Invalid tier in Valkey — should default to "free".
	mr.Set("raven:org_tier:org-invalid", "platinum")
	tier = resolver.Resolve(context.Background(), "org-invalid")
	if tier != "free" {
		t.Errorf("expected 'free' for invalid tier, got %q", tier)
	}
}

// TestValkeyTierResolverDownFallback verifies that when Valkey is down,
// the resolver falls back to "free".
func TestValkeyTierResolverDownFallback(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	resolver := NewValkeyTierResolver(client, nil)

	mr.Close()

	tier := resolver.Resolve(context.Background(), "org-whatever")
	if tier != "free" {
		t.Errorf("expected 'free' when Valkey is down, got %q", tier)
	}
}

// TestStaticTierResolver verifies the static resolver returns a fixed tier.
func TestStaticTierResolver(t *testing.T) {
	resolver := &StaticTierResolver{Tier: "enterprise"}
	tier := resolver.Resolve(context.Background(), "any-org")
	if tier != "enterprise" {
		t.Errorf("expected 'enterprise', got %q", tier)
	}
}

// TestRouteGroupSeparation verifies that general and completion route groups
// have independent counters for the same org.
func TestRouteGroupSeparation(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := TierConfig{
		Free: TierLimits{GeneralRPM: 2, CompletionRPM: 2},
	}
	resolver := &StaticTierResolver{Tier: "free"}

	rGeneral := newOrgTierRouter(rl, resolver, cfg, RouteGroupGeneral, "org-rg-sep")
	rCompletion := newOrgTierRouter(rl, resolver, cfg, RouteGroupCompletion, "org-rg-sep")

	// Exhaust general limit.
	for i := 0; i < 2; i++ {
		doRequest(rGeneral)
	}
	w := doRequest(rGeneral)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("general: expected 429, got %d", w.Code)
	}

	// Completion should still be available.
	w = doRequest(rCompletion)
	if w.Code != http.StatusOK {
		t.Fatalf("completion should still be available, got %d", w.Code)
	}
}
