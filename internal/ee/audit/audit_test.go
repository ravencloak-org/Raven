package audit_test

import (
	"testing"
	_ "github.com/ravencloak-org/Raven/internal/ee/audit"
)

// TestPackageCompiles verifies that the audit package compiles successfully.
func TestPackageCompiles(t *testing.T) {
	t.Log("audit package compiled successfully")
}
