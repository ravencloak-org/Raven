package email

import (
	"strings"
	"testing"
)

const testSecret = "this-is-a-32-byte-long-dummy-unit-test-secret"

func TestSignAndVerifyRoundTrip(t *testing.T) {
	tok, err := SignUnsubscribeToken(testSecret, "user-123", "ws-456")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if tok == "" || !strings.Contains(tok, ".") {
		t.Fatalf("unexpected token shape: %q", tok)
	}
	uid, wsID, err := VerifyUnsubscribeToken(testSecret, tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if uid != "user-123" || wsID != "ws-456" {
		t.Fatalf("decoded wrong values: %s %s", uid, wsID)
	}
}

func TestVerifyWithWrongSecretFails(t *testing.T) {
	tok, err := SignUnsubscribeToken(testSecret, "a", "b")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	_, _, err = VerifyUnsubscribeToken(strings.Repeat("x", 48), tok)
	if err == nil {
		t.Fatal("expected mismatch to fail verification")
	}
}

func TestSignRejectsShortSecret(t *testing.T) {
	_, err := SignUnsubscribeToken("short", "a", "b")
	if err == nil {
		t.Fatal("expected short-secret error")
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	tok, _ := SignUnsubscribeToken(testSecret, "user-1", "ws-1")
	parts := strings.SplitN(tok, ".", 2)
	tampered := strings.TrimSuffix(parts[0], "A") + "B." + parts[1]
	if _, _, err := VerifyUnsubscribeToken(testSecret, tampered); err == nil {
		t.Fatal("expected tampered token to fail verification")
	}
}
