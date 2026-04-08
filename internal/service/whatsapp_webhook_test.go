package service_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/ravencloak-org/Raven/internal/service"
)

func TestVerifyWebhook_Success(t *testing.T) {
	svc := service.NewWhatsAppWebhookService(nil, nil, "my-verify-token", "app-secret")

	challenge, err := svc.VerifyWebhook("subscribe", "my-verify-token", "challenge-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if challenge != "challenge-abc" {
		t.Errorf("challenge = %q, want 'challenge-abc'", challenge)
	}
}

func TestVerifyWebhook_BadMode(t *testing.T) {
	svc := service.NewWhatsAppWebhookService(nil, nil, "tok", "secret")

	_, err := svc.VerifyWebhook("unsubscribe", "tok", "c")
	if err == nil {
		t.Fatal("expected error for bad mode")
	}
}

func TestVerifyWebhook_TokenMismatch(t *testing.T) {
	svc := service.NewWhatsAppWebhookService(nil, nil, "correct-token", "secret")

	_, err := svc.VerifyWebhook("subscribe", "wrong-token", "c")
	if err == nil {
		t.Fatal("expected error for token mismatch")
	}
}

func TestVerifyWebhook_EmptyChallenge(t *testing.T) {
	svc := service.NewWhatsAppWebhookService(nil, nil, "tok", "secret")

	_, err := svc.VerifyWebhook("subscribe", "tok", "")
	if err == nil {
		t.Fatal("expected error for empty challenge")
	}
}

func TestValidateSignature_ValidHMAC(t *testing.T) {
	appSecret := "test-secret-key"
	svc := service.NewWhatsAppWebhookService(nil, nil, "", appSecret)

	payload := []byte(`{"object":"whatsapp_business_account"}`)

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(payload)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !svc.ValidateSignature(payload, sig) {
		t.Error("expected signature to be valid")
	}
}

func TestValidateSignature_InvalidHMAC(t *testing.T) {
	svc := service.NewWhatsAppWebhookService(nil, nil, "", "real-secret")

	payload := []byte(`{"test":"data"}`)
	badSig := "sha256=0000000000000000000000000000000000000000000000000000000000000000"

	if svc.ValidateSignature(payload, badSig) {
		t.Error("expected signature to be invalid")
	}
}

func TestValidateSignature_MissingPrefix(t *testing.T) {
	svc := service.NewWhatsAppWebhookService(nil, nil, "", "secret")

	payload := []byte(`{}`)
	if svc.ValidateSignature(payload, "no-prefix") {
		t.Error("expected signature to be invalid without sha256= prefix")
	}
}

func TestValidateSignature_EmptySecret_SkipsValidation(t *testing.T) {
	svc := service.NewWhatsAppWebhookService(nil, nil, "", "")

	payload := []byte(`{}`)
	// When app secret is empty, validation is skipped (dev mode).
	if !svc.ValidateSignature(payload, "") {
		t.Error("expected validation to pass when app secret is empty")
	}
}

func TestValidateSignature_BadHex(t *testing.T) {
	svc := service.NewWhatsAppWebhookService(nil, nil, "", "secret")

	payload := []byte(`{}`)
	if svc.ValidateSignature(payload, "sha256=not-valid-hex!!") {
		t.Error("expected signature to be invalid with bad hex")
	}
}
