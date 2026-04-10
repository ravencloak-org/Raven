// Package keycloak provides a minimal Keycloak Admin REST API client used for
// automated realm provisioning during tenant onboarding.
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AdminClient calls the Keycloak Admin REST API using a client-credentials OAuth 2.0
// token obtained from the configured admin realm.
type AdminClient struct {
	baseURL      string // e.g. "http://localhost:8080"
	adminRealm   string // realm used to obtain the admin token, e.g. "master"
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

// NewAdminClient constructs an AdminClient. Pass nil for httpClient to use http.DefaultClient.
func NewAdminClient(baseURL, adminRealm, clientID, clientSecret string, httpClient *http.Client) *AdminClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &AdminClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		adminRealm:   adminRealm,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   httpClient,
	}
}

// tokenResponse holds the relevant fields from the Keycloak token endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

// getToken obtains a client-credentials Bearer token from Keycloak.
func (c *AdminClient) getToken(ctx context.Context) (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, c.adminRealm)

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("keycloak admin: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak admin: token request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("keycloak admin: read token body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak admin: token endpoint returned %d: %s", resp.StatusCode, body)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("keycloak admin: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("keycloak admin: empty access_token in response")
	}
	return tr.AccessToken, nil
}

// ImportRealm posts a realm representation JSON to the Admin API.
// It is idempotent: a 409 Conflict (realm already exists) is treated as success.
func (c *AdminClient) ImportRealm(ctx context.Context, realmJSON []byte) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("keycloak admin: obtain token: %w", err)
	}

	importURL := fmt.Sprintf("%s/admin/realms", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, importURL, bytes.NewReader(realmJSON))
	if err != nil {
		return fmt.Errorf("keycloak admin: build import request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("keycloak admin: import request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusConflict:
		// Realm already exists — idempotent, treat as success.
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak admin: import realm returned %d: %s", resp.StatusCode, body)
	}
}
