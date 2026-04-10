// Package keycloakrealm embeds the default Keycloak realm template JSON.
// The template is used by the provisioner service to create per-tenant realms.
package keycloakrealm

import (
	_ "embed"
)

//go:embed raven-realm.json
var Template []byte
