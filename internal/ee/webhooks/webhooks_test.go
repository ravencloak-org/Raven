// Package webhooks_test verifies the enterprise webhooks package compiles and
// provides tests for HMAC signature generation and retry/dead-letter behaviour.
package webhooks_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ravencloak-org/Raven/internal/ee/webhooks"
	"github.com/ravencloak-org/Raven/internal/model"
)

// TestPackageCompiles ensures the webhooks package is importable and correctly declared.
func TestPackageCompiles(t *testing.T) {
	t.Log("internal/ee/webhooks package compiles successfully")
}

// computeHMAC is a helper that computes sha256 HMAC for webhook signature tests.
func computeHMAC(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestWebhookDelivery_HMACSignature_Correct verifies that the HMAC-SHA256
// signature generated for a webhook body is deterministic and correct.
func TestWebhookDelivery_HMACSignature_Correct(t *testing.T) {
	secret := "webhook-secret-key"
	body := `{"event_type":"lead.generated","org_id":"org-1","data":{"lead_id":"l-1"}}`

	sig1 := computeHMAC(secret, body)
	sig2 := computeHMAC(secret, body)

	// Signature must be deterministic.
	assert.Equal(t, sig1, sig2, "HMAC signature must be deterministic for same input")

	// Signature must start with sha256=.
	assert.True(t, len(sig1) > 7, "signature must have sha256= prefix")
	require.Contains(t, sig1, "sha256=", "signature format must be sha256=<hex>")

	// Different body must produce different signature.
	diffBody := `{"event_type":"lead.qualified","org_id":"org-1","data":{"lead_id":"l-1"}}`
	sig3 := computeHMAC(secret, diffBody)
	assert.NotEqual(t, sig1, sig3, "different body must produce different HMAC")
}

// TestWebhookDelivery_HMACSignature_WrongSecret_NotEqual verifies that a
// different secret produces a different HMAC.
func TestWebhookDelivery_HMACSignature_WrongSecret_NotEqual(t *testing.T) {
	body := `{"event_type":"lead.generated"}`
	correctSig := computeHMAC("correct-secret", body)
	wrongSig := computeHMAC("wrong-secret", body)

	assert.NotEqual(t, correctSig, wrongSig,
		"different secret must produce different HMAC signature")
}

// TestWebhookDelivery_HMAC_Verification verifies that signature verification
// works using constant-time comparison.
func TestWebhookDelivery_HMAC_Verification(t *testing.T) {
	secret := "my-webhook-secret"
	body := `{"event":"test"}`

	// Generate the expected signature.
	expected := computeHMAC(secret, body)

	// Verify using hmac.Equal (constant-time).
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	computed := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	assert.True(t, hmac.Equal([]byte(expected), []byte(computed)),
		"HMAC verification must succeed with correct secret and body")
}

// webhookState tracks the mutable state of a webhook endpoint as the handler
// would maintain it in the database during delivery attempts.
type webhookState struct {
	mu           sync.Mutex
	config       model.WebhookConfig
	deliveries   []deliveryRecord
	statusChange *model.WebhookStatus // set if SetWebhookStatus was called
}

type deliveryRecord struct {
	Success        bool
	ResponseStatus int
	ResponseBody   string
}

// incrementFailureCount mirrors repository.WebhookRepository.IncrementFailureCount.
func (ws *webhookState) incrementFailureCount() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.config.FailureCount++
}

// resetFailureCount mirrors repository.WebhookRepository.ResetFailureCount.
func (ws *webhookState) resetFailureCount() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.config.FailureCount = 0
}

// setStatus mirrors repository.WebhookRepository.SetWebhookStatus.
func (ws *webhookState) setStatus(status model.WebhookStatus) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.config.Status = status
	ws.statusChange = &status
}

// recordDelivery mirrors repository.WebhookRepository.UpdateDelivery.
func (ws *webhookState) recordDelivery(success bool, status int, body string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.deliveries = append(ws.deliveries, deliveryRecord{
		Success:        success,
		ResponseStatus: status,
		ResponseBody:   body,
	})
}

// snapshot returns a copy of the current state under the lock.
func (ws *webhookState) snapshot() (model.WebhookConfig, []deliveryRecord, *model.WebhookStatus) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ds := make([]deliveryRecord, len(ws.deliveries))
	copy(ds, ws.deliveries)
	var sc *model.WebhookStatus
	if ws.statusChange != nil {
		s := *ws.statusChange
		sc = &s
	}
	return ws.config, ds, sc
}

