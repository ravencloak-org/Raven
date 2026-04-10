package keycloak_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kcadmin "github.com/ravencloak-org/Raven/internal/integration/keycloak"
)

// fakeKeycloak starts an httptest server that mimics the token and realm-import
// endpoints. tokenStatus controls the HTTP status returned by the token endpoint;
// importStatus controls the import endpoint.
func fakeKeycloak(t *testing.T, tokenStatus, importStatus int) (*httptest.Server, *int32, *int32) {
	t.Helper()
	var tokenCalls, importCalls int32

	mux := http.NewServeMux()
	mux.HandleFunc("/realms/master/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tokenCalls, 1)
		if tokenStatus != http.StatusOK {
			w.WriteHeader(tokenStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-token"})
	})
	mux.HandleFunc("/admin/realms", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&importCalls, 1)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(importStatus)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &tokenCalls, &importCalls
}

func newClient(srv *httptest.Server) *kcadmin.AdminClient {
	return kcadmin.NewAdminClient(srv.URL, "master", "admin-cli", "secret", srv.Client())
}

func TestImportRealm_Created(t *testing.T) {
	srv, _, importCalls := fakeKeycloak(t, http.StatusOK, http.StatusCreated)
	err := newClient(srv).ImportRealm(context.Background(), []byte(`{"realm":"test"}`))
	require.NoError(t, err)
	assert.EqualValues(t, 1, atomic.LoadInt32(importCalls))
}

func TestImportRealm_Conflict_Idempotent(t *testing.T) {
	srv, _, importCalls := fakeKeycloak(t, http.StatusOK, http.StatusConflict)
	err := newClient(srv).ImportRealm(context.Background(), []byte(`{"realm":"test"}`))
	require.NoError(t, err, "409 Conflict should be treated as success (idempotent)")
	assert.EqualValues(t, 1, atomic.LoadInt32(importCalls))
}

func TestImportRealm_ServerError(t *testing.T) {
	srv, _, _ := fakeKeycloak(t, http.StatusOK, http.StatusInternalServerError)
	err := newClient(srv).ImportRealm(context.Background(), []byte(`{"realm":"test"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestImportRealm_TokenError(t *testing.T) {
	srv, _, _ := fakeKeycloak(t, http.StatusUnauthorized, http.StatusCreated)
	err := newClient(srv).ImportRealm(context.Background(), []byte(`{"realm":"test"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}
