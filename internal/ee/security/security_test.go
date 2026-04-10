// Package security_test verifies the enterprise security package compiles and
// provides tests for the WAF rule engine components defined via the middleware layer.
package security_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	_ "github.com/ravencloak-org/Raven/internal/ee/security"
)

// TestPackageCompiles ensures the security package is importable and correctly declared.
// The blank import above forces the compiler to build the package; if it has
// syntax errors or missing dependencies this test file will not compile.
func TestPackageCompiles(t *testing.T) {
	// The EE security package is currently a stub (package declaration only).
	// Once exported types are added, this test should instantiate or reference them.
	t.Skip("TODO: exercise real security package API once exported types exist")
}

// TestWAFRuleEval_BlockPattern verifies that a WAF block rule matched against
// a request body containing the pattern returns the "block" action.
// This is a concept test that validates the expected contract for the WAF rule
// engine. It does not import or exercise any exported API from the security package.
func TestWAFRuleEval_BlockPattern_Concept(t *testing.T) {
	t.Skip("TODO: exercise real security package WAF API once exported types exist")
	// The WAF rule engine will evaluate HTTP request attributes against
	// configured rules. This test documents the expected contract.
	type Rule struct {
		Pattern string
		Action  string
		// Priority removed — evalRule uses declaration order, not priority.
	}

	evalRule := func(body string, rules []Rule) string {
		for _, r := range rules {
			if containsPattern(body, r.Pattern) {
				return r.Action
			}
		}
		return "allow"
	}

	rules := []Rule{
		{Pattern: "DROP TABLE", Action: "block"},
	}

	action := evalRule("SELECT * FROM users; DROP TABLE users;", rules)
	assert.Equal(t, "block", action, "SQL injection pattern must be blocked")

	cleanAction := evalRule("What is the capital of France?", rules)
	assert.Equal(t, "allow", cleanAction, "clean request must be allowed")
}

// TestWAFRuleEval_AllowOverridesBlock verifies priority-based rule ordering.
// This is a concept test that does not exercise the real security package API.
func TestWAFRuleEval_AllowOverridesBlock_Concept(t *testing.T) {
	t.Skip("TODO: exercise real security package WAF API once exported types exist")
	type Rule struct {
		Pattern  string
		Action   string
		Priority int
	}

	// Higher priority rule wins.
	evalByPriority := func(body string, rules []Rule) string {
		bestPriority := -1
		bestAction := "allow"
		for _, r := range rules {
			if containsPattern(body, r.Pattern) && r.Priority > bestPriority {
				bestPriority = r.Priority
				bestAction = r.Action
			}
		}
		return bestAction
	}

	rules := []Rule{
		{Pattern: "DROP TABLE", Action: "block", Priority: 5},
		{Pattern: "DROP TABLE", Action: "allow", Priority: 10}, // higher priority
	}

	action := evalByPriority("DROP TABLE users;", rules)
	assert.Equal(t, "allow", action, "allow rule with higher priority must override block")
}

// TestWAFRuleEval_LogRule_PassesThrough verifies log-only rules do not block.
// This is a concept test that does not exercise the real security package API.
func TestWAFRuleEval_LogRule_PassesThrough_Concept(t *testing.T) {
	t.Skip("TODO: exercise real security package WAF API once exported types exist")
	type Rule struct {
		Pattern string
		Action  string
	}

	evalRule := func(body string, rules []Rule) (action string, shouldLog bool) {
		for _, r := range rules {
			if containsPattern(body, r.Pattern) {
				if r.Action == "log" {
					return "allow", true
				}
				return r.Action, false
			}
		}
		return "allow", false
	}

	rules := []Rule{
		{Pattern: "admin", Action: "log"},
	}

	action, logged := evalRule("admin panel access", rules)
	assert.Equal(t, "allow", action, "log-only rule must not block the request")
	assert.True(t, logged, "log-only rule must generate an audit entry")
}

// containsPattern is a helper used within security tests.
func containsPattern(s, pattern string) bool {
	return len(s) > 0 && len(pattern) > 0 && containsSubstr(s, pattern)
}

func containsSubstr(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
