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

// zitadelCfg returns a ZitadelConfig pointing at the given httptest server.
// The server URL is used as the domain with Secure=false so the issuer URL
// resolves to "http://<host>:<port>".
func zitadelCfg(srv *httptest.Server) *config.ZitadelConfig {
	// Strip the "http://" scheme — ZitadelConfig.Domain holds host:port only.
	domain := srv.URL[len("http://"):]
	return &config.ZitadelConfig{
		Domain:   domain,
		ClientID: "raven",
		Secure:   false,
	}
}

// setupRouter wires a test Gin router with the JWT middleware and a simple
// 200 OK handler at GET /protected that echoes external_id and email.
func setupRouter(cfg *config.ZitadelConfig) *gin.Engine {
	r := gin.New()
	protected := r.Group("/protected")
	protected.Use(JWTMiddleware(cfg))
	protected.GET("", func(c *gin.Context) {
		externalID, _ := c.Get(string(ContextKeyExternalID))
		email, _ := c.Get(string(ContextKeyEmail))
		c.JSON(http.StatusOK, gin.H{
			"external_id": externalID,
			"email":       email,
		})
	})
	return r
}

// validClaims builds a Claims struct that should pass validation.
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
		Email: "user@example.com",
		Name:  "Test User",
	}
}

// --------------------------------------------------------------------------
// Table-driven tests
// --------------------------------------------------------------------------

func TestJWTMiddleware(t *testing.T) {
	kp := newTestKeyPair(t)
	jwksSrv := startJWKSServer(t, kp)

	// The JWKS server serves keys at any path; JWTMiddleware will hit
	// <issuerURL>/oauth/v2/keys which the test server handles with the same
	// handler regardless of path.
	cfg := zitadelCfg(jwksSrv)
	issuerURL := jwksSrv.URL // http://<host>:<port>
	router := setupRouter(cfg)

	tests := []struct {
		name        string
		buildRequest func() *http.Request
		wantStatus  int
		wantErrCode string // expected value of {"error": "<code>"}
		wantExtID   string // only checked on 200
		wantEmail   string // only checked on 200
	}{
		{
			name: "valid JWT grants access",
			buildRequest: func() *http.Request {
				claims := validClaims(issuerURL, "zitadel-sub-42")
				tok := kp.sign(t, claims)
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				return req
			},
			wantStatus: http.StatusOK,
			wantExtID:  "zitadel-sub-42",
			wantEmail:  "user@example.com",
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
				claims := validClaims("https://evil.example.com", "user-bad-iss")
				tok := kp.sign(t, claims)
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				return req
			},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_token",
		},
		{
			// Audience validation is intentionally skipped — see parseJWT comment.
			name: "wrong audience is accepted (audience validation skipped)",
			buildRequest: func() *http.Request {
				claims := validClaims(issuerURL, "user-bad-aud")
				claims.Audience = jwt.ClaimStrings{"wrong-audience"}
				tok := kp.sign(t, claims)
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				return req
			},
			wantStatus: http.StatusOK,
			wantExtID:  "user-bad-aud",
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
				if tc.wantExtID != "" {
					assert.Equal(t, tc.wantExtID, body["external_id"])
				}
				if tc.wantEmail != "" {
					assert.Equal(t, tc.wantEmail, body["email"])
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
	cfg := zitadelCfg(jwksSrv)
	issuerURL := jwksSrv.URL

	r := gin.New()
	protected := r.Group("/protected")
	protected.Use(JWTMiddleware(cfg))
	protected.GET("", func(c *gin.Context) {
		externalID, _ := c.Get(string(ContextKeyExternalID))
		email, _ := c.Get(string(ContextKeyEmail))
		userName, _ := c.Get(string(ContextKeyUserName))
		claimsVal, _ := c.Get(string(ContextKeyClaims))

		assert.Equal(t, "sub-ctx-test", externalID)
		assert.Equal(t, "ctx@example.com", email)
		assert.Equal(t, "Context User", userName)
		assert.NotNil(t, claimsVal)

		// ContextKeyUserID and ContextKeyOrgID are NOT set by JWTMiddleware;
		// they are populated by downstream auth handlers after a DB lookup.
		userID, exists := c.Get(string(ContextKeyUserID))
		assert.False(t, exists, "user_id should not be set by JWTMiddleware")
		assert.Nil(t, userID)

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
		Email: "ctx@example.com",
		Name:  "Context User",
	}
	tok := kp.sign(t, claims)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireOrg verifies that RequireOrg allows requests with an org ID set
// and blocks those without one.
func TestRequireOrg(t *testing.T) {
	r := gin.New()
	r.GET("/with-org", func(c *gin.Context) {
		c.Set(string(ContextKeyOrgID), "org-abc")
		c.Next()
	}, RequireOrg(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/without-org", RequireOrg(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("org present returns 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/with-org", nil))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("org absent returns 403", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/without-org", nil))
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
