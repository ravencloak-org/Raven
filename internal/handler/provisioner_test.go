package handler_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/handler"
)

// stubProvisioner implements handler.Provisioner for testing.
type stubProvisioner struct {
	err error
}

func (s *stubProvisioner) ProvisionRealm(_ context.Context, _ string) error {
	return s.err
}

func newProvisionerRouter(svc handler.Provisioner, key string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewProvisionerHandler(svc, key)
	r.POST("/internal/provision-realm", h.ProvisionRealm)
	return r
}

func TestProvisionRealm_OK(t *testing.T) {
	r := newProvisionerRouter(&stubProvisioner{}, "secret-key")

	body := bytes.NewBufferString(`{"realm":"acme"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/provision-realm", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", "secret-key")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "acme")
}

func TestProvisionRealm_MissingKey(t *testing.T) {
	r := newProvisionerRouter(&stubProvisioner{}, "secret-key")

	body := bytes.NewBufferString(`{"realm":"acme"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/provision-realm", body)
	req.Header.Set("Content-Type", "application/json")
	// No X-Internal-Key header
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestProvisionRealm_WrongKey(t *testing.T) {
	r := newProvisionerRouter(&stubProvisioner{}, "secret-key")

	body := bytes.NewBufferString(`{"realm":"acme"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/provision-realm", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", "wrong-key")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestProvisionRealm_BadBody(t *testing.T) {
	r := newProvisionerRouter(&stubProvisioner{}, "secret-key")

	body := bytes.NewBufferString(`{}`) // missing required 'realm'
	req := httptest.NewRequest(http.MethodPost, "/internal/provision-realm", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", "secret-key")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProvisionRealm_ServiceError(t *testing.T) {
	// Error handler must be registered for c.Error() to produce a response.
	gin.SetMode(gin.TestMode)
	eng := gin.New()
	eng.Use(func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": c.Errors.Last().Error()})
		}
	})
	h := handler.NewProvisionerHandler(&stubProvisioner{err: errors.New("keycloak down")}, "key")
	eng.POST("/internal/provision-realm", h.ProvisionRealm)

	body := bytes.NewBufferString(`{"realm":"acme"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/provision-realm", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", "key")
	w := httptest.NewRecorder()

	eng.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestProvisionRealm_NoKeyConfigured_AllowsAny(t *testing.T) {
	// When internalKey is empty, the endpoint is open (trusted network assumed).
	r := newProvisionerRouter(&stubProvisioner{}, "")

	body := bytes.NewBufferString(`{"realm":"open-realm"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/provision-realm", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
