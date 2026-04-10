package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/service"
)

// mockKCAdmin implements KeycloakAdminClient for testing.
type mockKCAdmin struct {
	called    bool
	realmJSON []byte
	err       error
}

func (m *mockKCAdmin) ImportRealm(_ context.Context, realmJSON []byte) error {
	m.called = true
	m.realmJSON = realmJSON
	return m.err
}

func TestProvisionRealm_Success(t *testing.T) {
	mock := &mockKCAdmin{}
	svc := service.NewProvisionerService(mock)

	err := svc.ProvisionRealm(context.Background(), "acme-corp")
	require.NoError(t, err)
	assert.True(t, mock.called, "ImportRealm should have been called")

	// Verify the realm name was patched into the JSON payload.
	assert.Contains(t, string(mock.realmJSON), `"acme-corp"`)
}

func TestProvisionRealm_PropagatesError(t *testing.T) {
	mock := &mockKCAdmin{err: errors.New("keycloak unreachable")}
	svc := service.NewProvisionerService(mock)

	err := svc.ProvisionRealm(context.Background(), "broken-realm")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken-realm")
}
