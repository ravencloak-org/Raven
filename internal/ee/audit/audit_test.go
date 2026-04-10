package audit_test

import (
	"testing"
	_ "github.com/ravencloak-org/Raven/internal/ee/audit"
)

// TestPackageCompiles verifies that the audit package compiles successfully.
// The blank import above forces the compiler to build the package; if it has
// syntax errors or missing dependencies this test file will not compile.
func TestPackageCompiles(t *testing.T) {
	// blank import above guarantees compilation
}
