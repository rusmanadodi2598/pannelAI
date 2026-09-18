// Command app-serv adapts the provider registry to the service's runtime lookup.
//
// @file      cmd/app-serv/provider_index_url_test.go
// @for       Tests that a custom node reaches a callable upstream URL.
// @uses      testing, internal/domain, internal/provider.
// @reason    SPEC-API-001 §7.4 says a node "becomes routable exactly like a
//
//	registry provider", and resolving a model string is only half of
//	that: the connector must also be handed a URL that reaches the
//	node's chat endpoint. A node stores a base, so the path has to be
//	appended — a bare base sends every call to the host root. It sits
//	in its own file because provider_index_test.go covers resolution
//	and is near the §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package main

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// TestRuntimeProviderIndex_CustomNodeGetsACallableURL asks the connector the
// runtime overlay hands the data plane for the URL, so the test covers the whole
// seam — stored row, synthesized entry, connector — and not just the entry.
func TestRuntimeProviderIndex_CustomNodeGetsACallableURL(t *testing.T) {
	cases := []struct {
		name     string
		nodeType domain.NodeType
		apiType  string
		wantPath string
	}{
		{name: "an OpenAI-compatible chat node", nodeType: domain.NodeOpenAICompatible,
			apiType: domain.NodeAPIChat, wantPath: "/chat/completions"},
		{name: "an OpenAI-compatible responses node", nodeType: domain.NodeOpenAICompatible,
			apiType: domain.NodeAPIResponses, wantPath: "/responses"},
		{name: "an Anthropic-compatible node", nodeType: domain.NodeAnthropicCompatible,
			wantPath: "/messages"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := storedNode(t, tc.nodeType, tc.apiType)
			runtimeIndex := newRuntimeProviderIndex(embeddedIndex(t), nodeListerStub{[]domain.ProviderNode{node}}, nil)
			entry, ok := runtimeIndex.Provider(node.ID())
			if !ok {
				t.Fatalf("Provider(%q) = not found, want the custom node", node.ID())
			}

			connectors, err := provider.NewConnectors(provider.DefaultFactory)
			if err != nil {
				t.Fatalf("NewConnectors() error = %v", err)
			}
			got, err := connectors.For(entry).Endpoint(
				provider.Request{Provider: entry},
				provider.StaticKey(node.ID(), "", "sk-x"))
			if err != nil {
				t.Fatalf("Endpoint() error = %v", err)
			}
			want := node.BaseURL() + tc.wantPath
			if got != want {
				t.Fatalf("Endpoint() = %q, want %q", got, want)
			}
		})
	}
}
