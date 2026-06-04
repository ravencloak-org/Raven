// Package service contains the business-logic layer for Raven.
//
// passkey.go is a thin wrapper around the SuperTokens core's WebAuthn REST
// API. It exists so the handler layer can stay HTTP-transport-agnostic and
// tests can inject a fake HTTP client (via httptest.NewServer) without
// reaching for the real SuperTokens SDK.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PasskeyCredential represents one WebAuthn credential as the SuperTokens
// core returns it on a list response. We intentionally model only the
// fields Raven needs — the core may include additional metadata
// (transports, attestation type, sign count) that we ignore.
type PasskeyCredential struct {
	CredentialID string     `json:"credentialId"`
	CreatedAt    time.Time  `json:"-"`
	LastUsedAt   *time.Time `json:"-"`
}

// passkeyCredentialWire is the JSON representation as the SuperTokens core
// emits it. Timestamps come back as epoch milliseconds rather than RFC3339
// strings — same convention used by every other SuperTokens core endpoint.
type passkeyCredentialWire struct {
	CredentialID string `json:"credentialId"`
	CreatedAt    int64  `json:"createdAt,omitempty"`
	LastUsedAt   *int64 `json:"lastUsedAt,omitempty"`
}

// passkeyListResponse is the body shape SuperTokens core returns from
// GET /recipe/webauthn/user/credentials.
type passkeyListResponse struct {
	Status      string                  `json:"status"`
	Credentials []passkeyCredentialWire `json:"credentials"`
}

// passkeyDeleteResponse is the body shape SuperTokens core returns from
// DELETE /recipe/webauthn/user/credential.
type passkeyDeleteResponse struct {
	Status string `json:"status"`
}

// ErrPasskeyNotFound signals that a credential lookup or delete targeted a
// credential the core has no record of. Handlers translate this into 404.
var ErrPasskeyNotFound = errors.New("passkey credential not found")

// httpDoer is the minimal HTTP interface PasskeyService needs. *http.Client
// satisfies it; tests can pass a fake.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// PasskeyService talks to the SuperTokens core's WebAuthn recipe over REST.
type PasskeyService struct {
	// baseURL is the SuperTokens core base URL (e.g. http://supertokens:3567).
	// Trailing slashes are stripped on construction so paths join cleanly.
	baseURL string
	// apiKey is the SuperTokens core API key. Sent as the `api-key` header
	// on every request. May be empty when the core is configured without a
	// key (local dev).
	apiKey string
	// client is the HTTP client used for every outbound call. Injectable so
	// tests can swap in a transport that points at httptest.NewServer.
	client httpDoer
}

// NewPasskeyService constructs a service. connectionURI is the same value
// the Go SDK uses (RAVEN_SUPERTOKENS_CONNECTION_URI / config.supertokens.
// connection_uri). apiKey may be empty.
//
// If client is nil a default *http.Client with a 5-second timeout is used —
// passkey list/delete are interactive paths so a long stall would hang the
// caller's Settings page.
func NewPasskeyService(connectionURI, apiKey string, client httpDoer) *PasskeyService {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &PasskeyService{
		baseURL: strings.TrimRight(connectionURI, "/"),
		apiKey:  apiKey,
		client:  client,
	}
}

// ListByUser returns every WebAuthn credential the SuperTokens core has
// recorded for the given external (SuperTokens) user ID. Returns an empty
// slice + nil error when the user has no credentials.
func (s *PasskeyService) ListByUser(ctx context.Context, externalUserID string) ([]PasskeyCredential, error) {
	if externalUserID == "" {
		return nil, errors.New("PasskeyService.ListByUser: empty externalUserID")
	}

	// Query-string carries the userId — SuperTokens core REST convention is
	// to take the user ID as a query parameter on `/recipe/<recipe>/user/*`
	// endpoints rather than as a path component.
	u := s.baseURL + "/recipe/webauthn/user/credentials?userId=" + url.QueryEscape(externalUserID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	s.addAuthHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("supertokens core unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supertokens core returned %d: %s", resp.StatusCode, string(body))
	}

	var listResp passkeyListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}
	if listResp.Status != "" && listResp.Status != "OK" {
		return nil, fmt.Errorf("supertokens core list status %q", listResp.Status)
	}

	out := make([]PasskeyCredential, 0, len(listResp.Credentials))
	for _, c := range listResp.Credentials {
		cred := PasskeyCredential{CredentialID: c.CredentialID}
		if c.CreatedAt > 0 {
			cred.CreatedAt = time.UnixMilli(c.CreatedAt).UTC()
		}
		if c.LastUsedAt != nil && *c.LastUsedAt > 0 {
			t := time.UnixMilli(*c.LastUsedAt).UTC()
			cred.LastUsedAt = &t
		}
		out = append(out, cred)
	}
	return out, nil
}

// DeleteCredential asks the SuperTokens core to delete a single WebAuthn
// credential. The caller is responsible for verifying ownership BEFORE
// invoking this (e.g. by calling ListByUser and confirming the credential
// belongs to the session user).
//
// Returns ErrPasskeyNotFound when the core responds that the credential
// does not exist; the handler should map this to HTTP 404.
func (s *PasskeyService) DeleteCredential(ctx context.Context, externalUserID, credentialID string) error {
	if externalUserID == "" {
		return errors.New("PasskeyService.DeleteCredential: empty externalUserID")
	}
	if credentialID == "" {
		return errors.New("PasskeyService.DeleteCredential: empty credentialID")
	}

	u := s.baseURL +
		"/recipe/webauthn/user/credential?userId=" + url.QueryEscape(externalUserID) +
		"&credentialId=" + url.QueryEscape(credentialID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	s.addAuthHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("supertokens core unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ErrPasskeyNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supertokens core returned %d: %s", resp.StatusCode, string(body))
	}

	var dr passkeyDeleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		// 200 without a body — accept it.
		return nil
	}
	switch dr.Status {
	case "", "OK":
		return nil
	case "CREDENTIAL_NOT_FOUND_ERROR", "UNKNOWN_CREDENTIAL_ERROR":
		return ErrPasskeyNotFound
	default:
		return fmt.Errorf("supertokens core delete status %q", dr.Status)
	}
}

// addAuthHeaders sets the standard SuperTokens core headers on a request.
func (s *PasskeyService) addAuthHeaders(req *http.Request) {
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}
	// SuperTokens core requires this CDI version header on every request
	// (the value pins the request/response contract). 5.1 is the version
	// shipped with the SuperTokens images Raven runs in compose/k8s.
	req.Header.Set("cdi-version", "5.1")
	req.Header.Set("Accept", "application/json")
}
