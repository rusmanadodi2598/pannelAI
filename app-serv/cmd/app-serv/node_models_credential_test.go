// Command app-serv adapts a provider node's model list to HTTP.
//
// @file      cmd/app-serv/node_models_credential_test.go
// @for       The two credential-placement cases of the node model read: a
//
//	destination the guard refuses, and a node with no credential at all.
//
// @uses      internal/netguard, internal/service, context, net/http,
//
//	net/http/httptest, strings, testing, time.
//
// @reason    Both cases are about what the adapter does *not* send — no request
//
//	to a refused address, no Authorization header for an empty credential
//	— which is a different question from how it parses an answer. Keeping
//	them apart keeps each file's reason readable and both inside the
//	AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// TestNodeModelSource_RefusesADeniedDestination is Review Focus 1: a node whose
// base URL resolves to an internal address must fall back with a refusal, and
// must not reach the server at all.
func TestNodeModelSource_RefusesADeniedDestination(t *testing.T) {
	var reached bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		_, _ = w.Write([]byte(`{"data":[{"id":"should-not-be-seen"}]}`))
	}))
	defer server.Close()

	// A default guard, with no allowlist: loopback is refused.
	guard, err := netguard.NewGuard(nil)
	if err != nil {
		t.Fatalf("netguard.NewGuard() error = %v", err)
	}
	source := newNodeModelSource(
		staticNodeLookup{id: "openai-compatible-01TEST", baseURL: server.URL, format: "openai", apiType: "chat"}.lookup,
		testConnectors(t), guard, staticCredential("sk-live-abcdef"), time.Minute,
	)

	list, err := source.ListNodeModels(context.Background(), "openai-compatible-01TEST")
	if err != nil {
		t.Fatalf("ListNodeModels() error = %v, want a fallback, not a fault", err)
	}
	if list.Source != service.ModelSourceRegistry {
		t.Fatalf("Source = %q, want %q for a refused destination", list.Source, service.ModelSourceRegistry)
	}
	if list.Warning == "" {
		t.Fatal("a refused destination must say so in the warning")
	}
	if reached {
		t.Fatal("the request reached a destination the guard refused; the check must run before the dial")
	}
	if strings.Contains(list.Warning, "sk-live-abcdef") {
		t.Fatalf("the refusal warning carries the credential: %q", list.Warning)
	}
}
