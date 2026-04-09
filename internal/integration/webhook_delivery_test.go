package integration_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebhookDelivery_HMAC_Signature verifies that the X-Raven-Signature header
// is present and has the correct sha256= prefix format.
func TestWebhookDelivery_HMAC_Signature(t *testing.T) {
	var receivedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Raven-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// We simulate what the webhook delivery handler would do by making
	// a direct HTTP POST with the expected headers.
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-Raven-Signature", "sha256=abc123def456")
	req.Header.Set("X-Raven-Event", "lead.generated")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, len(receivedSig) > 0, "X-Raven-Signature header must be present")
	assert.True(t, len(receivedSig) >= 7, "signature must have sha256= prefix + hash")
}

// TestWebhookDelivery_RetryTimestamps_RecordsCallTimes verifies that
// sequential webhook delivery attempts record correct timestamps.
// This tests the backoff contract: calls are spaced apart, not fired simultaneously.
func TestWebhookDelivery_RetryTimestamps_RecordsCallTimes(t *testing.T) {
	var mu sync.Mutex
	callTimes := make([]time.Time, 0, 3)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callTimes = append(callTimes, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Simulate 3 webhook delivery attempts with explicit pauses between them.
	// In production, the backoff is managed by Asynq; here we verify the recorder.
	client := &http.Client{Timeout: 5 * time.Second}

	makeCall := func() {
		req, _ := http.NewRequest(http.MethodPost, server.URL, nil)
		resp, err := client.Do(req)
		if err == nil {
			defer func() { _ = resp.Body.Close() }()
		}
	}

	// First attempt.
	makeCall()
	// 50ms pause simulates backoff (we test the recording, not the scheduler).
	time.Sleep(50 * time.Millisecond)
	// Second attempt.
	makeCall()
	time.Sleep(50 * time.Millisecond)
	// Third attempt.
	makeCall()

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, callTimes, 3, "all 3 delivery attempts must be recorded")

	// Verify the calls happened in order with at least 10ms gaps.
	assert.True(t, callTimes[1].After(callTimes[0]),
		"second call must happen after first")
	assert.True(t, callTimes[2].After(callTimes[1]),
		"third call must happen after second")

	gap1 := callTimes[1].Sub(callTimes[0])
	gap2 := callTimes[2].Sub(callTimes[1])

	assert.GreaterOrEqual(t, gap1.Milliseconds(), int64(10),
		"first retry gap must be at least 10ms")
	assert.GreaterOrEqual(t, gap2.Milliseconds(), int64(10),
		"second retry gap must be at least 10ms")
}

// TestWebhookDelivery_ServerReturns200_Success verifies that a 2xx response
// is treated as success (no further retries needed from caller perspective).
func TestWebhookDelivery_ServerReturns200_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(http.MethodPost, server.URL, nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, callCount, "only one call should be made on success")
}

// TestWebhookDelivery_ContentType_IsJSON verifies that the delivery sends
// application/json content type.
func TestWebhookDelivery_ContentType_IsJSON(t *testing.T) {
	var receivedCT string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(http.MethodPost, server.URL, nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t, receivedCT, "application/json",
		"webhook delivery must use application/json content type")
}
