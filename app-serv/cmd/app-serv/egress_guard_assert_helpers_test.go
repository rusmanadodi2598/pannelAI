// Command app-serv adapts the provider registry to the service's runtime lookup
//
// @file      cmd/app-serv/egress_guard_assert_helpers_test.go
// @for       The source-reading helpers the egress-guard assertions are built
//
//	from.
//
// @uses      go/ast, go/parser, go/token, os, path/filepath, strings, testing.
// @reason    The assertions in egress_guard_assert_test.go are statements about
//
//	construction shapes, so they read this package's own source. Keeping
//	the reading apart from the assertions keeps each file's reason
//	readable: one file says what must hold, the other says how the
//	source is asked.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseCompositionRoot parses this package's non-test Go files.
func parseCompositionRoot(t *testing.T) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the composition root: %v", err)
	}
	fset := token.NewFileSet()
	files := make(map[string]*ast.File)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files[name] = parsed
	}
	if len(files) == 0 {
		t.Fatal("no composition-root source found; the assertion would pass vacuously")
	}
	return files
}

// findFunc returns the named function declaration, failing when it is absent so
// a rename cannot make this test stop checking anything.
func findFunc(t *testing.T, files map[string]*ast.File, file, function string) *ast.FuncDecl {
	t.Helper()
	parsed, ok := files[file]
	if !ok {
		t.Fatalf("%s is not in the composition root; update guardedAdapters", file)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == function {
			return fn
		}
	}
	t.Fatalf("%s does not declare %s; update guardedAdapters", file, function)
	return nil
}

// hasGuardParam reports whether the function takes a parameter of type
// *netguard.Guard with the given name.
func hasGuardParam(fn *ast.FuncDecl, name string) bool {
	if fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		selector, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		qualified, ok := selector.X.(*ast.SelectorExpr)
		if !ok || qualified.Sel.Name != "Guard" {
			continue
		}
		pkg, ok := qualified.X.(*ast.Ident)
		if !ok || pkg.Name != "netguard" {
			continue
		}
		for _, ident := range field.Names {
			if ident.Name == name {
				return true
			}
		}
	}
	return false
}

// funcCalls reports whether the function body mentions the given selector.
func funcCalls(fn *ast.FuncDecl, selector string) bool {
	if fn.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == selector {
			found = true
		}
		return true
	})
	return found
}

// isHTTPClientBuild reports whether the call is dataplane.NewHTTPClient.
func isHTTPClientBuild(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "NewHTTPClient" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "dataplane"
}

// exprMentionsGuardDialer reports whether any argument expression reaches
// guard.NewDialer, which is what makes the client's dialer the guarded one.
func exprMentionsGuardDialer(call *ast.CallExpr) bool {
	found := false
	for _, arg := range call.Args {
		ast.Inspect(arg, func(node ast.Node) bool {
			inner, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := inner.Fun.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "NewDialer" {
				found = true
			}
			return true
		})
	}
	return found
}
