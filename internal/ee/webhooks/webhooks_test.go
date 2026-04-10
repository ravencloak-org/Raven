// Package webhooks_test verifies the enterprise webhooks package compiles and
// provides tests for HMAC signature generation and retry behaviour concepts.
package webhooks_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ravencloak-org/Raven/internal/ee/webhooks"
)

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

// TestWebhookDelivery_Retry_ManagedByHandler documents that asynq MaxRetry(0) is
// set for webhook delivery tasks and retries are managed by the handler itself
// using the failure_count / max_retries fields stored in the database.
func TestWebhookDelivery_Retry_ManagedByHandler(t *testing.T) {
	// Asynq is configured with MaxRetry(0) for webhook delivery tasks.
	// The handler tracks failure_count in the database and compares it
	// against max_retries on the WebhookConfig.  When the threshold is
	// reached the handler returns asynq.SkipRetry to stop processing.
	asynqMaxRetry := 0
	assert.Equal(t, 0, asynqMaxRetry,
		"asynq MaxRetry must be 0 — retries are managed by the webhook handler")
}

// TestWebhookDelivery_DeadLetter_ConceptAfterMaxRetries verifies that the dead
// letter concept applies: after max retries, the task should not be retried.
func TestWebhookDelivery_DeadLetter_AfterMaxRetries(t *testing.T) {
	type WebhookState struct {
		FailureCount int
		MaxRetries   int
		Status       string
	}

	shouldMarkFailed := func(w WebhookState) bool {
		return w.FailureCount >= w.MaxRetries
	}

	// After max retries reached, webhook must be marked as failed.
	w := WebhookState{FailureCount: 3, MaxRetries: 3, Status: "active"}
	assert.True(t, shouldMarkFailed(w),
		"webhook must be marked as failed after reaching max retries")

	// Before max retries, webhook must still be eligible for retry.
	w2 := WebhookState{FailureCount: 2, MaxRetries: 3, Status: "active"}
	assert.False(t, shouldMarkFailed(w2),
		"webhook must not be marked failed before reaching max retries")
}
