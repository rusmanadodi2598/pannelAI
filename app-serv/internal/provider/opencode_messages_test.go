// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_messages_test.go
// @for       The Anthropic Messages leaf of the OpenCode connector: the
// //
//
//	anthropic-version header that wire requires.
//
// @uses      testing, net/http.
// @reason    The reference writes anthropic-version on every request whose URL
//
//	ends in /messages (executors/opencode.js:484, commit 2b65c49f), and
//	the connector routes a claude-target model to that leaf without it.
//	The header is a property of the wire rather than of a provider, so it
//	is decided from the URL the connector itself built, and the test pins
//	that it is written there and nowhere else.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package provider

import (
	"net/http"
	"testing"
)

// TestOpenCode_ApplyAuthSetsAnthropicVersionOnTheMessagesLeaf pins the rule the
// Anthropic wire enforces: a request to /messages must name the API version it
// was written against, so the connector writes the reference's own constant
// (providers/shared.js:24) whenever the URL it built ends at that leaf.
func TestOpenCode_ApplyAuthSetsAnthropicVersionOnTheMessagesLeaf(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name     string
		url      string
		wantHead string
	}{
		{
			name:     "the messages leaf",
			url:      "https://opencode.ai/zen/v1/messages",
			wantHead: "2023-06-01",
		},
		{
			name:     "the messages leaf with a query string",
			url:      "https://opencode.ai/zen/v1/messages?beta=true",
			wantHead: "2023-06-01",
		},
		{
			name:     "the go lane's messages leaf",
			url:      "https://opencode.ai/zen/go/v1/messages",
			wantHead: "2023-06-01",
		},
		{
			name:     "the chat leaf carries none",
			url:      "https://opencode.ai/zen/v1/chat/completions",
			wantHead: "",
		},
		{
			name:     "the responses leaf carries none",
			url:      "https://opencode.ai/zen/v1/responses",
			wantHead: "",
		},
		{
			name:     "a path that merely ends in messages is not the leaf",
			url:      "https://opencode.ai/zen/v1/notmessages",
			wantHead: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, tc.url, nil)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			if err := connector.ApplyAuth(request, NoCredential("ep-1")); err != nil {
				t.Fatalf("ApplyAuth() error = %v", err)
			}
			if got := request.Header.Get("anthropic-version"); got != tc.wantHead {
				t.Fatalf("anthropic-version = %q, want %q", got, tc.wantHead)
			}
		})
	}

	// A version an operator declared for the entry is theirs, not ours to
	// overwrite: the registry is the place a provider's wire version is stated,
	// and a connector that replaced it would make the declaration a lie.
	t.Run("a declared version is kept", func(t *testing.T) {
		entry := opencodeEntry("https://opencode.ai", "openai")
		entry.Transport.Headers = map[string]string{"anthropic-version": "2024-01-01"}
		declared := NewOpenCode(entry)
		request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/messages", nil)
		if err != nil {
			t.Fatalf("building request: %v", err)
		}
		// The core copies the entry's headers onto the request before ApplyAuth
		// runs, so the test does the same rather than assuming the order.
		request.Header.Set("anthropic-version", "2024-01-01")
		if err := declared.ApplyAuth(request, NoCredential("ep-1")); err != nil {
			t.Fatalf("ApplyAuth() error = %v", err)
		}
		if got := request.Header.Get("anthropic-version"); got != "2024-01-01" {
			t.Fatalf("anthropic-version = %q, want the declared version kept", got)
		}
	})
}
