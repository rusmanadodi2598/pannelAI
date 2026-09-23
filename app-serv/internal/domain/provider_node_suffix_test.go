// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/provider_node_suffix_test.go
// @for       The base-URL suffix rule: a stored node base URL must not already
// //
//
//	carry the path the transport appends (SPEC-API-001 §7.4).
//
// @uses      testing, time.
// @reason    The reference sanitizes a pasted endpoint URL on both write paths
//
//	(create and update) because the runtime appends the path itself, and
//	a doubled path is a 404 the operator reads as a credential or model
//	problem. The rule lives in the domain rather than in either handler
//	so the two paths cannot drift apart, which is the shape the defect
//	had. Ported from the reference at v0.5.85.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-23
package domain

import (
	"testing"
)

// TestProviderNode_StripsTheAppendedSuffix pins the rule that a stored base URL
// carries no trailing slash and no copy of the path the transport appends.
//
// The table covers the shapes an operator actually pastes (a full endpoint URL
// from a vendor's documentation, one with a trailing slash, one already
// correct) plus a benign case that must stay untouched, so the fix cannot pass
// by mangling every URL into the same shape.
func TestProviderNode_StripsTheAppendedSuffix(t *testing.T) {
	cases := []struct {
		name      string
		nodeType  NodeType
		apiType   string
		input     string
		want      string
		wantError bool
	}{
		{
			name:     "anthropic base that already ends in /messages",
			nodeType: NodeAnthropicCompatible,
			input:    "https://api.anthropic.com/v1/messages",
			want:     "https://api.anthropic.com/v1",
		},
		{
			name:     "anthropic base with a trailing slash after /messages",
			nodeType: NodeAnthropicCompatible,
			input:    "https://api.anthropic.com/v1/messages/",
			want:     "https://api.anthropic.com/v1",
		},
		{
			name:     "anthropic base with only a trailing slash",
			nodeType: NodeAnthropicCompatible,
			input:    "https://api.anthropic.com/v1/",
			want:     "https://api.anthropic.com/v1",
		},
		{
			name:     "anthropic base already correct stays untouched",
			nodeType: NodeAnthropicCompatible,
			input:    "https://api.anthropic.com/v1",
			want:     "https://api.anthropic.com/v1",
		},
		{
			name:     "openai base keeps a path that is not its own endpoint",
			nodeType: NodeOpenAICompatible,
			apiType:  NodeAPIChat,
			input:    "https://llm.corp.test/v1",
			want:     "https://llm.corp.test/v1",
		},
		{
			name:     "openai base with a trailing slash loses only the slash",
			nodeType: NodeOpenAICompatible,
			apiType:  NodeAPIChat,
			input:    "https://llm.corp.test/v1/",
			want:     "https://llm.corp.test/v1",
		},
		{
			name:     "openai base must not lose an unrelated /messages segment",
			nodeType: NodeOpenAICompatible,
			apiType:  NodeAPIChat,
			input:    "https://llm.corp.test/messages",
			want:     "https://llm.corp.test/messages",
		},
		{
			name:     "responses base keeps its own path untouched",
			nodeType: NodeOpenAICompatible,
			apiType:  NodeAPIResponses,
			input:    "https://resp.corp.test/v1",
			want:     "https://resp.corp.test/v1",
		},
		{
			name:      "a non-http scheme is still refused",
			nodeType:  NodeAnthropicCompatible,
			input:     "ftp://api.anthropic.com/v1/messages",
			wantError: true,
		},
		{
			name:      "a scheme-less value is still refused",
			nodeType:  NodeOpenAICompatible,
			apiType:   NodeAPIChat,
			input:     "api.anthropic.com/v1",
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := NewProviderNode("", "Corp", "corp", tc.nodeType, tc.apiType, tc.input, nodeNow)
			if tc.wantError {
				if err == nil {
					t.Fatalf("NewProviderNode(%q) error = nil, want a refusal", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewProviderNode(%q) error = %v", tc.input, err)
			}
			if got := node.BaseURL(); got != tc.want {
				t.Fatalf("BaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
