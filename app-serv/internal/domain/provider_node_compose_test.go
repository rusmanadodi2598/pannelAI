// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/provider_node_compose_test.go
// @for       The base-URL rules on the mutation path, and the composition that
//
//	makes the rule matter: base + appended path must be one path.
//
// @uses      testing, time.
// @reason    The create path and the mutation path must agree, and the property
//
//	that decides whether they do is the composed URL rather than the
//	stored string. Split from the create-path table to keep both files
//	under AGENTS.md §1.1.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-23
package domain

import (
	"testing"
)

// TestProviderNode_RebaseStripsTheAppendedSuffix pins the same rule on the
// mutation path, because the defect existed precisely because the reference
// sanitized create and update in two separate places.
func TestProviderNode_RebaseStripsTheAppendedSuffix(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"full anthropic endpoint", "https://api.anthropic.com/v1/messages", "https://api.anthropic.com/v1"},
		{"trailing slash", "https://api.anthropic.com/v1/", "https://api.anthropic.com/v1"},
		{"both", "https://api.anthropic.com/v1/messages/", "https://api.anthropic.com/v1"},
		{"already correct", "https://api.anthropic.com/v1", "https://api.anthropic.com/v1"},
		{"different host, same rule", "https://claude.corp.test/v1/messages", "https://claude.corp.test/v1"},
		{"a suffix that is not the node's own is kept", "https://api.anthropic.com/v1/messages-extra", "https://api.anthropic.com/v1/messages-extra"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := newNode(t, "Corp", "corp", NodeAnthropicCompatible, "", "https://api.anthropic.com/v1")
			if err := node.Rebase(tc.input, nodeNow); err != nil {
				t.Fatalf("Rebase(%q) error = %v", tc.input, err)
			}
			if got := node.BaseURL(); got != tc.want {
				t.Fatalf("BaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestProviderNode_StrippedURLStillComposesOnePath is the property that matters:
// after the rule, appending the node's own path must produce the endpoint once,
// not twice. It is stated as a composition rather than a string compare so the
// test fails for the right reason if either half regresses.
func TestProviderNode_StrippedURLStillComposesOnePath(t *testing.T) {
	cases := []struct {
		name      string
		nodeType  NodeType
		apiType   string
		input     string
		wantFinal string
	}{
		{
			name:      "anthropic",
			nodeType:  NodeAnthropicCompatible,
			input:     "https://api.anthropic.com/v1/messages",
			wantFinal: "https://api.anthropic.com/v1/messages",
		},
		{
			name:      "anthropic with slash",
			nodeType:  NodeAnthropicCompatible,
			input:     "https://api.anthropic.com/v1/messages/",
			wantFinal: "https://api.anthropic.com/v1/messages",
		},
		{
			name:      "openai chat",
			nodeType:  NodeOpenAICompatible,
			apiType:   NodeAPIChat,
			input:     "https://llm.corp.test/v1/",
			wantFinal: "https://llm.corp.test/v1/chat/completions",
		},
		{
			name:      "openai responses",
			nodeType:  NodeOpenAICompatible,
			apiType:   NodeAPIResponses,
			input:     "https://resp.corp.test/v1",
			wantFinal: "https://resp.corp.test/v1/responses",
		},
		{
			name:      "openai chat already correct",
			nodeType:  NodeOpenAICompatible,
			apiType:   NodeAPIChat,
			input:     "https://llm.corp.test/v1",
			wantFinal: "https://llm.corp.test/v1/chat/completions",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := newNode(t, "Corp", "corp", tc.nodeType, tc.apiType, tc.input)
			if got := node.BaseURL() + nodePath(node); got != tc.wantFinal {
				t.Fatalf("composed = %q, want %q", got, tc.wantFinal)
			}
		})
	}
}

// nodePath mirrors the path the registry's synthesized transport appends. It is
// declared here rather than imported because the domain must not depend on the
// registry package; the registry side is pinned by its own test.
func nodePath(n ProviderNode) string {
	if n.Type() == NodeAnthropicCompatible {
		return "/messages"
	}
	if n.APIType() == NodeAPIResponses {
		return "/responses"
	}
	return "/chat/completions"
}
