// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/custom_node_synthesize_test.go
// @for       Table-driven tests for synthesizing a custom node and extending an index.
// @uses      testing, internal/registry.
// @reason    SPEC-API-001 §7.4 makes custom nodes P1: a synthesized provider
// grants a user's own upstream a place in the routing table, so a wrong
// format, priority, or alias, and any mutation of the shared index visible to
// another request, are routing faults pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import "testing"

func TestIndex_SynthesizeCustomNode(t *testing.T) {
	base, err := loadFixture(t, `
revision: test@1
providers:
  - id: deepseek
    alias: ds
    category: apikey
    models:
      - id: deepseek-chat
        name: DeepSeek Chat
`)
	if err != nil {
		t.Fatalf("load error = %v", err)
	}

	cases := []struct {
		name     string
		node     CustomNode
		wantOK   bool
		wantID   string
		wantFmt  string
		wantPath string
		wantAuth string
	}{
		{
			name: "openai compatible node speaks the openai format",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "chat-1", Name: "My Corp", Prefix: "mycorp",
				APIType: "chat", BaseURL: "https://mycorp.test/v1",
			},
			wantOK: true, wantID: OpenAICompatiblePrefix + "chat-1",
			wantFmt: "openai", wantPath: "/chat/completions", wantAuth: AuthAPIKey,
		},
		{
			name: "an openai responses node speaks its own format",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "responses-9", Name: "Resp Proxy", Prefix: "resp",
				APIType: "responses", BaseURL: "https://resp.test/v1",
			},
			wantOK: true, wantID: OpenAICompatiblePrefix + "responses-9",
			wantFmt: FormatOpenAIResponses, wantPath: "/responses", wantAuth: AuthAPIKey,
		},
		{
			name: "anthropic compatible node speaks the claude format",
			node: CustomNode{
				ID: AnthropicCompatiblePrefix + "2", Name: "Claude Proxy", Prefix: "claudeproxy",
				BaseURL: "https://proxy.test/v1",
			},
			wantOK: true, wantID: AnthropicCompatiblePrefix + "2",
			wantFmt: "claude", wantPath: "/messages", wantAuth: AuthAPIKey,
		},
		{
			name: "prefix colliding with a registry id is refused",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "4", Name: "Collide", Prefix: "deepseek",
				APIType: "chat", BaseURL: "https://collide.test/v1",
			},
		},
		{
			name: "prefix colliding with a registry alias is refused",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "5", Name: "Collide", Prefix: "ds",
				APIType: "chat", BaseURL: "https://collide.test/v1",
			},
		},
		{
			name: "a malformed node is refused before any collision check",
			node: CustomNode{
				ID: OpenAICompatiblePrefix + "6", Name: "Bad", Prefix: "deepseek",
				BaseURL: "https://collide.test/v1",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := base.Synthesize(tc.node)
			if !tc.wantOK {
				if err == nil {
					t.Fatalf("Synthesize() = %+v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Synthesize() error = %v", err)
			}
			if got.ID != tc.wantID {
				t.Fatalf("ID = %q, want %q", got.ID, tc.wantID)
			}
			if !got.Custom {
				t.Fatal("a synthesized provider must be marked Custom")
			}
			if got.Category != "apikey" {
				t.Fatalf("Category = %q, want apikey", got.Category)
			}
			if got.AuthType != tc.wantAuth {
				t.Fatalf("AuthType = %q, want %q", got.AuthType, tc.wantAuth)
			}
			if got.Transport.Format != tc.wantFmt {
				t.Fatalf("Format = %q, want %q", got.Transport.Format, tc.wantFmt)
			}
			// A node stores a base, so the path that completes it has to come
			// from the node's kind. A wrong one sends every call to a 404.
			if got.Transport.ChatPath != tc.wantPath {
				t.Fatalf("ChatPath = %q, want %q", got.Transport.ChatPath, tc.wantPath)
			}
			if got.Transport.BaseURL != tc.node.BaseURL {
				t.Fatalf("BaseURL = %q, want %q", got.Transport.BaseURL, tc.node.BaseURL)
			}
			if got.Priority != CustomPriority {
				t.Fatalf("Priority = %d, want %d", got.Priority, CustomPriority)
			}
			if got.Alias != tc.node.Prefix {
				t.Fatalf("Alias = %q, want the prefix %q", got.Alias, tc.node.Prefix)
			}
		})
	}
}

// TestIndex_WithCustomLeavesTheReceiverUntouched pins the immutability the
// request path depends on: adding a node for one request must not change what
// another request resolves.
func TestIndex_WithCustomLeavesTheReceiverUntouched(t *testing.T) {
	base, err := loadFixture(t, `
providers:
  - id: deepseek
    category: apikey
`)
	if err != nil {
		t.Fatalf("load error = %v", err)
	}

	node := CustomNode{
		ID: OpenAICompatiblePrefix + "chat-1", Name: "My Corp", Prefix: "mycorp",
		APIType: "chat", BaseURL: "https://mycorp.test/v1",
	}
	withNode, err := base.WithCustom(node)
	if err != nil {
		t.Fatalf("WithCustom() error = %v", err)
	}

	if _, ok := withNode.Provider("mycorp"); !ok {
		t.Fatal("the extended index must resolve the added node's prefix")
	}
	if _, ok := base.Provider("mycorp"); ok {
		t.Fatal("the receiver index must not gain the node")
	}
	if base.Count() != 1 || withNode.Count() != 2 {
		t.Fatalf("counts = %d and %d, want 1 and 2", base.Count(), withNode.Count())
	}
	if withNode.Revision() != base.Revision() {
		t.Fatal("the extended index must keep the same revision")
	}

	cases := []struct {
		name  string
		nodes []CustomNode
	}{
		{
			name: "two nodes sharing a prefix are refused as a set",
			nodes: []CustomNode{
				node,
				{
					ID: OpenAICompatiblePrefix + "chat-2", Name: "Other", Prefix: "mycorp",
					APIType: "chat", BaseURL: "https://other.test/v1",
				},
			},
		},
		{
			name: "a node colliding with the base index is refused",
			nodes: []CustomNode{
				{
					ID: OpenAICompatiblePrefix + "chat-3", Name: "Clash", Prefix: "deepseek",
					APIType: "chat", BaseURL: "https://clash.test/v1",
				},
			},
		},
		{
			name: "a malformed node in the set is refused",
			nodes: []CustomNode{
				{
					ID: OpenAICompatiblePrefix + "chat-4", Name: "Bad", Prefix: "badnode",
					BaseURL: "https://bad.test/v1",
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := base.WithCustom(tc.nodes...); err == nil {
				t.Fatal("WithCustom() = nil error, want a refusal")
			}
		})
	}
}
