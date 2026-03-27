package db_test

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/db"
)

func TestSetOrgIDQuery_ValidUUID(t *testing.T) {
	orgID := "550e8400-e29b-41d4-a716-446655440000"
	got := db.SetOrgIDQuery(orgID)
	want := "SET LOCAL app.current_org_id = '550e8400-e29b-41d4-a716-446655440000'"
	if got != want {
		t.Errorf("unexpected query:\n got:  %s\n want: %s", got, want)
	}
}

func TestSetOrgIDQuery_Format(t *testing.T) {
	// Ensure the query format is stable — changing it would break RLS enforcement.
	got := db.SetOrgIDQuery("abc-123")
	want := "SET LOCAL app.current_org_id = 'abc-123'"
	if got != want {
		t.Errorf("unexpected query format: %s", got)
	}
}
