package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestNoRouterRouteRegistrationsAfterRootGroup parses main.go and asserts
// that no route registrations are made directly on `router` after
// `root := router.Group(cfg.Server.PathPrefix)` is declared. Every
// route-registration call (router.GET, router.POST, router.Group, ...)
// must be on `root` so RAVEN_PATH_PREFIX is honoured. router.Use(...)
// middleware calls are exempt — they apply to all groups by design.
//
// Catches the class of regression where a single route is registered on
// the bare router and 404s silently in path-prefixed deployments such
// as https://demo.ravencloak.org/raven/.
func TestNoRouterRouteRegistrationsAfterRootGroup(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	var rootLine int
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE {
			return true
		}
		for _, lhs := range as.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == "root" {
				rootLine = fset.Position(as.Pos()).Line
				return false
			}
		}
		return true
	})
	if rootLine == 0 {
		t.Fatal("could not locate `root := ...` declaration in main.go")
	}

	var bad []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok || x.Name != "router" {
			return true
		}
		line := fset.Position(call.Pos()).Line
		if line <= rootLine {
			return true
		}
		switch sel.Sel.Name {
		case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "Any", "Group":
			bad = append(bad, fmt.Sprintf("L%d: router.%s(...) — should be root.%s(...)", line, sel.Sel.Name, sel.Sel.Name))
		}
		return true
	})

	if len(bad) > 0 {
		t.Errorf("route registrations on bare `router` after `root :=` declaration:\n  %s", strings.Join(bad, "\n  "))
	}
}
