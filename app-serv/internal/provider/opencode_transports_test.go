// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_transports_test.go
// @for       The multi-endpoint rule: a model is served on the endpoint its own
//
//	declared formats allow, with that endpoint's credential placement.
//
// @uses      testing, net/http, internal/registry.
// @reason    The reference's opencode-go and opencode-zen entries declare three
//
//	endpoints each (`transports[]`), and a model's `supportedFormats`
//	decides which of them may answer it. Measured on the previous port:
//	minimax-m2.7 (target claude) was sent to
//	".../chat/completions/zen/v1/messages", a URL that concatenates the
//	chat endpoint with the messages path, and its credential was placed as
//	an Authorization bearer where the wire reads x-api-key. Both are
//	failures the upstream answers with a 4xx that names no cause, so the
//	rule is pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package provider

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// opencodeGoEntry builds an entry shaped like the embedded opencode-go one: a
// chat base URL plus the three-endpoint table, so a test exercises the same
// resolution the registry feeds.
func opencodeGoEntry() registry.Provider {
	return registry.Provider{
		ID:       "opencode-go",
		AuthType: registry.AuthAPIKey,
		Transport: registry.Transport{
			BaseURL: "https://opencode.ai/zen/go/v1/chat/completions",
			Format:  registry.DefaultFormat,
		},
		Transports: []registry.TransportEndpoint{
			{
				Format:  registry.DefaultFormat,
				BaseURL: "https://opencode.ai/zen/go/v1/chat/completions",
				Auth:    registry.AuthConfig{Header: "Authorization", Scheme: "bearer", Combined: true},
			},
			{
				Format:  "claude",
				BaseURL: "https://opencode.ai/zen/go/v1/messages",
				Auth: registry.AuthConfig{
					Header: "x-api-key", Scheme: "raw", Combined: true,
					APIKey: &registry.AuthScheme{Header: "x-api-key", Scheme: "raw"},
				},
			},
			{
				Format:  registry.FormatOpenAIResponses,
				BaseURL: "https://opencode.ai/zen/go/v1/responses",
				Auth:    registry.AuthConfig{Header: "Authorization", Scheme: "bearer", Combined: true},
			},
		},
		Models: []registry.Model{
			{ID: "minimax-m2.7", Name: "MiniMax M2.7", SupportedFormats: []string{"openai", "claude"}},
			{ID: "kimi-k2.6", Name: "Kimi K2.6", SupportedFormats: []string{"openai"}},
			{ID: "grok-4.6", Name: "Grok 4.6", TargetFormat: registry.FormatOpenAIResponses, SupportedFormats: []string{registry.FormatOpenAIResponses}},
		},
	}
}

// TestOpenCode_MultiEndpointURLUsesTheModelsOwnTransport pins the URL rule: a
// model that supports the wire named by its target format is served on that
// endpoint's URL, never on a path built by appending a leaf to the chat URL.
func TestOpenCode_MultiEndpointURLUsesTheModelsOwnTransport(t *testing.T) {
	connector := NewOpenCode(opencodeGoEntry())
	entry := opencodeGoEntry()

	cases := []struct {
		name    string
		model   registry.Model
		wantURL string
	}{
		{
			name:    "a claude-target model answers on the messages endpoint",
			model:   registry.Model{ID: "minimax-m2.7", TargetFormat: "claude", SupportedFormats: []string{"openai", "claude"}},
			wantURL: "https://opencode.ai/zen/go/v1/messages",
		},
		{
			name:    "a chat model answers on the chat endpoint",
			model:   registry.Model{ID: "kimi-k2.6", SupportedFormats: []string{"openai"}},
			wantURL: "https://opencode.ai/zen/go/v1/chat/completions",
		},
		{
			name:    "a responses-only model answers on the responses endpoint",
			model:   registry.Model{ID: "grok-4.6", TargetFormat: registry.FormatOpenAIResponses, SupportedFormats: []string{registry.FormatOpenAIResponses}},
			wantURL: "https://opencode.ai/zen/go/v1/responses",
		},
		{
			name:    "a model with no target format falls back to the entry's own format",
			model:   registry.Model{ID: "an-undeclared-model"},
			wantURL: "https://opencode.ai/zen/go/v1/chat/completions",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := connector.Endpoint(Request{Provider: entry, Model: tc.model}, StaticKey("ep-1", "k1", "sk-go"))
			if err != nil {
				t.Fatalf("Endpoint() error = %v", err)
			}
			if got != tc.wantURL {
				t.Fatalf("Endpoint() = %q, want %q", got, tc.wantURL)
			}
		})
	}
}

// TestOpenCode_MultiEndpointAuthFollowsTheEndpoint pins the second half of the
// rule: the credential is placed the way the chosen endpoint declares it. The
// Messages endpoint reads `x-api-key` raw, so sending a bearer there would
// authenticate incorrectly rather than fail loudly.
func TestOpenCode_MultiEndpointAuthFollowsTheEndpoint(t *testing.T) {
	connector := NewOpenCode(opencodeGoEntry())
	entry := opencodeGoEntry()
	cred := StaticKey("ep-1", "k1", "sk-go")

	cases := []struct {
		name        string
		model       registry.Model
		wantHeader  string
		wantValue   string
		absentOther string
	}{
		{
			name:        "the messages endpoint takes a raw x-api-key",
			model:       registry.Model{ID: "minimax-m2.7", TargetFormat: "claude", SupportedFormats: []string{"openai", "claude"}},
			wantHeader:  "x-api-key",
			wantValue:   "sk-go",
			absentOther: "Authorization",
		},
		{
			name:        "the chat endpoint takes a bearer",
			model:       registry.Model{ID: "kimi-k2.6", SupportedFormats: []string{"openai"}},
			wantHeader:  "Authorization",
			wantValue:   "Bearer sk-go",
			absentOther: "x-api-key",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := connector.Endpoint(Request{Provider: entry, Model: tc.model}, cred)
			if err != nil {
				t.Fatalf("Endpoint() error = %v", err)
			}
			request, err := http.NewRequest(http.MethodPost, url, nil)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			if err := connector.ApplyAuth(request, cred); err != nil {
				t.Fatalf("ApplyAuth() error = %v", err)
			}
			if got := request.Header.Get(tc.wantHeader); got != tc.wantValue {
				t.Fatalf("%s = %q, want %q", tc.wantHeader, got, tc.wantValue)
			}
			if got := request.Header.Get(tc.absentOther); got != "" {
				t.Fatalf("%s = %q, want the other header untouched", tc.absentOther, got)
			}
		})
	}
}

// TestOpenCode_MultiEndpointRefusesAnUnsupportedWire pins the guard: a model
// that declares `supportedFormats` and does not include the wire its target
// format names must be refused rather than sent to an endpoint that will not
// answer it. The reference applies the same guard in chatCore.js:89-101, where a
// model without the source format falls back to translation instead of the
// endpoint.
func TestOpenCode_MultiEndpointRefusesAnUnsupportedWire(t *testing.T) {
	connector := NewOpenCode(opencodeGoEntry())
	entry := opencodeGoEntry()
	// kimi declares only `openai`, so a claude target has no endpoint that
	// serves it on this provider.
	model := registry.Model{ID: "kimi-k2.6", TargetFormat: "claude", SupportedFormats: []string{"openai"}}

	got, err := connector.Endpoint(Request{Provider: entry, Model: model}, StaticKey("ep-1", "k1", "sk-go"))
	if err == nil {
		t.Fatalf("Endpoint() = %q, want a refusal for a wire the model does not support", got)
	}
}
