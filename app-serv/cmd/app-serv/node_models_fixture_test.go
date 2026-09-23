// Command app-serv adapts a provider node's model list to HTTP.
//
// @file      cmd/app-serv/node_models_fixture_test.go
// @for       The doubles the node model-list tests share: a one-node lookup, a
//
//	fixed credential, and the connector set.
//
// @uses      internal/provider, context, testing.
// @reason    Two test files cover the adapter — the parse/fallback cases and the
//
//	cache window — and both need the same three doubles. Sharing them here
//	keeps each test file about its own assertions, and keeps both inside
//	the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// testConnectors builds the connector set the adapter authenticates through.
// It is the same construction the boot uses, so a credential is placed by the
// code the gateway places it with rather than by a test double.
func testConnectors(t *testing.T) *provider.Connectors {
	t.Helper()
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("provider.NewConnectors() error = %v", err)
	}
	return connectors
}

// staticNodeLookup resolves one node id, so the adapter's node read is not a
// store. Any other id is unresolved, which is what makes the "unknown node" case
// a real one rather than a second spelling of "known".
type staticNodeLookup struct {
	id      string
	baseURL string
	format  string
	apiType string
}

// lookup is the func shape the adapter takes, so a test states the node it wants
// resolved without building a store.
func (l staticNodeLookup) lookup(id string) (nodeTarget, bool) {
	if l.id != "" && id != l.id {
		return nodeTarget{}, false
	}
	return nodeTarget{BaseURL: l.baseURL, Format: l.format, APIType: l.apiType}, true
}

// staticCredential answers a fixed credential for every node.
func staticCredential(value string) nodeCredentialSource {
	return func(context.Context, string) (string, error) { return value, nil }
}

// equalStrings compares two slices element-wise, so a test states its
// expectation in order.
func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
