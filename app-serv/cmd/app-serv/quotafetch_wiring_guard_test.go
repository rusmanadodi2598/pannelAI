// Command app-serv wires the observability graph's egress obligation.
//
// @file      cmd/app-serv/quotafetch_wiring_guard_test.go
// @for       The structural assertion that the boot sequence installs the
//
//	process egress client into the quota fetch package.
//
// @uses      go/ast, go/parser, go/token, os, strings, testing.
// @reason    R08 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: the quota fetch
//
//	package's shared client was built bare, so every family's read
//	left without the egress guard. The install is a boot-sequence
//	shape — the thing being asserted is that the composition root
//	calls it, not what a request does afterwards — so, like
//	egress_guard_assert_test.go, this reads the wiring's own source.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-10-03
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestObservabilityWiring_InstallsTheQuotaEgressClient pins that the wiring that
// builds the quota service also installs the guarded egress client into
// quotafetch, because without that call the poll workers' reads leave unguarded
// however carefully the package's default is built.
func TestObservabilityWiring_InstallsTheQuotaEgressClient(t *testing.T) {
	src, err := os.ReadFile("observability_wiring.go")
	if err != nil {
		t.Fatalf("reading observability_wiring.go: %v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "observability_wiring.go", src, 0)
	if err != nil {
		t.Fatalf("parsing observability_wiring.go: %v", err)
	}

	installed := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "UseEgressClient" {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != "quotafetch" {
			return true
		}
		installed = true
		if len(call.Args) != 1 {
			t.Errorf("quotafetch.UseEgressClient called with %d arguments at %s, want the egress client", len(call.Args), fset.Position(call.Pos()))
			return true
		}
		if !strings.Contains(nodeSrcText(fset, src, call.Args[0]), "egress") {
			t.Errorf("quotafetch.UseEgressClient at %s does not receive the egress client", fset.Position(call.Pos()))
		}
		return true
	})
	if !installed {
		t.Fatal("observability_wiring.go never calls quotafetch.UseEgressClient, so the quota reads leave without the egress guard (draft 042 R08)")
	}
}

// nodeSrcText renders one AST node back to source, so an argument's identity is
// judged by what the wiring actually wrote rather than by a guess at its shape.
func nodeSrcText(fset *token.FileSet, src []byte, node ast.Node) string {
	start := fset.Position(node.Pos()).Offset
	end := fset.Position(node.End()).Offset
	return string(src[start:end])
}
