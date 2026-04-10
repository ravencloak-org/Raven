package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProvisionHandler exposes the internal realm auto-provisioning endpoint.
// It is wired outside the auth middleware group — callers must be restricted
// to the compose-internal network via firewall / network policy.
type ProvisionHandler struct {
	adminURL           string
	adminClientID      string
	adminClientSecret  string
}

// NewProvisionHandler creates a ProvisionHandler with the given Keycloak admin credentials.
func NewProvisionHandler(adminURL, adminClientID, adminClientSecret string) *ProvisionHandler {
	return &ProvisionHandler{
		adminURL:          adminURL,
		adminClientID:     adminClientID,
		adminClientSecret: adminClientSecret,
	}
}

type provisionRealmRequest struct {
	RealmName string `json:"realm_name" binding:"required"`
}

type provisionRealmResponse struct {
	Realm string `json:"realm"`
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

	token, err := h.obtainAdminToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to obtain admin token: " + err.Error()})
		return
	}

	realmName := req.RealmName

	// Check whether the realm already exists.
	exists, err := h.realmExists(token, realmName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check realm existence: " + err.Error()})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "realm already exists", "realm": realmName})
		return
	}

	// Build the realm representation.
	realmBody := buildRealmPayload(realmName)
	realmJSON, err := json.Marshal(realmBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal realm payload"})
		return
	}

	adminRealmsURL := strings.TrimRight(h.adminURL, "/") + "/admin/realms"
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, adminRealmsURL, bytes.NewReader(realmJSON))
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
func (h *ProvisionHandler) obtainAdminToken() (string, error) {
	tokenURL := strings.TrimRight(h.adminURL, "/") + "/realms/master/protocol/openid-connect/token"

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", h.adminClientID)
	form.Set("client_secret", h.adminClientSecret)

	resp, err := http.PostForm(tokenURL, form) //nolint:gosec
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
func (h *ProvisionHandler) realmExists(token, realmName string) (bool, error) {
	checkURL := strings.TrimRight(h.adminURL, "/") + "/admin/realms/" + url.PathEscape(realmName)
	req, err := http.NewRequest(http.MethodGet, checkURL, nil)
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
func buildRealmPayload(realmName string) map[string]interface{} {
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
				"redirectUris":              []string{"*"},
				"webOrigins":                []string{"*"},
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
