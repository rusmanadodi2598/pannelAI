// Command app-serv adapts the provider registry to the service's runtime lookup.
//
// @file      cmd/app-serv/provider_index_test.go
// @for       Tests that a stored custom node becomes a routable provider.
// @uses      testing, context, errors, internal/dataplane, internal/domain,
//
//	internal/provider, internal/registry.
//
// @reason    SPEC-API-001 §7.4 says a provider node "becomes routable exactly
//
//	like a registry provider", and that claim spans three layers: the
//	domain mints the id, the registry synthesizes the entry from it, and
//	the resolver routes by it. Each layer's own test can pass while the
//	seam between them is broken — which is what happened: the domain
//	minted `pnd_…` ids that the registry refuses to synthesize, so the
//	overlay failed and fell back to the embedded registry for every
//	request. Only a test that runs the whole path catches that.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// nodeListerStub serves the stored nodes the adapter overlays.
type nodeListerStub struct {
	nodes []domain.ProviderNode
}

func (s nodeListerStub) List(context.Context) ([]domain.ProviderNode, error) {
	return s.nodes, nil
}

// lookupStub answers the catalog reads with "nothing configured", which is what
// a fresh deployment looks like: no combos, no aliases, no disabled models.
type lookupStub struct{}

func (lookupStub) Combo(context.Context, string) ([]string, bool, error) { return nil, false, nil }
func (lookupStub) Alias(context.Context, string) (string, bool, error)   { return "", false, nil }
func (lookupStub) Disabled(context.Context, string, string) (bool, error) {
	return false, nil
}
func (lookupStub) DisabledPairs(context.Context) ([]domain.ModelRef, error) { return nil, nil }
func (lookupStub) ComboNames(context.Context) ([]string, error)             { return nil, nil }

// embeddedIndex is a registry with one built-in provider, which is what the
// runtime overlay starts from.
func embeddedIndex(t *testing.T) *registry.Index {
	t.Helper()
	doc := registry.Document{
		Revision: "index@test",
		Providers: []registry.Provider{{
			ID:       "openai",
			Category: "apikey",
			AuthType: registry.AuthAPIKey,
			Transport: registry.Transport{
				Format:  registry.DefaultFormat,
				BaseURL: "https://api.openai.com/v1/chat/completions",
			},
		}},
	}
	index, err := registry.NewIndex(doc)
	if err != nil {
		t.Fatalf("NewIndex() error = %v", err)
	}
	return index
}

// storedNode builds a node the way the management API does, so the test sees the
// id the domain actually mints rather than one written out by hand.
func storedNode(t *testing.T, nodeType domain.NodeType, apiType string) domain.ProviderNode {
	t.Helper()
	node, err := domain.NewProviderNode("", "My Corp", "mycorp", nodeType, apiType,
		"https://upstream.test/v1/chat/completions", time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewProviderNode() error = %v", err)
	}
	return node
}

// TestRuntimeProviderIndex_CustomNodeIsRoutable runs the whole path a request
// takes for a custom node: stored row, synthesized entry, resolved model string.
func TestRuntimeProviderIndex_CustomNodeIsRoutable(t *testing.T) {
	cases := []struct {
		name       string
		nodeType   domain.NodeType
		apiType    string
		wantPrefix string
		wantTarget string
	}{
		{name: "an OpenAI-compatible node routes as openai", nodeType: domain.NodeOpenAICompatible,
			apiType: domain.NodeAPIChat, wantPrefix: domain.NodeIDPrefixOpenAI, wantTarget: dataplane.TargetOpenAI},
		{name: "an Anthropic-compatible node routes as claude", nodeType: domain.NodeAnthropicCompatible,
			wantPrefix: domain.NodeIDPrefixAnthropic, wantTarget: dataplane.TargetClaude},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			node := storedNode(t, tc.nodeType, tc.apiType)
			if got := node.ID(); len(got) <= len(tc.wantPrefix) || got[:len(tc.wantPrefix)] != tc.wantPrefix {
				t.Fatalf("stored id = %q, want it to carry the %q prefix", got, tc.wantPrefix)
			}

			runtimeIndex := newRuntimeProviderIndex(embeddedIndex(t), nodeListerStub{[]domain.ProviderNode{node}}, nil)

			// The management side resolves the node by both of its names: the id
			// the panel stores on an endpoint, and the prefix a model string uses.
			for _, name := range []string{node.ID(), node.Prefix()} {
				if _, ok := runtimeIndex.Provider(name); !ok {
					t.Fatalf("Provider(%q) = not found, want the custom node", name)
				}
			}

			resolver, err := dataplane.NewResolver(runtimeIndex, lookupStub{})
			if err != nil {
				t.Fatalf("NewResolver() error = %v", err)
			}
			resolved, err := resolver.Resolve(ctx, node.Prefix()+"/gpt-4o-mini")
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if resolved.Provider.ID != node.ID() {
				t.Fatalf("resolved provider = %q, want the node %q", resolved.Provider.ID, node.ID())
			}
			if resolved.Target != tc.wantTarget {
				t.Fatalf("resolved target = %q, want %q", resolved.Target, tc.wantTarget)
			}
			// A node declares no model list, so the client's id passes through.
			if resolved.UpstreamID != "gpt-4o-mini" {
				t.Fatalf("upstream model = %q, want the client's id", resolved.UpstreamID)
			}
		})
	}
}

// TestRuntimeProviderIndex_EmbeddedRegistryRefusesTheNode is the negative
// control: the same model string fails against the boot-frozen registry alone.
// Without it, a passing test above could mean the string resolves for some other
// reason than the overlay.
func TestRuntimeProviderIndex_EmbeddedRegistryRefusesTheNode(t *testing.T) {
	node := storedNode(t, domain.NodeOpenAICompatible, domain.NodeAPIChat)
	resolver, err := dataplane.NewResolver(embeddedIndex(t), lookupStub{})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	_, err = resolver.Resolve(context.Background(), node.Prefix()+"/gpt-4o-mini")
	var dpErr *dataplane.Error
	if !errors.As(err, &dpErr) || dpErr.Code != dataplane.CodeModelNotFound {
		t.Fatalf("Resolve() error = %v, want MODEL_NOT_FOUND from the frozen registry", err)
	}
}

// TestHTTPEndpointProber_ResolvesACustomNode covers the probe path: the operator
// who presses "test" on an endpoint under a custom node must reach the node's
// own URL, not be told the provider is unknown.
func TestHTTPEndpointProber_ResolvesACustomNode(t *testing.T) {
	node := storedNode(t, domain.NodeOpenAICompatible, domain.NodeAPIChat)
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}

	prober := newHTTPEndpointProber(
		newRuntimeProviderIndex(embeddedIndex(t), nodeListerStub{[]domain.ProviderNode{node}}, nil), connectors)

	entry, _, err := prober.resolve(node.ID())
	if err != nil {
		t.Fatalf("resolve(%q) error = %v, want the custom node", node.ID(), err)
	}
	if !entry.Custom || entry.Transport.BaseURL != node.BaseURL() {
		t.Fatalf("resolved entry = %+v, want the synthesized custom node", entry)
	}

	// The same id against the frozen registry is the configuration fault the
	// adapter reports, which is what makes the assertion above meaningful.
	frozen := newHTTPEndpointProber(embeddedIndex(t), connectors)
	if _, _, err := frozen.resolve(node.ID()); err == nil {
		t.Fatal("resolve() against the frozen registry = nil error, want the unknown-provider fault")
	}
}
