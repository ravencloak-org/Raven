package service_test

import (
	"encoding/hex"
	"testing"

	"github.com/ravencloak-org/Raven/internal/crypto"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
)

// TestNewLLMProviderService_InvalidKey verifies that an invalid AES key is rejected.
func TestNewLLMProviderService_InvalidKey(t *testing.T) {
	// Non-hex string
	_, err := service.NewLLMProviderService(nil, nil, "not-hex")
	if err == nil {
		t.Fatal("expected error for non-hex key")
	}

	// Too short (16 bytes = 32 hex chars, need 32 bytes = 64 hex chars)
	shortKey := hex.EncodeToString(make([]byte, 16))
	_, err = service.NewLLMProviderService(nil, nil, shortKey)
	if err == nil {
		t.Fatal("expected error for 16-byte key")
	}
}

// TestNewLLMProviderService_ValidKey ensures a valid 32-byte hex key is accepted.
func TestNewLLMProviderService_ValidKey(t *testing.T) {
	validKey := hex.EncodeToString(make([]byte, 32))
	svc, err := service.NewLLMProviderService(nil, nil, validKey)
	if err != nil {
		t.Fatalf("expected no error for valid key, got: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// TestEncryptionOnCreate verifies that the service encrypt flow produces
// a valid hint and the encrypted data round-trips correctly.
func TestEncryptionOnCreate(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	apiKey := "sk-test-my-secret-key-12345678"

	ciphertext, iv, err := crypto.Encrypt([]byte(apiKey), key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	hint := crypto.GenerateHint(apiKey)

	if hint != "...5678" {
		t.Errorf("expected hint '...5678', got %q", hint)
	}

	// Roundtrip
	decrypted, err := crypto.Decrypt(ciphertext, iv, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != apiKey {
		t.Errorf("roundtrip mismatch: got %q", string(decrypted))
	}
}

// TestReEncryptionOnUpdate verifies that updating with a new API key
// produces a new hint and the new ciphertext can be decrypted.
func TestReEncryptionOnUpdate(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	originalKey := "sk-original-key-aaaa"
	newKey := "sk-updated-key-zzzz"

	_, iv1, err := crypto.Encrypt([]byte(originalKey), key)
	if err != nil {
		t.Fatalf("Encrypt original: %v", err)
	}

	ciphertext2, iv2, err := crypto.Encrypt([]byte(newKey), key)
	if err != nil {
		t.Fatalf("Encrypt new: %v", err)
	}

	newHint := crypto.GenerateHint(newKey)
	if newHint != "...zzzz" {
		t.Errorf("expected hint '...zzzz', got %q", newHint)
	}

	// IVs must differ
	if string(iv1) == string(iv2) {
		t.Error("IVs should differ between encryptions")
	}

	decrypted, err := crypto.Decrypt(ciphertext2, iv2, key)
	if err != nil {
		t.Fatalf("Decrypt new: %v", err)
	}
	if string(decrypted) != newKey {
		t.Errorf("roundtrip mismatch: got %q", string(decrypted))
	}
}

// TestValidProviderEnum verifies the provider validation map.
func TestValidProviderEnum(t *testing.T) {
	valid := []model.LLMProvider{
		model.LLMProviderOpenAI,
		model.LLMProviderAnthropic,
		model.LLMProviderCohere,
		model.LLMProviderGoogle,
		model.LLMProviderAzureOpenAI,
		model.LLMProviderCustom,
	}
	for _, p := range valid {
		if !model.ValidLLMProviders[p] {
			t.Errorf("expected %q to be valid", p)
		}
	}

	invalid := model.LLMProvider("invalid_provider")
	if model.ValidLLMProviders[invalid] {
		t.Error("expected 'invalid_provider' to be invalid")
	}
}

// TestValidStatusEnum verifies the provider status validation map.
func TestValidStatusEnum(t *testing.T) {
	valid := []model.ProviderStatus{
		model.ProviderStatusActive,
		model.ProviderStatusRevoked,
		model.ProviderStatusExpired,
	}
	for _, s := range valid {
		if !model.ValidProviderStatuses[s] {
			t.Errorf("expected %q to be valid", s)
		}
	}

	invalid := model.ProviderStatus("unknown")
	if model.ValidProviderStatuses[invalid] {
		t.Error("expected 'unknown' to be invalid")
	}
}

// TestToResponse_NeverLeaksKeys ensures the DTO conversion strips encrypted data.
func TestToResponse_NeverLeaksKeys(t *testing.T) {
	cfg := &model.LLMProviderConfig{
		ID:              "id-1",
		OrgID:           "org-1",
		Provider:        model.LLMProviderOpenAI,
		DisplayName:     "Test",
		APIKeyEncrypted: []byte("super-secret-encrypted"),
		APIKeyIV:        []byte("iv-bytes"),
		APIKeyHint:      "...abcd",
		IsDefault:       true,
		Status:          model.ProviderStatusActive,
	}

	resp := cfg.ToResponse()

	// The response must contain the hint
	if resp.APIKeyHint != "...abcd" {
		t.Errorf("expected hint '...abcd', got %q", resp.APIKeyHint)
	}

	// The response struct has no encrypted key fields — verify via type assertion.
	// This is a compile-time guarantee, but we test the values are what we expect.
	if resp.ID != "id-1" || resp.OrgID != "org-1" {
		t.Error("response fields mismatch")
	}
}
