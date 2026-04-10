package security_test

import (
	"testing"

	_ "github.com/ravencloak-org/Raven/internal/ee/security"
)

// TestPackageCompiles ensures the security package is importable and correctly declared.
// The blank import above forces the compiler to build the package; if it has
// syntax errors or missing dependencies this test file will not compile.
func TestPackageCompiles(t *testing.T) {
	// blank import above guarantees compilation
}
