// Command app-serv adapts the provider registry to the service's runtime lookup
//
// @file      cmd/app-serv/egress_guard_assert_test.go
// @for       The structural assertion that every egress adapter the composition
//
//	root builds is bound to the process egress guard.
//
// @uses      go/ast, go/parser, go/token, os, path/filepath, strings, testing.
// @reason    SPEC-API-001 §9.9 and OWASP A01 require every outbound dial to go
//
//	through internal/netguard. Review is what enforced that, and review is
//	what draft 017 §4.8 found insufficient: the guard was wired into the
//	probe alone while the class of operator-supplied destinations kept
//	growing. A rule that depends on a reviewer noticing is a rule that
//	decays, so this reads the composition root's own source and fails
//	when a dialer is built without a guard parameter.
//
//	It parses source rather than inspecting runtime values because the
//	thing being asserted is a construction shape, not a behaviour: a
//	client built with a nil guard compiles and works until the first
//	request to a private address.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"go/ast"
	"strings"
	"testing"
)

// guardedAdapter names one constructor in the composition root that builds an
// egress dialer, and the parameter that must carry the guard.
//
// The list is explicit rather than discovered so that adding a new adapter is a
// deliberate edit here: a test that enumerated "anything whose name ends in
// Prober" would silently accept an adapter named something else.
type guardedAdapter struct {
	// file is the composition-root file the constructor lives in.
	file string
	// function is the constructor's name.
	function string
	// guardParam is the parameter that must have type *netguard.Guard.
	guardParam string
	// reason records why this adapter is on the list, so a reader can judge
	// whether a removal is safe.
	reason string
}

// guardedAdapters is every constructor in this package that builds a dialer.
var guardedAdapters = []guardedAdapter{
	{
		file:       "provider_probe.go",
		function:   "newHTTPEndpointProber",
		guardParam: "guard",
		reason:     "probes an endpoint's validate URL and a node's base URL, both operator-supplied",
	},
	{
		file:       "proxy_probe.go",
		function:   "newProxyProber",
		guardParam: "guard",
		reason:     "dials a proxy host an operator typed",
	},
	{
		file:       "egress_wiring.go",
		function:   "buildEgress",
		guardParam: "",
		reason:     "builds the process guard itself, so it receives config rather than a guard",
	},
}

// egressExceptions lists the places that build an HTTP client without the
// guard's dialer. It is empty on purpose: the assertion below fails if it grows,
// so an exception has to be argued for in review rather than added quietly.
var egressExceptions = map[string]string{}

// TestEgressAdaptersTakeTheGuard asserts the construction shape of every
// adapter that builds a dialer.
func TestEgressAdaptersTakeTheGuard(t *testing.T) {
	files := parseCompositionRoot(t)

	for _, adapter := range guardedAdapters {
		fn := findFunc(t, files, adapter.file, adapter.function)
		if adapter.guardParam == "" {
			// The guard's own builder: it must call netguard.NewGuard instead.
			if !funcCalls(fn, "NewGuard") {
				t.Fatalf("%s.%s is listed as building the process guard but does not call netguard.NewGuard",
					adapter.file, adapter.function)
			}
			continue
		}
		if !hasGuardParam(fn, adapter.guardParam) {
			t.Fatalf("%s.%s builds a dialer without a *netguard.Guard parameter named %q (%s); an unguarded dialer is the OWASP A01 hole draft 017 §4.8 names",
				adapter.file, adapter.function, adapter.guardParam, adapter.reason)
		}
	}
}

// TestEveryDialerInTheCompositionRootIsTheGuards asserts the half that does not
// depend on where a dialer is built: every `NewDialer` call in this package is
// made on a guard.
//
// It is separate from the constructor check because the two shapes are both
// legitimate — newHTTPEndpointProber builds its dialer in the constructor,
// newProxyProber builds its transport per call inside ProbeProxy — and a check
// written for one would have missed the other. What must hold either way is that
// the dialer comes from a guard rather than from a bare net.Dialer.
func TestEveryDialerInTheCompositionRootIsTheGuards(t *testing.T) {
	files := parseCompositionRoot(t)
	found := 0
	for name, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "NewDialer" {
				return true
			}
			found++
			if !strings.Contains(exprText(sel.X), "guard") {
				t.Fatalf("%s builds a dialer on %q, which is not a guard; a bare net.Dialer is the OWASP A01 hole draft 017 §4.8 names",
					name, exprText(sel.X))
			}
			return true
		})
	}
	if found == 0 {
		t.Fatal("no guard dialer found in the composition root; the assertion would pass vacuously")
	}
	t.Logf("asserted %d guard dialers", found)
}

// exprText renders a selector's receiver for an error message. It is a
// best-effort rendering, not a printer: the message names what the source says
// so a reader can find the line.
func exprText(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return exprText(typed.X) + "." + typed.Sel.Name
	default:
		return "the expression"
	}
}

// TestEveryHTTPClientInTheCompositionRootIsGuarded asserts the second half of
// the rule: no client is built on a dialer that is not the guard's.
func TestEveryHTTPClientInTheCompositionRootIsGuarded(t *testing.T) {
	if len(egressExceptions) != 0 {
		t.Fatalf("egressExceptions holds %d entries; every exception must be argued for in review and the count kept at zero", len(egressExceptions))
	}
	files := parseCompositionRoot(t)
	for name, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !isHTTPClientBuild(call) {
				return true
			}
			if !exprMentionsGuardDialer(call) {
				t.Fatalf("%s builds an HTTP client with no guard dialer; add `Dialer: guard.NewDialer(...)` or record the exception with its reason (OWASP A01, SPEC-API-001 §9.9)",
					name)
			}
			return true
		})
	}
}
