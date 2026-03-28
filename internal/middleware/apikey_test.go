package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAPIKeyLookup implements APIKeyLookup for tests.
type mockAPIKeyLookup struct {
	lookupFn func(ctx context.Context, keyHash string) (*APIKeyLookupResult, error)
}

func (m *mockAPIKeyLookup) LookupByHash(ctx context.Context, keyHash string) (*APIKeyLookupResult, error) {
	return m.lookupFn(ctx, keyHash)
}

// testAPIKeyHash returns the SHA-256 hex digest for a test key.
func testAPIKeyHash(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func setupAPIKeyRouter(lookup APIKeyLookup) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(APIKeyAuth(lookup))
	r.GET("/test", func(c *gin.Context) {
		orgID, _ := c.Get(string(ContextKeyOrgID))
		kbID, _ := c.Get(string(ContextKeyKBID))
		apiKeyID, _ := c.Get(string(ContextKeyAPIKeyID))
		c.JSON(http.StatusOK, gin.H{
			"org_id":     orgID,
			"kb_id":      kbID,
			"api_key_id": apiKeyID,
		})
	})
	return r
}

func TestAPIKeyAuth(t *testing.T) {
	const testKey = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	testHash := testAPIKeyHash(testKey)

	tests := []struct {
		name        string
		apiKey      string
		origin      string
		lookup      *mockAPIKeyLookup
		wantStatus  int
		wantErrCode string
		wantOrgID   string
		wantKBID    string
	}{
		{
			name:        "missing X-API-Key returns 401",
			apiKey:      "",
			lookup:      &mockAPIKeyLookup{lookupFn: func(_ context.Context, _ string) (*APIKeyLookupResult, error) { return nil, nil }},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "missing_api_key",
		},
		{
			name:   "invalid key returns 401",
			apiKey: "bad-key",
			lookup: &mockAPIKeyLookup{lookupFn: func(_ context.Context, _ string) (*APIKeyLookupResult, error) {
				return nil, errors.New("not found")
			}},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_api_key",
		},
		{
			name:   "nil result returns 401",
			apiKey: "bad-key",
			lookup: &mockAPIKeyLookup{lookupFn: func(_ context.Context, _ string) (*APIKeyLookupResult, error) {
				return nil, nil
			}},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_api_key",
		},
		{
			name:   "revoked key returns 401",
			apiKey: testKey,
			lookup: &mockAPIKeyLookup{lookupFn: func(_ context.Context, hash string) (*APIKeyLookupResult, error) {
				if hash == testHash {
					return &APIKeyLookupResult{
						ID: "key-1", OrgID: "org-1", KnowledgeBaseID: "kb-1",
						Status: "revoked",
					}, nil
				}
				return nil, nil
			}},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "api_key_revoked",
		},
		{
			name:   "valid key with no domain restriction passes",
			apiKey: testKey,
			lookup: &mockAPIKeyLookup{lookupFn: func(_ context.Context, hash string) (*APIKeyLookupResult, error) {
				if hash == testHash {
					return &APIKeyLookupResult{
						ID: "key-1", OrgID: "org-1", KnowledgeBaseID: "kb-1",
						AllowedDomains: nil, RateLimit: 60, Status: "active",
					}, nil
				}
				return nil, nil
			}},
			wantStatus: http.StatusOK,
			wantOrgID:  "org-1",
			wantKBID:   "kb-1",
		},
		{
			name:   "valid key with matching origin passes",
			apiKey: testKey,
			origin: "https://app.example.com",
			lookup: &mockAPIKeyLookup{lookupFn: func(_ context.Context, hash string) (*APIKeyLookupResult, error) {
				if hash == testHash {
					return &APIKeyLookupResult{
						ID: "key-1", OrgID: "org-1", KnowledgeBaseID: "kb-1",
						AllowedDomains: []string{"app.example.com"}, RateLimit: 60, Status: "active",
					}, nil
				}
				return nil, nil
			}},
			wantStatus: http.StatusOK,
			wantOrgID:  "org-1",
			wantKBID:   "kb-1",
		},
		{
			name:   "valid key with non-matching origin returns 403",
			apiKey: testKey,
			origin: "https://evil.example.com",
			lookup: &mockAPIKeyLookup{lookupFn: func(_ context.Context, hash string) (*APIKeyLookupResult, error) {
				if hash == testHash {
					return &APIKeyLookupResult{
						ID: "key-1", OrgID: "org-1", KnowledgeBaseID: "kb-1",
						AllowedDomains: []string{"app.example.com"}, RateLimit: 60, Status: "active",
					}, nil
				}
				return nil, nil
			}},
			wantStatus:  http.StatusForbidden,
			wantErrCode: "domain_not_allowed",
		},
		{
			name:   "domain restriction with no origin returns 403",
			apiKey: testKey,
			origin: "",
			lookup: &mockAPIKeyLookup{lookupFn: func(_ context.Context, hash string) (*APIKeyLookupResult, error) {
				if hash == testHash {
					return &APIKeyLookupResult{
						ID: "key-1", OrgID: "org-1", KnowledgeBaseID: "kb-1",
						AllowedDomains: []string{"app.example.com"}, RateLimit: 60, Status: "active",
					}, nil
				}
				return nil, nil
			}},
			wantStatus:  http.StatusForbidden,
			wantErrCode: "domain_not_allowed",
		},
		{
			name:   "wildcard domain matches subdomain",
			apiKey: testKey,
			origin: "https://sub.example.com",
			lookup: &mockAPIKeyLookup{lookupFn: func(_ context.Context, hash string) (*APIKeyLookupResult, error) {
				if hash == testHash {
					return &APIKeyLookupResult{
						ID: "key-1", OrgID: "org-1", KnowledgeBaseID: "kb-1",
						AllowedDomains: []string{"*.example.com"}, RateLimit: 60, Status: "active",
					}, nil
				}
				return nil, nil
			}},
			wantStatus: http.StatusOK,
			wantOrgID:  "org-1",
		},
		{
			name:   "wildcard domain matches root domain",
			apiKey: testKey,
			origin: "https://example.com",
			lookup: &mockAPIKeyLookup{lookupFn: func(_ context.Context, hash string) (*APIKeyLookupResult, error) {
				if hash == testHash {
					return &APIKeyLookupResult{
						ID: "key-1", OrgID: "org-1", KnowledgeBaseID: "kb-1",
						AllowedDomains: []string{"*.example.com"}, RateLimit: 60, Status: "active",
					}, nil
				}
				return nil, nil
			}},
			wantStatus: http.StatusOK,
			wantOrgID:  "org-1",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r := setupAPIKeyRouter(tc.lookup)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.apiKey != "" {
				req.Header.Set("X-API-Key", tc.apiKey)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)

			if tc.wantErrCode != "" {
				var body map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, tc.wantErrCode, body["error"])
			}

			if tc.wantStatus == http.StatusOK {
				var body map[string]interface{}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				if tc.wantOrgID != "" {
					assert.Equal(t, tc.wantOrgID, body["org_id"])
				}
				if tc.wantKBID != "" {
					assert.Equal(t, tc.wantKBID, body["kb_id"])
				}
			}
		})
	}
}

func TestIsDomainAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed []string
		want    bool
	}{
		{"empty origin rejected", "", []string{"example.com"}, false},
		{"exact match", "https://example.com", []string{"example.com"}, true},
		{"case insensitive", "https://Example.COM", []string{"example.com"}, true},
		{"wildcard subdomain", "https://app.example.com", []string{"*.example.com"}, true},
		{"wildcard deep subdomain", "https://deep.sub.example.com", []string{"*.example.com"}, true},
		{"wildcard root match", "https://example.com", []string{"*.example.com"}, true},
		{"no match", "https://other.com", []string{"example.com"}, false},
		{"referer as origin with path", "https://example.com/page", []string{"example.com"}, true},
		{"origin with port", "https://example.com:8080", []string{"example.com"}, true},
		{"empty allowlist entry skipped", "https://example.com", []string{"", "example.com"}, true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := isDomainAllowed(tc.origin, tc.allowed)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"https://example.com", "example.com"},
		{"https://example.com:8080/path", "example.com"},
		{"http://sub.example.com/page?q=1", "sub.example.com"},
		{"example.com", "example.com"},
		{"", ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			got := extractHost(tc.raw)
			assert.Equal(t, tc.want, got)
		})
	}
}
