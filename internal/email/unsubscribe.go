package email

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// UnsubscribeSecretEnv is the environment variable holding the HMAC key used
// to sign unsubscribe tokens. Rotating this value invalidates every
// outstanding link — which is the intended failure mode for a key leak.
const UnsubscribeSecretEnv = "UNSUBSCRIBE_TOKEN_SECRET"

// ErrInvalidUnsubscribeToken is returned when a token fails signature or
// shape validation.
var ErrInvalidUnsubscribeToken = errors.New("email: invalid unsubscribe token")

// SignUnsubscribeToken returns a base64url-encoded HMAC-SHA256 token over
// "<userID>|<workspaceID>". The token is deliberately stateless so we never
// need to write a row when generating an email — only when verifying one.
//
// secret must be at least 32 bytes of high-entropy key material.
func SignUnsubscribeToken(secret, userID, workspaceID string) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("email: unsubscribe secret must be >= 32 bytes")
	}
	if userID == "" || workspaceID == "" {
		return "", errors.New("email: userID and workspaceID are required")
	}
	payload := fmt.Sprintf("%s|%s", userID, workspaceID)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	// Embed the payload in the token so verification is parameter-free.
	// The signature is computed ONLY over the payload — not the encoding.
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig, nil
}

// VerifyUnsubscribeToken returns the (userID, workspaceID) encoded in token
// when its HMAC matches. Constant-time comparison prevents timing attacks.
//
// Refuses to operate with secrets shorter than 32 bytes (matching
// SignUnsubscribeToken's contract) so a misconfigured unsubscribe handler
// cannot accept forged tokens signed with an empty/short HMAC key.
func VerifyUnsubscribeToken(secret, token string) (userID, workspaceID string, err error) {
	if len(secret) < 32 {
		return "", "", errors.New("email: unsubscribe secret must be >= 32 bytes")
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", "", ErrInvalidUnsubscribeToken
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", ErrInvalidUnsubscribeToken
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payloadBytes)
	expect := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expect), []byte(parts[1])) {
		return "", "", ErrInvalidUnsubscribeToken
	}
	fields := strings.SplitN(string(payloadBytes), "|", 2)
	if len(fields) != 2 || fields[0] == "" || fields[1] == "" {
		return "", "", ErrInvalidUnsubscribeToken
	}
	return fields[0], fields[1], nil
}
