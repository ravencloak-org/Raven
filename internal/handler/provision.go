package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProvisionHandler exposes the internal realm auto-provisioning endpoint.
// Callers must authenticate with a bearer token matching the configured internal secret.
type ProvisionHandler struct {
	adminURL          string
	adminClientID     string
	adminClientSecret string
	internalSecret    string
}

// NewProvisionHandler creates a ProvisionHandler with the given Keycloak admin credentials.
func NewProvisionHandler(adminURL, adminClientID, adminClientSecret, internalSecret string) *ProvisionHandler {
	return &ProvisionHandler{
		adminURL:          adminURL,
		adminClientID:     adminClientID,
		adminClientSecret: adminClientSecret,
		internalSecret:    internalSecret,
	}
}

type provisionRealmRequest struct {
	RealmName    string   `json:"realm_name" binding:"required"`
	RedirectURIs []string `json:"redirect_uris"`
	WebOrigins   []string `json:"web_origins"`
}

type provisionRealmResponse struct {
	Realm string `json:"realm"`
}

// RequireInternalAuth is middleware that validates the internal API bearer token.
func (h *ProvisionHandler) RequireInternalAuth(c *gin.Context) {
	if h.internalSecret == "" {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "internal endpoint not configured"})
		return
	}
	auth := c.GetHeader("Authorization")
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] != h.internalSecret {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Next()
}

// ProvisionRealm handles POST /internal/provision-realm.
// It calls the Keycloak Admin REST API to create a realm with the raven-app
// client and org_admin / org_member roles.  Returns 200 on success or 409
// when the realm already exists.
func (h *ProvisionHandler) ProvisionRealm(c *gin.Context) {
	var req provisionRealmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	token, err := h.obtainAdminToken(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to obtain admin token: " + err.Error()})
		return
	}

	realmName := req.RealmName

	// Check whether the realm already exists.
	exists, err := h.realmExists(ctx, token, realmName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check realm existence: " + err.Error()})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "realm already exists", "realm": realmName})
		return
	}

	// Require explicit redirect URIs and web origins — never default to wildcard.
	if len(req.RedirectURIs) == 0 || len(req.WebOrigins) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "redirect_uris and web_origins are required"})
		return
	}
	redirectURIs := req.RedirectURIs
	webOrigins := req.WebOrigins

	// Build the realm representation.
	realmBody := buildRealmPayload(realmName, redirectURIs, webOrigins)
	realmJSON, err := json.Marshal(realmBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal realm payload"})
		return
	}

	adminRealmsURL := strings.TrimRight(h.adminURL, "/") + "/admin/realms"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, adminRealmsURL, bytes.NewReader(realmJSON))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request: " + err.Error()})
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "keycloak request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusConflict {
		c.JSON(http.StatusConflict, gin.H{"error": "realm already exists", "realm": realmName})
		return
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("keycloak returned %d: %s", resp.StatusCode, string(body))})
		return
	}

	c.JSON(http.StatusOK, provisionRealmResponse{Realm: realmName})
}

// obtainAdminToken exchanges client credentials for a Keycloak master-realm access token.
func (h *ProvisionHandler) obtainAdminToken(ctx context.Context) (string, error) {
	tokenURL := strings.TrimRight(h.adminURL, "/") + "/realms/master/protocol/openid-connect/token"

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", h.adminClientID)
	form.Set("client_secret", h.adminClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in response")
	}
	return result.AccessToken, nil
}

// realmExists returns true when the given realm is already registered in Keycloak.
func (h *ProvisionHandler) realmExists(ctx context.Context, token, realmName string) (bool, error) {
	checkURL := strings.TrimRight(h.adminURL, "/") + "/admin/realms/" + url.PathEscape(realmName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
}

// buildRealmPayload constructs the Keycloak realm JSON with the raven-app
// client and org_admin / org_member roles.
func buildRealmPayload(realmName string, redirectURIs, webOrigins []string) map[string]interface{} {
	return map[string]interface{}{
		"realm":               realmName,
		"displayName":         "Raven Platform",
		"enabled":             true,
		"registrationAllowed": true,
		"loginWithEmailAllowed": true,
		"sslRequired":         "external",
		"accessTokenLifespan": 300,
		"emailTheme":          "keycloak",
		"loginTheme":          "keycloak",
		"roles": map[string]interface{}{
			"realm": []map[string]interface{}{
				{
					"name":        "org_admin",
					"description": "Organization-level administrator",
					"composite":   false,
					"clientRole":  false,
				},
				{
					"name":        "org_member",
					"description": "Organization member with standard access",
					"composite":   false,
					"clientRole":  false,
				},
			},
		},
		"defaultRoles": []string{"org_member"},
		"clients": []map[string]interface{}{
			{
				"clientId":                  "raven-app",
				"name":                      "Raven App",
				"enabled":                   true,
				"publicClient":              true,
				"standardFlowEnabled":       true,
				"directAccessGrantsEnabled": false,
				"serviceAccountsEnabled":    false,
				"protocol":                  "openid-connect",
				"redirectUris":              redirectURIs,
				"webOrigins":                webOrigins,
				"attributes": map[string]string{
					"pkce.code.challenge.method": "S256",
				},
				"defaultClientScopes": []string{
					"openid", "profile", "email",
				},
			},
		},
	}
}
