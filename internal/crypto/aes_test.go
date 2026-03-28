package crypto_test

import (
	"bytes"
	"testing"

	"github.com/ravencloak-org/Raven/internal/crypto"
)

func validKey() []byte {
	return []byte("01234567890123456789012345678901") // exactly 32 bytes
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	key := validKey()
	plaintext := []byte("sk-proj-abc123secretkey")

	ciphertext, iv, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext must differ from plaintext")
	}
	if len(iv) == 0 {
		t.Fatal("iv must not be empty")
	}

	decrypted, err := crypto.Decrypt(ciphertext, iv, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("roundtrip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncrypt_UniqueNonce(t *testing.T) {
	key := validKey()
	plaintext := []byte("same-input")

	_, iv1, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("first Encrypt: %v", err)
	}
	_, iv2, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("second Encrypt: %v", err)
	}

	if bytes.Equal(iv1, iv2) {
		t.Error("two encryptions must produce different nonces")
	}
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	short := []byte("too-short")
	_, _, err := crypto.Encrypt([]byte("data"), short)
	if err == nil {
		t.Fatal("expected error for short key")
	}
	if err != crypto.ErrInvalidKeyLength {
		t.Errorf("expected ErrInvalidKeyLength, got %v", err)
	}
}

func TestDecrypt_InvalidKeyLength(t *testing.T) {
	_, err := crypto.Decrypt([]byte("data"), []byte("iv"), []byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
	if err != crypto.ErrInvalidKeyLength {
		t.Errorf("expected ErrInvalidKeyLength, got %v", err)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key := validKey()
	plaintext := []byte("secret-data")

	ciphertext, iv, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	wrongKey := []byte("99999999999999999999999999999999")
	_, err = crypto.Decrypt(ciphertext, iv, wrongKey)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := validKey()
	plaintext := []byte("integrity-check")

	ciphertext, iv, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Tamper with ciphertext
	ciphertext[0] ^= 0xff

	_, err = crypto.Decrypt(ciphertext, iv, key)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext (GCM authentication should fail)")
	}
}

func TestGenerateHint(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		want   string
	}{
		{"normal key", "sk-proj-abc123xyz", "...3xyz"},
		{"short key (4 chars)", "abcd", "...abcd"},
		{"very short key (2 chars)", "ab", "...ab"},
		{"empty key", "", "..."},
		{"exactly 5 chars", "12345", "...2345"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crypto.GenerateHint(tt.apiKey)
			if got != tt.want {
				t.Errorf("GenerateHint(%q) = %q, want %q", tt.apiKey, got, tt.want)
			}
		})
	}
}
