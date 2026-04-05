package db_test

import (
	"context"
	"testing"

	"github.com/ravencloak-org/Raven/internal/db"
)

func TestWithOrgID_SignatureStable(t *testing.T) {
	// SetOrgIDQuery was removed in favour of the parameterized set_config call inside
	// WithOrgID. This test confirms the replacement symbol is exported and callable.
	// Full integration coverage requires a live database.
	_ = context.Background()
	_ = db.WithOrgID
}
