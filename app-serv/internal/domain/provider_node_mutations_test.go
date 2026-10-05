// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/provider_node_mutations_test.go
// @for       Identifier assignment and the PATCH surface of a custom provider node.
// @uses      testing, time.
// @reason    A supplied id must carry the node type's own prefix, and a PATCH
//
//	must not be able to move a node onto another prefix, which is how
//	two node types would end up sharing a routing rule.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-04

package domain

import (
	"testing"
)

// TestProviderNode_SuppliedIDCarriesTheTypePrefix covers the other arm: a caller
// that supplies an id gets it normalized, so a bare ULID cannot produce a node
// the registry cannot synthesize.
func TestProviderNode_SuppliedIDCarriesTheTypePrefix(t *testing.T) {
	cases := []struct {
		name    string
		given   string
		nodeTyp NodeType
		apiType string
		want    string
	}{
		{name: "a bare id gains the openai prefix", given: "node-1",
			nodeTyp: NodeOpenAICompatible, apiType: NodeAPIChat, want: NodeIDPrefixOpenAI + "node-1"},
		{name: "an already prefixed id is kept as written", given: NodeIDPrefixOpenAI + "node-1",
			nodeTyp: NodeOpenAICompatible, apiType: NodeAPIChat, want: NodeIDPrefixOpenAI + "node-1"},
		{name: "a bare id gains the anthropic prefix", given: "node-2",
			nodeTyp: NodeAnthropicCompatible, want: NodeIDPrefixAnthropic + "node-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := NewProviderNode(tc.given, "N", "p", tc.nodeTyp, tc.apiType, "https://p.test/v1", nodeNow)
			if err != nil {
				t.Fatalf("NewProviderNode() error = %v", err)
			}
			if node.ID() != tc.want {
				t.Fatalf("id = %q, want %q", node.ID(), tc.want)
			}
		})
	}
}
func TestProviderNode_Mutations(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(n *ProviderNode) error
		wantName string
		wantPref string
		wantURL  string
		wantErr  bool
	}{
		{
			name:     "valid rename",
			mutate:   func(n *ProviderNode) error { return n.Rename("Renamed", nodeNow) },
			wantName: "Renamed", wantPref: "p", wantURL: "https://p.test/v1",
		},
		{
			name:    "blank rename",
			mutate:  func(n *ProviderNode) error { return n.Rename("  ", nodeNow) },
			wantErr: true,
		},
		{
			name:     "valid reprefix",
			mutate:   func(n *ProviderNode) error { return n.Reprefix("new-prefix", nodeNow) },
			wantName: "N", wantPref: "new-prefix", wantURL: "https://p.test/v1",
		},
		{
			name:    "reprefix with a slash",
			mutate:  func(n *ProviderNode) error { return n.Reprefix("a/b", nodeNow) },
			wantErr: true,
		},
		{
			name:     "valid rebase",
			mutate:   func(n *ProviderNode) error { return n.Rebase("https://new.test/v1", nodeNow) },
			wantName: "N", wantPref: "p", wantURL: "https://new.test/v1",
		},
		{
			name:    "rebase to a relative url",
			mutate:  func(n *ProviderNode) error { return n.Rebase("/v1", nodeNow) },
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := newNode(t, "N", "p", NodeOpenAICompatible, NodeAPIChat, "https://p.test/v1")
			err := tc.mutate(&node)
			if tc.wantErr {
				if err == nil {
					t.Fatal("mutation = nil error, want an error")
				}
				// A refused mutation must leave the node untouched.
				if node.Name() != "N" || node.Prefix() != "p" || node.BaseURL() != "https://p.test/v1" {
					t.Fatalf("a refused mutation changed the node: %+v", node)
				}
				return
			}
			if err != nil {
				t.Fatalf("mutation error = %v", err)
			}
			if node.Name() != tc.wantName || node.Prefix() != tc.wantPref || node.BaseURL() != tc.wantURL {
				t.Fatalf("node = (%q, %q, %q), want (%q, %q, %q)",
					node.Name(), node.Prefix(), node.BaseURL(), tc.wantName, tc.wantPref, tc.wantURL)
			}
		})
	}
}
