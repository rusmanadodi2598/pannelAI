// Command app-serv adapts the provider registry to the service's runtime lookup
//
// @file      cmd/app-serv/provider_wiring_guard_test.go
// @for       The structural assertion that the Qoder connector's exchange client is the guarded egress client the boot sequence builds.
// @uses      go/ast, go/parser, go/token, os, testing.
// @reason    R07 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: the composition root passed a nil client to NewQoder, so the Personal Access Token exchange, the catalog, and the identity reads left through a client the egress guard never saw. The wiring shape is a construction, not a runtime behaviour, a nil client compiles and works until the first exchange, so, like egress_guard_assert_test.go, this reads the composition root's own source and fails when the call hands the connector nil.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-03
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// TestProviderWiring_QoderRidesTheGuardedClient pins that buildProviderRuntime
// takes the guarded client as a parameter and that every NewQoder call in the
// composition root passes it on, because a nil there silently rebuilds the
// unguarded exchange client the egress policy exists to prevent.
func TestProviderWiring_QoderRidesTheGuardedClient(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "provider_wiring.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing provider_wiring.go: %v", err)
	}

	sawBuilder := false
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.FuncDecl)
		if !ok || decl.Name.Name != "buildProviderRuntime" {
			return true
		}
		sawBuilder = true
		for _, field := range decl.Type.Params.List {
			star, ok := field.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			sel, ok := star.X.(*ast.SelectorExpr)
			if ok && sel.X.(*ast.Ident) != nil && sel.Sel.Name == "Client" {
				return true
			}
		}
		t.Error("buildProviderRuntime does not take an *http.Client parameter, so it cannot pass the guarded egress client to the connectors")
		return true
	})
	if !sawBuilder {
		t.Fatal("buildProviderRuntime was not found in provider_wiring.go")
	}

	src, err := os.ReadFile("provider_wiring.go")
	if err != nil {
		t.Fatalf("reading provider_wiring.go: %v", err)
	}
	file, err = parser.ParseFile(fset, "provider_wiring.go", src, 0)
	if err != nil {
		t.Fatalf("parsing provider_wiring.go: %v", err)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "NewQoder" {
			return true
		}
		if len(call.Args) != 2 {
			t.Errorf("provider.NewQoder called with %d arguments at %s, want 2", len(call.Args), fset.Position(call.Pos()))
			return true
		}
		if ident, ok := call.Args[1].(*ast.Ident); ok && ident.Name == "nil" {
			t.Errorf("provider.NewQoder is called with a nil client at %s, the PAT exchange would leave the egress guard (draft 042 R07)", fset.Position(call.Pos()))
		}
		return true
	})
}
