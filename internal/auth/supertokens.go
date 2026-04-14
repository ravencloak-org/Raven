package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SuperTokensProvider implements AuthProvider using the SuperTokens Core HTTP API.
type SuperTokensProvider struct {
	connectionURI string // e.g. "http://supertokens:3567"
	apiKey        string
	httpClient    *http.Client
}

// NewSuperTokensProvider creates a provider that verifies sessions against
// the SuperTokens Core running at connectionURI.
func NewSuperTokensProvider(connectionURI, apiKey string) *SuperTokensProvider {
	return &SuperTokensProvider{
		connectionURI: strings.TrimRight(connectionURI, "/"),
		apiKey:        apiKey,
		httpClient:    &http.Client{},
	}
}

// VerifySession extracts the access token from cookies, calls the SuperTokens
// Core session/verify endpoint, then fetches user details.
func (p *SuperTokensProvider) VerifySession(r *http.Request) (*SessionInfo, error) {
	// 1. Extract access token from cookie
	accessToken, err := extractAccessToken(r)
	if err != nil {
		return nil, fmt.Errorf("missing session: %w", err)
	}

	// 2. Verify session with SuperTokens Core
	sessionResp, err := p.verifyAccessToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("session verification failed: %w", err)
	}

	// 3. Get user info
	userInfo, err := p.getUserInfo(sessionResp.UserID)
	if err != nil {
		return nil, fmt.Errorf("user lookup failed: %w", err)
	}

	return userInfo, nil
}

// RevokeSession removes the session identified by the request's access token.
func (p *SuperTokensProvider) RevokeSession(r *http.Request) error {
	accessToken, err := extractAccessToken(r)
	if err != nil {
		return fmt.Errorf("missing session: %w", err)
	}

	sessionResp, err := p.verifyAccessToken(accessToken)
	if err != nil {
		return fmt.Errorf("session verification failed: %w", err)
	}

	return p.removeSession(sessionResp.Handle)
}

// --- Internal types ---

type sessionVerifyRequest struct {
	AccessToken     string `json:"accessToken"`
	EnableAntiCSRF  bool   `json:"enableAntiCsrf"`
	DoAntiCSRFCheck bool   `json:"doAntiCsrfCheck"`
	CheckDatabase   bool   `json:"checkDatabase"`
}

type sessionVerifyResponse struct {
	Status  string      `json:"status"`
	Session sessionData `json:"session"`
}

type sessionData struct {
	Handle string `json:"handle"`
	UserID string `json:"userId"`
}

type userGetResponse struct {
	Status string      `json:"status"`
	User   userDetails `json:"user"`
}

type userDetails struct {
	ID         string          `json:"id"`
	Emails     []string        `json:"emails"`
	ThirdParty []thirdPartyInfo `json:"thirdParty,omitempty"`
}

type thirdPartyInfo struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
}

// --- Helpers ---

func extractAccessToken(r *http.Request) (string, error) {
	cookie, err := r.Cookie("sAccessToken")
	if err != nil {
		// Fallback: check Authorization header
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer "), nil
		}
		return "", fmt.Errorf("no sAccessToken cookie or Authorization header")
	}
	return cookie.Value, nil
}

func (p *SuperTokensProvider) verifyAccessToken(accessToken string) (*sessionData, error) {
	body := sessionVerifyRequest{
		AccessToken:     accessToken,
		EnableAntiCSRF:  false,
		DoAntiCSRFCheck: false,
		CheckDatabase:   false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, p.connectionURI+"/recipe/session/verify", strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("api-key", p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SuperTokens Core unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SuperTokens returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result sessionVerifyResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if result.Status != "OK" {
		return nil, fmt.Errorf("session invalid: %s", result.Status)
	}

	return &result.Session, nil
}

func (p *SuperTokensProvider) getUserInfo(userID string) (*SessionInfo, error) {
	req, err := http.NewRequest(http.MethodGet, p.connectionURI+"/user/id?userId="+userID, nil)
	if err != nil {
		return nil, err
	}
	if p.apiKey != "" {
		req.Header.Set("api-key", p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SuperTokens Core unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user lookup returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result userGetResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if result.Status != "OK" {
		return nil, fmt.Errorf("user not found: %s", result.Status)
	}

	info := &SessionInfo{
		ExternalID: result.User.ID,
	}
	if len(result.User.Emails) > 0 {
		info.Email = result.User.Emails[0]
	}

	return info, nil
}

func (p *SuperTokensProvider) removeSession(sessionHandle string) error {
	body := map[string]interface{}{
		"sessionHandles": []string{sessionHandle},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, p.connectionURI+"/recipe/session/remove", strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("api-key", p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SuperTokens Core unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session removal returned %d: %s", resp.StatusCode, string(b))
	}

	return nil
}
