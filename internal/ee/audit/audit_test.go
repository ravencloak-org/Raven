package audit_test

import (
	"testing"
	_ "github.com/ravencloak-org/Raven/internal/ee/audit"
)

// TestPackageCompiles verifies that the audit package compiles successfully.
// The blank import above forces the compiler to build the package; if it has
// syntax errors or missing dependencies this test file will not compile.
func TestPackageCompiles(t *testing.T) {
	// The audit EE package is currently a stub (package declaration only).
	// Once exported types are added, this test should instantiate or reference them.
	t.Skip("TODO: exercise real audit package API once exported types exist")
}
