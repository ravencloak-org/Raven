package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ravencloak-org/Raven/internal/keycloakrealm"
)

// KeycloakAdminClient is the interface that admin clients must satisfy.
type KeycloakAdminClient interface {
	ImportRealm(ctx context.Context, realmJSON []byte) error
}

// provisionerServiceImpl provisions Keycloak realms for new tenants.
type provisionerServiceImpl struct {
	kc KeycloakAdminClient
}

// NewProvisionerService creates a new ProvisionerService backed by kc.
func NewProvisionerService(kc KeycloakAdminClient) *provisionerServiceImpl {
	return &provisionerServiceImpl{kc: kc}
}

// ProvisionRealm creates (or idempotently ensures the existence of) a Keycloak
// realm named realmName, using the bundled realm template.
func (s *provisionerServiceImpl) ProvisionRealm(ctx context.Context, realmName string) error {
	// Unmarshal the template so we can patch realm-specific fields.
	var realmDoc map[string]any
	if err := json.Unmarshal(keycloakrealm.Template, &realmDoc); err != nil {
		return fmt.Errorf("provisioner: parse realm template: %w", err)
	}

	realmDoc["realm"] = realmName
	realmDoc["displayName"] = realmName

	patched, err := json.Marshal(realmDoc)
	if err != nil {
		return fmt.Errorf("provisioner: marshal patched realm: %w", err)
	}

	if err := s.kc.ImportRealm(ctx, patched); err != nil {
		return fmt.Errorf("provisioner: import realm %q: %w", realmName, err)
	}
	return nil
}