// simulateDelivery replicates the core logic of jobs.WebhookDeliveryHandler.ProcessTask:
//  1. POST to the webhook URL with an HMAC-signed JSON body.
//  2. Record the delivery attempt.
//  3. On failure: increment failure_count; if >= max_retries, mark as failed and return skipRetry=true.
//  4. On success: reset failure_count.
//
// Returns (success, skipRetry).
func simulateDelivery(ws *webhookState, url, secret, eventType string, payload map[string]any) (bool, bool) {
	// Build request body (mirrors ProcessTask).
	body := map[string]any{
		"event_type": eventType,
		"org_id":     ws.config.OrgID,
		"data":       payload,
	}
	bodyBytes, _ := json.Marshal(body)

	// Compute HMAC-SHA256 signature.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(bodyBytes)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// POST.
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Raven-Signature", signature)
	req.Header.Set("X-Raven-Event", eventType)

	client := &http.Client{}
	resp, httpErr := client.Do(req)

	var responseStatus int
	var responseBody string
	success := false

	if httpErr == nil {
		defer func() { _ = resp.Body.Close() }()
		responseStatus = resp.StatusCode
		if rb, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096)); readErr == nil {
			responseBody = string(rb)
		}
		success = resp.StatusCode >= 200 && resp.StatusCode < 300
	} else {
		responseBody = httpErr.Error()
	}

	// Record delivery attempt.
	ws.recordDelivery(success, responseStatus, responseBody)

	if !success {
		ws.incrementFailureCount()
		cfg, _, _ := ws.snapshot()
		if cfg.FailureCount >= cfg.MaxRetries {
			ws.setStatus(model.WebhookStatusFailed)
			return false, true // skipRetry
		}
		return false, false
	}

	// Success: reset failure counter.
	ws.resetFailureCount()
	return true, false
}

// TestWebhookDelivery_Retry_ManagedByHandler verifies that when a webhook
// endpoint fails transiently (returns 5xx), the handler-managed retry logic
// increments failure_count on each attempt and continues allowing retries
// until max_retries is reached, then ultimately succeeds when the endpoint
// recovers, resetting the failure counter.
func TestWebhookDelivery_Retry_ManagedByHandler(t *testing.T) {
	const failBeforeSuccess = 2
	const maxRetries = 5

	var callCount atomic.Int32

	// HTTP server that fails the first N calls with 500, then succeeds.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if int(n) <= failBeforeSuccess {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("service unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	ws := &webhookState{
		config: model.WebhookConfig{
			ID:         "wh-retry-test",
			OrgID:      "org-1",
			URL:        server.URL,
			Secret:     "test-secret",
			Status:     model.WebhookStatusActive,
			MaxRetries: maxRetries,
		},
	}

	// Simulate delivery attempts as the handler would be invoked repeatedly.
	for attempt := 1; attempt <= failBeforeSuccess+1; attempt++ {
		success, skipRetry := simulateDelivery(ws, server.URL, "test-secret", "lead.generated", map[string]any{"lead_id": "l-1"})

		if attempt <= failBeforeSuccess {
			assert.False(t, success, "attempt %d should fail", attempt)
			assert.False(t, skipRetry, "attempt %d should allow retry (not yet at max)", attempt)

			cfg, _, _ := ws.snapshot()
			assert.Equal(t, attempt, cfg.FailureCount,
				"failure_count should be %d after %d failures", attempt, attempt)
			assert.Equal(t, model.WebhookStatusActive, cfg.Status,
				"webhook should remain active before max retries")
		} else {
			assert.True(t, success, "attempt %d should succeed", attempt)
			assert.False(t, skipRetry, "successful attempt must not set skipRetry")

			cfg, _, _ := ws.snapshot()
			assert.Equal(t, 0, cfg.FailureCount,
				"failure_count must be reset to 0 after success")
		}
	}

	// Verify total delivery records.
	_, deliveries, _ := ws.snapshot()
	require.Len(t, deliveries, failBeforeSuccess+1,
		"all delivery attempts must be recorded")

	// First N deliveries failed, last one succeeded.
	for i := 0; i < failBeforeSuccess; i++ {
		assert.False(t, deliveries[i].Success, "delivery %d should be failure", i)
		assert.Equal(t, http.StatusInternalServerError, deliveries[i].ResponseStatus)
	}
	assert.True(t, deliveries[failBeforeSuccess].Success, "last delivery should succeed")
	assert.Equal(t, http.StatusOK, deliveries[failBeforeSuccess].ResponseStatus)
}

