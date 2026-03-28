// Package crypto provides AES-256-GCM encryption utilities for securing
// sensitive data such as LLM provider API keys.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidKeyLength is returned when the encryption key is not exactly 32 bytes.
var ErrInvalidKeyLength = errors.New("encryption key must be exactly 32 bytes for AES-256")

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// The key must be exactly 32 bytes. Returns (ciphertext, iv/nonce, error).
func Encrypt(plaintext []byte, key []byte) ([]byte, []byte, error) {
	if len(key) != 32 {
		return nil, nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("nonce generation: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt decrypts ciphertext encrypted with AES-256-GCM.
// The key must be exactly 32 bytes and iv must be the nonce used during encryption.
func Decrypt(ciphertext []byte, iv []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm.Open: %w", err)
	}

	return plaintext, nil
}

// GenerateHint returns a masked hint of the API key for display purposes.
// Shows the last 4 characters prefixed with "...". If the key has fewer
// than 4 characters, the entire key is shown with the prefix.
func GenerateHint(apiKey string) string {
	if len(apiKey) <= 4 {
		return "..." + apiKey
	}
	return "..." + apiKey[len(apiKey)-4:]
}
