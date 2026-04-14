package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockAuthProvider is a test double for auth.AuthProvider.
type mockAuthProvider struct {
	info *auth.SessionInfo
	err  error
}

func (m *mockAuthProvider) VerifySession(_ *http.Request) (*auth.SessionInfo, error) {
	return m.info, m.err
}

func (m *mockAuthProvider) RevokeSession(_ *http.Request) error {
	return nil
}

// setupSessionRouter builds a test Gin router using SessionMiddleware and a
// simple handler that echoes external_id and email from the context.
func setupSessionRouter(provider auth.AuthProvider) *gin.Engine {
	r := gin.New()
	protected := r.Group("/protected")
	protected.Use(SessionMiddleware(provider))
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

// TestSessionMiddleware verifies that SessionMiddleware sets context keys on
// success and returns 401 when the provider rejects the session.
func TestSessionMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		provider    auth.AuthProvider
		wantStatus  int
		wantErrCode string
		wantExtID   string
		wantEmail   string
	}{
		{
			name: "valid session grants access",
			provider: &mockAuthProvider{info: &auth.SessionInfo{
				ExternalID: "st-user-42",
				Email:      "user@example.com",
				Name:       "Test User",
			}},
			wantStatus: http.StatusOK,
			wantExtID:  "st-user-42",
			wantEmail:  "user@example.com",
		},
		{
			name:        "invalid session returns 401",
			provider:    &mockAuthProvider{err: errors.New("session expired")},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_session",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			router := setupSessionRouter(tc.provider)
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

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

// TestSessionMiddlewareContextKeys verifies all expected context keys are set
// for a valid session so downstream handlers can rely on them.
func TestSessionMiddlewareContextKeys(t *testing.T) {
	provider := &mockAuthProvider{info: &auth.SessionInfo{
		ExternalID: "sub-ctx-test",
		Email:      "ctx@example.com",
		Name:       "Context User",
	}}

	r := gin.New()
	protected := r.Group("/protected")
	protected.Use(SessionMiddleware(provider))
	protected.GET("", func(c *gin.Context) {
		externalID, _ := c.Get(string(ContextKeyExternalID))
		email, _ := c.Get(string(ContextKeyEmail))
		userName, _ := c.Get(string(ContextKeyUserName))

		assert.Equal(t, "sub-ctx-test", externalID)
		assert.Equal(t, "ctx@example.com", email)
		assert.Equal(t, "Context User", userName)

		// ContextKeyUserID and ContextKeyOrgID are NOT set by SessionMiddleware;
		// they are populated by downstream auth handlers after a DB lookup.
		userID, exists := c.Get(string(ContextKeyUserID))
		assert.False(t, exists, "user_id should not be set by SessionMiddleware")
		assert.Nil(t, userID)

		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
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