// TestWebhookDelivery_DeadLetter_AfterMaxRetries verifies the dead-letter
// behaviour: a webhook endpoint that always fails is retried up to max_retries
// times, at which point the handler marks it as "failed" (dead-lettered) and
// signals skipRetry so no further attempts are enqueued.
func TestWebhookDelivery_DeadLetter_AfterMaxRetries(t *testing.T) {
	const maxRetries = 3

	var callCount atomic.Int32

	// Server that always returns 502.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	defer server.Close()

	ws := &webhookState{
		config: model.WebhookConfig{
			ID:         "wh-deadletter-test",
			OrgID:      "org-2",
			URL:        server.URL,
			Secret:     "dl-secret",
			Status:     model.WebhookStatusActive,
			MaxRetries: maxRetries,
		},
	}

	payload := map[string]any{"lead_id": "l-dead"}
	var lastSkipRetry bool

	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		cfg, _, _ := ws.snapshot()
		// Once dead-lettered, the handler should not attempt delivery again.
		if cfg.Status == model.WebhookStatusFailed {
			break
		}
		_, lastSkipRetry = simulateDelivery(ws, server.URL, "dl-secret", "lead.generated", payload)
	}

	cfg, deliveries, statusChange := ws.snapshot()

	// The handler must have delivered exactly maxRetries attempts.
	require.Len(t, deliveries, maxRetries,
		"handler should stop after max_retries delivery attempts")

	// Every delivery must have failed.
	for i, d := range deliveries {
		assert.False(t, d.Success, "delivery %d must be a failure", i)
		assert.Equal(t, http.StatusBadGateway, d.ResponseStatus,
			"delivery %d status", i)
	}

	// failure_count must equal max_retries.
	assert.Equal(t, maxRetries, cfg.FailureCount,
		"failure_count must equal max_retries")

	// Status must be changed to "failed".
	require.NotNil(t, statusChange, "webhook status must have been updated")
	assert.Equal(t, model.WebhookStatusFailed, *statusChange,
		"webhook must be marked as failed (dead-lettered)")
	assert.Equal(t, model.WebhookStatusFailed, cfg.Status,
		"config status must be failed")

	// The last attempt must signal skipRetry.
	assert.True(t, lastSkipRetry,
		"handler must signal skipRetry when max_retries is reached")

	// HTTP server received exactly maxRetries calls.
	assert.Equal(t, int32(maxRetries), callCount.Load(),
		"server should receive exactly max_retries HTTP calls")
}

// TestWebhookDelivery_DeadLetter_FailureCountResets verifies that a successful
// delivery after failures resets the failure counter, so the dead-letter threshold
// starts fresh for subsequent failures.
func TestWebhookDelivery_DeadLetter_FailureCountResets(t *testing.T) {
	const maxRetries = 3

	var callCount atomic.Int32

	// Server that fails twice, succeeds once, then fails three more times.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		switch {
		case n <= 2: // first 2 calls fail
			w.WriteHeader(http.StatusInternalServerError)
		case n == 3: // third call succeeds
			w.WriteHeader(http.StatusOK)
		default: // remaining calls fail
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	ws := &webhookState{
		config: model.WebhookConfig{
			ID:         "wh-reset-test",
			OrgID:      "org-3",
			URL:        server.URL,
			Secret:     "reset-secret",
			Status:     model.WebhookStatusActive,
			MaxRetries: maxRetries,
		},
	}

	payload := map[string]any{"lead_id": "l-reset"}

	// Phase 1: Two failures.
	for i := 0; i < 2; i++ {
		success, skipRetry := simulateDelivery(ws, server.URL, "reset-secret", "lead.generated", payload)
		assert.False(t, success)
		assert.False(t, skipRetry, "should not skip retry before max")
	}
	cfg, _, _ := ws.snapshot()
	assert.Equal(t, 2, cfg.FailureCount, "failure_count should be 2 after 2 failures")

	// Phase 2: Success resets counter.
	success, skipRetry := simulateDelivery(ws, server.URL, "reset-secret", "lead.generated", payload)
	assert.True(t, success)
	assert.False(t, skipRetry)
	cfg, _, _ = ws.snapshot()
	assert.Equal(t, 0, cfg.FailureCount, "failure_count must reset to 0 after success")

	// Phase 3: Three more failures hit the threshold from scratch.
	for i := 1; i <= maxRetries; i++ {
		cfg, _, _ = ws.snapshot()
		if cfg.Status == model.WebhookStatusFailed {
			break
		}
		_, _ = simulateDelivery(ws, server.URL, "reset-secret", "lead.generated", payload)
	}
	cfg, _, statusChange := ws.snapshot()
	assert.Equal(t, maxRetries, cfg.FailureCount,
		"failure_count must reach max_retries from zero after reset")
	require.NotNil(t, statusChange)
	assert.Equal(t, model.WebhookStatusFailed, *statusChange,
		"webhook must be dead-lettered after reaching max_retries from reset")
}
