package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// rsaKeyPair holds an RSA key pair and its JWK representation for tests.
type rsaKeyPair struct {
	private *rsa.PrivateKey
	keyID   string
}

// newTestKeyPair generates a 2048-bit RSA key pair for use in tests.
func newTestKeyPair(t *testing.T) *rsaKeyPair {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return &rsaKeyPair{private: priv, keyID: "test-key-1"}
}

// jwksJSON returns a JWKS document containing the public key of this pair.
func (kp *rsaKeyPair) jwksJSON() []byte {
	pub := &kp.private.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())

	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": kp.keyID,
				"alg": "RS256",
				"n":   n,
				"e":   e,
			},
		},
	}
	b, _ := json.Marshal(jwks)
	return b
}

// sign mints a JWT with the given claims signed by the test key.
func (kp *rsaKeyPair) sign(t *testing.T, claims jwt.Claims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kp.keyID
	raw, err := tok.SignedString(kp.private)
	require.NoError(t, err)
	return raw
}

// startJWKSServer starts an httptest server that serves a static JWKS document.
// The caller is responsible for closing the server.
func startJWKSServer(t *testing.T, kp *rsaKeyPair) *httptest.Server {
	t.Helper()
	jwksData := kp.jwksJSON()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksData)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// keycloakCfg returns a KeycloakConfig whose IssuerURL points at the given
// httptest server, using the server URL directly as issuer.
func keycloakCfg(issuerURL string) *config.KeycloakConfig {
	return &config.KeycloakConfig{IssuerURL: issuerURL}
}

// setupRouter wires a test Gin router with the JWT middleware and a simple
// 200 OK handler at GET /protected.
func setupRouter(cfg *config.KeycloakConfig) *gin.Engine {
	r := gin.New()
	protected := r.Group("/protected")
	protected.Use(JWTMiddleware(cfg))
	protected.GET("", func(c *gin.Context) {
		userID, _ := c.Get(string(ContextKeyUserID))
		orgID, _ := c.Get(string(ContextKeyOrgID))
		c.JSON(http.StatusOK, gin.H{
			"user_id": userID,
			"org_id":  orgID,
		})
	})
	return r
}

// validClaims builds a full Claims struct that should pass validation.
func validClaims(issuerURL, subject string) *Claims {
	now := time.Now()
	return &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuerURL,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{"raven"},
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		OrgID:         "org-abc-123",
		OrgRole:       "admin",
		WorkspaceIDs:  []string{"ws-1", "ws-2"},
		KBPermissions: []string{"read", "write"},
		Email:         "user@example.com",
	}
}

// --------------------------------------------------------------------------
// Table-driven tests
// --------------------------------------------------------------------------

func TestJWTMiddleware(t *testing.T) {
	kp := newTestKeyPair(t)
	jwksSrv := startJWKSServer(t, kp)

	// The issuer URL is the JWKS server URL itself for test purposes; the
	// middleware appends /protocol/openid-connect/certs to it when fetching
	// JWKS, so we need the server to handle any path.
	issuerURL := jwksSrv.URL

	cfg := keycloakCfg(issuerURL)
	router := setupRouter(cfg)

	tests := []struct {
		name           string
		buildRequest   func() *http.Request
		wantStatus     int
		wantErrCode    string // expected value of {"error": "<code>"}
		wantUserID     string // only checked on 200
		wantOrgID      string // only checked on 200
	}{
		{
			name: "valid JWT grants access",
			buildRequest: func() *http.Request {
				claims := validClaims(issuerURL, "user-sub-42")
				tok := kp.sign(t, claims)
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				return req
			},
			wantStatus: http.StatusOK,
			wantUserID: "user-sub-42",
			wantOrgID:  "org-abc-123",
		},
		{
			name: "missing Authorization header returns missing_token",
			buildRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/protected", nil)
			},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "missing_token",
		},
		{
			name: "malformed Authorization header (no Bearer) returns missing_token",
			buildRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Token abc123")
				return req
			},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "missing_token",
		},
		{
			name: "expired token returns token_expired",
			buildRequest: func() *http.Request {
				claims := validClaims(issuerURL, "user-expired")
				claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))
				claims.NotBefore = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
				tok := kp.sign(t, claims)
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				return req
			},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "token_expired",
		},
		{
			name: "invalid signature returns invalid_token",
			buildRequest: func() *http.Request {
				// Sign with a different key so the JWKS validation fails.
				otherKP := newTestKeyPair(t)
				otherKP.keyID = kp.keyID // use same kid so the key IS found but sig fails
				claims := validClaims(issuerURL, "user-bad-sig")
				tok := otherKP.sign(t, claims)
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				return req
			},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_token",
		},
		{
			name: "wrong issuer returns invalid_token",
			buildRequest: func() *http.Request {
				claims := validClaims("https://evil.example.com/realms/raven", "user-bad-iss")
				tok := kp.sign(t, claims)
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				return req
			},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_token",
		},
		{
			name: "API key path stubs through with 200",
			buildRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("X-API-Key", "raven_test_key_abc")
				return req
			},
			wantStatus: http.StatusOK,
			wantUserID: "api-key-subject-placeholder",
		},
		{
			name: "empty Bearer token returns invalid_token",
			buildRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Bearer ")
				return req
			},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_token",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, tc.buildRequest())

			assert.Equal(t, tc.wantStatus, w.Code)

			if tc.wantErrCode != "" {
				var body authError
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, tc.wantErrCode, body.Error)
			}

			if tc.wantStatus == http.StatusOK {
				var body map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				if tc.wantUserID != "" {
					assert.Equal(t, tc.wantUserID, body["user_id"])
				}
				if tc.wantOrgID != "" {
					assert.Equal(t, tc.wantOrgID, body["org_id"])
				}
			}
		})
	}
}

// TestContextKeys verifies that all expected context keys are populated for a
// valid JWT so downstream handlers can rely on them.
func TestContextKeys(t *testing.T) {
	kp := newTestKeyPair(t)
	jwksSrv := startJWKSServer(t, kp)
	issuerURL := jwksSrv.URL
	cfg := keycloakCfg(issuerURL)

	r := gin.New()
	protected := r.Group("/protected")
	protected.Use(JWTMiddleware(cfg))
	protected.GET("", func(c *gin.Context) {
		userID, _ := c.Get(string(ContextKeyUserID))
		orgID, _ := c.Get(string(ContextKeyOrgID))
		orgRole, _ := c.Get(string(ContextKeyOrgRole))
		wsIDs, _ := c.Get(string(ContextKeyWorkspaceIDs))
		kbPerms, _ := c.Get(string(ContextKeyKBPermissions))
		email, _ := c.Get(string(ContextKeyEmail))
		claimsVal, _ := c.Get(string(ContextKeyClaims))

		assert.Equal(t, "sub-ctx-test", userID)
		assert.Equal(t, "org-xyz", orgID)
		assert.Equal(t, "editor", orgRole)
		assert.Equal(t, []string{"ws-a"}, wsIDs)
		assert.Equal(t, []string{"read"}, kbPerms)
		assert.Equal(t, "ctx@example.com", email)
		assert.NotNil(t, claimsVal)

		c.Status(http.StatusOK)
	})

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuerURL,
			Subject:   "sub-ctx-test",
			Audience:  jwt.ClaimStrings{"raven"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		OrgID:         "org-xyz",
		OrgRole:       "editor",
		WorkspaceIDs:  []string{"ws-a"},
		KBPermissions: []string{"read"},
		Email:         "ctx@example.com",
	}
	tok := kp.sign(t, claims)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
