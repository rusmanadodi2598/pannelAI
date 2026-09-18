// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/provider_node_test.go
// @for       Table-driven tests for the custom node's prefix, type, and format
//
//	rules.
//
// @uses      testing, time.
// @reason    A node's prefix is a model-string namespace and its api type picks
//
//	the endpoint the gateway calls, so a wrong value here routes a
//	request to a URL that cannot serve it; both rules are domain
//	invariants and are pinned as such.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"testing"
	"time"
)

var nodeNow = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

func newNode(t *testing.T, name, prefix string, nodeType NodeType, apiType, baseURL string) ProviderNode {
	t.Helper()
	node, err := NewProviderNode("", name, prefix, nodeType, apiType, baseURL, nodeNow)
	if err != nil {
		t.Fatalf("NewProviderNode() error = %v", err)
	}
	return node
}

func TestNewProviderNode_RejectsMalformedInput(t *testing.T) {
	cases := []struct {
		name     string
		label    string
		prefix   string
		nodeType NodeType
		apiType  string
		baseURL  string
		wantErr  bool
	}{
		{
			name: "openai chat node", label: "My Corp", prefix: "mycorp",
			nodeType: NodeOpenAICompatible, apiType: NodeAPIChat, baseURL: "https://mycorp.test/v1",
		},
		{
			name: "openai responses node", label: "Resp", prefix: "resp",
			nodeType: NodeOpenAICompatible, apiType: NodeAPIResponses, baseURL: "https://resp.test/v1",
		},
		{
			name: "anthropic node needs no api type", label: "Proxy", prefix: "prox",
			nodeType: NodeAnthropicCompatible, baseURL: "http://localhost:8081/v1",
		},
		{
			name: "empty name", prefix: "p", nodeType: NodeOpenAICompatible,
			apiType: NodeAPIChat, baseURL: "https://p.test/v1", wantErr: true,
		},
		{
			name: "blank name", label: "   ", prefix: "p", nodeType: NodeOpenAICompatible,
			apiType: NodeAPIChat, baseURL: "https://p.test/v1", wantErr: true,
		},
		{
			name: "empty prefix", label: "N", nodeType: NodeOpenAICompatible,
			apiType: NodeAPIChat, baseURL: "https://p.test/v1", wantErr: true,
		},
		{
			name: "prefix with a slash", label: "N", prefix: "a/b",
			nodeType: NodeOpenAICompatible, apiType: NodeAPIChat, baseURL: "https://p.test/v1", wantErr: true,
		},
		{
			name: "prefix with whitespace", label: "N", prefix: "a b",
			nodeType: NodeOpenAICompatible, apiType: NodeAPIChat, baseURL: "https://p.test/v1", wantErr: true,
		},
		{
			name: "openai node without an api type", label: "N", prefix: "p",
			nodeType: NodeOpenAICompatible, baseURL: "https://p.test/v1", wantErr: true,
		},
		{
			name: "openai node with an unknown api type", label: "N", prefix: "p",
			nodeType: NodeOpenAICompatible, apiType: "embedding", baseURL: "https://p.test/v1", wantErr: true,
		},
		{
			name: "anthropic node carrying an api type", label: "N", prefix: "p",
			nodeType: NodeAnthropicCompatible, apiType: NodeAPIChat, baseURL: "https://p.test/v1", wantErr: true,
		},
		{
			name: "unknown node type", label: "N", prefix: "p",
			nodeType: NodeType("custom-embedding"), apiType: NodeAPIChat, baseURL: "https://p.test/v1", wantErr: true,
		},
		{
			name: "relative base url", label: "N", prefix: "p",
			nodeType: NodeOpenAICompatible, apiType: NodeAPIChat, baseURL: "/v1", wantErr: true,
		},
		{
			name: "non-http base url", label: "N", prefix: "p",
			nodeType: NodeOpenAICompatible, apiType: NodeAPIChat, baseURL: "ftp://p.test", wantErr: true,
		},
		{
			name: "base url without a host", label: "N", prefix: "p",
			nodeType: NodeOpenAICompatible, apiType: NodeAPIChat, baseURL: "https://", wantErr: true,
		},
		{
			name: "empty base url", label: "N", prefix: "p",
			nodeType: NodeOpenAICompatible, apiType: NodeAPIChat, wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewProviderNode("", tc.label, tc.prefix, tc.nodeType, tc.apiType, tc.baseURL, nodeNow)
			if tc.wantErr && err == nil {
				t.Fatal("NewProviderNode() = nil error, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("NewProviderNode() error = %v", err)
			}
		})
	}
}

// TestProviderNode_Format covers the mapping that decides which endpoint the
// gateway calls. Treating a responses node as chat would send its payload to
// /chat/completions, which cannot serve it.
func TestProviderNode_Format(t *testing.T) {
	cases := []struct {
		name     string
		nodeType NodeType
		apiType  string
		want     string
	}{
		{name: "openai chat", nodeType: NodeOpenAICompatible, apiType: NodeAPIChat, want: "openai"},
		{name: "openai responses", nodeType: NodeOpenAICompatible, apiType: NodeAPIResponses, want: "openai-responses"},
		{name: "anthropic compatible", nodeType: NodeAnthropicCompatible, want: "claude"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := newNode(t, "N", "p", tc.nodeType, tc.apiType, "https://p.test/v1")
			if got := node.Format(); got != tc.want {
				t.Fatalf("Format() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProviderNode_NewNodeGetsAnID(t *testing.T) {
	first := newNode(t, "N", "p", NodeOpenAICompatible, NodeAPIChat, "https://p.test/v1")
	second := newNode(t, "N", "p", NodeOpenAICompatible, NodeAPIChat, "https://p.test/v1")
	if first.ID() == "" {
		t.Fatal("a minted node must carry an id")
	}
	if len(first.ID()) <= len(IDPrefixNode) || first.ID()[:len(IDPrefixNode)] != IDPrefixNode {
		t.Fatalf("id %q must carry the %q prefix", first.ID(), IDPrefixNode)
	}
	if first.ID() == second.ID() {
		t.Fatal("two minted nodes must not share an id")
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
